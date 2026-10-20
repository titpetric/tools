package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// apiSymbol is one exported declaration, as "splint diff" reports it.
type apiSymbol struct {
	// Key identifies the declaration across revisions, as the import path
	// followed by the receiver type and name.
	Key string `json:"key"`

	// Package is the import path holding the declaration, Name the declared
	// name qualified with its receiver type, and Kind one of type, const,
	// var or func.
	Package string `json:"package"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`

	// Exported reports whether the symbol is part of the API: an exported
	// name in an importable package. The report lists everything else for the
	// reader, and it counts towards no release.
	Exported bool `json:"exported"`

	// Signature holds a func as its source declares it, and stays empty for
	// every other kind.
	Signature string `json:"signature,omitempty"`

	// Underlying is the shape a type declaration carries: "struct",
	// "interface", or the type its definition names. Every other kind leaves
	// it empty.
	Underlying string `json:"underlying,omitempty"`

	// Fields are the exported fields of a struct, or the methods of an
	// interface, in name order.
	Fields []apiField `json:"fields,omitempty"`
}

// apiField is one exported field of a struct, or one method of an interface.
// The exported fields of a type are as much a promise to a consumer as a func
// signature is, so the release report covers them too.
type apiField struct {
	// Name is the field name, and for an embedded field the name that reaches
	// it.
	Name string `json:"name"`

	// Type is the field type, and for an interface method its signature.
	Type string `json:"type"`

	// Tag is the struct tag, unmodified, and is empty when there is none.
	Tag string `json:"tag,omitempty"`

	// Embedded reports a field declared without a name of its own.
	Embedded bool `json:"embedded,omitempty"`
}

// The changes a field of a type undergoes between two revisions.
const (
	fieldAdded   = "added"
	fieldChanged = "changed"
	fieldRemoved = "removed"
)

// apiFieldChange is one exported field a release adds to, drops from, or
// reshapes on a type both revisions carry.
type apiFieldChange struct {
	Name   string `json:"name"`
	Change string `json:"change"`

	// Old and New are the field on either side. An added field has no Old,
	// and a removed one has no New.
	Old *apiField `json:"old,omitempty"`
	New *apiField `json:"new,omitempty"`
}

// apiTypeChange is a type both revisions carry whose exported fields moved.
type apiTypeChange struct {
	Key     string `json:"key"`
	Package string `json:"package"`
	Name    string `json:"name"`

	// Underlying is the shape the type keeps across both revisions.
	Underlying string `json:"underlying"`

	Fields   []apiFieldChange `json:"fields"`
	Breaking bool             `json:"breaking"`
}

// IsInterface reports whether the type is an interface, whose fields form a
// method set and carry no data.
func (c apiTypeChange) IsInterface() bool {
	return c.Underlying == "interface"
}

// String renders the declaration as it reads in source, a type with its shape.
func (s apiSymbol) String() string {
	if s.Signature != "" {
		return tidySignature(s.Signature)
	}
	if s.Underlying != "" {
		return s.Kind + " " + s.Name + " " + s.Underlying
	}
	return s.Kind + " " + s.Name
}

// tidySignature drops the one parameter name that never carries information:
// "ctx context.Context" reads as "context.Context", since the type says
// everything the conventional name repeats. Every other name/type pair stays
// as declared.
func tidySignature(signature string) string {
	signature = strings.ReplaceAll(signature, "(ctx context.Context", "(context.Context")
	return strings.ReplaceAll(signature, ", ctx context.Context", ", context.Context")
}

// qualifySignature prefixes a signature with the receiver the symbol name
// carries. A compared signature starts with the bare method name, so two
// methods named alike on different types of the same package would otherwise
// read as the same change.
func qualifySignature(name, signature string) string {
	if _, method, ok := strings.Cut(name, "."); ok && strings.HasPrefix(signature, method+" ") {
		return name + strings.TrimPrefix(signature, method)
	}
	return signature
}

