package main

import (
	"fmt"
	"go/version"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/titpetric/tools/worktree/components"
	"github.com/titpetric/tools/worktree/config"
)

// parseGoVersion validates a --go flag value and returns it in the form used
// by the go directive: "1.27" or "1.27.1". A leading "go" is accepted, so
// both "1.27" and "go1.27" are valid.
func parseGoVersion(value string) (string, error) {
	goVersion := strings.TrimPrefix(strings.TrimSpace(value), "go")
	if !modfile.GoVersionRE.MatchString(goVersion) {
		return "", fmt.Errorf("invalid go version %q", value)
	}
	return goVersion, nil
}

// goVersionChange formats a go directive change as a status line.
func goVersionChange(before, after string) string {
	if before == "" {
		return "+ go " + after
	}
	return "go " + before + " → " + after
}

// setGoVersion rewrites the go directive of the go.mod in dir, returning the
// version it replaced. The file is left untouched when it already declares
// goVersion.
func setGoVersion(dir, goVersion string) (string, error) {
	path := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", err
	}

	var before string
	if mod.Go != nil {
		before = mod.Go.Version
	}
	if before == goVersion {
		return before, nil
	}
	if err := mod.AddGoStmt(goVersion); err != nil {
		return before, err
	}
	if staleToolchain(mod.Toolchain, goVersion) {
		mod.DropToolchainStmt()
	}
	mod.Cleanup()
	return before, writeModFile(path, mod.Syntax)
}

// setGoWorkVersion rewrites the go directive of the go.work file at path,
// returning the version it replaced.
func setGoWorkVersion(path, goVersion string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	work, err := modfile.ParseWork(path, data, nil)
	if err != nil {
		return "", err
	}

	var before string
	if work.Go != nil {
		before = work.Go.Version
	}
	if before == goVersion {
		return before, nil
	}
	if err := work.AddGoStmt(goVersion); err != nil {
		return before, err
	}
	if staleToolchain(work.Toolchain, goVersion) {
		work.DropToolchainStmt()
	}
	work.Cleanup()
	return before, writeModFile(path, work.Syntax)
}

// staleToolchain reports whether a toolchain directive is older than
// goVersion, which would leave the file invalid. Go commands add a newer
// toolchain back when they need one.
func staleToolchain(toolchain *modfile.Toolchain, goVersion string) bool {
	return toolchain != nil && version.Compare(toolchain.Name, "go"+goVersion) < 0
}

// writeModFile formats a parsed go.mod or go.work file back to path, keeping
// the file mode it had.
func writeModFile(path string, syntax *modfile.FileSyntax) error {
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return os.WriteFile(path, modfile.Format(syntax), perm)
}

// findGoWorkFiles returns the go.work files under root. It applies the same
// scan rules as the workspace listing, so the files it rewrites are the ones
// the table showed.
func findGoWorkFiles(root string, scan config.Scan) []string {
	var files []string
	s := newScanner(scan, root)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if s.skip(path, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "go.work" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// updateGoWorkVersions sets the go directive of every go.work file under root,
// reporting the files it changed.
func updateGoWorkVersions(w io.Writer, root, goVersion string, scan config.Scan, styled bool) error {
	for _, path := range findGoWorkFiles(root, scan) {
		before, err := setGoWorkVersion(path, goVersion)
		if err != nil {
			return fmt.Errorf("failed to update %s: %w", relPath(path), err)
		}
		if before == goVersion {
			continue
		}
		line := relPath(path) + ": " + goVersionChange(before, goVersion)
		fmt.Fprintln(w, colorLines(line, components.ColorAmber, styled))
	}
	return nil
}
