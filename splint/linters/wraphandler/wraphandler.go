// Package wraphandler reports an exported handler with no unexported wrapper.
//
// It is a port of the gofsck analyzer of the same name, reimplemented against
// the splint model: the check is the same idea and the reading is different,
// because a document is not a syntax tree.
package wraphandler

import (
	"context"
	"fmt"
	"unicode"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "wraphandler"

// RuleUnwrapped is the one rule this linter reports under.
const RuleUnwrapped = "unwrapped"

// The two argument types a function has to take to be an http.HandlerFunc.
const (
	responseWriter = "http.ResponseWriter"
	requestPointer = "*http.Request"
)

// Linter reports an exported handler with no unexported wrapper.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports every exported handler that nothing but a server can call.
//
// A handler returns nothing, so what it decided goes to the response writer
// and a test has to stand up a server to read it back. The convention that
// gets around it is a thin exported handler over an unexported function of the
// same name returning an error, which a test calls directly. A handler with no
// such function behind it is what this reports.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if def.Package.TestPackage {
			continue
		}

		found := handlers(def)
		if len(found) == 0 {
			continue
		}

		metric := results.count(def.Package, len(found))
		for _, decl := range found {
			if hasWrapper(def, decl) {
				metric.Wrapped++
				continue
			}
			results.add(Result{
				Rule:     RuleUnwrapped,
				Symbol:   decl.Symbol(),
				Position: decl.Position(def.Package),
				Message: fmt.Sprintf("%s is an http.HandlerFunc with no %s(w, r) error behind it, so only a server can call it",
					decl.Symbol(), wrapperSymbol(decl)),
			})
		}
	}

	return results, nil
}

// handlers is every exported handler a consumer of the package can mount,
// which is what the convention is measured against.
func handlers(def *model.Definition) model.DeclarationList {
	var found model.DeclarationList
	for _, decl := range def.Funcs {
		if !decl.IsExported() || decl.IsTestScope() || generated(def, decl) {
			continue
		}
		if isHandler(decl) {
			found = append(found, decl)
		}
	}
	return found
}

// generated reports a declaration in a file nobody wrote. A generator emits
// whatever shape it was written to emit, and no convention it did not follow
// is worth reporting to the person who ran it.
func generated(def *model.Definition, decl *model.Declaration) bool {
	file, known := def.Files.Find(decl.File)
	return known && file.Generated
}

// isHandler reports the http.HandlerFunc signature: a response writer and a
// request, and nothing coming back. A function taking the two and returning an
// error is already the testable half of the pair, so it is not a handler this
// check has anything to say about.
func isHandler(decl *model.Declaration) bool {
	if decl.Kind != model.FuncKind || len(decl.Returns) > 0 {
		return false
	}
	if len(decl.Arguments) != 2 {
		return false
	}
	return decl.Arguments[0] == responseWriter && decl.Arguments[1] == requestPointer
}

// hasWrapper reports the unexported counterpart of a handler: the same name
// with a lower first letter, on the same receiver, returning an error. The
// error is what makes it worth calling from a test, so a counterpart that
// returns nothing is another handler rather than the work behind one.
func hasWrapper(def *model.Definition, handler *model.Declaration) bool {
	name := wrapperName(handler.Name)
	receiver := handler.ReceiverTypeRef()

	for _, decl := range def.Funcs {
		if decl.Name != name || decl.ReceiverTypeRef() != receiver {
			continue
		}
		if decl.IsTestScope() {
			continue
		}
		if len(decl.Returns) == 1 && decl.Returns[0] == "error" {
			return true
		}
	}
	return false
}

// wrapperName is the name the counterpart goes by, which is the handler's with
// a lower first letter.
func wrapperName(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// wrapperSymbol names the counterpart the way a reader refers to it, so a
// finding about a method says which type the missing method belongs to.
func wrapperSymbol(decl *model.Declaration) string {
	name := wrapperName(decl.Name)
	if decl.Receiver != "" {
		return decl.ReceiverTypeRef() + "." + name
	}
	return name
}
