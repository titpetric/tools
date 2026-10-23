package integration

import (
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
)

// sqlDrivers are the import path prefixes of the SQL drivers a test reaches a
// real database through.
var sqlDrivers = []string{
	"modernc.org/sqlite",
	"github.com/mattn/go-sqlite3",
	"github.com/glebarez/go-sqlite",
	"github.com/ncruces/go-sqlite3",
	"github.com/jackc/pgx",
	"github.com/lib/pq",
	"github.com/go-sql-driver/mysql",
	"github.com/jmoiron/sqlx",
}

// crossing are the symbols of the standard library that cross the boundary,
// by import path, each with the words a finding says about it.
var crossing = map[string]struct {
	symbols []string
	verb    string
}{
	"os/exec":           {[]string{"Command", "CommandContext"}, "runs a process through"},
	"os":                {[]string{"StartProcess"}, "runs a process through"},
	"syscall":           {[]string{"Exec", "ForkExec", "StartProcess"}, "runs a process through"},
	"net":               {[]string{"Listen", "ListenTCP", "ListenUDP", "ListenUnix", "ListenPacket", "ListenIP", "ListenConfig", "Dial", "DialTimeout", "DialTCP", "DialUDP", "DialUnix", "DialIP", "Dialer"}, "opens a socket through"},
	"net/http/httptest": {[]string{"NewServer", "NewTLSServer", "NewUnstartedServer"}, "listens on a port through"},
	"net/http":          {[]string{"ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS", "Get", "Head", "Post", "PostForm"}, "goes over the network through"},
	"database/sql":      {[]string{"Open", "OpenDB"}, "opens a database through"},
}

// scope is one directory as the reasons read it: the declarations of its test
// files by name, and the imports of every file.
type scope struct {
	decls   map[string]*model.Declaration
	imports map[string][]model.ImportSpec
	storage func(string) bool

	// memo holds the reason found for a declaration, and visiting marks one
	// under evaluation, so two helpers calling each other end.
	memo     map[*model.Declaration]string
	visiting map[*model.Declaration]bool
}

// newScope indexes a directory.
func newScope(dir []*model.Definition, storage func(string) bool) *scope {
	s := &scope{
		decls:    map[string]*model.Declaration{},
		imports:  map[string][]model.ImportSpec{},
		storage:  storage,
		memo:     map[*model.Declaration]string{},
		visiting: map[*model.Declaration]bool{},
	}

	for _, def := range dir {
		for file, literals := range def.Imports {
			for _, literal := range literals {
				if spec, ok := importfmt.Parse(literal); ok {
					s.imports[file] = append(s.imports[file], spec)
				}
			}
		}
		for _, decl := range def.Order() {
			if !decl.IsTestScope() {
				continue
			}
			if decl.Receiver != "" {
				s.decls[model.TypeRef(decl.Receiver)+"."+decl.Name] = decl
				continue
			}
			for _, name := range decl.GetNames() {
				s.decls[name] = decl
			}
		}
	}

	return s
}

// reason says how a declaration crosses the boundary, and is empty for one
// that does not.
func (s *scope) reason(decl *model.Declaration) string {
	if reason, known := s.memo[decl]; known {
		return reason
	}
	if s.visiting[decl] {
		return ""
	}
	s.visiting[decl] = true
	defer delete(s.visiting, decl)

	reason := s.direct(decl)
	if reason == "" {
		reason = s.helpers(decl)
	}
	s.memo[decl] = reason
	return reason
}

// direct is a boundary the declaration crosses itself: a call into a package
// that does, or a literal naming an in-memory database.
func (s *scope) direct(decl *model.Declaration) string {
	for _, name := range decl.References.Keys() {
		path, ok := s.resolve(decl.File, name)
		if !ok {
			continue
		}
		if reason := s.reaches(path, name, decl.References[name]); reason != "" {
			return reason
		}
	}

	for _, literal := range decl.Literals {
		if strings.Contains(literal, ":memory:") || strings.Contains(literal, "mode=memory") {
			return fmt.Sprintf("opens sqlite %q", literal)
		}
	}

	return ""
}

// reaches says how a call into one package crosses the boundary.
func (s *scope) reaches(path, name string, symbols []string) string {
	sorted := append([]string(nil), symbols...)
	sort.Strings(sorted)

	if known, ok := crossing[path]; ok {
		for _, symbol := range sorted {
			for _, listed := range known.symbols {
				if symbol == listed {
					return fmt.Sprintf("%s %s.%s", known.verb, name, symbol)
				}
			}
		}
		return ""
	}

	if driver(path) {
		return "reaches the SQL driver " + path
	}

	if s.storage(path) {
		for _, symbol := range sorted {
			if strings.HasPrefix(symbol, "New") || strings.HasPrefix(symbol, "Open") || strings.HasPrefix(symbol, "Must") {
				return fmt.Sprintf("imports %s and calls %s.%s", path, name, symbol)
			}
		}
	}

	return ""
}

// helpers is a boundary the declaration crosses through a declaration of the
// test files it calls.
func (s *scope) helpers(decl *model.Declaration) string {
	for _, name := range decl.Globals.Keys() {
		targets := []string{name}
		for _, symbol := range decl.Globals[name] {
			targets = append(targets, name+"."+symbol)
		}
		for _, target := range targets {
			helper, ok := s.decls[target]
			if !ok || helper == decl {
				continue
			}
			if reason := s.reason(helper); reason != "" {
				return fmt.Sprintf("calls %s (%s), which %s", target, helper.File, reason)
			}
		}
	}
	return ""
}

// fileReason is a boundary a file crosses by what it imports alone: a SQL
// driver, registered for its side effect, or database/sql.
func (s *scope) fileReason(file string) string {
	for _, spec := range s.imports[file] {
		if driver(spec.Path) || spec.Path == "database/sql" {
			return "imports " + spec.Path
		}
	}
	return ""
}

// resolve returns the import path a file reaches under a name.
func (s *scope) resolve(file, name string) (string, bool) {
	for _, spec := range s.imports[file] {
		ref := spec.Ref()
		if ref == name || (spec.Name == "" && strings.TrimPrefix(ref, "go-") == name) {
			return spec.Path, true
		}
	}
	return "", false
}

// driver reports the import path of a SQL driver, or a package under one.
func driver(path string) bool {
	for _, prefix := range sqlDrivers {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
