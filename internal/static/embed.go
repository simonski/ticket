// Package static embeds and parses the built-in role and SDLC seed files
// shipped with the tk binary. These are used by `tk init` to populate a
// new database with sensible defaults.
package static

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

//go:embed roles/*.md
var rolesFS embed.FS

//go:embed sdlc/*.md
var sdlcFS embed.FS

// Role represents a parsed role seed file.
type Role struct {
	Filename           string
	Title              string
	Description        string
	AcceptanceCriteria string
}

// SdlcStageRole is a role reference within an SDLC stage.
type SdlcStageRole struct {
	RoleRef string // filename without .md, e.g. "engineer"
	Order   int
}

// SdlcStage represents a parsed stage within an SDLC seed file.
type SdlcStage struct {
	Name        string
	Description string
	Order       int
	Roles       []SdlcStageRole
}

// Sdlc represents a parsed SDLC seed file.
type Sdlc struct {
	Filename    string
	Name        string
	Description string
	Default     bool
	Stages      []SdlcStage
}

// LoadRoles reads and parses all role seed files from the embedded filesystem.
func LoadRoles() ([]Role, error) {
	entries, err := rolesFS.ReadDir("roles")
	if err != nil {
		return nil, err
	}
	var roles []Role
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		data, err := rolesFS.ReadFile("roles/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		role := parseRole(e.Name(), string(data))
		roles = append(roles, role)
	}
	return roles, nil
}

// SeedDatabase populates a database with all built-in roles and SDLCs from the
// embedded static files. It assigns roles to SDLC stages based on @role
// references. This is intended to be passed to store.Init as a SeedFunc.
func SeedDatabase(ctx context.Context, db *sql.DB) error {
	roles, err := LoadRoles()
	if err != nil {
		return err
	}
	roleIDByRef := make(map[string]int64)
	for _, r := range roles {
		created, createErr := store.CreateRole(ctx, db, nil, r.Title, r.Description, r.AcceptanceCriteria)
		if createErr != nil {
			continue
		}
		roleIDByRef[r.Filename] = created.ID
		roleIDByRef[strings.ToLower(r.Title)] = created.ID
	}
	sdlcs, err := LoadSdlcs()
	if err != nil {
		return err
	}
	for _, seed := range sdlcs {
		wf, createErr := store.CreateSdlc(ctx, db, seed.Name, seed.Description)
		if createErr != nil {
			continue
		}
		for _, s := range seed.Stages {
			stage, stageErr := store.AddSdlcStage(ctx, db, wf.ID, s.Name, s.Description, "", s.Order)
			if stageErr != nil {
				continue
			}
			for _, roleRef := range s.Roles {
				if rid, ok := roleIDByRef[roleRef.RoleRef]; ok {
					_ = store.AddSdlcStageRole(ctx, db, wf.ID, stage.ID, rid)
				}
			}
		}
	}
	return nil
}

// DefaultSdlcID returns the database ID of the SDLC marked default: true in
// the static seed files, by looking up its name in the given database.
func DefaultSdlcID(ctx context.Context, db *sql.DB) (int64, error) {
	sdlcs, err := LoadSdlcs()
	if err != nil {
		return 0, err
	}
	for _, s := range sdlcs {
		if s.Default {
			var id int64
			if err := db.QueryRowContext(ctx, `SELECT sdlc_id FROM sdlcs WHERE name = ?`, s.Name).Scan(&id); err == nil {
				return id, nil
			}
		}
	}
	// Fallback: return the first SDLC.
	var id int64
	err = db.QueryRowContext(ctx, `SELECT sdlc_id FROM sdlcs ORDER BY sdlc_id LIMIT 1`).Scan(&id)
	return id, err
}

// LoadSdlcs reads and parses all SDLC seed files from the embedded filesystem.
func LoadSdlcs() ([]Sdlc, error) {
	entries, err := sdlcFS.ReadDir("sdlc")
	if err != nil {
		return nil, err
	}
	var sdlcs []Sdlc
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := sdlcFS.ReadFile("sdlc/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sdlc := parseSdlc(e.Name(), string(data))
		sdlcs = append(sdlcs, sdlc)
	}
	return sdlcs, nil
}

// parseRole extracts frontmatter fields from a role markdown file.
func parseRole(filename, content string) Role {
	fm := parseFrontmatter(content)
	return Role{
		Filename:           strings.TrimSuffix(filename, ".md"),
		Title:              fm["title"],
		Description:        fm["description"],
		AcceptanceCriteria: fm["acceptance_criteria"],
	}
}

var stageHeadingRe = regexp.MustCompile(`(?m)^###\s+(\S+)\s*$`)
var orderRe = regexp.MustCompile(`(?m)^order:\s*(\d+)\s*$`)
var roleRefRe = regexp.MustCompile(`(?m)^\d+\.\s+@(\S+)`)

// parseSdlc extracts the name, description, and stages from an SDLC markdown file.
func parseSdlc(filename, content string) Sdlc {
	fm := parseFrontmatter(content)
	sdlc := Sdlc{
		Filename:    strings.TrimSuffix(filename, ".md"),
		Name:        fm["name"],
		Description: fm["description"],
		Default:     fm["default"] == "true",
	}

	// Split into stage sections by ### headings.
	headings := stageHeadingRe.FindAllStringSubmatchIndex(content, -1)
	for i, loc := range headings {
		name := content[loc[2]:loc[3]]
		// Section body runs from after the heading to the next heading (or end).
		bodyStart := loc[1]
		bodyEnd := len(content)
		if i+1 < len(headings) {
			bodyEnd = headings[i+1][0]
		}
		body := content[bodyStart:bodyEnd]

		stage := SdlcStage{Name: name}

		// Extract order.
		if m := orderRe.FindStringSubmatch(body); m != nil {
			stage.Order, _ = strconv.Atoi(m[1])
		}

		// Extract first non-heading, non-order text line as description.
		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "order:") ||
				strings.HasPrefix(trimmed, "Previous:") || strings.HasPrefix(trimmed, "Next:") ||
				strings.HasPrefix(trimmed, "Roles:") || strings.HasPrefix(trimmed, "Entry ") ||
				strings.HasPrefix(trimmed, "Exit ") || strings.HasPrefix(trimmed, "Acceptance ") ||
				strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "#") ||
				regexp.MustCompile(`^\d+\.`).MatchString(trimmed) {
				continue
			}
			stage.Description = trimmed
			break
		}

		// Extract role references.
		roleMatches := roleRefRe.FindAllStringSubmatch(body, -1)
		for j, rm := range roleMatches {
			stage.Roles = append(stage.Roles, SdlcStageRole{
				RoleRef: rm[1],
				Order:   j,
			})
		}

		sdlc.Stages = append(sdlc.Stages, stage)
	}

	// Sort stages by order.
	sort.Slice(sdlc.Stages, func(i, j int) bool {
		return sdlc.Stages[i].Order < sdlc.Stages[j].Order
	})

	return sdlc
}

// parseFrontmatter extracts key: value pairs from YAML-style frontmatter
// delimited by --- lines.
func parseFrontmatter(content string) map[string]string {
	fm := make(map[string]string)
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return fm
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		idx := strings.Index(trimmed, ":")
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		value := strings.TrimSpace(trimmed[idx+1:])
		fm[key] = value
	}
	return fm
}
