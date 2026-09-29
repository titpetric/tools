package config

import (
	"bytes"
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

// Encode renders the configuration document, with the version this build
// writes and a header pointing at the setup screen.
func Encode(cfg *Config) ([]byte, error) {
	out := *cfg
	out.Version = Version

	var buf bytes.Buffer
	buf.WriteString("# worktree configuration, written by \"worktree config\".\n")
	buf.WriteString("#\n")
	buf.WriteString("# This file is the complete configuration. The built-in defaults are not\n")
	buf.WriteString("# applied underneath it, so a setting removed from this file reads as off.\n")
	buf.WriteString("\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&out); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return buf.Bytes(), nil
}

// decode reads a configuration document into cfg. It is the other half of
// Encode, and lives beside it so the yaml codec is reached from one file.
func decode(cfg *Config, data []byte) error {
	return yaml.Unmarshal(data, cfg)
}
