package analyzer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

type Module struct {
	Filename   string
	Dir        string
	Path       string
	ImportPath string

	Valid bool
	Error error
}

func (m Module) String() string {
	if m.Error != nil {
		return fmt.Sprintf("%v (dir: %v, valid: %v, error: %v)", m.ImportPath, m.Dir, m.Valid, m.Error)
	}
	return fmt.Sprintf("%v (dir: %v, valid: %v)", m.ImportPath, m.Dir, m.Valid)
}

// ListModules finds all go.mod files under root and returns a slice of Modules.
func ListModules(root string, pattern string) ([]Module, error) {
	if root == "" {
		root = "."
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var modules []Module
	err = filepath.WalkDir(absRoot, func(filename string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// testdata is a module the toolchain ignores, and a fixture module
		// under it is not the code being reported on. The root is read
		// whatever it is called: a fixture is reached by being pointed at,
		// and skipping the thing that was asked for reads nothing at all.
		if d.IsDir() && filename != absRoot && skipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.Name() != "go.mod" {
			return nil
		}
		if pattern == "./..." || (pattern == "." && len(modules) == 0) {
			modules = append(modules, parseGoMod(filename, absRoot))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return modules, nil
}

// parseGoMod reads the go.mod file using x/mod/modfile
func parseGoMod(filename string, rootPath string) (result Module) {
	dir := filepath.Dir(filename)
	cleanDir := strings.TrimPrefix(dir, rootPath)

	result.Filename = filename
	result.Dir = dir
	result.Path = cleanDir

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		result.Error = err
		return
	}

	f, err := modfile.Parse(filename, data, nil)
	if err != nil {
		result.Error = err
		return
	}

	if f.Module == nil || f.Module.Mod.Path == "" {
		result.Error = errors.New("module declaration not found")
		return
	}

	importPath := f.Module.Mod.Path
	result.ImportPath = importPath

	if !strings.Contains(importPath, ".") {
		result.Error = errors.New("module not importable")
		return
	}

	if cleanDir != "" && !strings.HasSuffix(importPath, cleanDir) {
		result.Error = fmt.Errorf("invalid import path: %s, want suffix: %s", importPath, cleanDir)
		return
	}

	result.Valid = true
	return
}

// skipDir reports a directory no walk should descend into.
//
// Version control and tooling keep their own trees under a dot, a vendor
// directory is somebody else's code, and testdata is what the toolchain itself
// ignores. None of them is the code a report is about.
func skipDir(name string) bool {
	switch name {
	case "vendor", "testdata", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
