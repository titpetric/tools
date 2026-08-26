package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/titpetric/tools/worktree/components"
)

func getGitStatus(dir string) *gitStatus {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	// Find git root to determine relative path for scoping
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absDir
	rootOut, err := cmd.Output()
	if err != nil {
		return nil
	}
	gitRoot := strings.TrimSpace(string(rootOut))

	// Relative path from git root to module dir (for scoping)
	relPath, err := filepath.Rel(gitRoot, absDir)
	if err != nil {
		return nil
	}
	isSubdir := relPath != "."

	st := &gitStatus{}

	// Count modified files (working tree + staged)
	args := []string{"status", "--porcelain"}
	if isSubdir {
		args = append(args, "--", relPath)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				st.Modified++
			}
		}
	}

	// Get diff --numstat output (unstaged + staged combined)
	args = []string{"diff", "--numstat"}
	if isSubdir {
		args = append(args, "--", relPath)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = gitRoot
	out, err = cmd.Output()
	if err == nil {
		st.DiffLines = append(st.DiffLines, parseNumstat(string(out), relPath)...)
	}

	// Also include staged changes
	args = []string{"diff", "--cached", "--numstat"}
	if isSubdir {
		args = append(args, "--", relPath)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = gitRoot
	out, err = cmd.Output()
	if err == nil {
		st.DiffLines = append(st.DiffLines, parseNumstat(string(out), relPath)...)
	}

	// Count unpushed commits (scoped to subtree if applicable)
	args = []string{"log", "--oneline", "@{u}..HEAD"}
	if isSubdir {
		args = append(args, "--", relPath)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = gitRoot
	out, err = cmd.Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				st.Unpushed++
			}
		}
	}

	if st.Unpushed == 0 && st.Modified == 0 && len(st.DiffLines) == 0 {
		return nil
	}
	return st
}

// parseNumstat parses git diff --numstat output into "+X/-Y filename" format
func parseNumstat(output, relPath string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		ins, del, file := fields[0], fields[1], fields[2]
		// Strip relPath prefix if present
		if relPath != "." && strings.HasPrefix(file, relPath+"/") {
			file = strings.TrimPrefix(file, relPath+"/")
		}
		result = append(result, fmt.Sprintf("%s +%s/-%s", file, ins, del))
	}
	return result
}

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

func getGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func latestGitTag(dir string) string {
	cmd := exec.Command("git", "tag", "--list", "--sort=-v:refname", "v*")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func commitsSinceTag(dir, tag string) int {
	cmd := exec.Command("git", "rev-list", "--count", tag+"..HEAD", "--", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func commitMessagesSinceTag(dir, tag string) []string {
	cmd := exec.Command("git", "log", "--oneline", tag+"..HEAD", "--", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var msgs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			msgs = append(msgs, line)
		}
	}
	return msgs
}

// repoPaths returns the root of the git repository holding dir and the path of
// dir below it, which is "." when dir is the root itself.
//
// Commands taking a path are run from the root with that path, never from the
// directory itself: git reads a pathspec, and the tree of "<rev>:<path>", as
// relative to the current directory, so running them a level down would look
// for the path twice over.
func repoPaths(dir string) (root, rel string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = abs
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("not a git repository: %s", dir)
	}
	root = strings.TrimSpace(string(out))
	rel, err = filepath.Rel(root, abs)
	if err != nil {
		return "", "", err
	}
	return root, filepath.ToSlash(rel), nil
}

// commitLog is one commit of a module, as a release note lists it.
type commitLog struct {
	Hash    string
	Subject string
}

// commitLogSinceTag returns the commits made to the module in dir since tag,
// newest first. With no tag the whole history of the module is returned, which
// is what a first release covers.
func commitLogSinceTag(dir, tag string) []commitLog {
	return commitLogBetween(dir, tag, "HEAD")
}

// commitLogBetween returns the commits made to the module in dir between two
// revisions, newest first. An empty from is the start of history, which is
// what a first release covers.
func commitLogBetween(dir, from, to string) []commitLog {
	if to == "" {
		to = "HEAD"
	}

	args := []string{"log", "--format=%h%x00%s"}
	if from != "" {
		args = append(args, from+".."+to)
	} else {
		args = append(args, to)
	}
	// The pathspec is read relative to the working directory, so this is the
	// module's own history rather than the whole repository's.
	args = append(args, "--", ".")

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var commits []commitLog
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		hash, subject, ok := strings.Cut(line, "\x00")
		if !ok {
			continue
		}
		commits = append(commits, commitLog{Hash: hash, Subject: subject})
	}
	return commits
}

// repoURL returns the address a browser opens the repository of dir at, or an
// empty string when there is no origin to derive one from.
//
// Only "git remote get-url" is ever run: reading the address of a remote is
// all that is wanted here, and nothing about the repository's remotes is
// written.
func repoURL(dir string) string {
	return browsableRemote(firstCommandLine(dir, "git", "remote", "get-url", "origin"))
}

// browsableRemote rewrites a git remote as the https address of the same
// repository. The scp form "git@host:path" and an ssh URL both become
// "https://host/path"; anything else that is not already http is left out,
// since there is nothing to link to.
func browsableRemote(remote string) string {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	switch {
	case remote == "":
		return ""
	case strings.HasPrefix(remote, "https://"), strings.HasPrefix(remote, "http://"):
		return remote
	case strings.HasPrefix(remote, "ssh://"):
		remote = strings.TrimPrefix(remote, "ssh://")
	case strings.Contains(remote, "://"):
		return ""
	default:
		// The scp form separates the host from the path with a colon.
		host, path, ok := strings.Cut(remote, ":")
		if !ok || path == "" {
			return ""
		}
		remote = host + "/" + path
	}

	if _, address, ok := strings.Cut(remote, "@"); ok {
		remote = address
	}
	if !strings.Contains(remote, "/") {
		return ""
	}
	return "https://" + remote
}

func getGitHubIssues(dir string) []components.Issue {
	data, err := cachedGHIssueList(dir)
	if err != nil {
		return nil
	}
	return parseGHIssueList(data)
}

type ghIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

func parseGHIssueList(data []byte) []components.Issue {
	var raw []ghIssue
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var issues []components.Issue
	for _, r := range raw {
		date := r.CreatedAt
		if t, err := time.Parse(time.RFC3339, date); err == nil {
			date = t.Format("2006-01-02")
		}
		issues = append(issues, components.Issue{
			ID:    fmt.Sprintf("#%d", r.Number),
			Title: r.Title,
			Date:  date,
		})
	}
	return issues
}

func ghIssueCachePath(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	h := sha256.Sum256([]byte(absDir))
	name := "worktree-gh-issues-" + hex.EncodeToString(h[:8])
	return filepath.Join(os.TempDir(), name)
}

func cachedGHIssueList(dir string) ([]byte, error) {
	cachePath := ghIssueCachePath(dir)

	if info, err := os.Stat(cachePath); err == nil {
		if time.Since(info.ModTime()) < time.Hour {
			data, err := os.ReadFile(cachePath)
			if err == nil {
				return data, nil
			}
		}
	}

	cmd := exec.Command("gh", "issue", "list", "--json", "number,title,createdAt", "--limit", "20", "--state", "open")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	_ = os.WriteFile(cachePath, out, 0o644)
	return out, nil
}