// apiChange is an exported symbol whose signature moved between two revisions.
type apiChange struct {
	Key     string `json:"key"`
	Package string `json:"package"`
	Name    string `json:"name"`

	// Exported reports whether the symbol is part of the API.
	Exported bool `json:"exported"`

	// Old and New are the signature before and after, with parameter names
	// removed; the comparison runs on that form.
	Old string `json:"old"`
	New string `json:"new"`
}

// apiDiff is the exported symbol difference between a release tag and the
// working tree of a module, as reported by "splint diff".
type apiDiff struct {
	Removed []apiSymbol `json:"removed"`
	Added   []apiSymbol `json:"added"`
	Changed []apiChange `json:"changed"`

	// Types are the types both revisions carry whose exported fields moved,
	// which is the data model the release changes. An older tool reports
	// none, and the report leaves the section out.
	Types []apiTypeChange `json:"types"`

	Breaking bool `json:"breaking"`

	// Deps are the go.mod requirements the release adds, moves to another
	// version, or drops. Nothing here breaks a consumer: a dependency is not
	// API, so the report lists it and it does not decide the version. The two
	// models supply this list directly; the tool's own report of them plays
	// no part.
	Deps []DiffResult `json:"-"`

	// Skipped records why the comparison did not run and is empty when it
	// did. A comparison that could not run is never breaking, so a module
	// with an unreadable API gets a patch, with the reason printed.
	Skipped string `json:"-"`
}

// ExportedAdded returns the added symbols a consumer can see; a release
// counts these and ExportedRemoved. The report lists everything else, at no
// cost.
func (d apiDiff) ExportedAdded() []apiSymbol { return exportedOnly(d.Added) }

// ExportedRemoved returns the removed symbols that belong to the API.
func (d apiDiff) ExportedRemoved() []apiSymbol { return exportedOnly(d.Removed) }

// exportedOnly returns the symbols another module could reach.
func exportedOnly(symbols []apiSymbol) []apiSymbol {
	out := make([]apiSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.Exported {
			out = append(out, symbol)
		}
	}
	return out
}

// Summary describes the difference in one line.
func (d apiDiff) Summary() string {
	if d.Skipped != "" {
		return "api: not compared, " + d.Skipped
	}

	added, changed, removed := d.FieldCounts()
	summary := fmt.Sprintf("api: %d added, %d changed, %d removed", len(d.Added), len(d.Changed), len(d.Removed))
	if added+changed+removed > 0 {
		summary += fmt.Sprintf("; fields: %d added, %d changed, %d removed", added, changed, removed)
	}
	return summary
}

// BreakingFields returns how many exported fields moved in a way that costs a
// consumer something. That is every field a type loses or reshapes, and on an
// interface the methods it gains as well, since each of those stops an
// implementor compiling.
func (d apiDiff) BreakingFields() int {
	count := 0
	for _, change := range d.Types {
		for _, field := range change.Fields {
			if field.Change != fieldAdded || change.IsInterface() {
				count++
			}
		}
	}
	return count
}

// FieldCounts returns how many exported fields the release adds, reshapes and
// takes away, across every type it touches.
func (d apiDiff) FieldCounts() (added, changed, removed int) {
	for _, change := range d.Types {
		for _, field := range change.Fields {
			switch field.Change {
			case fieldAdded:
				added++
			case fieldChanged:
				changed++
			case fieldRemoved:
				removed++
			}
		}
	}
	return added, changed, removed
}

// Symbols returns the symbols behind the summary, one per line, in the order
// the report lists them: what the release adds first, what it takes away last.
func (d apiDiff) Symbols() []string {
	var lines []string
	for _, symbol := range d.Added {
		lines = append(lines, "+ "+symbol.Key)
	}
	for _, change := range d.Changed {
		lines = append(lines, "~ "+change.Key, "    "+change.Old, "    "+change.New)
	}
	for _, symbol := range d.Removed {
		lines = append(lines, "- "+symbol.Key)
	}
	return lines
}

// apiDiffSinceTag compares the exported API of the working tree in dir against
// the same module at tag.
func apiDiffSinceTag(dir, tag string) apiDiff {
	return apiDiffBetween(dir, tag, "")
}

