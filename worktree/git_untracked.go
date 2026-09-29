package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

// untrackedEntry is one path git reports as untracked: a file, or a directory
// standing in for everything below it. Path carries a trailing "/" when it is
// a directory, and the counts are then of the whole subtree.
type untrackedEntry struct {
	Path  string
	Dirs  int
	Files int
	Lines int
}

// getUntrackedFiles lists the untracked paths of the module in dir, each with
// the lines it adds.
//
// By default a directory holding nothing tracked stands in for the files below
// it, so a new subtree costs one line rather than one per file. Passing all
// lists every file, which is what -v and --all ask for.
func getUntrackedFiles(dir string, all bool) []string {
	root, rel, err := repoPaths(dir)
	if err != nil {
		return nil
	}

	var files []untrackedEntry
	for _, path := range listUntracked(root, rel, false) {
		files = append(files, untrackedEntry{
			Path:  path,
			Files: 1,
			Lines: countLines(filepath.Join(dir, path)),
		})
	}

	entries := files
	if !all {
		entries = collapseUntracked(listUntracked(root, rel, true), files)
	}

	var result []string
	for _, entry := range entries {
		result = append(result, formatUntrackedEntry(entry))
	}
	return result
}

// listUntracked returns the untracked paths of the module below rel, relative
// to the module itself. Collapsing asks git to report a directory holding
// nothing tracked as one entry, with a trailing "/", instead of listing it.
func listUntracked(root, rel string, collapse bool) []string {
	args := []string{"ls-files", "--others", "--exclude-standard"}
	if collapse {
		args = append(args, "--directory", "--no-empty-directory")
	}
	if rel != "." {
		args = append(args, "--", rel)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if rel != "." {
			line = strings.TrimPrefix(line, rel+"/")
		}
		paths = append(paths, line)
	}
	return paths
}

// collapseUntracked folds each untracked file into the collapsed path standing
// in for it, summing the files, the lines they hold, and the directories they
// lie in below that path. A collapsed path that is a file has only itself to
// account for and is carried over as it is.
func collapseUntracked(collapsed []string, files []untrackedEntry) []untrackedEntry {
	var entries []untrackedEntry
	for _, path := range collapsed {
		if !strings.HasSuffix(path, "/") {
			entries = append(entries, findUntracked(files, path))
			continue
		}

		entry := untrackedEntry{Path: path}
		dirs := map[string]bool{}
		for _, file := range files {
			if !strings.HasPrefix(file.Path, path) {
				continue
			}
			entry.Files += file.Files
			entry.Lines += file.Lines
			if dir := filepath.Dir(file.Path); dir != strings.TrimSuffix(path, "/") {
				dirs[dir] = true
			}
		}
		entry.Dirs = len(dirs)
		entries = append(entries, entry)
	}
	return entries
}

// findUntracked returns the entry of one file, or an empty entry of that path
// when the two listings disagree about it.
func findUntracked(files []untrackedEntry, path string) untrackedEntry {
	for _, file := range files {
		if file.Path == path {
			return file
		}
	}
	return untrackedEntry{Path: path, Files: 1}
}

// formatUntrackedEntry renders one untracked path for the git state cell. A
// file is named with the lines it adds; a directory is named in orange, with
// what the subtree below it holds.
func formatUntrackedEntry(e untrackedEntry) string {
	if !strings.HasSuffix(e.Path, "/") {
		return fmt.Sprintf("%s %s+%d%s", e.Path, components.ColorGreen, e.Lines, components.ColorReset)
	}

	var counts []string
	if e.Dirs > 0 {
		counts = append(counts, "+"+plural(e.Dirs, "dir", "dirs"))
	}
	counts = append(counts,
		"+"+plural(e.Files, "file", "files"),
		fmt.Sprintf("+%d SLOC", e.Lines),
	)

	return fmt.Sprintf("%s%s%s %s%s%s",
		components.ColorDarkOrange, e.Path, components.ColorReset,
		components.ColorGreen, strings.Join(counts, ", "), components.ColorReset)
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}
