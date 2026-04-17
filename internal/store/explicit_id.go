package store

import "fmt"

func normalizeExplicitID(id *int64) (int64, bool, error) {
	if id == nil {
		return 0, false, nil
	}
	if *id <= 0 {
		return 0, false, fmt.Errorf("id must be greater than zero")
	}
	return *id, true, nil
}