// apiModels holds the model of every revision a run has read, so a chain of
// comparisons reads each revision once. A release is the new side of one
// comparison and the old side of the next, which without this is two unpacked
// archives and two extractions for the same tag.
//
// The models live below one temporary directory, which Close removes.
type apiModels struct {
	work   string
	models map[string]string
	errs   map[string]error
	next   int

	// empty is the model of a revision that does not exist, written the first
	// time a comparison measures from the start of history.
	empty string

	// disk is the cache the models of commits outlive the run in.
	disk *verdictCache
}

// newAPIModels opens a cache, backed by the one on disk unless cached is
// false. The caller closes it once the comparisons that share it finish.
func newAPIModels(cached bool) (*apiModels, error) {
	work, err := os.MkdirTemp("", "worktree-api-")
	if err != nil {
		return nil, err
	}
	return &apiModels{
		work:   work,
		models: map[string]string{},
		errs:   map[string]error{},
		disk:   openVerdictCache(cached),
	}, nil
}

// Close removes the models the cache holds.
func (m *apiModels) Close() error {
	return os.RemoveAll(m.work)
}

// model returns the path of the model of one revision of the module in dir,
// reading it on first use. A revision that fails to read stays failed, so the
// report names a tag missing from the repository once and does not unpack it
// again for every range naming it.
func (m *apiModels) model(dir, ref string) (string, error) {
	// The cache skips the working tree: the run reads it where it stands, as
	// the one revision that can change under a run.
	if ref == "" {
		return extractRef(dir, "", filepath.Join(m.work, m.name()))
	}

	key := dir + "\x00" + ref
	if model, ok := m.models[key]; ok {
		return model, nil
	}
	if err, ok := m.errs[key]; ok {
		return "", err
	}

	// A commit an earlier run read comes back from disk, skipping the unpack
	// and the model step, so a report over the whole history does not scan the
	// whole history twice.
	work := filepath.Join(m.work, m.name())
	entry := m.disk.path(dir, ref)
	if model := work + ".json"; m.disk.load(entry, model) {
		m.models[key] = model
		return model, nil
	}

	model, err := extractRef(dir, ref, work)
	if err != nil {
		m.errs[key] = err
		return "", err
	}
	m.disk.store(entry, model)
	m.models[key] = model
	return model, nil
}

// name returns a directory name no other revision in the cache is using.
func (m *apiModels) name() string {
	m.next++
	return strconv.Itoa(m.next)
}

// base returns the model of the revision a comparison measures from. An
// empty ref is the start of history, a revision no one can read; base models
// it as a module holding no packages, so a first release reports everything
// it exports as added.
func (m *apiModels) base(dir, ref string) (string, error) {
	if ref != "" {
		return m.model(dir, ref)
	}
	if m.empty != "" {
		return m.empty, nil
	}

	// A model is the list of packages of a revision, so a revision
	// that holds none of them is the empty list.
	path := filepath.Join(m.work, "empty.json")
	if err := os.WriteFile(path, []byte("[]\n"), 0o644); err != nil {
		return "", err
	}
	m.empty = path
	return path, nil
}

// diff compares the exported API of two revisions of the module in dir, reading
// each through the cache. This works as apiDiffBetween with the revisions
// remembered.
func (m *apiModels) diff(dir, oldRef, newRef string) apiDiff {
	if _, err := exec.LookPath("splint"); err != nil {
		return apiDiff{Skipped: "splint is not installed"}
	}

	oldModel, err := m.base(dir, oldRef)
	if err != nil {
		return apiDiff{Skipped: fmt.Sprintf("read %s: %v", baseName(oldRef), err)}
	}
	newModel, err := m.model(dir, newRef)
	if err != nil {
		return apiDiff{Skipped: fmt.Sprintf("read %s: %v", revName(newRef), err)}
	}
	return diffModels(oldModel, newModel)
}

