package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestAttrsAccessors(t *testing.T) {
	t.Parallel()
	a := Attrs{
		"s":   "hello",
		"f":   float64(7),
		"i":   int(8),
		"i64": int64(9),
		"n":   json.Number("10"),
		"b":   true,
	}
	if a.GetString("s") != "hello" {
		t.Errorf("GetString(s) = %q", a.GetString("s"))
	}
	if a.GetString("missing") != "" || a.GetString("f") != "" {
		t.Error("GetString should return empty for missing/non-string")
	}
	if a.GetInt("f") != 7 || a.GetInt("i") != 8 || a.GetInt("i64") != 9 || a.GetInt("n") != 10 {
		t.Errorf("GetInt numeric coercions wrong: %d %d %d %d", a.GetInt("f"), a.GetInt("i"), a.GetInt("i64"), a.GetInt("n"))
	}
	if a.GetInt("s") != 0 || a.GetInt("b") != 0 || a.GetInt("missing") != 0 {
		t.Error("GetInt should return 0 for non-numeric/missing")
	}

	// SetString: set, then delete via empty.
	a.SetString("k", "v")
	if a.GetString("k") != "v" {
		t.Error("SetString did not set")
	}
	a.SetString("k", "")
	if _, ok := a["k"]; ok {
		t.Error("SetString(empty) should delete the key")
	}
	// SetInt: set, then delete via zero.
	a.SetInt("z", 5)
	if a.GetInt("z") != 5 {
		t.Error("SetInt did not set")
	}
	a.SetInt("z", 0)
	if _, ok := a["z"]; ok {
		t.Error("SetInt(0) should delete the key")
	}
	// nil-map mutators are no-ops (must not panic).
	var nilAttrs Attrs
	nilAttrs.SetString("x", "y")
	nilAttrs.SetInt("x", 1)
}

func TestMarshalParseAttrs(t *testing.T) {
	t.Parallel()
	if s, err := marshalAttrs(nil); err != nil || s != "{}" {
		t.Fatalf("marshalAttrs(nil) = %q, %v", s, err)
	}
	if s, err := marshalAttrs(Attrs{}); err != nil || s != "{}" {
		t.Fatalf("marshalAttrs(empty) = %q, %v", s, err)
	}
	s, err := marshalAttrs(Attrs{"a": "b"})
	if err != nil || s != `{"a":"b"}` {
		t.Fatalf("marshalAttrs = %q, %v", s, err)
	}
	for _, empty := range []string{"", "   ", "null"} {
		got, err := parseAttrs(empty)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("parseAttrs(%q) = %#v, %v", empty, got, err)
		}
	}
	got, err := parseAttrs(`{"a":"b","n":3}`)
	if err != nil || got.GetString("a") != "b" || got.GetInt("n") != 3 {
		t.Fatalf("parseAttrs valid = %#v, %v", got, err)
	}
	if _, err := parseAttrs(`{not json`); err == nil {
		t.Fatal("parseAttrs(invalid) expected error")
	}
}

func TestGuidanceMapFromAttr(t *testing.T) {
	t.Parallel()
	m := guidanceMapFromAttr(map[string]any{"default": "x", "develop": "y", "bad": 1})
	if m["default"] != "x" || m["develop"] != "y" {
		t.Fatalf("guidanceMapFromAttr = %#v", m)
	}
	if _, ok := m["bad"]; ok {
		t.Error("non-string values should be dropped")
	}
	if guidanceMapFromAttr("not a map") != nil || guidanceMapFromAttr(nil) != nil {
		t.Error("non-object should yield nil")
	}
	if guidanceMapFromAttr(map[string]any{}) != nil {
		t.Error("empty object should yield nil")
	}
}

func TestAttrsExtractExprAndListErrors(t *testing.T) {
	t.Parallel()
	if _, err := attrsExtractExpr("", "bad key"); err == nil {
		t.Error("attrsExtractExpr(invalid key) expected error")
	}
	expr, err := attrsExtractExpr("t", "severity")
	if err != nil || expr != "json_extract(t.attrs, '$.severity')" {
		t.Fatalf("attrsExtractExpr = %q, %v", expr, err)
	}

	db, _ := attrsTestDB(t)
	ctx := context.Background()
	if _, err := ListTicketsByAttr(ctx, db, 0, "severity", "high"); err == nil {
		t.Error("ListTicketsByAttr(projectID=0) expected error")
	}
	if _, err := ListTicketsByAttr(ctx, db, 1, "bad key", "x"); err == nil {
		t.Error("ListTicketsByAttr(invalid key) expected error")
	}
}

func TestBackupDatabase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.db")
	if err := Init(src, "admin", "secret12"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	// empty paths rejected
	if err := BackupDatabase("", "x"); err == nil {
		t.Error("BackupDatabase(empty src) expected error")
	}
	// missing source rejected
	if err := BackupDatabase(filepath.Join(dir, "nope.db"), filepath.Join(dir, "out.db")); err == nil {
		t.Error("BackupDatabase(missing src) expected error")
	}
	// happy path
	dst := filepath.Join(dir, "backup.db")
	if err := BackupDatabase(src, dst); err != nil {
		t.Fatalf("BackupDatabase() error = %v", err)
	}
	if v, err := DetectSchemaVersion(dst); err != nil || v != CurrentSchemaVersion {
		t.Fatalf("backup version = %d, %v", v, err)
	}
	// existing target rejected
	if err := BackupDatabase(src, dst); err == nil {
		t.Error("BackupDatabase(existing target) expected error")
	}
}
