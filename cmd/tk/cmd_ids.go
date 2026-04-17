package main

func optionalInt64Flag(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}
