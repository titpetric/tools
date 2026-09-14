package coverage

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestModulePath(t *testing.T) {
	def := func(module string) *model.Definition {
		if module == "" {
			return &model.Definition{}
		}
		return &model.Definition{Module: &model.Module{Path: module}}
	}

	for name, test := range map[string]struct {
		defs model.DefinitionList
		want string
	}{
		"none":               {nil, ""},
		"no go.mod":          {model.DefinitionList{def("")}, ""},
		"one module":         {model.DefinitionList{def("example.com/a"), def("example.com/a")}, "example.com/a"},
		"module and no mod":  {model.DefinitionList{def(""), def("example.com/a")}, "example.com/a"},
		"two modules answer": {model.DefinitionList{def("example.com/a"), def("example.com/b")}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := modulePath(test.defs); got != test.want {
				t.Errorf("modulePath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRelativePackage(t *testing.T) {
	for name, test := range map[string]struct {
		module     string
		importPath string
		want       string
	}{
		"root":            {"example.com/a", "example.com/a", "."},
		"below root":      {"example.com/a", "example.com/a/cmd/tool", "cmd/tool"},
		"no module":       {"", "example.com/a/cmd/tool", "example.com/a/cmd/tool"},
		"outside module":  {"example.com/a", "example.com/b/cmd", "example.com/b/cmd"},
		"shared prefix":   {"example.com/a", "example.com/ab/cmd", "example.com/ab/cmd"},
		"module is empty": {"", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := relativePackage(test.module, test.importPath); got != test.want {
				t.Errorf("relativePackage(%q, %q) = %q, want %q", test.module, test.importPath, got, test.want)
			}
		})
	}
}

func TestCollapseColumn(t *testing.T) {
	rows := [][]string{
		{"ok", "a", "one"},
		{"ok", "a", "two"},
		{"ok", "b", "three"},
		{"ok", "b", "four"},
		{"ok", "a", "five"},
	}
	collapseColumn(rows, 1)

	// The fifth row repeats "a" after "b", which is a new group rather than a
	// continuation of the first, so it is written out again.
	want := []string{"a", "", "b", "", "a"}
	for i, row := range rows {
		if row[1] != want[i] {
			t.Errorf("row %d package = %q, want %q", i, row[1], want[i])
		}
	}

	// Nothing else in the row moves.
	if rows[4][2] != "five" || rows[0][0] != "ok" {
		t.Errorf("collapseColumn touched a column it was not given: %v", rows)
	}
}

func TestCollapseColumnShortRow(t *testing.T) {
	rows := [][]string{{"a"}, {"a", "b"}, {"a", "b"}}
	collapseColumn(rows, 1)
	if rows[1][1] != "b" || rows[2][1] != "" {
		t.Errorf("collapseColumn over a short row = %v", rows)
	}
}

// coveredRoot is a document of one module and two packages, one function
// covered and one not.
func coveredRoot() *model.DocumentRoot {
	module := &model.Module{Path: "example.com/a"}

	return &model.DocumentRoot{
		SchemaVersion: model.SchemaVersion,
		Packages: model.DefinitionList{
			{
				Package: model.Package{
					ImportPath: "example.com/a",
					Package:    "a",
					Complexity: &model.Complexity{Coverage: 100, Cognitive: 1, Lines: 3},
				},
				Module: module,
				Funcs: model.DeclarationList{
					{Kind: model.FuncKind, Name: "Do", Complexity: &model.Complexity{Coverage: 100, Cognitive: 1}},
				},
			},
			{
				Package: model.Package{
					ImportPath: "example.com/a/b",
					Package:    "b",
					Complexity: &model.Complexity{Coverage: 0, Cognitive: 9, Lines: 12},
				},
				Module: module,
				Funcs: model.DeclarationList{
					{Kind: model.FuncKind, Name: "Skip", Receiver: "*T", Complexity: &model.Complexity{Coverage: 0, Cognitive: 9}},
				},
			},
		},
	}
}

func TestData(t *testing.T) {
	rows := Data(coveredRoot(), false)
	if len(rows) != 1 {
		t.Fatalf("Data() without verbose = %d rows, want the covered one alone", len(rows))
	}
	if rows[0].Function != "Do" || rows[0].Coverage != 100 {
		t.Errorf("Data()[0] = %+v, want Do at 100", rows[0])
	}

	rows = Data(coveredRoot(), true)
	if len(rows) != 2 {
		t.Fatalf("Data() with verbose = %d rows, want both", len(rows))
	}
	if rows[1].Function != "T.Skip" {
		t.Errorf("a method is named by its receiver, got %q", rows[1].Function)
	}
}

func TestWrite(t *testing.T) {
	var out strings.Builder
	if err := Write(&out, coveredRoot(), Options{Verbose: true}); err != nil {
		t.Fatal(err)
	}

	report := out.String()
	for _, want := range []string{"| Status", "| Do", "100.00%", markPass, markFail, "| T.Skip"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report is missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "example.com/a") {
		t.Errorf("a one module report names packages relative to it:\n%s", report)
	}
}
