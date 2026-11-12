// Package checker runs a rule file over a directory tree.
package checker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/titpetric/tools/ruleslint/model"
)

// Runner checks a tree: where the tree is, and where a linter writes.
type Runner struct {
	Root string
	Out  io.Writer
}

// Run checks the tree against cfg, and refuses a root that is not a directory.
func (r Runner) Run(cfg *model.Config) (model.Checks, error) {
	info, err := os.Stat(r.Root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", r.Root)
	}

	var checks model.Checks
	for _, dir := range cfg.Dirs {
		checks = append(checks, r.dir(dir)...)
	}
	return append(checks, r.lint(cfg.Linters)...), nil
}

// dir checks every directory a dir block resolves to against its entries.
func (r Runner) dir(block *model.Dir) model.Checks {
	matches := expand(r.Root, ".", block.Pattern, nil)
	var dirs []match
	for _, one := range matches {
		if one.dir {
			dirs = append(dirs, one)
		}
	}

	if len(dirs) == 0 && !free(block.Pattern, nil) {
		detail := "no such directory"
		if len(matches) > 0 {
			detail = "not a directory"
		}
		return model.Checks{{Line: block.Line, Path: block.Pattern, Rule: "dir " + block.Pattern, Detail: detail}}
	}

	var checks model.Checks
	for _, one := range dirs {
		for _, entry := range block.Entries {
			checks = append(checks, r.entry(entry, one)...)
		}
	}
	return checks
}

// entry checks one path pattern under a dir block, and the text its matches hold.
func (r Runner) entry(entry *model.Entry, parent match) model.Checks {
	pattern := subst(entry.Pattern, parent.binds)
	matches := expand(r.Root, parent.path, entry.Pattern, parent.binds)

	var checks model.Checks
	if !free(entry.Pattern, parent.binds) {
		one := model.Check{Line: entry.Line, Path: parent.path, Rule: pattern, OK: len(matches) > 0}
		if !one.OK {
			one.Detail = "missing " + pattern
		}
		checks = append(checks, one)
	}

	for _, file := range matches {
		for _, rule := range entry.Rules {
			checks = append(checks, r.rule(rule, file))
		}
	}
	return checks
}

// rule reads a matched file and reports whether it holds what require names and holds nothing deny names.
func (r Runner) rule(rule model.Rule, file match) model.Check {
	text := subst(rule.Text, file.binds)
	one := model.Check{Line: rule.Line, Path: file.path, Rule: rule.Key() + ": '" + text + "'"}

	if file.dir {
		one.Detail = "is a directory and holds no text"
		return one
	}

	data, err := os.ReadFile(filepath.Join(r.Root, file.path))
	switch found := holds(string(data), text); {
	case err != nil:
		one.Detail = err.Error()
	case rule.Deny && found:
		one.Detail = "holds '" + text + "'"
	case rule.Deny || found:
		one.OK = true
	default:
		one.Detail = "does not hold '" + text + "'"
	}
	return one
}

// lint runs the commands of the linters block, each in the root of the tree.
func (r Runner) lint(linters []model.Linter) model.Checks {
	var checks model.Checks
	for _, linter := range linters {
		cmd := exec.Command("sh", "-c", linter.Command)
		cmd.Dir = r.Root
		cmd.Stdout, cmd.Stderr = r.Out, r.Out

		one := model.Check{Line: linter.Line, Rule: linter.Command}
		if err := cmd.Run(); err != nil {
			one.Detail = linter.Command + ": " + err.Error()
		} else {
			one.OK = true
		}
		checks = append(checks, one)
	}
	return checks
}
