package main

import (
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/worktree/config"
)

func TestParseIgnoreRuleSkipsBlanksAndComments(t *testing.T) {
	for _, line := range []string{"", "   ", "# comment", "/"} {
		if _, ok := parseIgnoreRule(line); ok {
			t.Fatalf("parseIgnoreRule(%q) returned a rule", line)
		}
	}
}

func TestIgnoreRuleMatch(t *testing.T) {
	tests := []struct {
		pattern string
		rel     string
		isDir   bool
		want    bool
	}{
		{pattern: "vendor", rel: "vendor", isDir: true, want: true},
		{pattern: "vendor", rel: "apps/vendor", isDir: true, want: true},
		{pattern: "vendor", rel: "vendor", want: true},
		{pattern: "vendor/", rel: "vendor", isDir: true, want: true},
		{pattern: "vendor/", rel: "vendor", want: false},
		{pattern: "/vendor", rel: "vendor", isDir: true, want: true},
		{pattern: "/vendor", rel: "apps/vendor", isDir: true, want: false},
		{pattern: "apps/vendor", rel: "apps/vendor", isDir: true, want: true},
		{pattern: "apps/vendor", rel: "src/apps/vendor", isDir: true, want: false},
		{pattern: "*.cov", rel: "coverage.cov", want: true},
		{pattern: "*.cov", rel: "apps/coverage.cov", want: true},
		{pattern: "apps/*/build", rel: "apps/booking/build", isDir: true, want: true},
		{pattern: "apps/*/build", rel: "apps/a/b/build", isDir: true, want: false},
		{pattern: "apps/**/build", rel: "apps/a/b/build", isDir: true, want: true},
		{pattern: "**/build", rel: "apps/build", isDir: true, want: true},
		{pattern: "build/**", rel: "build", isDir: true, want: false},
		{pattern: "build/**", rel: "build/out", want: true},
		{pattern: `\#temp`, rel: "#temp", want: true},
	}

	for _, test := range tests {
		rule, ok := parseIgnoreRule(test.pattern)
		if !ok {
			t.Fatalf("parseIgnoreRule(%q) returned no rule", test.pattern)
		}
		if got := rule.match(test.rel, test.isDir); got != test.want {
			t.Errorf("parseIgnoreRule(%q).match(%q, %v) = %v, want %v", test.pattern, test.rel, test.isDir, got, test.want)
		}
	}
}

func TestIgnoreStackInnermostFileWins(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "build\n*.cov\n")
	writeTestFile(t, filepath.Join(root, "apps", ".gitignore"), "!build\n")

	stack := ignoreStack(nil).push(root)
	if !stack.ignored(filepath.Join(root, "build"), true) {
		t.Fatal("ignoreStack did not ignore a directory matched by the root .gitignore")
	}
	if !stack.ignored(filepath.Join(root, "apps", "unit.cov"), false) {
		t.Fatal("ignoreStack did not ignore a nested file matched by the root .gitignore")
	}

	stack = stack.push(filepath.Join(root, "apps"))
	if stack.ignored(filepath.Join(root, "apps", "build"), true) {
		t.Fatal("ignoreStack did not honor the negation in the nested .gitignore")
	}
	if !stack.ignored(filepath.Join(root, "apps", "unit.cov"), false) {
		t.Fatal("nested .gitignore without a matching rule dropped the root rules")
	}
}

func TestIgnoreStackPruneUnwindsToSibling(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "apps", ".gitignore"), "build\n")

	stack := ignoreStack(nil).push(root).push(filepath.Join(root, "apps"))
	if got := len(stack); got != 1 {
		t.Fatalf("len(ignoreStack) = %d, want 1", got)
	}

	sibling := filepath.Join(root, "libs", "build")
	stack = stack.prune(sibling)
	if len(stack) != 0 {
		t.Fatalf("prune() kept %d files, want 0", len(stack))
	}
	if stack.ignored(sibling, true) {
		t.Fatal("ignoreStack applied the apps/.gitignore to a sibling directory")
	}
}

// TestScannerAppliesIgnorePathsWithoutGitignore checks a configured ignore
// path excludes a directory whether or not the .gitignore rules are on, and
// that the root of the walk is never excluded.
func TestScannerAppliesIgnorePathsWithoutGitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "build\n")

	scan := config.Scan{IgnorePaths: []string{"node_modules"}}
	s := newScanner(scan, root)

	if s.skip(root, true) {
		t.Fatal("scanner excluded the root of the walk")
	}
	if !s.skip(filepath.Join(root, "node_modules"), true) {
		t.Fatal("scanner did not exclude a configured ignore path")
	}
	if s.skip(filepath.Join(root, "build"), true) {
		t.Fatal("scanner applied a .gitignore rule with the rules turned off")
	}
	// An ignore path names a directory, so a file of that name is kept.
	if s.skip(filepath.Join(root, "node_modules"), false) {
		t.Fatal("scanner excluded a file matching an ignore path")
	}
}

// TestScannerSkipsTestdata checks a testdata directory is excluded without
// being configured, and that a walk started inside one still runs.
func TestScannerSkipsTestdata(t *testing.T) {
	root := t.TempDir()
	s := newScanner(config.Scan{}, root)

	if !s.skip(filepath.Join(root, "testdata"), true) {
		t.Fatal("scanner descended into a testdata directory")
	}
	if !s.skip(filepath.Join(root, "apps", "testdata"), true) {
		t.Fatal("scanner descended into a nested testdata directory")
	}
	// The name only excludes a directory, so a file called testdata is kept.
	if s.skip(filepath.Join(root, "testdata"), false) {
		t.Fatal("scanner excluded a file named testdata")
	}

	inside := newScanner(config.Scan{}, filepath.Join(root, "testdata"))
	if inside.skip(filepath.Join(root, "testdata"), true) {
		t.Fatal("scanner excluded the root of the walk")
	}
}

// TestScannerAppliesGitignoreWhenEnabled checks the .gitignore rules apply
// with the setting on, and that an ignore path still wins over a negation.
func TestScannerAppliesGitignoreWhenEnabled(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "build\n!vendor\n")

	scan := config.Scan{EnableGitignore: true, IgnorePaths: []string{"vendor"}}
	s := newScanner(scan, root)

	if s.skip(root, true) {
		t.Fatal("scanner excluded the root of the walk")
	}
	if !s.skip(filepath.Join(root, "build"), true) {
		t.Fatal("scanner did not apply a .gitignore rule")
	}
	if !s.skip(filepath.Join(root, "vendor"), true) {
		t.Fatal("an ignore path did not override the negation re-including it")
	}
	if s.skip(filepath.Join(root, "apps"), true) {
		t.Fatal("scanner excluded a directory no rule names")
	}
}
