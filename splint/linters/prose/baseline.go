package prose

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Baseline is a corpus measured once and kept.
//
// The default is the Go standard library: two million words of comments by a
// team that writes Go for a living. A shape they use freely belongs to the
// language. One they never use belongs to the writer.
type Baseline struct {
	// Name is the tree the prose came from, and Size the gram length. A run
	// differing on either rebuilds.
	Name string `json:"name"`
	Size int    `json:"size"`

	// Counts and Density are the measure.
	Counts  *Counts `json:"counts"`
	Density Density `json:"density"`
}

// CachePath is where the kept measure goes: beside splint.yml, in the
// configuration directory for the current user.
func CachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "splint-prose.json"), nil
}

// LoadBaseline measures a tree, or reads the cache holding the same measure.
func LoadBaseline(path string, size int) (*Baseline, error) {
	file, pathErr := CachePath()
	if pathErr == nil {
		if held, err := readBaseline(file); err == nil && held.Name == path && held.Size == size {
			return held, nil
		}
	}

	corpus, err := readTree(path)
	if err != nil {
		return nil, err
	}

	baseline := &Baseline{
		Name:    path,
		Size:    size,
		Counts:  Count(corpus, size),
		Density: Measure(corpus),
	}

	if pathErr == nil {
		writeBaseline(file, baseline)
	}

	return baseline, nil
}

// readBaseline reads the kept measure.
func readBaseline(file string) (*Baseline, error) {
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	baseline := &Baseline{}
	if err := json.Unmarshal(body, baseline); err != nil {
		return nil, err
	}
	if baseline.Counts == nil || baseline.Counts.Words == 0 {
		return nil, fmt.Errorf("%s holds no counts", file)
	}

	return baseline, nil
}

// writeBaseline keeps the measure, and says nothing where it cannot: the run
// that wanted the baseline already holds one.
func writeBaseline(file string, baseline *Baseline) {
	body, err := json.Marshal(baseline)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return
	}
	os.WriteFile(file, body, 0o644)
}

// The trees whose prose answers to nobody here.
var baselineSkip = map[string]bool{
	"vendor":   true,
	"testdata": true,
	".git":     true,
}

// readTree reads the comment blocks of a tree of Go, and builds the baseline.
//
// It walks the files and skips the splint parsers. The standard library is no
// tree splint lints. A parse of it would resolve every package through the
// toolchain to reach comments the text already holds.
func readTree(root string) (*Corpus, error) {
	corpus := &Corpus{}

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if baselineSkip[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		readGo(corpus, root, path)
		return nil
	}

	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, err
	}
	if len(corpus.Blocks) == 0 {
		return nil, fmt.Errorf("%s holds no Go comments", root)
	}

	return corpus, nil
}

// readGo adds one file's comment blocks.
//
// A file that fails to parse drops out. The baseline is two million words and
// one unreadable file changes no rate it reports.
func readGo(corpus *Corpus, root, path string) {
	source, err := os.ReadFile(path)
	if err != nil {
		return
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil || generated(file) {
		return
	}

	name := path
	if rel, relErr := filepath.Rel(root, path); relErr == nil {
		name = filepath.ToSlash(rel)
	}

	for _, group := range file.Comments {
		text := strings.TrimSpace(group.Text())
		if text == "" || license(text) {
			continue
		}
		corpus.add(name, fset.Position(group.Pos()).Line, text, false)
	}
}

// generated reports whether a file says a program wrote it, in the form
// cmd/go states.
func generated(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, line := range group.List {
			if strings.HasPrefix(line.Text, "// Code generated ") && strings.HasSuffix(line.Text, " DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

// license reports whether a block is a copyright header. Every file of a
// large tree repeats the same one, and counting it measures the file count.
func license(text string) bool {
	return strings.HasPrefix(text, "Copyright ") || strings.HasPrefix(text, "Use of this source code")
}
