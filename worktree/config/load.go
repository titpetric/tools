package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
