// Package deprecated reports every use of a symbol whose doc comment marks it
// Deprecated, at the line the use is written on.
//
// The doc comment is the Go convention: a paragraph that opens
// "Deprecated:" and says what to use instead. The parse keeps the doc, so the
// linter builds the set of marked symbols first and then asks the reference
// index which declarations reach one. Each reaching file is read again with
// the AST, because the document records a reference against the declaration
// and not the line it sits on, and a reader removing a symbol needs the line.
package deprecated

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/refindex"
)

// Name selects the linter and labels its issues.
const Name = "deprecated"

// RuleUse is the one rule this linter reports under.
const RuleUse = "use"

// Linter reports a use of a symbol marked Deprecated.
type Linter struct{}

// New returns a linter that reads the Deprecated paragraph of every doc comment.
func New() *Linter {
	return &Linter{}
}

// Name selects the linter on a --linters list and labels its issues.
func (l *Linter) Name() string {
	return Name
}

// marked is one symbol a doc comment marked Deprecated.
type marked struct {
	key      refindex.Key
	reason   string
	position model.Position
}

// Lint reports every use of a symbol marked Deprecated in its doc comment.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	results := Results{}
	symbols := markedSymbols(root)
	if len(symbols) == 0 {
		return results, nil
	}

	index := refindex.Build(root)
	files := newSources(root)

	for _, symbol := range symbols {
		for _, ref := range index.Refs(symbol.key) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			from := ref.Decl.Position(ref.Pkg)
			uses := files.uses(symbol, ref, root.Root)
			if len(uses) == 0 {
				uses = []model.Position{from}
			}

			for _, at := range uses {
				results.add(Result{
					Symbol:   symbol.key.Symbol,
					Position: at,
					Message:  fmt.Sprintf("uses deprecated %s: %s", symbol.key.Symbol, symbol.reason),
				})
			}
		}
	}

	sort.SliceStable(results.findings, func(i, j int) bool {
		left, right := results.findings[i].Position, results.findings[j].Position
		if left.File != right.File {
			return left.File < right.File
		}
		return left.Line < right.Line
	})

	return results, nil
}

// markedSymbols is every declaration whose doc carries a Deprecated paragraph.
func markedSymbols(root *model.DocumentRoot) []marked {
	var out []marked

	for _, def := range root.Packages {
		if def.Package.TestPackage {
			continue
		}

		for _, decl := range declarations(def) {
			reason := deprecatedText(decl.Doc)
			if reason == "" {
				continue
			}

			out = append(out, marked{
				key:      refindex.Key{ImportPath: def.Package.ImportPath, Symbol: decl.Symbol()},
				reason:   reason,
				position: decl.Position(def.Package),
			})
		}
	}

	return out
}

// declarations is the types, constants, variables and funcs of a definition.
func declarations(def *model.Definition) model.DeclarationList {
	out := model.DeclarationList{}
	out = append(out, def.Types...)
	out = append(out, def.Consts...)
	out = append(out, def.Vars...)
	out = append(out, def.Funcs...)
	return out
}

// deprecatedText returns the reason a doc comment gives, or "" for one that marks nothing.
func deprecatedText(doc string) string {
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if reason, ok := strings.CutPrefix(line, "Deprecated:"); ok {
			return strings.TrimSpace(reason)
		}
	}
	return ""
}

// sources parses the files a use may sit in, once.
type sources struct {
	root  string
	fset  *token.FileSet
	files map[string]*ast.File
}

// newSources returns a reader of root's files.
func newSources(root *model.DocumentRoot) *sources {
	return &sources{root: root.Root, fset: token.NewFileSet(), files: map[string]*ast.File{}}
}

// parsed returns the file at a root relative path, or nil when it does not parse.
func (s *sources) parsed(name string) *ast.File {
	if file, known := s.files[name]; known {
		return file
	}
	file, err := parser.ParseFile(s.fset, filepath.Join(s.root, name), nil, 0)
	if err != nil {
		file = nil
	}
	s.files[name] = file
	return file
}

// uses returns every position a marked symbol is written at in a reference's file.
func (s *sources) uses(symbol marked, ref refindex.Ref, root string) []model.Position {
	at := ref.Decl.Position(ref.Pkg)
	if root == "" {
		return nil
	}
	file := s.parsed(at.File)
	if file == nil {
		return nil
	}

	aliases := importAliases(file)
	method := strings.Contains(symbol.key.Symbol, ".")
	name := symbol.key.Symbol
	if method {
		_, name, _ = strings.Cut(name, ".")
	}

	sel := map[*ast.Ident]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		expr, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		sel[expr.Sel] = true
		return true
	})

	var found []model.Position
	seen := map[int]bool{}
	record := func(ident *ast.Ident) {
		pos := s.fset.Position(ident.Pos())
		if seen[pos.Line] {
			return
		}
		seen[pos.Line] = true
		found = append(found, model.Position{Package: at.Package, File: at.File, Line: pos.Line})
	}

	ast.Inspect(file, func(node ast.Node) bool {
		expr, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if method {
			if expr.Sel.Name == name {
				record(expr.Sel)
			}
			return true
		}
		owner, ok := expr.X.(*ast.Ident)
		if !ok || aliases[owner.Name] != symbol.key.ImportPath || expr.Sel.Name != name {
			return true
		}
		record(expr.Sel)
		return true
	})

	if method || ref.Pkg.ImportPath != symbol.key.ImportPath {
		return found
	}

	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok || ident.Name != name || sel[ident] {
			return true
		}
		pos := s.fset.Position(ident.Pos())
		if pos.Line == symbol.position.Line && at.File == symbol.position.File {
			return true
		}
		record(ident)
		return true
	})

	return found
}

// importAliases maps the name a file reaches an import by to its path.
func importAliases(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, spec := range file.Imports {
		quoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(quoted)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		out[name] = quoted
	}
	return out
}
