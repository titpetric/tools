package integration

import (
	"fmt"
	"path"
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
// by import path, each with the words a finding says about it. The list
// names a method as Type.Method, the way a resolved call records one.
var crossing = map[string]struct {
	symbols []string
	verb    string
}{
	"os/exec":           {[]string{"Command", "CommandContext"}, "runs a process through"},
	"os":                {[]string{"StartProcess"}, "runs a process through"},
	"syscall":           {[]string{"Exec", "ForkExec", "StartProcess"}, "runs a process through"},
	"net":               {[]string{"Listen", "ListenTCP", "ListenUDP", "ListenUnix", "ListenPacket", "ListenIP", "ListenConfig", "Dial", "DialTimeout", "DialTCP", "DialUDP", "DialUnix", "DialIP", "Dialer", "Dialer.Dial", "Dialer.DialContext", "ListenConfig.Listen", "ListenConfig.ListenPacket"}, "opens a socket through"},
	"net/http/httptest": {[]string{"NewServer", "NewTLSServer", "NewUnstartedServer"}, "listens on a port through"},
	"net/http":          {[]string{"ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS", "Get", "Head", "Post", "PostForm", "Server.ListenAndServe", "Server.ListenAndServeTLS", "Server.Serve", "Server.ServeTLS"}, "goes over the network through"},
	"database/sql":      {[]string{"Open", "OpenDB"}, "opens a database through"},
}

// node is one function of the document and the definition holding it.
type node struct {
	decl *model.Declaration
	def  *model.Definition
}

// step is how a function crosses the boundary: by a reason of its own, or
// through the function it calls next.
type step struct {
	reason string
	next   *model.Declaration

	// depth is how many calls down the chain the reason sits, 0 for a
	// function with a reason of its own.
	depth int
}

// scope is the document as the boundary rule reads it: every function, what
// it calls, and whether that crosses the boundary, directly or down a chain
// of calls into the module.
type scope struct {
	storage func(string) bool

	// funcs and tests find a function by the import path of its package and
	// its symbol, the declarations outside the test files and inside them.
	funcs map[string]node
	tests map[string]node

	// imports are the imports of every file, by directory and filename.
	imports map[string]map[string][]model.ImportSpec

	// crosses holds the functions that cross the boundary, each with its
	// step, read once for the document, and database the ones that reach a
	// database, which the testmain rule reads.
	crosses  map[*model.Declaration]step
	database map[*model.Declaration]step
	nodes    map[*model.Declaration]node
}

// newScope indexes the document and works out which functions cross.
func newScope(root *model.DocumentRoot, storage func(string) bool) *scope {
	s := &scope{
		storage:  storage,
		funcs:    map[string]node{},
		tests:    map[string]node{},
		imports:  map[string]map[string][]model.ImportSpec{},
		crosses:  map[*model.Declaration]step{},
		database: map[*model.Declaration]step{},
		nodes:    map[*model.Declaration]node{},
	}

	for _, def := range root.Packages {
		if def == nil {
			continue
		}
		files := s.imports[def.Package.Path]
		if files == nil {
			files = map[string][]model.ImportSpec{}
			s.imports[def.Package.Path] = files
		}
		for file, literals := range def.Imports {
			for _, literal := range literals {
				if spec, ok := importfmt.Parse(literal); ok {
					files[file] = append(files[file], spec)
				}
			}
		}

		owner := basePath(def)
		for _, decl := range def.Funcs {
			n := node{decl: decl, def: def}
			s.nodes[decl] = n
			index := s.funcs
			if decl.IsTestScope() {
				index = s.tests
			}
			index[owner+" "+decl.Symbol()] = n
		}
	}

	s.solve()
	return s
}

// solve finds every function that crosses: the ones with a reason of their
// own first, then their callers, a ring at a time, so the chain a finding
// names is the shortest there is.
//
// The pass follows a function whose calls the parse recorded, as the ast
// parser does, down its calls; one without them, from the simple parser or a
// file behind a build tag, down the references and the package level names
// it reaches.
func (s *scope) solve() {
	order := make([]*model.Declaration, 0, len(s.nodes))
	for decl := range s.nodes {
		order = append(order, decl)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := s.nodes[order[i]], s.nodes[order[j]]
		if a.def.Package.Path != b.def.Package.Path {
			return a.def.Package.Path < b.def.Package.Path
		}
		if a.decl.File != b.decl.File {
			return a.decl.File < b.decl.File
		}
		return a.decl.Line < b.decl.Line
	})

	callers := map[*model.Declaration][]*model.Declaration{}
	for _, decl := range order {
		for _, callee := range s.edges(s.nodes[decl]) {
			if callee != decl {
				callers[callee] = append(callers[callee], decl)
			}
		}
	}

	s.spread(order, callers, s.crosses, false)
	s.spread(order, callers, s.database, true)
}

// spread fills one map of steps: the functions with a reason of their own,
// then their callers, breadth first.
func (s *scope) spread(order []*model.Declaration, callers map[*model.Declaration][]*model.Declaration, steps map[*model.Declaration]step, database bool) {
	var queue []*model.Declaration
	for _, decl := range order {
		if reason := s.direct(s.nodes[decl], database); reason != "" {
			steps[decl] = step{reason: reason}
			queue = append(queue, decl)
		}
	}

	for len(queue) > 0 {
		callee := queue[0]
		queue = queue[1:]
		for _, caller := range callers[callee] {
			if _, known := steps[caller]; known {
				continue
			}
			steps[caller] = step{next: callee, depth: steps[callee].depth + 1}
			queue = append(queue, caller)
		}
	}
}

