package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// parseGoWork returns relative paths listed under 'use' in go.work
func parseGoWork(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	work, err := modfile.ParseWork(file, data, nil)
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(work.Use))
	for _, use := range work.Use {
		dirs = append(dirs, use.Path)
	}
	return dirs, nil
}

func readModulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}

	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", err
	}

	if mod.Module == nil {
		return "", fmt.Errorf("no module declaration")
	}

	return mod.Module.Mod.Path, nil
}

// readGoVersion returns the go directive of the go.mod in dir, or "" when the
// directory has no readable go.mod or the file declares no go version.
func readGoVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}

	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil || mod.Go == nil {
		return ""
	}

	return mod.Go.Version
}

// isGoModule reports whether dir holds a go.mod.
func isGoModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func readReadmeTitle(dir string) string {
	f, err := os.Open(filepath.Join(dir, "README.md"))
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func readRequiresVersioned(dir string) ([]requireInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, err
	}

	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	var reqs []requireInfo
	for _, r := range mod.Require {
		reqs = append(reqs, requireInfo{
			path:    r.Mod.Path,
			version: r.Mod.Version,
		})
	}
	return reqs, nil
}
