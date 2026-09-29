package components

import (
	"strings"
	"testing"
)

// TestShortName and TestShortPath cover the two ways a module path is
// shortened for a column: to its last segment, and to everything after the
// host, which is what keeps the module column narrow without losing the owner.
func TestShortName(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/titpetric/tools/worktree": "worktree",
		"charm.land/bubbletea/v2":             "v2",
		"worktree":                            "worktree",
		"":                                    ".",
	} {
		if got := ShortName(in); got != want {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortPath(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/titpetric/tools": "titpetric/tools",
		"charm.land/bubbletea/v2":    "charm.land/bubbletea/v2",
		"gopkg.in/yaml.v3":           "gopkg.in/yaml.v3",
	} {
		if got := ShortPath(in); got != want {
			t.Errorf("ShortPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestModule(t *testing.T) {
	got := Module("./worktree")
	if got.Height() != 1 || !strings.Contains(got.Line(0), "./worktree") {
		t.Errorf("Module() = %q, want one line naming the path", got)
	}
}
