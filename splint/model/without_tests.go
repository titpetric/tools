package model

import (
	"strings"
)

// WithoutTests returns nil for nil,
// and a view without test packages and test files otherwise.
func (d *DocumentRoot) WithoutTests() *DocumentRoot {
	if d == nil {
		return nil
	}

	out := *d
	out.Packages = make(DefinitionList, 0, len(d.Packages))
	out.Tagged = nil

	for _, def := range d.Packages {
		if def == nil || def.TestPackage {
			continue
		}

		kept := *def
		kept.Files = def.Files.Filter(func(f File) bool { return !f.Test })
		kept.Imports = withoutTestFiles(def.Imports)
		kept.Excluded = withoutTestFiles(def.Excluded)
		kept.Types = def.Types.Filter(notTestScope)
		kept.Consts = def.Consts.Filter(notTestScope)
		kept.Vars = def.Vars.Filter(notTestScope)
		kept.Funcs = def.Funcs.Filter(notTestScope)

		out.Packages = append(out.Packages, &kept)
	}

	return &out
}

// notTestScope reports a declaration the toolchain compiles into the package
// itself, outside its test binary.
func notTestScope(d *Declaration) bool {
	return !d.IsTestScope()
}

// withoutTestFiles is the import set with the test files dropped. The set uses
// the filename as its key, so what a test file imports is one key.
func withoutTestFiles(in StringSet) StringSet {
	if in == nil {
		return nil
	}

	out := make(StringSet, len(in))
	for name, imports := range in {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		out[name] = imports
	}
	return out
}
