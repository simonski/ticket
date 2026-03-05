package web

import "embed"

// Static contains the embedded frontend assets served by the HTTP server.
//
//go:embed static/*
var Static embed.FS
