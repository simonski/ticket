package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var projectPrefixPattern = regexp.MustCompile(`^[A-Z]{1,5}$`)

func normalizeProjectPrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ToUpper(prefix))
	if len(prefix) > 5 {
		prefix = prefix[:5]
	}
	return prefix
}

func validateProjectPrefix(prefix string) error {
	if !projectPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("project prefix %q must be 1 to 5 uppercase letters", prefix)
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

func nextUniqueProjectPrefix(ctx context.Context, db *sql.DB, desired string) (string, error) {
	desired = normalizeProjectPrefix(desired)
	if err := validateProjectPrefix(desired); err != nil {
		return "", err
	}
	for i := 0; i < 16384; i++ {
		candidate := desired
		if i > 0 {
			suffix := alphabeticProjectPrefixSuffix(i)
			base := desired
			if len(base)+len(suffix) > 5 {
				base = base[:5-len(suffix)]
			}
			candidate = base + suffix
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE prefix = ?`, candidate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not allocate unique prefix for %q", desired)
}

func alphabeticProjectPrefixSuffix(index int) string {
	if index <= 0 {
		return ""
	}
	buf := make([]byte, 0, 4)
	for index > 0 {
		index--
		buf = append([]byte{byte('A' + (index % 26))}, buf...)
		index /= 26
	}
	return string(buf)
}

func ticketTypeCode(ticketType string) (string, error) {
	switch normalizeTicketType(ticketType) {
	case "epic":
		return "E", nil
	case "story":
		return "Y", nil
	case "task":
		return "T", nil
	case "bug":
		return "B", nil
	case "feature":
		return "F", nil
	case "idea":
		return "I", nil
	case "spike":
		return "S", nil
	case "chore":
		return "C", nil
	case "note":
		return "N", nil
	case "question":
		return "Q", nil
	case "requirement":
		return "R", nil
	case "decision":
		return "D", nil
	case "action":
		return "A", nil
	default:
		return "", fmt.Errorf("invalid ticket type %q", ticketType)
	}
}

func generateTicketKey(prefix, ticketType string, sequence int64) (string, error) {
	// Validate the ticket type even though we no longer embed it in the key.
	if _, err := ticketTypeCode(ticketType); err != nil {
		return "", err
	}
	prefix = normalizeProjectPrefix(prefix)
	if err := validateProjectPrefix(prefix); err != nil {
		return "", err
	}
	if sequence <= 0 {
		return "", fmt.Errorf("ticket sequence must be positive")
	}
	// All projects now use PREFIX-N format (no type code embedded).
	return fmt.Sprintf("%s-%d", prefix, sequence), nil
}
