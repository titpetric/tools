package tests_test

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/fix"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// The corpus is the Go module cache, which is every dependency this machine
// has ever downloaded: thousands of modules written by people who never heard
// of this formatter, in every shape a Go file comes in.
//
// It is what a formatter has to be tested against. A fixture holds the cases
// somebody thought of; the cache holds the ones nobody did.
const (
	// corpusModules is how many modules a run reads, newest path order first.
	// SPLINT_CORPUS overrides it, and 0 reads every module in the cache.
	corpusModules = 300

	// corpusEnv names the override.
	corpusEnv = "SPLINT_CORPUS"
)

// TestCorpusDoesNotCorruptSource runs the fixer over the module cache and
// checks that nothing but the import declarations changed.
//
// Nothing is written. The plan is applied in memory and the result is compared
// against the source it came from on the two things a formatter must never
// touch: the file still parses, and every declaration that is not an import
// prints exactly as it did before. A rewrite that deleted a function, moved a
// brace or truncated a file fails on the second.
//
// The imports are checked separately, against what the plan said it would add
// and remove. An import path appearing or vanishing without the plan saying so
// is a file that no longer builds.
func TestCorpusDoesNotCorruptSource(t *testing.T) {
	cache := modcache(t)

	modules := corpus(t, cache, corpusLimit(t))
	if len(modules) == 0 {
		t.Skip("the module cache holds no modules")
	}

	var files, rewritten, skipped int

	for _, dir := range modules {
		module := modulePath(dir)
		if module == "" {
			continue
		}

		plan, err := planCorpus(dir, module)
		if err != nil {
			// A module the parser cannot read is not a formatter defect. The
			// cache holds modules for other toolchains and other languages.
			skipped++
			continue
		}

		for _, file := range plan.Files {
			source, err := os.ReadFile(file.Path)
			if err != nil {
				continue
			}
			files++

			out, ok := fix.Splice(string(source), file.Edits)
			if !ok {
				// The edit did not land on an import declaration and nothing
				// would have been written. That is the guard doing its job.
				continue
			}
			if out == string(source) {
				continue
			}
			rewritten++

			checkFile(t, module, file, string(source), out)
		}
	}

	t.Logf("%d modules read, %d skipped, %d files rewritten of %d planned", len(modules), skipped, rewritten, files)

	if rewritten == 0 {
		t.Error("the corpus rewrote nothing, so it checked nothing")
	}
}

// checkFile compares a file against its rewrite.
func checkFile(t *testing.T, module string, file fix.FileFix, before, after string) {
	t.Helper()

	where := module + " " + file.Name

	beforeDecls, beforeImports, err := shape(before)
	if err != nil {
		// A file that did not parse before is one this cannot judge.
		return
	}

	afterDecls, afterImports, err := shape(after)
	if err != nil {
		t.Errorf("%s: the rewrite does not parse: %v", where, err)
		return
	}

	if beforeDecls != afterDecls {
		t.Errorf("%s: the rewrite changed something that is not an import%s", where, firstDifference(beforeDecls, afterDecls))
		return
	}

	want := expected(beforeImports, file.Added, file.Removed)
	if !equal(want, afterImports) {
		t.Errorf("%s: the imports are not what the plan said\n  before: %v\n  added: %v\n  removed: %v\n  want: %v\n  got: %v",
			where, beforeImports, file.Added, file.Removed, want, afterImports)
	}
}

// firstDifference is where two renderings part, with a little either side, so
// a failure names a line rather than printing two files.
func firstDifference(before, after string) string {
	at := 0
	for at < len(before) && at < len(after) && before[at] == after[at] {
		at++
	}

	window := func(in string) string {
		from := max(at-60, 0)
		to := min(at+60, len(in))
		return strings.ReplaceAll(in[from:to], "\n", "\\n")
	}

	return "\n  at byte " + strconv.Itoa(at) + "\n  before: " + window(before) + "\n  after:  " + window(after)
}

