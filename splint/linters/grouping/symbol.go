package grouping

import (
	"path"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// symbol is one exported declaration in the terms the filename rules are
// written in: a name, the type it belongs to, and the file it was found in.
//
// A method belongs to its receiver and a constructor belongs to what it
// returns, so both arrive here with a receiver set and are checked the same
// way. A type belongs to nothing and carries the name alone.
type symbol struct {
	// kind is "type" or "func", and is what the message calls the symbol.
	kind string

	// name is the declared name, and receiver the type it hangs off, empty for
	// a type declaration.
	name     string
	receiver string

	// file is the base filename it was found in, which is what the rules are
	// written against, and position is the same file as a reader opens it.
	file     string
	position model.Position

	// fallback is the file named for the package, which holds whatever the
	// package has not split out yet.
	fallback string
}

// String names the symbol the way a reader refers to it.
func (s symbol) String() string {
	if s.receiver != "" {
		return s.receiver + "." + s.name
	}
	return s.name
}

// match returns the filenames a reader would have expected the symbol in, how
// many filenames were accepted in all, and whether the file it is actually in
// is one of them.
//
// A type is checked as a receiver with no name, which is what makes the walk
// below read the same for both. The first pass asks for the whole name; the
// ones after it break the receiver and then the name into words and ask again,
// because a file named for one word of a compound symbol is a file the symbol
// belongs in.
func (s symbol) match() (expected []string, total int, matched bool) {
	name, receiver := s.name, s.receiver
	if receiver == "" {
		receiver, name = name, ""
	}

	base := path.Base(s.file)
	baseStem := strings.TrimSuffix(base, ".go")

	accepted := matchFilenames(name, receiver, s.fallback)
	expected, total = s.canonical(), len(accepted)

	if checkPatterns(accepted, base, baseStem) {
		return expected, total, true
	}

	if name == "" || receiver == "" {
		return expected, total, false
	}

	// ServiceDiscovery.Start is at home in service_start.go and in
	// discovery_start.go, either of which says what the file is about.
	for _, part := range splitCamelCase(receiver) {
		if checkPatterns(matchFilenames(name, part, s.fallback), base, baseStem) {
			return expected, total, true
		}
	}

	// The same in reverse: Service.FooClient is at home in client.go, which is
	// the file the name is really about.
	for _, part := range splitCamelCase(name) {
		if checkPatterns(matchFilenames(part, receiver, s.fallback), base, baseStem) {
			return expected, total, true
		}
		if checkPatterns(matchFilenames(part, "", s.fallback), base, baseStem) {
			return expected, total, true
		}
	}

	// A method named for what it does, in the file named for that alone.
	if checkPatterns(matchFilenames(name, "", s.fallback), base, baseStem) {
		return expected, total, true
	}

	return expected, total, false
}

// canonical are the filenames worth naming in a message: the receiver and the
// name together, each of them alone, and nothing else.
//
// The full list a symbol is accepted in runs to a dozen names and reading it
// would say less than reading three. The count is reported alongside so the
// message does not pretend these are all of them.
func (s symbol) canonical() []string {
	name, receiver := s.name, s.receiver
	if receiver == "" {
		receiver, name = name, ""
	}

	var locations []string
	if name != "" && receiver != "" {
		locations = append(locations, matchFilename(receiver+name))
	}
	if receiver != "" {
		locations = append(locations, matchFilename(receiver))
	}
	if name != "" {
		locations = append(locations, matchFilename(name))
	}
	return locations
}

// collect reads the exported symbols of one package that the rule has anything
// to say about.
//
// Types are read for the structs among them: an interface, an alias and a
// named primitive are all a declaration of a shape rather than of a thing with
// behaviour, and none of them earns a file. Functions are read for the type
// they belong to, which is the receiver of a method and the first return of a
// constructor. A function belonging to no type is left alone, because there is
// no type to name a file after.
func collect(def *model.Definition) []symbol {
	generated := generatedFiles(def)
	fallback := def.Package.Package + "*.go"

	var symbols []symbol

	for _, decl := range def.Types {
		if skip(decl, generated) || decl.Type != "" {
			continue
		}
		for _, name := range decl.GetNames() {
			if !isExported(name) {
				continue
			}
			symbols = append(symbols, symbol{
				kind:     "type",
				name:     name,
				file:     decl.File,
				position: decl.Position(def.Package),
				fallback: fallback,
			})
		}
	}

	for _, decl := range def.Funcs {
		if skip(decl, generated) || !isExported(decl.Name) {
			continue
		}

		owner := receiverName(decl.Receiver)
		if owner == "" {
			owner = returnName(decl)
		}
		if !isExported(owner) {
			continue
		}

		symbols = append(symbols, symbol{
			kind:     "func",
			name:     decl.Name,
			receiver: owner,
			file:     decl.File,
			position: decl.Position(def.Package),
			fallback: fallback,
		})
	}

	return symbols
}

// skip reports a declaration no rule judges: one the toolchain compiles into
// the test binary, and one a generator wrote.
func skip(decl *model.Declaration, generated map[string]bool) bool {
	return decl.IsTestScope() || generated[decl.File]
}

// generatedFiles are the files of a package nobody wrote, which is where the
// filename says what the generator was pointed at rather than what the symbol
// is.
func generatedFiles(def *model.Definition) map[string]bool {
	files := map[string]bool{}
	for _, file := range def.Files {
		if file.Generated {
			files[file.Name] = true
		}
	}
	return files
}

// receiverName is the type a method hangs off, with the pointer and the type
// parameters taken off: a method on *List[T] belongs to List.
func receiverName(receiver string) string {
	name := model.TypeRef(receiver)
	if index := strings.IndexByte(name, '['); index >= 0 {
		name = name[:index]
	}
	return name
}

// returnName is the type a constructor is named for, which is the first thing
// it returns.
//
// Only a plain type counts. A slice, a map, a channel or an instantiated
// generic is a shape built out of a type rather than the type itself, and a
// function returning one is not the constructor of anything. Neither is a
// generic function returning one of its own type parameters: the T that
// Exec[T] returns is whatever the caller asked for, not a type to name a
// file after.
func returnName(decl *model.Declaration) string {
	if len(decl.Returns) == 0 {
		return ""
	}

	name := strings.TrimLeft(decl.Returns[0], "*")
	if strings.ContainsAny(name, "[]{}() \t,") {
		return ""
	}
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		return name[index+1:]
	}
	if decl.HasTypeParam(name) {
		return ""
	}
	return name
}
