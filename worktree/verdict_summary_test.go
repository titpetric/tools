package main

import (
	"path/filepath"
	"testing"
)

func TestVerdictSummary(t *testing.T) {
	tests := []struct {
		name string
		in   verdict
		want string
	}{
		{
			name: "first release",
			in: verdict{
				Version: "v0.0.1",
				API:     apiDiff{Added: []apiSymbol{{Key: "x.A", Exported: true}, {Key: "x.B", Exported: true}}},
			},
			want: "First release: v0.0.1, 2 exported symbols are added.",
		},
		{
			name: "first release, the API was not read",
			in:   verdict{Version: "v0.0.1", API: apiDiff{Skipped: "splint is not installed"}},
			want: "First release: v0.0.1, the API was not read, splint is not installed.",
		},
		{
			name: "patch",
			in:   verdict{Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch},
			want: "Patch release: v1.0.1, no exported symbols were removed since v1.0.0.",
		},
		{
			name: "minor, one removal",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A", Exported: true}}, Breaking: true},
			},
			want: "Minor release: v1.1.0, because 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "minor, removals and signature changes",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{
					Removed:  []apiSymbol{{Key: "x.A", Exported: true}, {Key: "x.B", Exported: true}},
					Changed:  []apiChange{{Key: "x.C"}},
					Breaking: true,
				},
			},
			want: "Minor release: v1.1.0, because 2 exported symbols were removed and 1 signature changed since v1.0.0.",
		},
		{
			name: "nothing to compare",
			in: verdict{
				Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch,
				API: apiDiff{Skipped: "splint is not installed"},
			},
			want: "Patch release: v1.0.1, the API was not compared, splint is not installed.",
		},
		{
			name: "a release that was made",
			in:   verdict{Version: "v1.1.0", Since: "v1.0.0", Released: true},
			want: "Released v1.1.0: no exported symbols were removed since v1.0.0.",
		},
		{
			name: "a release that was made, breaking",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Released: true,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A", Exported: true}}, Breaking: true},
			},
			want: "Released v1.1.0: 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "a release with nothing before it",
			in: verdict{
				Version: "v1.0.0", Released: true,
				API: apiDiff{Added: []apiSymbol{{Key: "x.A", Exported: true}}},
			},
			want: "Released v1.0.0: the first release, 1 exported symbol is added.",
		},
	}

	for _, test := range tests {
		if got := test.in.Summary(); got != test.want {
			t.Errorf("%s: Summary() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestVerdictMovingGoSeriesCostsAMinor(t *testing.T) {
	requireSplint(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Nothing of the API moves, only the language version.
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.GoBefore != "1.24" || got.GoAfter != "1.27" {
		t.Errorf("readVerdict() = {GoBefore: %q, GoAfter: %q}, want {1.24, 1.27}", got.GoBefore, got.GoAfter)
	}
	if got.Release != releaseMinor || got.Version != "v0.2.0" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q}, want {minor, v0.2.0}", got.Release, got.Version)
	}
	if want := "Minor release: v0.2.0, because go moved from 1.24 to 1.27 since v0.1.0."; got.Summary() != want {
		t.Errorf("Summary() = %q, want %q", got.Summary(), want)
	}

	// Once that release is made, a point release of the same series is not
	// worth another minor.
	runGit(t, root, "tag", "alpha/v0.2.0")
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27.3\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27.3")

	got, err = readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Release != releasePatch || got.Version != "v0.2.1" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q} for a point release, want {patch, v0.2.1}", got.Release, got.Version)
	}
}

func TestVerdictBreakageNamesTheDataModel(t *testing.T) {
	tests := []struct {
		title string
		types []apiTypeChange
		want  string
	}{{
		title: "a field a struct lost costs a consumer something",
		types: []apiTypeChange{{
			Name: "Config", Underlying: "struct", Breaking: true,
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldRemoved}},
		}},
		want: "Minor release: v1.1.0, because 1 exported field moved since v1.0.0.",
	}, {
		title: "a field a struct gained costs nothing",
		types: []apiTypeChange{{
			Name: "Config", Underlying: "struct",
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldAdded}},
		}},
		want: "Patch release: v1.1.0, no exported symbols were removed since v1.0.0.",
	}, {
		title: "a method an interface gained stops every implementor compiling",
		types: []apiTypeChange{{
			Name: "Store", Underlying: "interface", Breaking: true,
			Fields: []apiFieldChange{{Name: "Put", Change: fieldAdded}},
		}},
		want: "Minor release: v1.1.0, because 1 exported field moved since v1.0.0.",
	}}

	for _, test := range tests {
		v := verdict{Version: "v1.1.0", Since: "v1.0.0", Release: releasePatch}
		v.API.Types = test.types
		for _, change := range test.types {
			v.API.Breaking = v.API.Breaking || change.Breaking
		}
		if v.API.Breaking {
			v.Release = releaseMinor
		}

		if got := v.Summary(); got != test.want {
			t.Errorf("%s: Summary() = %q, want %q", test.title, got, test.want)
		}
	}
}
