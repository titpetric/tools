package config

import (
	_ "embed"
)

// DefaultConfig is the built-in configuration document. It holds the value of
// every setting when FileName does not exist below the home directory, and the comments
// in it are the reference for the file the setup screen writes.
//
//go:embed config.yml
var DefaultConfig []byte
