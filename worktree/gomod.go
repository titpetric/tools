package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// readToolchain returns the toolchain directive of the go.mod in dir, or ""
// when the file declares none.
func readToolchain(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}

	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil || mod.Toolchain == nil {
		return ""
	}

	return mod.Toolchain.Name
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
	return parseRequires(data)
}

// parseRequires reads the requirements of a go.mod held in memory; Diff
// compares a go.mod fetched from git, or read out of a fixture, this way.
func parseRequires(data []byte) ([]requireInfo, error) {
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	var reqs []requireInfo
	for _, r := range mod.Require {
		reqs = append(reqs, requireInfo{
			path:     r.Mod.Path,
			version:  r.Mod.Version,
			indirect: r.Indirect,
		})
	}
	return reqs, nil
}

// DiffResult is one go.mod dependency a release adds, moves to another
// version, or drops.
type DiffResult struct {
	// Path is the required module, the key Diff matches the two revisions
	// on.
	Path string

	// Change holds added, changed or removed, named the way the data model table
	// names what happens to a field.
	Change string

	// Old and New are the version required before and after. Old is empty for
	// a dependency the release adds, and New for one it drops.
	Old string
	New string
}

// Version renders the change the way the table lists it: the version, or the
// two either side of a move.
func (r DiffResult) Version() string {
	if r.Change == fieldChanged {
		return r.Old + " -> " + r.New
	}
	if r.Change == fieldRemoved {
		return r.Old
	}
	return r.New
}

// Diff reports what moved between two go.mod dependency sets, the older one
// first, in module path order.
//
// Diff leaves out indirect requirements. go mod tidy writes them and nobody
// picks them, and a routine tidy rewrites dozens of them, burying the handful
// of direct ones that form the actual release note. A dependency direct on
// either side counts as direct, so Diff reports one the module took on an
// import of either way.
func Diff(before, after []requireInfo) []DiffResult {
	old, cur := requireIndex(before), requireIndex(after)

	var changes []DiffResult
	for path, was := range old {
		is, held := cur[path]
		switch {
		case !held:
			if !was.indirect {
				changes = append(changes, DiffResult{Path: path, Change: fieldRemoved, Old: was.version})
			}
		case was.version != is.version && (!was.indirect || !is.indirect):
			changes = append(changes, DiffResult{
				Path: path, Change: fieldChanged, Old: was.version, New: is.version,
			})
		}
	}
	for path, is := range cur {
		if _, held := old[path]; held || is.indirect {
			continue
		}
		changes = append(changes, DiffResult{Path: path, Change: fieldAdded, New: is.version})
	}

	// The map walk follows whatever order the runtime hands out, so Diff
	// sorts the result to make two identical runs identical.
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Change != changes[j].Change {
			return categoryOrder(changes[i].Change) < categoryOrder(changes[j].Change)
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

// requireIndex keys a dependency list by module path, the key Diff compares
// two revisions of a go.mod through.
func requireIndex(reqs []requireInfo) map[string]requireInfo {
	index := make(map[string]requireInfo, len(reqs))
	for _, r := range reqs {
		index[r.path] = r
	}
	return index
}
