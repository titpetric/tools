package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

	// Definition is a type as it is declared, without its doc comment, and is
	// only read when the comparison asked for sources.
	Definition string `json:"definition,omitempty"`
}

// String renders the declaration the way it reads in source.
func (s apiSymbol) String() string {
	if s.Signature != "" {
		return s.Signature
	}
	return s.Kind + " " + s.Name
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
	Removed  []apiSymbol `json:"removed"`
	Added    []apiSymbol `json:"added"`
	Changed  []apiChange `json:"changed"`
	Breaking bool        `json:"breaking"`

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
	return fmt.Sprintf("api: %d removed, %d changed, %d added", len(d.Removed), len(d.Changed), len(d.Added))
}

// Symbols returns the symbols behind the summary, one per line, removals and
// signature changes first.
func (d apiDiff) Symbols() []string {
	var lines []string
	for _, symbol := range d.Removed {
		lines = append(lines, "- "+symbol.Key)
	}
	for _, change := range d.Changed {
		lines = append(lines, "~ "+change.Key, "    "+change.Old, "    "+change.New)
	}
	for _, symbol := range d.Added {
		lines = append(lines, "+ "+symbol.Key)
	}
	return lines
}

// apiDiffSinceTag compares the exported API of the working tree in dir against
// the same module at tag.
func apiDiffSinceTag(dir, tag string) apiDiff {
	return apiDiffBetween(dir, tag, "", false)
}

// apiDiffBetween compares the exported API of two revisions of the module in
// dir. An empty ref is the working tree; a named one is unpacked into a
// temporary directory rather than checked out, so the working tree is left
// alone either way.
//
// Sources are extracted only when the caller means to print the body of a
// type: they carry every function body with them and multiply the size of the
// model.
//
// Every reason the comparison cannot run comes back as a Skipped result rather
// than an error: an unreadable API is treated as non breaking and said to be,
// so a missing tool does not stop a run.
func apiDiffBetween(dir, oldRef, newRef string, sources bool) apiDiff {
	if oldRef == "" {
		return apiDiff{Skipped: "no release tag to compare against"}
	}
	if _, err := exec.LookPath("go-fsck"); err != nil {
		return apiDiff{Skipped: "go-fsck is not installed"}
	}

	work, err := os.MkdirTemp("", "worktree-api-")
	if err != nil {
		return apiDiff{Skipped: err.Error()}
	}
	defer func() { _ = os.RemoveAll(work) }()

	oldModel, err := extractRef(dir, oldRef, filepath.Join(work, "old"), sources)
	if err != nil {
		return apiDiff{Skipped: fmt.Sprintf("read %s: %v", oldRef, err)}
	}
	newModel, err := extractRef(dir, newRef, filepath.Join(work, "new"), sources)
	if err != nil {
		return apiDiff{Skipped: fmt.Sprintf("read %s: %v", revName(newRef), err)}
	}

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
func extractRef(dir, ref, work string, sources bool) (string, error) {
	source := dir
	if ref != "" {
		source = work
		if err := gitArchive(dir, ref, work); err != nil {
			return "", err
		}
	}

	model := work + ".json"
	if err := goFsckExtract(source, model, sources); err != nil {
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

// goFsckExtract writes the model of the module in dir to out.
//
// The model is read from the syntax tree and package load errors are
// discarded, so this works on a source tree that was never built, which is
// what the unpacked tag is. The environment says so: with no module cache to
// find the requirements in, the go tool would otherwise try to download them.
func goFsckExtract(dir, out string, sources bool) error {
	args := []string{"extract", "-i", dir, "-r", "-o", out}
	if sources {
		args = append(args, "--include-sources")
	}
	cmd := exec.Command("go-fsck", args...)
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
