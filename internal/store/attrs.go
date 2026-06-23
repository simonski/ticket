package store

import (
	"encoding/json"
	"strings"
)

// Attrs is the extensible attribute bag stored in the `attrs` column on the
// high-churn entities (tickets, projects, roles, workflow_stages). It is the
// default home for new optional, sparse, display-only, and per-type fields: such a
// field can be added in Go without a schema migration or schema-version bump. See
// docs/design/extensible-schema.md and docs/adr/0001-json-attribute-bags.md.
//
// The bag is stored as TEXT JSON (not binary JSONB) so it survives the generic
// snapshot export/import used by backups and the migration rebuild path. It is
// queryable via json_extract; fields that must be filtered/sorted are "promoted"
// with an expression index rather than a new column.
type Attrs map[string]any

// marshalAttrs serializes an Attrs bag to compact JSON text for the TEXT attrs
// column. A nil or empty bag yields "{}" so the column is never NULL when written.
func marshalAttrs(a Attrs) (string, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseAttrs parses attrs JSON text (as returned by json(attrs)) into an Attrs
// bag. Empty, whitespace-only, or SQL-NULL text yields an empty (non-nil) bag so
// callers can always read and mutate the result safely.
func parseAttrs(text string) (Attrs, error) {
	t := strings.TrimSpace(text)
	if t == "" || t == "null" {
		return Attrs{}, nil
	}
	var a Attrs
	if err := json.Unmarshal([]byte(t), &a); err != nil {
		return nil, err
	}
	if a == nil {
		a = Attrs{}
	}
	return a, nil
}

// guidanceMapFromAttr converts a decoded attrs value (a nested JSON object) into a
// GuidanceMap. Used when a guidance map (dor/dod/ac) is folded into the bag as a
// nested object (TK-113/TK-115). Returns nil for a missing or non-object value.
func guidanceMapFromAttr(v any) GuidanceMap {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(GuidanceMap, len(m))
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GetString returns the value at key as a string, or "" if absent or not a string.
func (a Attrs) GetString(key string) string {
	if v, ok := a[key].(string); ok {
		return v
	}
	return ""
}

// GetInt returns the value at key as an int. JSON numbers decode to float64, which
// is handled here. Returns 0 if absent or not numeric.
func (a Attrs) GetInt(key string) int {
	switch v := a[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

// SetString sets key to a non-empty string value, deleting the key when val is
// empty so the stored bag stays sparse. It is a no-op on a nil map.
func (a Attrs) SetString(key, val string) {
	if a == nil {
		return
	}
	if val == "" {
		delete(a, key)
		return
	}
	a[key] = val
}

// SetInt sets key to a non-zero int value, deleting the key when val is zero so the
// stored bag stays sparse. It is a no-op on a nil map.
func (a Attrs) SetInt(key string, val int) {
	if a == nil {
		return
	}
	if val == 0 {
		delete(a, key)
		return
	}
	a[key] = val
}
