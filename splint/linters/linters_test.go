package linters_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/model"
)

func TestAll(t *testing.T) {
	all := linters.All()
	if len(all) == 0 {
		t.Fatal("All() returned no linters")
	}

	seen := map[string]bool{}
	for _, linter := range all {
		name := linter.Name()
		if name == "" {
			t.Errorf("a linter has no name: %T", linter)
		}
		if seen[name] {
			t.Errorf("two linters are called %q", name)
		}
		seen[name] = true

		// Every linter reads an empty document without complaint, which is
		// what a package holding nothing amounts to.
		report, err := linter.Lint(context.Background(), &model.DocumentRoot{})
		if err != nil {
			t.Errorf("%s: Lint() error = %v", name, err)
			continue
		}
		if report.Linter() != name {
			t.Errorf("%s: report says %q", name, report.Linter())
		}
		if report.Len() != 0 {
			t.Errorf("%s: found %d issues in an empty document", name, report.Len())
		}
	}
}

func TestNames(t *testing.T) {
	if got, want := len(linters.Names()), len(linters.All()); got != want {
		t.Errorf("Names() has %d entries for %d linters", got, want)
	}
}

func TestNamed(t *testing.T) {
	// No selection is every linter.
	selected, unknown := linters.Named()
	if len(selected) != len(linters.All()) || unknown != nil {
		t.Errorf("Named() = %d linters, %v", len(selected), unknown)
	}

	selected, unknown = linters.Named("godoc")
	if len(selected) != 1 || selected[0].Name() != "godoc" || unknown != nil {
		t.Errorf("Named(godoc) = %#v, %v", selected, unknown)
	}

	// A name nothing answers to is reported rather than ignored, so a typo in
	// a flag is not a linter silently not running.
	selected, unknown = linters.Named("godoc", "nosuch")
	if len(selected) != 1 {
		t.Errorf("Named() selected %d linters", len(selected))
	}
	if len(unknown) != 1 || unknown[0] != "nosuch" {
		t.Errorf("Named() unknown = %v", unknown)
	}
}
