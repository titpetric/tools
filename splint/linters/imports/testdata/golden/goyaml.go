package main

import (
	"github.com/goccy/go-yaml"
)

// Marshal writes v as YAML.
//
// The package at the end of that path is called yaml, and the path does not
// say so. The import is not unused and the name is not missing, and a
// formatter reading the last segment of the path alone would report both.
func Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}
