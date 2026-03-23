package main

import "strings"

// moduleLink returns a GitHub URL for a Go module path.
// For submodules (e.g. github.com/user/repo/subdir), it generates
// a link to the subdirectory: https://github.com/user/repo/tree/main/subdir
func moduleLink(modPath string) string {
	path := strings.TrimPrefix(modPath, "github.com/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) <= 2 {
		// Simple repo: github.com/user/repo
		return "https://" + modPath
	}

	// Submodule: github.com/user/repo/subdir/...
	repo := "https://github.com/" + parts[0] + "/" + parts[1]
	subdir := parts[2]
	return repo + "/tree/main/" + subdir
}
