package server

import (
	"strings"
	"testing"
)

func TestVersionStaticAssetsAppendsCacheBuster(t *testing.T) {
	html := `<link rel="stylesheet" href="/site.css">` +
		`<script src="/app.js"></script>` +
		`<script src="/api.js"></script>` +
		`<script src="/landing.js"></script>`

	got := versionStaticAssets(html, "0.1.1066")

	for _, want := range []string{
		`href="/site.css?v=0.1.1066"`,
		`src="/app.js?v=0.1.1066"`,
		`src="/api.js?v=0.1.1066"`,
		`src="/landing.js?v=0.1.1066"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in versioned HTML, got:\n%s", want, got)
		}
	}
}

func TestVersionStaticAssetsNoVersionIsNoOp(t *testing.T) {
	html := `<script src="/app.js"></script>`
	if got := versionStaticAssets(html, "  "); got != html {
		t.Errorf("expected unchanged HTML for blank version, got %q", got)
	}
}
