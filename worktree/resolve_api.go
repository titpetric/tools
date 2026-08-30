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

// apiSymbol is one exported declaration, as "go-fsck diff" reports it.
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

	// Signature is a func as it is declared, and is empty for every other
	// kind.
	Signature string `json:"signature,omitempty"`

	// Underlying is the shape a type is declared with: "struct", "interface",
	// or the type it is defined as. It is empty for every other kind.
	Underlying string `json:"underlying,omitempty"`

	// Fields are the exported fields of a struct, or the methods of an
	// interface, in name order.
	Fields []apiField `json:"fields,omitempty"`
}

// apiField is one exported field of a struct, or one method of an interface.
// The exported fields of a type are as much a promise to a consumer as a func
// signature is, so a release is reported on them too.
type apiField struct {
	// Name is the field name, and for an embedded field the name it is
	// reached by.
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

	// Old and New are the field on either side. Old is absent for a field that
	// was added, and New for one that was removed.
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

// IsInterface reports whether the type is an interface, whose fields are read
// as a method set rather than as data.
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

// apiChange is an exported symbol whose signature moved between two revisions.
type apiChange struct {
	Key     string `json:"key"`
	Package string `json:"package"`
	Name    string `json:"name"`

	// Old and New are the signature before and after, with parameter names
	// removed, which is what they were compared on.
	Old string `json:"old"`
	New string `json:"new"`
}

// apiDiff is the exported symbol difference between a release tag and the
// working tree of a module, as reported by "go-fsck diff".
type apiDiff struct {
	Removed []apiSymbol `json:"removed"`
	Added   []apiSymbol `json:"added"`
	Changed []apiChange `json:"changed"`

	// Types are the types both revisions carry whose exported fields moved,
	// which is the data model the release changes. An older go-fsck reports
	// none, and the report leaves the section out.
	Types []apiTypeChange `json:"types"`

	Breaking bool `json:"breaking"`

	// Skipped records why the comparison did not run and is empty when it
	// did. A comparison that could not run is never breaking, so a module
	// whose API cannot be read gets a patch, with the reason printed.
	Skipped string `json:"-"`
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
	// time a comparison is measured from the start of history.
	empty string

	// disk is the cache the models of commits outlive the run in.
	disk *verdictCache
}

// newAPIModels opens a cache, backed by the one on disk unless it was turned
// off. The caller closes it once the comparisons that share it are done.
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
// reading it the first time it is asked for. A revision that could not be read
// stays failed, so a tag missing from the repository is reported once and not
// unpacked again for every range naming it.
func (m *apiModels) model(dir, ref string) (string, error) {
	// The working tree is not cached: it is read where it stands, and it is
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

	// A commit read on an earlier run is read back rather than unpacked and
	// modelled again, which is what keeps a report over the whole history from
	// scanning the whole history twice.
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

// base returns the model of the revision a comparison is measured from. An
// empty ref is the start of history, which is not a revision anyone can read
// and is modelled as a module holding no packages, so a first release reports
// everything it exports as added rather than reporting nothing at all.
func (m *apiModels) base(dir, ref string) (string, error) {
	if ref != "" {
		return m.model(dir, ref)
	}
	if m.empty != "" {
		return m.empty, nil
	}

	// A go-fsck model is the list of packages of a revision, so a revision
	// that holds none of them is the empty list.
	path := filepath.Join(m.work, "empty.json")
	if err := os.WriteFile(path, []byte("[]\n"), 0o644); err != nil {
		return "", err
	}
	m.empty = path
	return path, nil
}

// diff compares the exported API of two revisions of the module in dir, reading
// each through the cache. It is apiDiffBetween with the revisions remembered.
func (m *apiModels) diff(dir, oldRef, newRef string) apiDiff {
	if _, err := exec.LookPath("go-fsck"); err != nil {
		return apiDiff{Skipped: "go-fsck is not installed"}
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
// working tree; a named revision is unpacked into a temporary directory rather
// than checked out, so the working tree is left alone either way.
//
// Every reason the comparison cannot run comes back as a Skipped result rather
// than an error: an unreadable API is treated as non breaking and said to be,
// so a missing tool does not stop a run.
func apiDiffBetween(dir, oldRef, newRef string) apiDiff {
	if _, err := exec.LookPath("go-fsck"); err != nil {
		return apiDiff{Skipped: "go-fsck is not installed"}
	}

	models, err := newAPIModels(true)
	if err != nil {
		return apiDiff{Skipped: err.Error()}
	}
	defer func() { _ = models.Close() }()

	return models.diff(dir, oldRef, newRef)
}

// diffModels compares two models that have already been extracted.
func diffModels(oldModel, newModel string) apiDiff {
	out, err := exec.Command("go-fsck", "diff", "--old", oldModel, "--new", newModel, "--json").CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Unknown command") {
			return apiDiff{Skipped: "the installed go-fsck has no diff command"}
		}
		return apiDiff{Skipped: fmt.Sprintf("go-fsck diff: %v", firstLine(string(out)))}
	}

	var diff apiDiff
	if err := json.Unmarshal(out, &diff); err != nil {
		return apiDiff{Skipped: fmt.Sprintf("go-fsck diff: %v", err)}
	}
	return diff
}

// extractRef writes the model of one revision of the module in dir, unpacking
// it below work first when it is a named one, and returns the model path. An
// empty ref is the working tree, which is read where it stands.
func extractRef(dir, ref, work string) (string, error) {
	source := dir
	if ref != "" {
		source = work
		if err := gitArchive(dir, ref, work); err != nil {
			return "", err
		}
	}

	model := work + ".json"
	if err := goFsckExtract(source, model); err != nil {
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

// baseName names the revision a comparison is measured from, where an empty ref
// is the start of history rather than the working tree.
func baseName(ref string) string {
	if ref == "" {
		return "the start of history"
	}
	return ref
}

// goFsckExtract writes the model of the module in dir to out.
//
// The model is read from the syntax tree and package load errors are
// discarded, so this works on a source tree that was never built, which is
// what the unpacked tag is. The environment says so: with no module cache to
// find the requirements in, the go tool would otherwise try to download them.
//
// Sources are not asked for. They carry every function body with them and
// multiply the size of the model, and the fields a report is written from are
// recorded either way.
func goFsckExtract(dir, out string) error {
	cmd := exec.Command("go-fsck", "extract", "-i", dir, "-r", "-o", out)
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", firstLine(string(output)))
	}
	return nil
}

// gitArchive unpacks the module in dir as it stands at ref into dest. A module
// that is not at the root of its repository is asked for by path, so only its
// own subtree is written.
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

	// The reader is drained before waiting, so git is never left blocked on a
	// pipe nobody is reading.
	untarErr := untar(stdout, dest)
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return err
	}
	return untarErr
}

// untar writes the regular files of a tar stream below dest. Entries pointing
// outside dest are refused rather than followed.
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

// firstLine returns the first non empty line of command output, which is where
// the go and git tools put the reason something failed.
func firstLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "no output"
}
