package web

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

const DefaultSite = "default"

// Static contains the embedded frontend assets served by the HTTP server.
//
//go:embed default/* shared/*
var Static embed.FS

func AvailableSites() []string {
	return []string{DefaultSite}
}

// overlayFS resolves Open calls against primary first, then fallback.
// This lets site-specific files shadow shared assets while still serving
// anything not present in the site directory from the shared pool.
type overlayFS struct {
	primary  fs.FS
	fallback fs.FS
}

func (o overlayFS) Open(name string) (fs.File, error) {
	f, err := o.primary.Open(name)
	if err == nil {
		return f, nil
	}
	return o.fallback.Open(name)
}

// SiteFS returns the filesystem for the named site.
// The only valid site is "default" (or ""). It returns an overlay:
// default/ files take precedence, shared/ provides the fallback.
func SiteFS(name string) (fs.FS, error) {
	clean := strings.TrimSpace(name)
	if clean != "" && clean != DefaultSite {
		return nil, fmt.Errorf("unknown embedded site %q", clean)
	}
	shared, err := fs.Sub(Static, "shared")
	if err != nil {
		return nil, err
	}
	primary, err := fs.Sub(Static, "default")
	if err != nil {
		return nil, err
	}
	return overlayFS{primary: primary, fallback: shared}, nil
}
