package main

import (
	"os/exec"
	"strings"
	"sync"
)

var gitOriginByRoot sync.Map

func detectGitOriginAt(root string) string {
	if cached, ok := gitOriginByRoot.Load(root); ok {
		return cached.(string)
	}
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output() // #nosec G204 -- command and arguments are fixed; root is a trusted local path
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
	if remote != "" {
		gitOriginByRoot.Store(root, remote)
	}
	return remote
}
