// Package api serves the store.
package api

import "os/exec"

// Handle serves nothing.
func Handle() string { return "ok" }

// Start runs a process, which a test calling it crosses into.
func Start() error { return exec.Command("true").Run() }
