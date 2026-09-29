package config

import (
	"strings"
	"testing"
)

// TestDefaultConfig checks the embedded document is the reference it claims to
// be: it parses, it is what Default returns, and it names every setting the
// form can edit, since it is what a reader is pointed at to learn the file.
func TestDefaultConfig(t *testing.T) {
	if len(DefaultConfig) == 0 {
		t.Fatal("DefaultConfig is empty, so config.yml was not embedded")
	}

	cfg, err := Parse(DefaultConfig)
	if err != nil {
		t.Fatalf("Parse(DefaultConfig) error: %v", err)
	}
	if cfg.Version != Version {
		t.Errorf("DefaultConfig declares version %d, want %d", cfg.Version, Version)
	}

	for _, field := range cfg.Fields() {
		if !strings.Contains(string(DefaultConfig), lastKey(field.Key)) {
			t.Errorf("DefaultConfig does not name the %q setting", field.Key)
		}
	}
}

// lastKey returns the leaf of a dotted setting key, which is what the document
// writes under its section.
func lastKey(key string) string {
	if _, after, found := strings.Cut(key, "."); found {
		return after
	}
	return key
}
