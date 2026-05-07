package store

import "fmt"

func normalizeExplicitID(id *int64) (explicitID int64, hasExplicitID bool, err error) {
	if id == nil {
		return 0, false, nil
	}
	if *id <= 0 {
		return 0, false, fmt.Errorf("id must be greater than zero")
	}
	return *id, true, nil
}
