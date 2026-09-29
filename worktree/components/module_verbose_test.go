package components

import (
	"strings"
	"testing"
)

// TestModuleVerbose checks the verbose module cell: the description loses the
// name in front of its dash, since the cell names the module underneath it
// anyway, and a module with no description writes one line instead of two.
func TestModuleVerbose(t *testing.T) {
	got := ModuleVerbose("worktree - Show workspace details", "./worktree", "github.com/titpetric/tools/worktree")
	if got.Height() != 2 {
		t.Fatalf("ModuleVerbose() = %d lines, want 2: %q", got.Height(), got)
	}
	if strings.Contains(got.Line(0), "worktree - ") {
		t.Errorf("ModuleVerbose() kept the name in front of the dash: %q", got.Line(0))
	}
	if !strings.Contains(got.Line(0), "Show workspace details") {
		t.Errorf("ModuleVerbose() dropped the description: %q", got.Line(0))
	}
	// The module line names the import path without its host, and the
	// directory it is checked out at.
	if !strings.Contains(got.Line(1), "titpetric/tools/worktree") || !strings.Contains(got.Line(1), "./worktree") {
		t.Errorf("ModuleVerbose() = %q, want the import path and the directory", got.Line(1))
	}

	bare := ModuleVerbose("", "./worktree", "github.com/titpetric/tools/worktree")
	if bare.Height() != 1 {
		t.Errorf("ModuleVerbose() without a description = %d lines, want 1: %q", bare.Height(), bare)
	}
}
