package diff

import (
	"sort"

	"github.com/titpetric/tools/splint/model"
)

// A go.mod entry moves the same three ways a field of a type does, and the
// report uses the same names for it.
const (
	requireAdded   = fieldAdded
	requireChanged = fieldChanged
	requireRemoved = fieldRemoved
)

// Require is one require line of a go.mod, as the report lists it.
type Require struct {
	// Path is the required module, and Version the version required of it.
	Path    string `json:"path"`
	Version string `json:"version"`

	// Indirect reports a require line that pins what a dependency of a
	// dependency resolves to, one the module does not import itself.
	Indirect bool `json:"indirect,omitempty"`
}

// Replace is one replace directive of a go.mod, as the report lists it.
type Replace struct {
	// Path is the module the directive replaces, and Version the single
	// version the directive applies to, empty when it applies to every version.
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`

	// NewPath is what it resolves to, a module path or a directory on disk,
	// and NewVersion the version of it, which a directory has none of.
	NewPath    string `json:"newPath"`
	NewVersion string `json:"newVersion,omitempty"`
}

// RequireChange is one require line a release adds, drops, or moves to another
// version.
type RequireChange struct {
	// Path is the required module the change applies to, the key the
	// comparison matches the two revisions on.
	Path string `json:"path"`

	// Change holds added, changed or removed.
	Change string `json:"change"`

	// Old and New are the require line before and after. An added line has
	// no Old, and a dropped one has no New.
	Old *Require `json:"old,omitempty"`
	New *Require `json:"new,omitempty"`
}

// ReplaceChange is one replace directive a release adds, drops, or repoints.
type ReplaceChange struct {
	// Path and Version are the module the directive replaces, together the
	// key the comparison matches the two revisions on: a go.mod may replace one
	// version of a module and leave the rest alone.
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`

	Change string `json:"change"`

	Old *Replace `json:"old,omitempty"`
	New *Replace `json:"new,omitempty"`
}

// VersionChange is a go.mod directive naming a version, before and after. The
// side a release introduced or dropped the directive on is empty.
type VersionChange struct {
	Old string `json:"old,omitempty"`
	New string `json:"new,omitempty"`
}

// ModuleChange is one module whose go.mod moved between two revisions.
//
// Nothing here is breaking on its own. A dependency is not API, and moving one
// takes nothing away from a consumer that a compiler will complain about. It
// still belongs in a release note: it decides what the consumer downloads,
// which lowest versions its own build has to meet, and whose code lands in
// the binary.
type ModuleChange struct {
	// Path is the module path, the key the comparison matches the two go.mod
	// files on.
	Path string `json:"path"`

	// Go and Toolchain are the go and toolchain directives when the release
	// moved them, and are absent otherwise. Raising the go directive drops
	// every consumer still on an older toolchain. No other entry here can stop
	// a build outright.
	Go        *VersionChange `json:"go,omitempty"`
	Toolchain *VersionChange `json:"toolchain,omitempty"`

	// Requires are the require line changes, in module path order.
	Requires []RequireChange `json:"requires,omitempty"`

	// Replaces are the replace directive changes, in module path order.
	Replaces []ReplaceChange `json:"replaces,omitempty"`
}

// modules returns the go.mod of every module behind the definitions, by
// module path.
//
// Extraction records the module on every package it produced, so the first
// package of a module carries what the rest repeat. A model extracted before
// it included go.mod, or from a tree holding none, reports no modules at all
// and compares as though nothing changed.
func modules(defs []*model.Definition) map[string]*model.Module {
	result := make(map[string]*model.Module)
	for _, def := range defs {
		if def.Module == nil || def.Module.Path == "" {
			continue
		}
		if _, ok := result[def.Module.Path]; !ok {
			result[def.Module.Path] = def.Module
		}
	}
	return result
}

