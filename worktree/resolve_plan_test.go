package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveOrder(t *testing.T) {
	tests := []struct {
		name   string
		mods   []string
		uses   map[string][]string
		order  []string
		cycles []string
	}{
		{
			name:  "unrelated modules sort by name",
			mods:  []string{"c", "a", "b"},
			uses:  map[string][]string{},
			order: []string{"a", "b", "c"},
		},
		{
			name:  "a chain runs from the outside in",
			mods:  []string{"app", "lib", "base"},
			uses:  map[string][]string{"app": {"lib"}, "lib": {"base"}},
			order: []string{"base", "lib", "app"},
		},
		{
			name:  "a diamond visits both sides before the consumer",
			mods:  []string{"app", "left", "right", "base"},
			uses:  map[string][]string{"app": {"left", "right"}, "left": {"base"}, "right": {"base"}},
			order: []string{"base", "left", "right", "app"},
		},
		{
			name:  "a dependency outside the selection is ignored",
			mods:  []string{"app"},
			uses:  map[string][]string{"app": {"lib"}},
			order: []string{"app"},
		},
		{
			name:   "a cycle is reported rather than ordered",
			mods:   []string{"a", "b", "c"},
			uses:   map[string][]string{"a": {"b"}, "b": {"a"}, "c": nil},
			order:  []string{"c"},
			cycles: []string{"a", "b"},
		},
		{
			name:  "a module requiring itself is not waiting on anything",
			mods:  []string{"a"},
			uses:  map[string][]string{"a": {"a"}},
			order: []string{"a"},
		},
	}

	for _, test := range tests {
		order, cycles := resolveOrder(test.mods, test.uses)
		if !reflect.DeepEqual(order, test.order) {
			t.Errorf("%s: resolveOrder() order = %#v, want %#v", test.name, order, test.order)
		}
		if !reflect.DeepEqual(cycles, test.cycles) {
			t.Errorf("%s: resolveOrder() cycles = %#v, want %#v", test.name, cycles, test.cycles)
		}
	}
}

func TestResolvePins(t *testing.T) {
	refs := versionRefs{
		"app": {"lib": "v1.0.0", "base": "v2.0.0"},
	}
	targets := map[string]string{
		"lib":  "v1.1.0", // released by this run
		"base": "v2.0.0", // already at the version it ends up at
		"none": "",       // no release to pin to
	}

	got := resolvePins("app", []string{"base", "lib", "none"}, refs, targets)
	want := []requireInfo{{path: "lib", version: "v1.1.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolvePins() = %#v, want %#v", got, want)
	}
}

func TestDirtyFilesIgnoresGoModAndScopesToTheModule(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	alpha := filepath.Join(root, "alpha")

	if got := dirtyFiles(alpha); got != nil {
		t.Fatalf("dirtyFiles() on a clean module = %#v, want nil", got)
	}

	// The files resolve commits itself are not what stops a run.
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(alpha, "go.sum"), "\n")
	if got := dirtyFiles(alpha); got != nil {
		t.Fatalf("dirtyFiles() with only go.mod changed = %#v, want nil", got)
	}

	// A change to another module is not this module's problem.
	writeTestFile(t, filepath.Join(root, "beta", "beta.go"), "package beta\n\n// changed\n")
	if got := dirtyFiles(alpha); got != nil {
		t.Fatalf("dirtyFiles() saw another module's change = %#v", got)
	}

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// changed\n")
	writeTestFile(t, filepath.Join(alpha, "new.go"), "package alpha\n")
	got := dirtyFiles(alpha)
	want := []string{"M alpha.go", "?? new.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirtyFiles() = %#v, want %#v", got, want)
	}
}

