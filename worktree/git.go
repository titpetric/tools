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

func formatGitSummary(st *gitStatus) string {
	var parts []string
	if st.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("unpushed: %d commits", st.Unpushed))
	}
	if st.Modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", st.Modified))
	}
	return strings.Join(parts, ", ")
}
