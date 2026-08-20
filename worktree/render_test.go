package main

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/worktree/components"
)

func TestLatestGoVersion(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		want     string
		ok       bool
	}{
		{"picks highest", []string{"1.24", "1.27", "1.25.1"}, "1.27.0", true},
		{"compares numerically", []string{"1.9", "1.10"}, "1.10.0", true},
		{"patch wins", []string{"1.27", "1.27.3"}, "1.27.3", true},
		{"rc beats prior release", []string{"1.27rc1", "1.26"}, "1.27.0-rc1", true},
		{"release beats its rc", []string{"1.27rc1", "1.27"}, "1.27.0", true},
		{"skips unparseable", []string{"", "tip", "1.25"}, "1.25.0", true},
		{"no versions", []string{"", "tip"}, "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var modules []moduleInfo
			for _, v := range test.versions {
				modules = append(modules, moduleInfo{GoVersion: v})
			}
			got, ok := latestGoVersion(modules)
			if ok != test.ok {
				t.Fatalf("latestGoVersion(%v) ok = %v, want %v", test.versions, ok, test.ok)
			}
			if ok && got.String() != test.want {
				t.Errorf("latestGoVersion(%v) = %q, want %q", test.versions, got, test.want)
			}
		})
	}
}

func TestGoVersionOutdated(t *testing.T) {
	latest, ok := ParseGoDirective("1.27")
	if !ok {
		t.Fatal("ParseGoDirective() failed")
	}

	tests := []struct {
		version string
		want    bool
	}{
		{"1.24", true},
		{"1.9", true},
		{"1.26.9", true},
		{"1.27", false},
		{"1.27.0", false},
		{"1.28", false},
		{"1.27rc1", true},
		{"tip", false},
		{"", false},
	}

	for _, test := range tests {
		if got := goVersionOutdated(test.version, latest); got != test.want {
			t.Errorf("goVersionOutdated(%q, 1.27) = %v, want %v", test.version, got, test.want)
		}
	}
}

// TestRenderTablesColorsOutdatedGoVersions checks that the go column of a
// module below the workspace's highest go directive renders amber, while the
// module holding that version keeps the default colour.
func TestRenderTablesColorsOutdatedGoVersions(t *testing.T) {
	modules := []moduleInfo{
		{Name: "example.com/old", Path: "./old", GoVersion: "1.24", GitState: &components.Git{}},
		{Name: "example.com/new", Path: "./new", GoVersion: "1.27", GitState: &components.Git{}},
	}

	var out strings.Builder
	renderTables(&out, modules, &Options{All: true}, true)

	for _, want := range []string{
		components.ColorAmber + "1.24" + components.ColorReset,
		components.ColorTeal + "1.27" + components.ColorReset,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("renderTables() output missing %q:\n%s", want, out.String())
		}
	}
}

// TestRenderTablesStripsModuleHost checks that the module column drops the
// "github.com/" prefix of an import path.
func TestRenderTablesStripsModuleHost(t *testing.T) {
	modules := []moduleInfo{
		{Name: "github.com/titpetric/tools", Path: "./tools", GitState: &components.Git{}},
	}

	var out strings.Builder
	renderTables(&out, modules, &Options{All: true, Verbose: true}, false)

	if strings.Contains(out.String(), "github.com/") {
		t.Errorf("renderTables() kept the github.com/ prefix:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "titpetric/tools") {
		t.Errorf("renderTables() output missing the module path:\n%s", out.String())
	}
}
