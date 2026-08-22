package config

import _ "embed"

// DefaultConfig is the built-in configuration document. It is the value of
// every setting when ~/.config/worktree.yml does not exist, and the comments
// in it are the reference for the file the setup screen writes.
//
//go:embed config.yml
var DefaultConfig []byte
