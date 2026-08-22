package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

// FileName is the configuration file, relative to the user home directory.
const FileName = ".config/worktree.yml"

// Path returns the location of the configuration document.
func Path() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, filepath.FromSlash(FileName)), nil
}

// homeDir returns the user home directory the configuration lives below.
func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return home, nil
}

// Default returns the built-in configuration, the values that apply when no
// document exists.
func Default() *Config {
	cfg, err := Parse(DefaultConfig)
	if err != nil {
		// The embedded document is part of the build, so a parse failure is
		// a programming error rather than something a run can recover from.
		panic("config: parse embedded config.yml: " + err.Error())
	}
	return cfg
}

// Parse decodes a configuration document. Absent settings are left at their
// zero value; nothing is filled in from the defaults.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Version > Version {
		return nil, fmt.Errorf("config version %d is newer than this build understands (%d)", cfg.Version, Version)
	}
	return cfg, nil
}

// Load reads the configuration document. When the file does not exist the
// built-in defaults are returned. When it does exist it is the whole
// configuration: the defaults are not applied underneath it, so a setting the
// file does not name reads as off.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFile(path)
}

// LoadFile reads the configuration document at path, returning the built-in
// defaults when it does not exist.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the configuration document, creating the directory it lives in.
// Every setting is written, including the ones left at their zero value, so
// the file that comes back is complete.
func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveFile(path, cfg)
}

// SaveFile writes the configuration document to path.
func SaveFile(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	data, err := Encode(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

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
