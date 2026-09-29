package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderVerdictNamesThePackageWhenSymbolsSpanMoreThanOne(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added, apiSymbol{
		Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const", Exported: true,
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	for _, want := range []string{"| Change | Package | Exported | Unexported |", "| /inner | const Name |  |", "| / | type Client struct |  |"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictNamesAPackageOncePerRunOfSymbols(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added,
		apiSymbol{
			Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const", Exported: true,
		},
		apiSymbol{
			Key: "example.com/x.Dial", Package: "example.com/x", Name: "Dial", Kind: "func", Signature: "func Dial () error",
		},
		apiSymbol{
			Key: "example.com/x/inner.Other", Package: "example.com/x/inner", Name: "Other", Kind: "const",
		},
	)

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	// The symbols of a package are gathered together, and only the first of
	// them names it. The removal below opens a group of its own, so the package
	// is named again there.
	for _, want := range []string{
		"| Added | / | type Client struct |  |",
		"|  |  |  | func Dial () error |",
		"|  | /inner | const Name |",
		"|  |  |  | const Other |",
		"| Removed | / | func Legacy () error |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "| /inner |"); n != 1 {
		t.Errorf("renderVerdict() named the package %d times, want once:\n%s", n, got)
	}
}

func TestRenderVerdictNamesTheCommitsBehindASymbol(t *testing.T) {
	requireSplint(t)

	alpha := commitScanRepo(t)
	v, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	// Greet was added by the first commit of the range and reshaped by the
	// last, so the row naming it names both, in the order they were made.
	added, changed := v.Commits[2].Hash, v.Commits[0].Hash

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	if !strings.Contains(got, "| Change | Package | Exported | Unexported | Commits |") {
		t.Fatalf("renderVerdict() wrote no commits column:\n%s", got)
	}
	if want := "`" + added + "`, `" + changed + "`"; !strings.Contains(got, want) {
		t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
	}
}

func TestRenderVerdictLinksTheCommitsBehindASymbol(t *testing.T) {
	v := sampleVerdict()
	v.CommitAPI = map[string]apiDiff{
		"abc1234": {Added: []apiSymbol{{Key: "example.com/x.Client"}}},
		"def5678": {Removed: []apiSymbol{{Key: "example.com/x.Legacy"}}},
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// A commit is linked in the API table the way the commit table links it.
	for _, want := range []string{
		"[`abc1234`](https://github.com/example/x/commit/abc1234) |",
		"[`def5678`](https://github.com/example/x/commit/def5678) |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictNamesAPackageByItsPathBelowTheModule(t *testing.T) {
	requireSplint(t)

	v, err := readVerdict(twoModelsRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// Two packages named model, kept apart by the path they sit at rather than
	// by the name they share.
	for _, want := range []string{"| /model | type Trace struct |  |", "| /frontend/model | type Page struct |  |"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "| model | type") {
		t.Errorf("renderVerdict() named a package by its name alone:\n%s", got)
	}
}

func TestCollapseRemovedMethods(t *testing.T) {
	symbols := []apiSymbol{
		{Key: "m.Disk", Package: "m", Name: "Disk", Kind: "type"},
		{Key: "m.Disk.Save", Package: "m", Name: "Disk.Save", Kind: "func"},
		{Key: "m.Disk.Len", Package: "m", Name: "Disk.Len", Kind: "func"},
		{Key: "m.Orphan.Close", Package: "m", Name: "Orphan.Close", Kind: "func"},
		{Key: "m.ValidID", Package: "m", Name: "ValidID", Kind: "func"},
		{Key: "other.Disk.Save", Package: "other", Name: "Disk.Save", Kind: "func"},
	}

	kept := collapseRemovedMethods(symbols)

	want := []string{"Disk", "Orphan.Close", "ValidID", "Disk.Save"}
	if len(kept) != len(want) {
		t.Fatalf("kept %d symbols, want %d: %+v", len(kept), len(want), kept)
	}
	for i, name := range want {
		if kept[i].Name != name {
			t.Fatalf("kept[%d] = %q, want %q", i, kept[i].Name, name)
		}
	}
}
