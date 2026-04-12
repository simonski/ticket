package store

import "fmt"

const (
	DefaultListLimit    = 100
	DefaultHistoryLimit = 50
)

func normalizePage(limit, offset, defaultLimit int) (int, int, error) {
	if limit < 0 {
		return 0, 0, fmt.Errorf("limit must be zero or greater")
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be zero or greater")
	}
	if limit == 0 {
		limit = defaultLimit
	}
	return limit, offset, nil
}
