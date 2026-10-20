package config

import (
	"strings"
	"testing"
)

// TestDefaultConfig checks the embedded document holds up as the reference: it
// parses, Default returns it, and it names every setting the form can edit,
// since readers learn the file from it.
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

// lastKey returns the leaf of a dotted setting key, the name the document
// writes under its section.
func lastKey(key string) string {
	if _, after, found := strings.Cut(key, "."); found {
		return after
	}
	return key
}
