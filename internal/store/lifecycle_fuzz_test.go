package store

import "testing"

func FuzzParseLifecycleStatus(f *testing.F) {
	f.Add("design/idle")
	f.Add("develop/active")
	f.Add("test/success")
	f.Add("done/fail")
	f.Add("done/complete")
	f.Add("")
	f.Add("noslash")
	f.Add("bad/open")
	f.Add("open/idle")

	f.Fuzz(func(t *testing.T, input string) {
		stage, state, err := ParseLifecycleStatus(input)
		if err != nil {
			return
		}
		if !ValidLifecycle(stage, state) {
			t.Fatalf("ParseLifecycleStatus(%q) = (%q, %q), want valid lifecycle", input, stage, state)
		}
		if rendered := RenderLifecycleStatus(stage, state); rendered == "" {
			t.Fatalf("RenderLifecycleStatus(%q, %q) = empty", stage, state)
		}
	})
}
