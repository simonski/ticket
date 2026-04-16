package web

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

const (
	DefaultSite = "default"
	Site2       = "site2"
)

// Static contains the embedded frontend assets served by the HTTP server.
//
//go:embed static/* site2/*
var Static embed.FS

func AvailableSites() []string {
	return []string{DefaultSite, Site2}
}

func SiteFS(name string) (fs.FS, error) {
	switch strings.TrimSpace(name) {
	case "", DefaultSite:
		return fs.Sub(Static, "static")
	case Site2:
		return fs.Sub(Static, "site2")
	default:
		return nil, fmt.Errorf("unknown embedded site %q", name)
	}
}
