package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// source is the tree a run reads and the git revision it compares with.
type source struct {
	// root is the source path as an absolute directory.
	root string

	// repo is the checkout the tree sits in, and base is the revision the
	// working tree is compared with.
	repo string
	base string
}

// change is one path the working tree changed, relative to the source path.
type change struct {
	// Path is the file as it is now; OldPath is where a rename moved it from.
	Path    string
	OldPath string
	Status  byte
}

// newSource resolves the tree and the revision it is compared with.
func newSource(ctx context.Context, opts Options) (*source, error) {
	root, err := filepath.Abs(opts.SourcePath)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", opts.SourcePath)
	}

	repo, err := git(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%s is not in a git checkout: %w", opts.SourcePath, err)
	}

	base := opts.Base
	if base == "" {
		base = "HEAD"
	}
	if _, err := git(ctx, root, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		return nil, fmt.Errorf("no revision %s: %w", base, err)
	}

	return &source{root: root, repo: strings.TrimSpace(repo), base: base}, nil
}

// module returns the module directory that holds dir's package, and the
// package's path below it.
//
// go test resolves a module from the directory it runs in. A tree with one
// go.mod at its root answers the root for every package. A tree holding a
// module per project, with no go.mod at its root, answers each package's own
// module, so a run over the whole tree starts every invocation inside the
// module that owns the package.
func (s *source) module(dir string) (string, string) {
	abs := filepath.Join(s.root, dir)
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			rel, err := filepath.Rel(d, abs)
			if err != nil {
				return d, "."
			}
			return d, rel
		}
		if parent := filepath.Dir(d); parent == d {
			return s.root, dir
		}
	}
}

// git runs git in dir and returns what it wrote.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// changes lists the Go files the working tree changed against the base.
//
// The list is relative to the source path, so a run in a subfolder sees the
// changes of that folder alone: --relative names each path from the directory
// the command started in, which is the source path.
func (s *source) changes(ctx context.Context) ([]change, error) {
	out, err := git(ctx, s.root, "diff", "--relative", "--name-status", "-M", "--diff-filter=ACDMRT", s.base, "--", ".")
	if err != nil {
		return nil, err
	}

	var changes []change
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		status := fields[0][0]
		one := change{Status: status, Path: fields[len(fields)-1]}
		if status == 'R' || status == 'C' {
			one.OldPath = fields[1]
		}
		if relevant(one.Path) {
			changes = append(changes, one)
		}
	}

	// A file git does not track is new to the tree in the same way an added
	// one is, and the diff above cannot see it.
	out, err = git(ctx, s.root, "ls-files", "--others", "--exclude-standard", "--", ".")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(strings.TrimSpace(out), "\n") {
		if path != "" && relevant(path) {
			changes = append(changes, change{Status: 'A', Path: path})
		}
	}

	return changes, nil
}

// relevant reports whether a changed path is Go source the tool reads.
//
// A testdata directory holds fixtures the Go toolchain does not build, and a
// vendor directory holds another author's source; neither is this tree's.
func relevant(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "vendor" || part == "testdata" {
			return false
		}
		if strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
			return false
		}
	}
	return true
}

// oldFile reads path as base had it, and reports false for a file base did not.
func (s *source) oldFile(ctx context.Context, path string) ([]byte, bool, error) {
	spec := s.base + ":./" + filepath.ToSlash(path)
	cmd := exec.CommandContext(ctx, "git", "-C", s.root, "show", spec)
	data, err := cmd.Output()
	if err != nil {
		// A file the revision does not hold is a file the change added, and
		// the diff already said so; there is no older side to compare with.
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 128 {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}