// apiDiffBetween compares the exported API of two revisions of the module in
// dir. An empty oldRef is the start of history and an empty newRef is the
// working tree; git archive unpacks a named revision into a temporary
// directory with no checkout, so the working tree stays untouched either way.
//
// Every reason the comparison cannot run comes back as a Skipped result and
// never as an error: the report counts an unreadable API as non breaking and
// says so, and a missing tool does not stop a run.
func apiDiffBetween(dir, oldRef, newRef string) apiDiff {
	if _, err := exec.LookPath("splint"); err != nil {
		return apiDiff{Skipped: "splint is not installed"}
	}

	models, err := newAPIModels(true)
	if err != nil {
		return apiDiff{Skipped: err.Error()}
	}
	defer func() { _ = models.Close() }()

	return models.diff(dir, oldRef, newRef)
}

// diffModels compares two models extractRef already wrote.
//
// A splint from before the diff verb reads "diff" as a lint pattern and
// stops on the --old flag it does not define; the flag error check detects
// exactly that case: the tool exists and the command does not.
//
// diffModels compares the go.mod each model carries itself, with no call to
// the tool, so the dependency report stays the same whichever splint the
// system has, and this module tests the rule behind it on its own.
func diffModels(oldModel, newModel string) apiDiff {
	out, err := exec.Command("splint", "diff", "--old", oldModel, "--new", newModel, "--json", "--include-unexported").CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "flag provided but not defined") {
			return apiDiff{Skipped: "the installed splint has no diff command"}
		}
		return apiDiff{Skipped: fmt.Sprintf("splint diff: %v", firstLine(string(out)))}
	}

	var diff apiDiff
	if err := json.Unmarshal(out, &diff); err != nil {
		return apiDiff{Skipped: fmt.Sprintf("splint diff: %v", err)}
	}
	diff.Deps = Diff(modelRequires(oldModel), modelRequires(newModel))
	return diff
}

// extractRef writes the model of one revision of the module in dir, unpacking
// it below work first for a named revision, and returns the model path. An
// empty ref means the working tree, which splint reads where it stands.
func extractRef(dir, ref, work string) (string, error) {
	source := dir
	if ref != "" {
		source = work
		if err := gitArchive(dir, ref, work); err != nil {
			return "", err
		}
	}

	model := work + ".json"
	if err := splintExtract(source, model); err != nil {
		return "", err
	}
	return model, nil
}

// revName names a revision for a message, where the working tree has no name
// of its own.
func revName(ref string) string {
	if ref == "" {
		return "the working tree"
	}
	return ref
}

// baseName names the revision a comparison measures from, where an empty ref
// means the start of history and never the working tree.
func baseName(ref string) string {
	if ref == "" {
		return "the start of history"
	}
	return ref
}

// splintExtract writes the model of the module in dir to out. The linters
// are off: this run is the extract, and a lint finding does not fail it.
//
// splint reads the model from the syntax tree and discards package load
// errors, so this works on a source tree no build ever touched, such as the
// unpacked tag. The environment reflects that: with no module cache to find
// the requirements in, the go tool would otherwise try to download them.
//
// The command leaves out sources. They carry every function body with them
// and multiply the size of the model, and the model records the fields a
// report draws on either way.
func splintExtract(dir, out string) error {
	cmd := exec.Command("splint", "--linters", "none", "-i", dir, "--output", out, "./...")
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", firstLine(string(output)))
	}
	return nil
}

// gitArchive unpacks the module in dir as it stands at ref into dest. For a
// module below the root of its repository the archive names its path, so git
// writes only its own subtree.
func gitArchive(dir, ref, dest string) error {
	root, rel, err := repoPaths(dir)
	if err != nil {
		return err
	}
	if rel != "." {
		ref += ":" + rel
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	cmd := exec.Command("git", "archive", "--format=tar", ref)
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// The copy drains the reader before the wait, so git never blocks on a
	// pipe nobody reads.
	untarErr := untar(stdout, dest)
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return err
	}
	return untarErr
}

// untar writes the regular files of a tar stream below dest. It returns an
// error for any entry pointing outside dest and follows none.
func untar(r io.Reader, dest string) error {
	reader := tar.NewReader(r)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		path := filepath.Join(dest, filepath.FromSlash(header.Name))
		if !isSubpath(dest, path) {
			return fmt.Errorf("archive entry outside the destination: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, reader)
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
		}
	}
}

// firstLine returns the first non empty line of command output: the line
// where the go and git tools put the reason something failed.
func firstLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "no output"
}