// shape is what a file holds: every declaration that is not an import, printed
// so two files can be compared on them, and the imports as "name path" pairs.
//
// The name is the one the file reaches the import by, which is the alias when
// there is one and the last segment of the path when there is not. That pair
// is what decides whether the file compiles: an alias added to a versioned
// path, or a redundant one taken off, changes how the import is written and
// not what it is called.
func shape(source string) (string, []string, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "x.go", source, parser.ParseComments)
	if err != nil {
		return "", nil, err
	}

	out := &strings.Builder{}
	out.WriteString("package " + file.Name.Name + "\n")

	var imports []string
	for _, node := range file.Decls {
		if decl, ok := node.(*ast.GenDecl); ok && decl.Tok == token.IMPORT {
			for _, spec := range decl.Specs {
				imported, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}
				path, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					path = imported.Path.Value
				}
				spec := model.ImportSpec{Path: path}
				if imported.Name != nil {
					spec.Name = imported.Name.Name
				}
				imports = append(imports, spec.Ref()+" "+spec.Path)
			}
			continue
		}

		var printed bytes.Buffer
		if err := printer.Fprint(&printed, fset, node); err != nil {
			return "", nil, err
		}
		out.Write(printed.Bytes())
		out.WriteString("\n")
	}

	sort.Strings(imports)
	return out.String(), imports, nil
}

// expected is the import set a rewrite should leave: what was there, without
// what the plan removed, with what it added.
func expected(before []string, added, removed []string) []string {
	drop := map[string]bool{}
	for _, one := range removed {
		drop[literal(one)] = true
	}

	seen := map[string]bool{}
	var out []string

	for _, one := range before {
		if drop[one] || seen[one] {
			continue
		}
		seen[one] = true
		out = append(out, one)
	}
	for _, one := range added {
		if entry := literal(one); !seen[entry] {
			seen[entry] = true
			out = append(out, entry)
		}
	}

	sort.Strings(out)
	return out
}

// literal turns the quoted form a plan reports, `tea "charm.land/x/v2"`, into
// the "name path" pair shape produces.
func literal(in string) string {
	in = strings.TrimSpace(in)

	open := strings.IndexByte(in, '"')
	if open < 0 {
		return in
	}

	path, err := strconv.Unquote(in[open:])
	if err != nil {
		return in
	}

	spec := model.ImportSpec{Name: strings.TrimSpace(in[:open]), Path: path}
	return spec.Ref() + " " + spec.Path
}

// equal reports two sorted lists holding the same entries.
func equal(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// planCorpus reads one module and works out what the fixer would write in it.
func planCorpus(dir, module string) (*fix.Plan, error) {
	options := splint.Options{
		SourcePath:     dir,
		Pattern:        "./...",
		IncludeTests:   true,
		IncludeImports: true,
	}

	root, err := simpleparser.New(options).Parse(context.Background())
	if err != nil {
		return nil, err
	}

	// The sorting keys come from the module itself: its own path is the
	// project group, and every module of the cache is read under its own.
	settings := config.Default()
	settings.Imports.Project = module

	return fix.Build(root, settings.ImportOptions(module), nil), nil
}

// modulePath is what a module calls itself, read from its go.mod.
func modulePath(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	return modfile.ModulePath(data)
}

// modcache is the directory the module cache sits in.
func modcache(t *testing.T) string {
	t.Helper()

	cache := os.Getenv("GOMODCACHE")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no GOMODCACHE and no home directory")
		}
		cache = filepath.Join(home, "go", "pkg", "mod")
	}

	if _, err := os.Stat(cache); err != nil {
		t.Skipf("no module cache at %s", cache)
	}

	return cache
}

// corpusLimit is how many modules a run reads.
func corpusLimit(t *testing.T) int {
	t.Helper()

	value := os.Getenv(corpusEnv)
	if value == "" {
		return corpusModules
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("%s=%q is not a number", corpusEnv, value)
	}

	return limit
}

// corpus are the module directories of the cache, in path order, up to limit.
//
// A module is a directory holding a go.mod. The cache nests them under the
// host and the path, and the download cache under it holds zips rather than
// source, so that one is left out.
func corpus(t *testing.T, cache string, limit int) []string {
	t.Helper()

	var found []string

	walk(cache, 0, func(dir string) bool {
		if filepath.Base(dir) == "cache" || filepath.Base(dir) == "sumdb" {
			return false
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			found = append(found, dir)
			return false
		}
		return limit <= 0 || len(found) < limit
	})

	sort.Strings(found)
	if limit > 0 && len(found) > limit {
		found = found[:limit]
	}

	return found
}

// walk visits every directory under root, to a depth the cache never exceeds,
// and stops descending where visit says to.
func walk(root string, depth int, visit func(string) bool) {
	if depth > 5 {
		return
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if !visit(dir) {
			continue
		}
		walk(dir, depth+1, visit)
	}
}