// compareModules reports how the go.mod of each module moved between two
// revisions, over the union of the module paths the two sides hold. A module
// only one side has meets an empty one in the comparison, so a module a
// release split out reports every require line it carries as added.
func compareModules(old, cur map[string]*model.Module, includeIndirect bool) []ModuleChange {
	paths := make(map[string]struct{}, len(old)+len(cur))
	for path := range old {
		paths[path] = struct{}{}
	}
	for path := range cur {
		paths[path] = struct{}{}
	}

	result := []ModuleChange{}
	for path := range paths {
		if change, ok := compareModule(path, old[path], cur[path], includeIndirect); ok {
			result = append(result, change)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

// compareModule reports how one go.mod moved, and whether it moved at all.
func compareModule(path string, was, is *model.Module, includeIndirect bool) (ModuleChange, bool) {
	if was == nil {
		was = &model.Module{}
	}
	if is == nil {
		is = &model.Module{}
	}

	change := ModuleChange{
		Path:      path,
		Go:        versionChange(was.GoVersion, is.GoVersion),
		Toolchain: versionChange(was.Toolchain, is.Toolchain),
		Requires:  compareRequires(was.Requires, is.Requires, includeIndirect),
		Replaces:  compareReplaces(was.Replaces, is.Replaces),
	}

	moved := change.Go != nil || change.Toolchain != nil ||
		len(change.Requires) > 0 || len(change.Replaces) > 0
	if !moved {
		return ModuleChange{}, false
	}
	return change, true
}

// compareRequires reports the require lines a release added, dropped, or moved
// to another version, matched by module path.
//
// compareRequires also reports a require line when only its indirect mark
// moved, since that means the module takes on an import it got for free
// before, or drops one.
//
// compareRequires skips indirect lines unless includeIndirect holds true. go
// mod tidy writes them with no human decision behind them, and a routine tidy
// rewrites dozens of them, which buries the handful of direct ones that make
// the actual release note. A line direct on either side counts as direct, so
// the report lists one that changed from indirect to direct either way.
func compareRequires(was, is []model.Require, includeIndirect bool) []RequireChange {
	var (
		old    = requireIndex(was)
		cur    = requireIndex(is)
		result []RequireChange
	)

	for path, before := range old {
		after, ok := cur[path]
		switch {
		case !ok:
			if includeIndirect || !before.Indirect {
				result = append(result, RequireChange{
					Path: path, Change: requireRemoved, Old: requirePtr(before),
				})
			}
		case before != after:
			if includeIndirect || !before.Indirect || !after.Indirect {
				result = append(result, RequireChange{
					Path: path, Change: requireChanged,
					Old: requirePtr(before), New: requirePtr(after),
				})
			}
		}
	}
	for path, after := range cur {
		if _, ok := old[path]; ok {
			continue
		}
		if includeIndirect || !after.Indirect {
			result = append(result, RequireChange{
				Path: path, Change: requireAdded, New: requirePtr(after),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

// compareReplaces reports the replace directives a release added, dropped or
// repointed, matched by the module and version they replace.
func compareReplaces(was, is []model.Replace) []ReplaceChange {
	var (
		old    = replaceIndex(was)
		cur    = replaceIndex(is)
		result []ReplaceChange
	)

	for key, before := range old {
		after, ok := cur[key]
		switch {
		case !ok:
			result = append(result, ReplaceChange{
				Path: before.Path, Version: before.Version,
				Change: requireRemoved, Old: replacePtr(before),
			})
		case before != after:
			result = append(result, ReplaceChange{
				Path: before.Path, Version: before.Version,
				Change: requireChanged,
				Old:    replacePtr(before), New: replacePtr(after),
			})
		}
	}
	for key, after := range cur {
		if _, ok := old[key]; !ok {
			result = append(result, ReplaceChange{
				Path: after.Path, Version: after.Version,
				Change: requireAdded, New: replacePtr(after),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Version < result[j].Version
	})
	return result
}

// requireIndex keys a require list by module path, the key the comparison
// matches two revisions of a go.mod through.
func requireIndex(requires []model.Require) map[string]Require {
	index := make(map[string]Require, len(requires))
	for _, require := range requires {
		index[require.Path] = Require{
			Path:     require.Path,
			Version:  require.Version,
			Indirect: require.Indirect,
		}
	}
	return index
}

// replaceIndex keys a replace list by the module and version it replaces. A
// go.mod may replace more than one version of the same module, so the path alone
// does not identify a directive.
func replaceIndex(replaces []model.Replace) map[string]Replace {
	index := make(map[string]Replace, len(replaces))
	for _, replace := range replaces {
		entry := Replace{
			Path:       replace.Path,
			Version:    replace.Version,
			NewPath:    replace.NewPath,
			NewVersion: replace.NewVersion,
		}
		index[entry.Path+"@"+entry.Version] = entry
	}
	return index
}

// versionChange reports a directive that moved, and nothing for one that held.
func versionChange(was, is string) *VersionChange {
	if was == is {
		return nil
	}
	return &VersionChange{Old: was, New: is}
}

// requirePtr and replacePtr return an entry to report, which the result holds
// by pointer so the json omits the side a change has none on.
func requirePtr(require Require) *Require {
	return &require
}

func replacePtr(replace Replace) *Replace {
	return &replace
}

// requireSummary renders a require line change as one line. A line that
// moved shows the version on either side of it, and one whose indirect mark
// flipped says so, since that can be the whole of the change.
func requireSummary(change RequireChange) string {
	switch change.Change {
	case requireChanged:
		line := change.Path + " " + change.Old.Version
		if change.Old.Version != change.New.Version {
			line += " -> " + change.New.Version
		}
		if change.Old.Indirect != change.New.Indirect {
			line += " (" + indirectWord(change.Old.Indirect) + " -> " + indirectWord(change.New.Indirect) + ")"
		}
		return line
	case requireRemoved:
		return requireLine(*change.Old)
	}
	return requireLine(*change.New)
}

// requireLine renders a require line the way go.mod writes it.
func requireLine(require Require) string {
	line := require.Path + " " + require.Version
	if require.Indirect {
		line += " // indirect"
	}
	return line
}

func indirectWord(indirect bool) string {
	if indirect {
		return "indirect"
	}
	return "direct"
}

// replaceSummary renders a replace change as one line, in the shape go.mod
// writes the directive.
func replaceSummary(change ReplaceChange) string {
	switch change.Change {
	case requireChanged:
		return replaceLine(*change.Old) + " -> " + replaceTarget(*change.New)
	case requireRemoved:
		return replaceLine(*change.Old)
	}
	return replaceLine(*change.New)
}

func replaceLine(replace Replace) string {
	line := replace.Path
	if replace.Version != "" {
		line += " " + replace.Version
	}
	return line + " => " + replaceTarget(replace)
}

// replaceTarget renders what a directive replaces a module with. A directory
// on disk carries no version.
func replaceTarget(replace Replace) string {
	if replace.NewVersion == "" {
		return replace.NewPath
	}
	return replace.NewPath + " " + replace.NewVersion
}

// countRequires returns how many go.mod entries moved across every module
// in the report, the number the summary line shows.
func countRequires(changes []ModuleChange) int {
	count := 0
	for _, change := range changes {
		count += len(change.Requires) + len(change.Replaces)
	}
	return count
}
