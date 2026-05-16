package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/simonski/ticket/internal/config"
)

func ensureDefaultLocalRemote(dbPath string) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	localURL, err := config.CanonicalizeRemoteURL(dbPath)
	if err != nil {
		return config.Config{}, err
	}
	if remote, ok := cfg.RemoteByURL(localURL); ok {
		if cfg.DefaultRemote == "" {
			cfg.DefaultRemote = remote.Name
		}
		err = config.Save(cfg)
		if err != nil {
			return config.Config{}, err
		}
		return cfg, nil
	}
	cfg, err = config.AddRemote(cfg, config.Remote{Name: "local", URL: localURL})
	if err != nil {
		return config.Config{}, err
	}
	cfg.DefaultRemote = "local"
	if err := config.Save(cfg); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
}

func ensureNamedLocalRemote(root, dbPath string) (config.Config, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, "", err
	}
	localURL, err := config.CanonicalizeRemoteURL(dbPath)
	if err != nil {
		return config.Config{}, "", err
	}
	if remote, ok := cfg.RemoteByURL(localURL); ok {
		err = config.Save(cfg)
		if err != nil {
			return config.Config{}, "", err
		}
		return cfg, remote.Name, nil
	}
	name := uniqueRemoteName(cfg, filepath.Base(root))
	cfg, err = config.AddRemote(cfg, config.Remote{Name: name, URL: localURL})
	if err != nil {
		return config.Config{}, "", err
	}
	if err := config.Save(cfg); err != nil {
		return config.Config{}, "", err
	}
	return cfg, name, nil
}

func uniqueRemoteName(cfg config.Config, preferred string) string {
	base := strings.TrimSpace(preferred)
	if base == "" {
		base = "local"
	}
	if _, ok := cfg.RemoteByName(base); !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, ok := cfg.RemoteByName(candidate); !ok {
			return candidate
		}
	}
}
