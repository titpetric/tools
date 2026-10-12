package modcheck_test

import (
	"testing"

	"github.com/titpetric/tools/splint/linters/modcheck"
	"github.com/titpetric/tools/splint/model"
)

// reached is a module requiring one dependency, imported by the one file of
// the one package that reaches it, and called once from a body.
//
// That is one file and one symbol, which is the shape the thin rule fires on.
// What the caller adds to the definition is what decides whether it should.
func reached(add func(*model.Definition)) *model.DocumentRoot {
	def := &model.Definition{
		Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
		Imports: model.StringSet{"a.go": {`"example.com/db"`}},
		Funcs: model.DeclarationList{{
			Kind: model.FuncKind, Name: "Open", File: "a.go",
			References: model.StringSet{"db": {"New"}},
		}},
	}
	if add != nil {
		add(def)
	}

	return &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/db", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{def},
	}
}

// TestLinter_LintTypePositionIsASymbol covers the dependency reached as a type
// rather than as a call.
//
// A reference is collected from a body, so a module named in a field, a
// parameter or a result is reached and recorded nowhere a reference can be
// read from. Counting the body alone reads a struct built around a *db.Conn
// and driving everything through it as a module reached once by name, which is
// the cheapest thing to inline and the furthest from what it is.
func TestLinter_LintTypePositionIsASymbol(t *testing.T) {
	for _, test := range []struct {
		name string
		add  func(*model.Definition)
	}{
		{
			name: "struct field",
			add: func(def *model.Definition) {
				def.Types = model.DeclarationList{{
					Kind: model.StructKind, Name: "Store", File: "a.go",
					Fields: model.FieldList{
						{Name: "name", Type: "string"},
						{Name: "conn", Type: "*db.Conn"},
					},
				}}
			},
		},
		{
			name: "parameter",
			add: func(def *model.Definition) {
				def.Funcs = append(def.Funcs, &model.Declaration{
					Kind: model.FuncKind, Name: "Use", File: "a.go",
					Arguments: []string{"*db.Conn", "string"},
				})
			},
		},
		{
			name: "return",
			add: func(def *model.Definition) {
				def.Funcs = append(def.Funcs, &model.Declaration{
					Kind: model.FuncKind, Name: "Rows", File: "a.go",
					Returns: []string{"[]db.Row", "error"},
				})
			},
		},
		{
			// A type the reduction has to see through two layers of: the
			// value of a map and a pointer to it.
			name: "map value",
			add: func(def *model.Definition) {
				def.Vars = model.DeclarationList{{
					Kind: model.VarKind, Name: "pool", File: "a.go",
					Type: "map[string]*db.Conn",
				}}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := lint(t, reached(test.add))

			if got := rules(report)[modcheck.RuleThin]; got != 0 {
				t.Errorf("thin = %d, want 0: the module is reached by name and as a type", got)
			}

			dep := report.Metrics().Packages["example.com/db"].(modcheck.Dependency)
			if dep.Files != 1 || dep.Symbols != 2 {
				t.Errorf("db = %d files, %d symbols, want 1 and 2", dep.Files, dep.Symbols)
			}
		})
	}
}

// TestLinter_LintTypePositionUnderAnAlias covers the type named through the
// name the file imports the package under. A type reads "sql.Conn" and which
// module sql is is decided by the import, the same as for a call.
func TestLinter_LintTypePositionUnderAnAlias(t *testing.T) {
	root := reached(func(def *model.Definition) {
		def.Imports = model.StringSet{"a.go": {`sql "example.com/db"`}}
		def.Funcs[0].References = model.StringSet{"sql": {"New"}}
		def.Types = model.DeclarationList{{
			Kind: model.StructKind, Name: "Store", File: "a.go",
			Fields: model.FieldList{{Name: "conn", Type: "*sql.Conn"}},
		}}
	})

	dep := lint(t, root).Metrics().Packages["example.com/db"].(modcheck.Dependency)
	if dep.Symbols != 2 {
		t.Errorf("symbols = %d, want 2: the alias is the name the file reaches the module by", dep.Symbols)
	}
}

// TestLinter_LintThinStaysThin covers the finding the type positions must not
// take away. One file, one symbol and nothing of the module named anywhere
// else is the dependency a caller is better off inlining, and a rule that
// stopped saying so would be worth nothing.
func TestLinter_LintThinStaysThin(t *testing.T) {
	report := lint(t, reached(func(def *model.Definition) {
		// Types naming no dependency: one built in, and one of a package the
		// file does not import, which resolves to nothing.
		def.Types = model.DeclarationList{{
			Kind: model.StructKind, Name: "Store", File: "a.go",
			Fields: model.FieldList{
				{Name: "name", Type: "string"},
				{Name: "clock", Type: "time.Time"},
			},
		}}
		def.Funcs = append(def.Funcs, &model.Declaration{
			Kind: model.FuncKind, Name: "Name", File: "a.go",
			Arguments: []string{"*Store"},
			Returns:   []string{"string", "error"},
		})
	}))

	if got := rules(report)[modcheck.RuleThin]; got != 1 {
		t.Fatalf("thin = %d, want 1: one file, one symbol, nothing else named", got)
	}

	dep := report.Metrics().Packages["example.com/db"].(modcheck.Dependency)
	if dep.Files != 1 || dep.Symbols != 1 {
		t.Errorf("db = %d files, %d symbols, want 1 and 1", dep.Files, dep.Symbols)
	}
}

// TestLinter_LintBlankImportIsNotThinInOneFile covers the shape the exemption
// is for: the one file that imports the driver writes the import twice, once
// for the init and once for the name, which is how a driver offering an error
// type of its own is reached.
//
// That is one file and one symbol, which is what the thin rule fires on, and
// the registration is what keeps it quiet. Nothing else can: the counts say
// the dependency is cheap and it is not.
func TestLinter_LintBlankImportIsNotThinInOneFile(t *testing.T) {
	root := &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/driver", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
			Imports: model.StringSet{
				"a.go": {`_ "example.com/driver"`, `"example.com/driver"`},
			},
			Funcs: model.DeclarationList{{
				Kind: model.FuncKind, Name: "Open", File: "a.go",
				References: model.StringSet{"driver": {"Error"}},
			}},
		}},
	}

	report := lint(t, root)

	dep := report.Metrics().Packages["example.com/driver"].(modcheck.Dependency)
	if dep.Files != 1 || dep.Symbols != 1 {
		t.Fatalf("driver = %d files, %d symbols, want 1 and 1: one file imports it and one name reaches it",
			dep.Files, dep.Symbols)
	}
	if got := rules(report)[modcheck.RuleThin]; got != 0 {
		t.Errorf("thin = %d, want 0 for a dependency imported for its side effect", got)
	}
}
