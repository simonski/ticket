package server

import (
	"strings"
	"testing"
)

func TestRenderSpecPromptGuidanceDescribesAllBlockTypes(t *testing.T) {
	guidance := renderSpecPromptGuidance()
	for _, want := range []string{"render", "\"text\"", "\"list\"", "\"table\"", "\"chart\"", "chartType"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("prompt guidance missing %q\n---\n%s", want, guidance)
		}
	}
}

func TestValidRenderSpec(t *testing.T) {
	cases := []struct {
		name string
		json string
		want bool
	}{
		{"table", `{"blocks":[{"type":"table","columns":["A"],"rows":[["1"]]}]}`, true},
		{"chart", `{"blocks":[{"type":"chart","chartType":"bar","labels":["Q1"],"series":[{"name":"x","data":[1]}]}]}`, true},
		{"mixed", `{"blocks":[{"type":"text","content":"hi"},{"type":"list","items":["a"]}]}`, true},
		{"unknown type", `{"blocks":[{"type":"pie"}]}`, false},
		{"empty blocks", `{"blocks":[]}`, false},
		{"no blocks key", `{"foo":1}`, false},
		{"broken json", `{"blocks":[{`, false},
	}
	for _, tc := range cases {
		if got := validRenderSpec(tc.json); got != tc.want {
			t.Errorf("%s: validRenderSpec=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeRenderBlockKeepsValidStripsInvalid(t *testing.T) {
	valid := "Here is the data.\n\n```render\n{\"blocks\":[{\"type\":\"list\",\"items\":[\"a\",\"b\"]}]}\n```\n"
	if got := sanitizeRenderBlock(valid); got != valid {
		t.Errorf("valid block should be preserved:\nwant %q\ngot  %q", valid, got)
	}

	invalid := "Here is the data.\n\n```render\n{not json at all\n```\n"
	got := sanitizeRenderBlock(invalid)
	if strings.Contains(got, "render") || strings.Contains(got, "not json") {
		t.Errorf("invalid render block should be stripped, got %q", got)
	}
	if !strings.Contains(got, "Here is the data.") {
		t.Errorf("prose should survive stripping, got %q", got)
	}

	plain := "Just a normal answer with no block.\n"
	if got := sanitizeRenderBlock(plain); got != plain {
		t.Errorf("plain text should be untouched, got %q", got)
	}
}

func TestFindRenderFence(t *testing.T) {
	s := "prose line\n```render\n{\"blocks\":[]}\n```\ntrailing"
	start, inner, end, ok := findRenderFence(s)
	if !ok {
		t.Fatalf("expected to find a render fence")
	}
	if strings.TrimSpace(inner) != `{"blocks":[]}` {
		t.Errorf("inner=%q", inner)
	}
	if s[:start] != "prose line\n" {
		t.Errorf("prefix=%q", s[:start])
	}
	if !strings.HasPrefix(s[end:], "trailing") {
		t.Errorf("suffix=%q", s[end:])
	}

	if _, _, _, ok := findRenderFence("no fence here at all"); ok {
		t.Errorf("did not expect a fence")
	}
}