func TestPlanResolveOrdersAndPredictsVersions(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")
	runGit(t, root, "tag", "beta/v0.2.0")

	// A commit to alpha earns it a release, which beta then has to pin to.
	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// Name is the module name.\nconst Name = \"alpha\"\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Name")

	chdir(t, root)
	modules := []moduleInfo{
		{Name: "example.com/beta", Path: "./beta", Uses: []string{"example.com/alpha"}},
		{Name: "example.com/alpha", Path: "./alpha"},
	}
	refs := versionRefs{"example.com/beta": {"example.com/alpha": "v0.1.0"}}

	plans, cycles := planResolve(modules, refs)
	if len(cycles) != 0 {
		t.Fatalf("planResolve() cycles = %#v, want none", cycles)
	}
	if len(plans) != 2 {
		t.Fatalf("planResolve() = %d plans, want 2", len(plans))
	}

	alpha, beta := plans[0], plans[1]
	if alpha.Module != "example.com/alpha" {
		t.Fatalf("planResolve() ordered %q first, want the dependency", alpha.Module)
	}
	if alpha.Latest != "v0.1.0" || alpha.TagPrefix != "alpha/" || alpha.Ahead != 1 {
		t.Errorf("alpha = {Latest: %q, TagPrefix: %q, Ahead: %d}, want {v0.1.0, alpha/, 1}", alpha.Latest, alpha.TagPrefix, alpha.Ahead)
	}
	if alpha.Release != releasePatch || alpha.Next != "v0.1.1" {
		t.Errorf("alpha = {Release: %q, Next: %q}, want {patch, v0.1.1}", alpha.Release, alpha.Next)
	}

	// beta has no commits of its own, and is released only because the
	// version it requires moved.
	if beta.Ahead != 0 {
		t.Errorf("beta.Ahead = %d, want 0", beta.Ahead)
	}
	want := []requireInfo{{path: "example.com/alpha", version: "v0.1.1"}}
	if !reflect.DeepEqual(beta.Pins, want) {
		t.Errorf("beta.Pins = %#v, want %#v", beta.Pins, want)
	}
	if beta.Next != "v0.2.1" {
		t.Errorf("beta.Next = %q, want v0.2.1", beta.Next)
	}
}

func TestPlanResolveOffersAReleasedModuleTheUpdateAnyway(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	chdir(t, root)
	plans, _ := planResolve([]moduleInfo{
		{Name: "example.com/alpha", Path: "./alpha"},
		{Name: "example.com/beta", Path: "./beta"},
	}, versionRefs{})

	// alpha has no commits and nothing in the workspace to move, but its
	// dependencies outside the workspace can still have moved, so it is
	// offered the update and released only if that rewrites go.mod.
	alpha := plans[0]
	if alpha.Skip != "" {
		t.Errorf("a released module was skipped: %q", alpha.Skip)
	}
	if !alpha.Conditional {
		t.Error("a released module with no commits was not made conditional")
	}
	if alpha.Release != releasePatch || alpha.Next != "v0.1.1" {
		t.Errorf("alpha = {Release: %q, Next: %q}, want {patch, v0.1.1}", alpha.Release, alpha.Next)
	}

	// An untagged module has nothing downstream can pin to, so it is left
	// alone.
	if !strings.Contains(plans[1].Skip, "no release tag") {
		t.Errorf("an untagged module was not skipped: %q", plans[1].Skip)
	}
}

func TestReleaseKind(t *testing.T) {
	tests := []struct {
		name string
		in   resolvePlan
		want string
	}{
		{
			name: "taking API away costs a minor",
			in:   resolvePlan{API: apiDiff{Breaking: true}},
			want: releaseMinor,
		},
		{
			name: "adding API is a patch",
			in:   resolvePlan{API: apiDiff{Added: []apiSymbol{{Key: "x.A"}}}},
			want: releasePatch,
		},
		{
			name: "a dependency update alone is a patch",
			in:   resolvePlan{Conditional: true},
			want: releasePatch,
		},
	}

	for _, test := range tests {
		if got := releaseKind(test.in); got != test.want {
			t.Errorf("%s: releaseKind() = %q, want %q", test.name, got, test.want)
		}
	}
}
