package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	Hash      string
	Subject   string
	Published bool
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

	args := []string{"log", "--format=%H%x00%h%x00%s"}
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

	published := publishedCommits(dir)
	var commits []commitLog
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		full, rest, ok := strings.Cut(line, "\x00")
		if !ok {
			continue
		}
		hash, subject, ok := strings.Cut(rest, "\x00")
		if !ok {
			continue
		}
		commits = append(commits, commitLog{Hash: hash, Subject: subject, Published: published[full]})
	}
	return commits
}

// publishedCommits returns the commits reachable from a remote-tracking ref.
// A local-only commit has no browser page yet, even when origin itself has a
// browsable URL, so verdict must leave its hash unlinked.
func publishedCommits(dir string) map[string]bool {
	cmd := exec.Command("git", "rev-list", "--remotes", "--", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	published := make(map[string]bool)
	for hash := range strings.FieldsSeq(string(out)) {
		published[hash] = true
	}
	return published
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
