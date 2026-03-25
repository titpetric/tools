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

func getUntrackedFiles(dir string) []string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absDir
	rootOut, err := cmd.Output()
	if err != nil {
		return nil
	}
	gitRoot := strings.TrimSpace(string(rootOut))

	relPath, err := filepath.Rel(gitRoot, absDir)
	if err != nil {
		return nil
	}

	args := []string{"ls-files", "--others", "--exclude-standard"}
	if relPath != "." {
		args = append(args, "--", relPath)
	}
	cmd = exec.Command("git", args...)
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		file := line
		if relPath != "." && strings.HasPrefix(file, relPath+"/") {
			file = strings.TrimPrefix(file, relPath+"/")
		}
		fullPath := filepath.Join(absDir, file)
		lineCount := countLines(fullPath)
		result = append(result, fmt.Sprintf("%s %s+%d%s", file, components.ColorGreen, lineCount, components.ColorReset))
	}
	return result
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

	_ = os.WriteFile(cachePath, out, 0644)
	return out, nil
}
