package test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// dirIndex is what the inventory knows of one package directory.
type dirIndex struct {
	Dir        string
	ImportPath string
	Tests      map[string]*testFunc
	Funcs      map[string][]*funcDecl // every function of the directory, by name
}

// inventory is every test the tree holds.
type inventory struct {
	Dirs map[string]*dirIndex
}

// inventory reads the test files under the source path, and the production
// files of a directory a change touched.
func (s *source) inventory(ctx context.Context, recursive bool, changed map[string]bool) (*inventory, error) {
	inv := &inventory{Dirs: map[string]*dirIndex{}}

	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == s.root {
				return nil
			}
			name := entry.Name()
			if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return fs.SkipDir
			}
			if !recursive {
				return fs.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(rel)
		test := strings.HasSuffix(path, "_test.go")
		// A production file is read only where a change touched the
		// directory: elsewhere the run needs the tests alone, which name the
		// symbol a selection is looking for.
		if !test && !changed[dir] {
			return nil
		}

		return s.addFile(inv, path, rel, dir, test)
	})
	if err != nil {
		return nil, err
	}

	return inv, nil
}

// addFile folds one file into the inventory.
func (s *source) addFile(inv *inventory, path, rel, dir string, test bool) error {
	parsed, err := parseFile(path, rel, dir)
	if err != nil {
		return err
	}

	index := inv.Dirs[dir]
	if index == nil {
		index = &dirIndex{
			Dir:        dir,
			ImportPath: s.importPath(dir),
			Tests:      map[string]*testFunc{},
			Funcs:      map[string][]*funcDecl{},
		}
		inv.Dirs[dir] = index
	}

	for _, one := range parsed.Funcs {
		index.Funcs[one.Name] = append(index.Funcs[one.Name], one)
		if one.Kind == kindFunc {
			continue
		}
		tf := &testFunc{
			Dir:       one.Dir,
			Name:      one.Name,
			Kind:      one.Kind,
			Refs:      one.Refs,
			Calls:     one.Calls,
			Qualified: one.Qualified,
			Imports:   map[string]bool{},
		}
		for _, path := range parsed.Imports {
			tf.Imports[path] = true
		}
		index.Tests[one.Name] = tf
	}
	return nil
}

// importPath names the package a directory holds.
//
// A directory under a module is its module path and its place below it. A
// directory outside a module has no import path, which leaves a cross-package
// reference to a name alone.
func (s *source) importPath(dir string) string {
	abs := filepath.Join(s.root, dir)
	for d := abs; ; d = filepath.Dir(d) {
		mod := filepath.Join(d, "go.mod")
		if data, err := os.ReadFile(mod); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					path := strings.TrimSpace(rest)
					rel, err := filepath.Rel(d, abs)
					if err != nil || rel == "." {
						return path
					}
					return path + "/" + filepath.ToSlash(rel)
				}
			}
		}
		if parent := filepath.Dir(d); parent == d {
			return ""
		}
	}
}

// upperFirst spells a name the way a test refers to it.
func upperFirst(name string) string {
	if name == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(r)) + name[size:]
}
