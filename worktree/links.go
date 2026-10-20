package main

import (
	"strings"
)

// moduleLink returns a GitHub URL for a Go module path.
// For submodules (e.g. github.com/owner/name/subdir), it generates
// a link to the subdirectory: https://github.com/owner/name/tree/main/subdir
func moduleLink(modPath string) string {
	path := strings.TrimPrefix(modPath, "github.com/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) <= 2 {
		// Single repository: github.com/owner/name
		return "https://" + modPath
	}

	// Submodule: github.com/owner/name/subdir/...
	repo := "https://github.com/" + parts[0] + "/" + parts[1]
	subdir := parts[2]
	return repo + "/tree/main/" + subdir
}
