package ticketmarkdown

import (
	"fmt"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

const Header = "<!-- tk:ticket-markdown/v1 -->"

var requiredSections = []string{"title", "description", "acceptance criteria"}

type Document struct {
	ID                 string
	Type               string
	Title              string
	Description        string
	AcceptanceCriteria string
}

func LooksLikeDocument(content string) bool {
	lines := strings.Split(normalizeLineEndings(content), "\n")
	for _, raw := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF"))
		if trimmed == "" {
			continue
		}
		return trimmed == Header
	}
	return false
}

func Render(ticket store.Ticket) string {
	var b strings.Builder
	b.WriteString(Header)
	b.WriteString("\n")
	b.WriteString("id: ")
	b.WriteString(strings.TrimSpace(ticket.ID))
	b.WriteString("\n")
	b.WriteString("type: ")
	b.WriteString(strings.TrimSpace(ticket.Type))
	b.WriteString("\n\n")
	writeSection(&b, "title", ticket.Title, "text")
	b.WriteString("\n\n")
	writeSection(&b, "description", ticket.Description, "markdown")
	b.WriteString("\n\n")
	writeSection(&b, "acceptance criteria", ticket.AcceptanceCriteria, "markdown")
	b.WriteString("\n")
	return b.String()
}

func Parse(content string) (Document, error) {
	lines := strings.Split(normalizeLineEndings(content), "\n")
	headerIndex := -1
	for i, raw := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF"))
		if trimmed == "" {
			continue
		}
		if trimmed != Header {
			return Document{}, fmt.Errorf("ticket markdown must start with %q", Header)
		}
		headerIndex = i
		break
	}
	if headerIndex < 0 {
		return Document{}, fmt.Errorf("ticket markdown must start with %q", Header)
	}

	i := headerIndex + 1
	doc := Document{}
	metadataSeen := map[string]bool{}
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			break
		}
		idx := strings.Index(trimmed, ":")
		if idx < 1 {
			return Document{}, fmt.Errorf("line %d: expected metadata in key: value form", i+1)
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
		value := strings.TrimSpace(trimmed[idx+1:])
		if value == "" {
			return Document{}, fmt.Errorf("line %d: %s requires a value", i+1, key)
		}
		if metadataSeen[key] {
			return Document{}, fmt.Errorf("line %d: duplicate metadata field %q", i+1, key)
		}
		metadataSeen[key] = true
		switch key {
		case "id":
			doc.ID = value
		case "type":
			doc.Type = value
		default:
			return Document{}, fmt.Errorf("line %d: unsupported metadata field %q", i+1, key)
		}
		i++
	}
	if strings.TrimSpace(doc.ID) == "" {
		return Document{}, fmt.Errorf("ticket markdown is missing required metadata field %q", "id")
	}
	if strings.TrimSpace(doc.Type) == "" {
		return Document{}, fmt.Errorf("ticket markdown is missing required metadata field %q", "type")
	}

	sectionsSeen := map[string]bool{}
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		headingLevel := countHeadingPrefix(trimmed)
		if headingLevel != 2 {
			return Document{}, fmt.Errorf("line %d: expected section heading starting with %q", i+1, "##")
		}
		sectionName := strings.ToLower(strings.TrimSpace(trimmed[headingLevel:]))
		if !isSupportedSection(sectionName) {
			return Document{}, fmt.Errorf("line %d: unsupported section %q", i+1, sectionName)
		}
		if sectionsSeen[sectionName] {
			return Document{}, fmt.Errorf("line %d: duplicate section %q", i+1, sectionName)
		}
		sectionsSeen[sectionName] = true
		i++
		if i >= len(lines) {
			return Document{}, fmt.Errorf("line %d: section %q is missing a fenced body", i, sectionName)
		}
		fenceCh, fenceN, _, ok := parseFenceMarker(strings.TrimSpace(lines[i]))
		if !ok {
			return Document{}, fmt.Errorf("line %d: section %q must begin with a fenced block", i+1, sectionName)
		}
		i++
		var block []string
		closed := false
		for i < len(lines) {
			trimmed = strings.TrimSpace(lines[i])
			if ch, n, rest, ok := parseFenceMarker(trimmed); ok && ch == fenceCh && n >= fenceN && rest == "" {
				closed = true
				i++
				break
			}
			block = append(block, lines[i])
			i++
		}
		if !closed {
			return Document{}, fmt.Errorf("section %q is missing a closing fence", sectionName)
		}
		value := strings.Join(block, "\n")
		switch sectionName {
		case "title":
			doc.Title = value
		case "description":
			doc.Description = value
		case "acceptance criteria":
			doc.AcceptanceCriteria = value
		}
	}

	for _, sectionName := range requiredSections {
		if !sectionsSeen[sectionName] {
			return Document{}, fmt.Errorf("ticket markdown is missing required section %q", sectionName)
		}
	}
	if strings.TrimSpace(doc.Title) == "" {
		return Document{}, fmt.Errorf("ticket markdown title section must not be empty")
	}
	return doc, nil
}

func writeSection(b *strings.Builder, name, value, info string) {
	fence := fenceFor(value)
	b.WriteString("## ")
	b.WriteString(name)
	b.WriteString("\n")
	b.WriteString(fence)
	if strings.TrimSpace(info) != "" {
		b.WriteString(info)
	}
	b.WriteString("\n")
	b.WriteString(normalizeLineEndings(value))
	if value != "" && !strings.HasSuffix(value, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence)
}

func fenceFor(value string) string {
	maxRun := 0
	run := 0
	for _, r := range value {
		if r == '`' {
			run++
			if run > maxRun {
				maxRun = run
			}
			continue
		}
		run = 0
	}
	if maxRun < 2 {
		maxRun = 2
	}
	return strings.Repeat("`", maxRun+1)
}

func isSupportedSection(name string) bool {
	for _, candidate := range requiredSections {
		if name == candidate {
			return true
		}
	}
	return false
}

func countHeadingPrefix(value string) int {
	count := 0
	for count < len(value) && value[count] == '#' {
		count++
	}
	return count
}

func normalizeLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

func parseFenceMarker(trimmed string) (ch byte, n int, rest string, ok bool) {
	if trimmed == "" {
		return 0, 0, "", false
	}
	ch = trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, "", false
	}
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	return ch, n, strings.TrimSpace(trimmed[n:]), true
}
