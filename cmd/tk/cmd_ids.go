package main

import "fmt"

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
