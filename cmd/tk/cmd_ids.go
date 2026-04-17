package main

import (
	"fmt"
	"strconv"
)

func optionalInt64Flag(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func printCreatedID(id any, enabled bool) bool {
	if !enabled {
		return false
	}
	fmt.Println(id)
	return true
}

func parseExpectedCount(flagName, raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be numeric: %w", flagName, err)
	}
	return value, nil
}
