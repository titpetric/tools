package importfmt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
)

// house is the rule this repository is formatted under: the standard order,
// titpetric as the company prefix, and splint as the project.
func house() importfmt.Options {
	opts := importfmt.NewOptions()
	opts.Company = []string{"github.com/titpetric/"}
	opts.Project = "github.com/titpetric/tools/splint"
	return opts
}

// TestFormatLiterals takes the imports a file holds and states the block it
// should hold instead.
//
// The input is the literal form Definition.Imports records, so a case reads
// the way an import is written.
func TestFormatLiterals(t *testing.T) {
	tests := []struct {
		name    string
		imports []string
		expect  string
	}{
		{
			name:    "nothing to write",
			imports: nil,
			expect:  "",
		},
		{
			name:    "one import is still a block",
			imports: []string{`"context"`},
			expect: `import (
	"context"
)`,
		},
		{
			name: "every group in order",
			imports: []string{
				`"github.com/titpetric/tools/splint/model"`,
				`. "example.com/dsl"`,
				`"github.com/stretchr/testify/assert"`,
				`"context"`,
				`_ "github.com/lib/pq"`,
				`"github.com/titpetric/exp/style"`,
			},
			expect: `import (
	"context"

	_ "github.com/lib/pq"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/exp/style"

	"github.com/titpetric/tools/splint/model"

	. "example.com/dsl"
)`,
		},
		{
			name:    "standard library sorted, one group",
			imports: []string{`"strings"`, `"context"`, `"net/http"`, `"fmt"`},
			expect: `import (
	"context"
	"fmt"
	"net/http"
	"strings"
)`,
		},
		{
			name:    "an alias that repeats the path comes off",
			imports: []string{`json "encoding/json"`, `"fmt"`},
			expect: `import (
	"encoding/json"
	"fmt"
)`,
		},
		{
			name:    "a versioned path gets the alias the path implies",
			imports: []string{`"github.com/go-pg/pg/v9"`, `"charm.land/bubbletea/v2"`},
			expect: `import (
	bubbletea "charm.land/bubbletea/v2"
	pg "github.com/go-pg/pg/v9"
)`,
		},
		{
			name:    "an alias the file chose is kept",
			imports: []string{`tea "charm.land/bubbletea/v2"`, `sp "github.com/titpetric/tools/splint/simpleparser"`},
			expect: `import (
	tea "charm.land/bubbletea/v2"

	sp "github.com/titpetric/tools/splint/simpleparser"
)`,
		},
		{
			name:    "a version with no name in front of it gets no alias",
			imports: []string{`"example.com/v2"`, `"context"`},
			expect: `import (
	"context"

	"example.com/v2"
)`,
		},
		{
			name:    "a gopkg.in version is a name and keeps none",
			imports: []string{`"gopkg.in/yaml.v3"`, `"context"`},
			expect: `import (
	"context"

	"gopkg.in/yaml.v3"
)`,
		},
		{
			name:    "the same import twice is written once",
			imports: []string{`"fmt"`, `"fmt"`, `"context"`},
			expect: `import (
	"context"
	"fmt"
)`,
		},
		{
			name:    "one path under two names is two imports",
			imports: []string{`"fmt"`, `printing "fmt"`},
			expect: `import (
	"fmt"
	printing "fmt"
)`,
		},
		{
			name:    "a blank import of a project package is still blanked",
			imports: []string{`_ "github.com/titpetric/tools/splint/linters"`, `"context"`},
			expect: `import (
	"context"

	_ "github.com/titpetric/tools/splint/linters"
)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expect, importfmt.FormatLiterals(test.imports, house()))
		})
	}
}

// TestFormatKeepsComments covers the two comments an import can carry, which
// are the two a rewrite can lose.
func TestFormatKeepsComments(t *testing.T) {
	specs := []model.ImportSpec{
		{Path: "context"},
		{Name: "_", Path: "github.com/lib/pq", Doc: "Postgres registers itself\nwith database/sql."},
		{Path: "net/http", Comment: "for the handler below"},
	}

	assert.Equal(t, `import (
	"context"
	"net/http" // for the handler below

	// Postgres registers itself
	// with database/sql.
	_ "github.com/lib/pq"
)`, importfmt.Format(specs, house()))
}

// TestGroup states which group each shape of import falls into.
func TestGroup(t *testing.T) {
	tests := []struct {
		spec  model.ImportSpec
		group string
	}{
		{model.ImportSpec{Path: "context"}, importfmt.GroupStd},
		{model.ImportSpec{Path: "net/http"}, importfmt.GroupStd},
		{model.ImportSpec{Path: "github.com/stretchr/testify/assert"}, importfmt.GroupGeneral},
		{model.ImportSpec{Path: "github.com/titpetric/exp/style"}, importfmt.GroupCompany},
		{model.ImportSpec{Path: "github.com/titpetric/tools/splint/model"}, importfmt.GroupProject},
		{model.ImportSpec{Path: "github.com/titpetric/tools/splint"}, importfmt.GroupProject},
		{model.ImportSpec{Name: "_", Path: "github.com/lib/pq"}, importfmt.GroupBlanked},
		{model.ImportSpec{Name: "_", Path: "os"}, importfmt.GroupBlanked},
		{model.ImportSpec{Name: ".", Path: "example.com/dsl"}, importfmt.GroupDotted},
	}

	for _, test := range tests {
		t.Run(test.spec.Literal(), func(t *testing.T) {
			assert.Equal(t, test.group, importfmt.Group(test.spec, house()))
		})
	}
}

// TestOrderNamesEveryGroup covers an order that leaves a group out: the
// imports of that group are still written, after the ones the order named.
func TestOrderNamesEveryGroup(t *testing.T) {
	opts := house()
	opts.Order = []string{importfmt.GroupProject, importfmt.GroupStd}

	assert.Equal(t, `import (
	"github.com/titpetric/tools/splint/model"

	"context"

	_ "github.com/lib/pq"

	"github.com/stretchr/testify/assert"
)`, importfmt.FormatLiterals([]string{
		`"context"`,
		`"github.com/stretchr/testify/assert"`,
		`_ "github.com/lib/pq"`,
		`"github.com/titpetric/tools/splint/model"`,
	}, opts))
}

// TestParse covers reading the literal form back.
func TestParse(t *testing.T) {
	tests := []struct {
		literal string
		expect  model.ImportSpec
		ok      bool
	}{
		{`"context"`, model.ImportSpec{Path: "context"}, true},
		{`tea "charm.land/bubbletea/v2"`, model.ImportSpec{Name: "tea", Path: "charm.land/bubbletea/v2"}, true},
		{`_ "github.com/lib/pq"`, model.ImportSpec{Name: "_", Path: "github.com/lib/pq"}, true},
		{`. "example.com/dsl"`, model.ImportSpec{Name: ".", Path: "example.com/dsl"}, true},
		{`nothing`, model.ImportSpec{}, false},
		{`""`, model.ImportSpec{}, false},
	}

	for _, test := range tests {
		t.Run(test.literal, func(t *testing.T) {
			spec, ok := importfmt.Parse(test.literal)
			require.Equal(t, test.ok, ok)
			assert.Equal(t, test.expect, spec)
		})
	}
}

// TestRefNamesTheImport covers the name a file reaches an import by, which is
// what decides whether the import is used and what a missing one resolves to.
func TestRefNamesTheImport(t *testing.T) {
	tests := []struct {
		spec model.ImportSpec
		ref  string
	}{
		{model.ImportSpec{Path: "context"}, "context"},
		{model.ImportSpec{Path: "net/http"}, "http"},
		{model.ImportSpec{Path: "charm.land/bubbletea/v2"}, "bubbletea"},
		{model.ImportSpec{Name: "tea", Path: "charm.land/bubbletea/v2"}, "tea"},
		{model.ImportSpec{Name: "_", Path: "github.com/lib/pq"}, ""},
		{model.ImportSpec{Name: ".", Path: "example.com/dsl"}, ""},
	}

	for _, test := range tests {
		t.Run(test.spec.Literal(), func(t *testing.T) {
			assert.Equal(t, test.ref, test.spec.Ref())
		})
	}
}
