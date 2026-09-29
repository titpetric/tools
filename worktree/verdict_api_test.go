package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

func TestRenderVerdictNamesThePackageWhenSymbolsSpanMoreThanOne(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added, apiSymbol{
		Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const", Exported: true,
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	for _, want := range []string{"| Change | Package | Symbol |", "| /inner | const Name |", "| / | type Client struct |"} {
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
	// them names it. Within a package the API comes first and what a consumer
	// cannot reach comes under the rule. The removal below opens a group of its
	// own, so the package is named again there.
	for _, want := range []string{
		"| Added | / | type Client struct |",
		"|  |  | --- |",
		"|  |  | func Dial () error |",
		"|  | /inner | const Name |",
		"|  |  | --- |",
		"|  |  | const Other |",
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

// TestRenderVerdictPartsTheAPIFromWhatIsInternal pins the rule the one symbol
// column is read through: within a package the API comes first, the rest comes
// under a divider, and the divider goes in once per package rather than once
// per symbol.
func TestRenderVerdictPartsTheAPIFromWhatIsInternal(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = []apiSymbol{
		{Key: "example.com/x.hidden", Package: "example.com/x", Name: "hidden", Kind: "func", Signature: "func hidden ()"},
		{Key: "example.com/x.Open", Package: "example.com/x", Name: "Open", Kind: "func", Exported: true, Signature: "func Open () error"},
		{Key: "example.com/x.shut", Package: "example.com/x", Name: "shut", Kind: "func", Signature: "func shut ()"},
	}
	v.API.Changed, v.API.Removed, v.API.Types = nil, nil, nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	rows := tableRows(out.String(), "| Change | Package | Symbol |")
	want := [][]string{
		{"Added", "/", "func Open () error"},
		{"", "", "---"},
		{"", "", "func hidden ()"},
		{"", "", "func shut ()"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("renderVerdict() API rows = %q, want %q", rows, want)
	}
}

// TestRenderVerdictDrawsNoRuleOverInternalSymbolsAlone checks a package whose
// symbols are all internal opens on the symbol rather than on a rule: there is
// nothing above it to be parted from.
func TestRenderVerdictDrawsNoRuleOverInternalSymbolsAlone(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = []apiSymbol{
		{Key: "example.com/x.hidden", Package: "example.com/x", Name: "hidden", Kind: "func", Signature: "func hidden ()"},
	}
	v.API.Changed, v.API.Removed, v.API.Types = nil, nil, nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	rows := tableRows(out.String(), "| Change | Package | Symbol |")
	want := [][]string{{"Added", "/", "func hidden ()"}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("renderVerdict() API rows = %q, want %q", rows, want)
	}
}

// TestRenderVerdictDrawsTheRuleOnATerminal checks the divider is a rule rather
// than three dashes when the report goes to a terminal, and that it is drawn
// across the symbol column alone.
func TestRenderVerdictDrawsTheRuleOnATerminal(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = []apiSymbol{
		{Key: "example.com/x.hidden", Package: "example.com/x", Name: "hidden", Kind: "func", Signature: "func hidden ()"},
		{Key: "example.com/x.Open", Package: "example.com/x", Name: "Open", Kind: "func", Exported: true, Signature: "func Open () error"},
	}
	v.API.Changed, v.API.Removed, v.API.Types = nil, nil, nil

	var out bytes.Buffer
	renderVerdict(&out, v, true)

	got := out.String()
	if strings.Contains(got, "---") {
		t.Errorf("renderVerdict() wrote the markdown divider to a terminal:\n%s", got)
	}
	if strings.Contains(got, components.Separator) {
		t.Errorf("renderVerdict() left the divider sentinel in the output:\n%s", got)
	}
	// The rule is as wide as the symbol column, which the longer of the two
	// symbols sets.
	rule := strings.Repeat("─", len("func Open () error"))
	if !strings.Contains(ansi.Strip(got), rule) {
		t.Errorf("renderVerdict() drew no rule across the symbol column:\n%s", got)
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

	if !strings.Contains(got, "| Change | Package | Symbol | Commits |") {
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
	for _, want := range []string{"| /model | type Trace struct |", "| /frontend/model | type Page struct |"} {
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
