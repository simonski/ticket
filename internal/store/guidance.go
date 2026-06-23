package store

import (
	"strings"
)

const DefaultGuidanceStageKey = "default"

type GuidanceMap map[string]string

type ResolvedGuidance struct {
	DOR    string `json:"dor,omitempty"`
	DOD    string `json:"dod,omitempty"`
	AC     string `json:"ac,omitempty"`
	HasDOR bool   `json:"-"`
	HasDOD bool   `json:"-"`
	HasAC  bool   `json:"-"`
}

func normalizeGuidanceStageKey(stage string) string {
	return strings.TrimSpace(strings.ToLower(stage))
}

func normalizeGuidanceMap(m GuidanceMap) GuidanceMap {
	if len(m) == 0 {
		return nil
	}
	out := make(GuidanceMap, len(m))
	for key, value := range m {
		stage := normalizeGuidanceStageKey(key)
		if stage == "" {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[stage] = trimmed
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func withLegacyAcceptanceCriteria(acceptanceCriteria string, acMap GuidanceMap) GuidanceMap {
	normalized := normalizeGuidanceMap(acMap)
	legacy := strings.TrimSpace(acceptanceCriteria)
	if legacy == "" {
		return normalized
	}
	if normalized == nil {
		return GuidanceMap{DefaultGuidanceStageKey: legacy}
	}
	if _, ok := normalized[DefaultGuidanceStageKey]; !ok {
		normalized[DefaultGuidanceStageKey] = legacy
	}
	return normalized
}

func (m GuidanceMap) Resolve(stage string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	normalizedStage := normalizeGuidanceStageKey(stage)
	if normalizedStage != "" {
		for key, value := range m {
			if normalizeGuidanceStageKey(key) != normalizedStage {
				continue
			}
			return value, true
		}
	}
	for key, value := range m {
		if normalizeGuidanceStageKey(key) != DefaultGuidanceStageKey {
			continue
		}
		return value, true
	}
	return "", false
}

func resolveGuidance(stage string, dorMap, dodMap, acMap GuidanceMap) ResolvedGuidance {
	resolved := ResolvedGuidance{}
	if value, ok := dorMap.Resolve(stage); ok {
		resolved.DOR = value
		resolved.HasDOR = true
	}
	if value, ok := dodMap.Resolve(stage); ok {
		resolved.DOD = value
		resolved.HasDOD = true
	}
	if value, ok := acMap.Resolve(stage); ok {
		resolved.AC = value
		resolved.HasAC = true
	}
	return resolved
}
