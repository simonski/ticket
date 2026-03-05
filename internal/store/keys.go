package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var projectPrefixPattern = regexp.MustCompile(`^[A-Z]{2,5}$`)

func normalizeProjectPrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ToUpper(prefix))
	if len(prefix) > 5 {
		prefix = prefix[:5]
	}
	return prefix
}

func validateProjectPrefix(prefix string) error {
	if !projectPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("project prefix %q must be 2 to 5 uppercase letters", prefix)
	}
	return nil
}

func deriveProjectPrefix(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "PRJ"
	}
	parts := strings.FieldsFunc(title, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		r := []rune(part)[0]
		if unicode.IsLetter(r) {
			out.WriteRune(unicode.ToUpper(r))
		}
		if out.Len() == 5 {
			break
		}
	}
	if out.Len() < 3 {
		for _, r := range title {
			if !unicode.IsLetter(r) {
				continue
			}
			out.WriteRune(unicode.ToUpper(r))
			if out.Len() == 5 {
				break
			}
		}
	}
	if out.Len() == 0 {
		return "PRJ"
	}
	value := out.String()
	if len(value) < 3 {
		value += strings.Repeat("X", 3-len(value))
	}
	if len(value) > 5 {
		value = value[:5]
	}
	return value
}

func nextUniqueProjectPrefix(db *sql.DB, desired string) (string, error) {
	desired = normalizeProjectPrefix(desired)
	if err := validateProjectPrefix(desired); err != nil {
		return "", err
	}
	for i := 0; i < 1000; i++ {
		candidate := desired
		if i > 0 {
			suffix := strconv.Itoa(i)
			base := desired
			if len(base)+len(suffix) > 5 {
				base = base[:5-len(suffix)]
			}
			candidate = base + suffix
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM projects WHERE prefix = ?`, candidate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique prefix for %q", desired)
}

func ticketTypeCode(ticketType string) (string, error) {
	switch normalizeTicketType(ticketType) {
	case "epic":
		return "E", nil
	case "task":
		return "T", nil
	case "bug":
		return "B", nil
	case "spike":
		return "S", nil
	case "chore":
		return "C", nil
	default:
		return "", fmt.Errorf("invalid task type %q", ticketType)
	}
}

func generateTicketKey(prefix, ticketType string, sequence int64) (string, error) {
	code, err := ticketTypeCode(ticketType)
	if err != nil {
		return "", err
	}
	prefix = normalizeProjectPrefix(prefix)
	if err := validateProjectPrefix(prefix); err != nil {
		return "", err
	}
	if sequence <= 0 {
		return "", fmt.Errorf("ticket sequence must be positive")
	}
	if prefix == defaultProjectPrefix {
		return fmt.Sprintf("%s-%d", prefix, sequence), nil
	}
	return fmt.Sprintf("%s-%s-%d", prefix, code, sequence), nil
}
