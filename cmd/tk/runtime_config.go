package main

import (
	"os"
	"strings"
	"sync"

	"github.com/simonski/ticket/internal/config"
)

var runtimeConfigState struct {
	mu     sync.Mutex
	key    string
	cfg    config.Config
	err    error
	loaded bool
}

func resetRuntimeConfigCache() {
	runtimeConfigState.mu.Lock()
	defer runtimeConfigState.mu.Unlock()
	runtimeConfigState.key = ""
	runtimeConfigState.cfg = config.Config{}
	runtimeConfigState.err = nil
	runtimeConfigState.loaded = false
}

func loadRuntimeConfig() (config.Config, error) {
	key := runtimeConfigCacheKey()

	runtimeConfigState.mu.Lock()
	if runtimeConfigState.loaded && runtimeConfigState.key == key {
		cfg, err := runtimeConfigState.cfg, runtimeConfigState.err
		runtimeConfigState.mu.Unlock()
		return cfg, err
	}
	runtimeConfigState.mu.Unlock()

	cfg, err := config.Load()

	runtimeConfigState.mu.Lock()
	runtimeConfigState.key = key
	runtimeConfigState.cfg = cfg
	runtimeConfigState.err = err
	runtimeConfigState.loaded = true
	runtimeConfigState.mu.Unlock()
	return cfg, err
}

func runtimeConfigCacheKey() string {
	cwd, _ := os.Getwd()
	return strings.Join([]string{
		cwd,
		strings.TrimSpace(os.Getenv("TICKET_HOME")),
		strings.TrimSpace(os.Getenv("TICKET_URL")),
		strings.TrimSpace(os.Getenv("TICKET_USERNAME")),
		strings.TrimSpace(os.Getenv("TICKET_PASSWORD")),
		strings.TrimSpace(os.Getenv("TICKET_TOKEN")),
		strings.TrimSpace(os.Getenv("TICKET_PROJECT")),
	}, "\x00")
}