// edges are the functions of the document a function calls.
func (s *scope) edges(n node) []*model.Declaration {
	var out []*model.Declaration
	add := func(found node, ok bool) {
		if ok {
			out = append(out, found.decl)
		}
	}

	if n.decl.Calls != nil {
		for _, target := range n.decl.Calls.Keys() {
			for _, symbol := range n.decl.Calls[target] {
				add(s.lookup(target, symbol))
			}
		}
		return out
	}

	for _, name := range n.decl.References.Keys() {
		target, ok := s.resolve(n, name)
		if !ok {
			continue
		}
		for _, symbol := range n.decl.References[name] {
			add(s.lookup(target, symbol))
		}
	}

	owner := basePath(n.def)
	if n.decl.IsTestScope() {
		owner += "_test"
	}
	for _, name := range n.decl.Globals.Keys() {
		add(s.lookup(owner, name))
		for _, symbol := range n.decl.Globals[name] {
			add(s.lookup(owner, name+"."+symbol))
		}
	}
	return out
}

// lookup finds a function by the import path of its package and its symbol.
// A path ending in _test is a test package, whose functions sit in the test
// files; the other path prefers the functions outside them.
func (s *scope) lookup(target, symbol string) (node, bool) {
	base := strings.TrimSuffix(target, "_test")
	key := base + " " + symbol
	first, second := s.funcs, s.tests
	if base != target {
		first, second = second, first
	}
	if found, ok := first[key]; ok {
		return found, true
	}
	found, ok := second[key]
	return found, ok
}

// reason says how a test crosses the boundary, and is empty for one that does
// not: its own reason, or the chain of calls down to the one that does. A
// chain longer than depth reads as clean: the deeper the crossing call, the
// likelier it sits behind a branch the test never takes.
func (s *scope) reason(decl *model.Declaration, depth int) string {
	return chain(s, s.crosses, decl, depth)
}

// databaseReason says how a function reaches a database, the same way.
func (s *scope) databaseReason(decl *model.Declaration, depth int) string {
	return chain(s, s.database, decl, depth)
}

// chain renders the steps from a function down to its reason.
func chain(s *scope, steps map[*model.Declaration]step, decl *model.Declaration, depth int) string {
	first, ok := steps[decl]
	if !ok || first.depth > depth {
		return ""
	}
	if first.next == nil {
		return first.reason
	}

	var labels []string
	seen := map[*model.Declaration]bool{decl: true}
	current := first
	for current.next != nil && !seen[current.next] {
		seen[current.next] = true
		labels = append(labels, label(s.nodes[current.next]))
		current = steps[current.next]
	}
	return fmt.Sprintf("calls %s, which %s", strings.Join(labels, " → "), current.reason)
}

// direct is a boundary the function crosses itself: a call into a package
// that does, or a literal naming an in-memory database. With database set,
// only a database counts.
func (s *scope) direct(n node, database bool) string {
	for _, name := range n.decl.References.Keys() {
		target, ok := s.resolve(n, name)
		if !ok {
			continue
		}
		if reason := s.reaches(target, name, n.decl.References[name], database); reason != "" {
			return reason
		}
	}

	for _, target := range n.decl.Calls.Keys() {
		if reason := s.reaches(target, path.Base(target), n.decl.Calls[target], database); reason != "" {
			return reason
		}
	}

	for _, literal := range n.decl.Literals {
		if strings.Contains(literal, ":memory:") || strings.Contains(literal, "mode=memory") {
			return fmt.Sprintf("opens sqlite %q", literal)
		}
	}

	return ""
}

// reaches says how a call into one package crosses the boundary, and with
// database set, only how it reaches a database.
func (s *scope) reaches(target, name string, symbols []string, database bool) string {
	sorted := append([]string(nil), symbols...)
	sort.Strings(sorted)

	if known, ok := crossing[target]; ok {
		if database && target != "database/sql" {
			return ""
		}
		for _, symbol := range sorted {
			for _, listed := range known.symbols {
				if symbol == listed {
					return fmt.Sprintf("%s %s.%s", known.verb, name, symbol)
				}
			}
		}
		return ""
	}

	if driver(target) {
		return "reaches the SQL driver " + target
	}

	if s.storage(target) {
		for _, symbol := range sorted {
			if strings.HasPrefix(symbol, "New") || strings.HasPrefix(symbol, "Open") || strings.HasPrefix(symbol, "Must") {
				return fmt.Sprintf("imports %s and calls %s.%s", target, name, symbol)
			}
		}
	}

	return ""
}

// resolve returns the import path a function's file reaches under a name.
func (s *scope) resolve(n node, name string) (string, bool) {
	for _, spec := range s.imports[n.def.Package.Path][n.decl.File] {
		ref := spec.Ref()
		if ref == name || (spec.Name == "" && strings.TrimPrefix(ref, "go-") == name) {
			return spec.Path, true
		}
	}
	return "", false
}

// label names a function in a chain the way a reader calls it: package.Func
// or package.Type.Method.
func label(n node) string {
	if n.decl == nil {
		return "?"
	}
	return strings.TrimSuffix(n.def.Package.Package, "_test") + "." + n.decl.Symbol()
}

// basePath is the import path a call names the functions of a definition by: a
// test definition carries the _test suffix, and a call into a white box test
// names the package it compiles into.
func basePath(def *model.Definition) string {
	return strings.TrimSuffix(def.Package.ImportPath, "_test")
}

// driver reports the import path of a SQL driver, or a package under one.
func driver(target string) bool {
	for _, prefix := range sqlDrivers {
		if target == prefix || strings.HasPrefix(target, prefix+"/") {
			return true
		}
	}
	return false
}
