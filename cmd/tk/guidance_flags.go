package main

import (
	"fmt"
	"strings"

	"github.com/simonski/ticket/internal/store"
)

func parseGuidanceMapFlag(raw string) (store.GuidanceMap, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := make(store.GuidanceMap)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid guidance map entry %q; want stage=value", part)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid guidance map entry %q; want stage=value", part)
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func mergeGuidanceMap(current store.GuidanceMap, defaultValue, mapValue string, setDefault, setMap bool) (store.GuidanceMap, error) {
	if !setDefault && !setMap {
		return nil, nil
	}
	merged := make(store.GuidanceMap, len(current)+1)
	for key, value := range current {
		merged[key] = value
	}
	if setMap {
		parsed, err := parseGuidanceMapFlag(mapValue)
		if err != nil {
			return nil, err
		}
		for key, value := range parsed {
			merged[key] = value
		}
	}
	if setDefault {
		defaultValue = strings.TrimSpace(defaultValue)
		if defaultValue == "" {
			delete(merged, store.DefaultGuidanceStageKey)
		} else {
			merged[store.DefaultGuidanceStageKey] = defaultValue
		}
	}
	if len(merged) == 0 {
		return store.GuidanceMap{}, nil
	}
	return merged, nil
}
