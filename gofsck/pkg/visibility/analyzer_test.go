package visibility

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestAnalyzer_Analyze(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedModule,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, ".")
	assert.NoError(t, err)
	assert.NotEmpty(t, pkgs)

	report, err := New().Analyze(pkgs)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Len(t, report.Packages, 1, "the test package is counted with the package it tests")

	assert.Equal(t, 1, report.Total)

	got := report.Packages[0]
	assert.Equal(t, "./pkg/visibility", got.Package)
	assert.Positive(t, got.Exported(), "New, Analyze and the report type are exported")
	assert.Positive(t, got.Internal(), "countFile and its neighbours are not")
	assert.Positive(t, got.Lines)
	assert.Less(t, got.InternalLines, got.Lines)
}

// countSource returns the counts of one source file, which is how the rules
// below are stated without a package on disk.
func countSource(t *testing.T, src string) PackageReport {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", src, parser.ParseComments)
	assert.NoError(t, err)

	blank := make(map[int]bool)
	for i, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "" {
			blank[i+1] = true
		}
	}

	var report PackageReport
	countFile(&report, fset, file, blank)
	return report
}

func TestCountFileCountsByTheNameOfTheSymbol(t *testing.T) {
	// A method counts as a func, and by the case of its own name: the
	// receiver being unexported says nothing about the method.
	got := countSource(t, `package p

type Tracer struct{}
type broker struct{}

func New() *Tracer { return nil }
func (t *Tracer) Observe() {}
func (t *Tracer) serveHTTP() {}
func (b *broker) notify() {}
func envValue(name string) string { return name }
`)

	assert.Equal(t, 1, got.ExportedTypes)
	assert.Equal(t, 1, got.InternalTypes)
	assert.Equal(t, 2, got.ExportedFuncs)
	assert.Equal(t, 3, got.InternalFuncs)
	assert.Equal(t, 3, got.Exported())
	assert.Equal(t, 4, got.Internal())
}

func TestCountFileLeavesOutBlanksAndComments(t *testing.T) {
	got := countSource(t, `package p

// helper does something.
func helper() int {
	// a comment inside the body

	return 1
}
`)

	// The body spans the brace, the return and the closing brace; the file
	// adds the package clause to them.
	assert.Equal(t, 3, got.InternalLines)
	assert.Equal(t, 4, got.Lines)
}

func TestPackageLabel(t *testing.T) {
	module := &packages.Module{Path: "example.com/x"}

	tests := map[string]string{
		"example.com/x":            "./",
		"example.com/x/pkg/inner":  "./pkg/inner",
		"example.com/other/vendor": "./vendor",
	}
	for pkgPath, want := range tests {
		got := packageLabel(&packages.Package{PkgPath: pkgPath, Module: module})
		assert.Equal(t, want, got, pkgPath)
	}

	assert.Equal(t, "example.com/x", packageLabel(&packages.Package{PkgPath: "example.com/x"}),
		"without module information the path is the label")
}

func TestSkipPackage(t *testing.T) {
	syntax := []*ast.File{{}}

	assert.True(t, skipPackage(&packages.Package{Name: "main", Syntax: syntax}))
	assert.True(t, skipPackage(&packages.Package{Name: "p_test", Syntax: syntax}))
	assert.True(t, skipPackage(&packages.Package{Name: "p", PkgPath: "example.com/x.test", Syntax: syntax}))
	assert.True(t, skipPackage(&packages.Package{Name: "p"}), "a package loaded without syntax is not measurable")
	assert.True(t, skipPackage(&packages.Package{Name: "internal", PkgPath: "example.com/x/internal", Syntax: syntax}),
		"an internal tree is scoped to the module whatever it exports")
	assert.True(t, skipPackage(&packages.Package{Name: "env", PkgPath: "example.com/x/internal/env", Syntax: syntax}))
	assert.False(t, skipPackage(&packages.Package{Name: "p", PkgPath: "example.com/x", Syntax: syntax}))
	assert.False(t, skipPackage(&packages.Package{Name: "p", PkgPath: "example.com/internalish", Syntax: syntax}),
		"the element has to be internal, not start with it")
}
