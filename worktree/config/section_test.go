package config

import (
	"testing"
)

// TestSection checks a section is a heading over the fields under it, which is
// one form block.
func TestSection(t *testing.T) {
	section := Section{Title: "Scan", Fields: []Field{{Title: "Enable Gitignore", Key: "scan.enable_gitignore"}}}

	if section.Title != "Scan" {
		t.Errorf("Section.Title = %q, want %q", section.Title, "Scan")
	}
	if len(section.Fields) != 1 {
		t.Errorf("Section.Fields = %d, want 1", len(section.Fields))
	}
}

// TestConfig_Sections checks the form covers the whole document: every section
// is named, holds fields, and every setting appears exactly once across them,
// which is what makes a round trip through the screen lossless.
func TestConfig_Sections(t *testing.T) {
	sections := Default().Sections()
	if len(sections) == 0 {
		t.Fatal("Config.Sections() returned nothing to edit")
	}

	seen := map[string]int{}
	for _, section := range sections {
		if section.Title == "" {
			t.Error("Config.Sections() returned a section with no heading")
		}
		if len(section.Fields) == 0 {
			t.Errorf("Config.Sections() returned the empty section %q", section.Title)
		}
		for _, field := range section.Fields {
			if field.Key == "" {
				t.Errorf("%s: a field has no key", section.Title)
			}
			seen[field.Key]++
		}
	}
	for key, count := range seen {
		if count != 1 {
			t.Errorf("Config.Sections() showed %q %d times, want once", key, count)
		}
	}

	// Fields is Sections flattened, so the two have to agree on the count.
	if got, want := len(Default().Fields()), len(seen); got != want {
		t.Errorf("Config.Fields() = %d settings, Config.Sections() = %d", got, want)
	}
}
