package main

import (
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

func pullRepos(w io.Writer, dirs []string, styled bool) {
	repos := make(map[string]struct{})
	for _, dir := range dirs {
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		repos[strings.TrimSpace(string(out))] = struct{}{}
	}

	paths := make([]string, 0, len(repos))
	for path := range repos {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var rows [][]string
	for _, path := range paths {
		remote := firstCommandLine(path, "git", "remote", "-v")
		branch := getGitBranch(path)
		before := firstCommandLine(path, "git", "rev-parse", "HEAD")
		cmd := exec.Command("git", "pull", "--quiet")
		cmd.Dir = path
		out, err := cmd.CombinedOutput()
		status := ""
		if err != nil {
			status = strings.TrimSpace(string(out))
			if status == "" {
				status = err.Error()
			}
		} else {
			after := firstCommandLine(path, "git", "rev-parse", "HEAD")
			if before == after {
				status = "Already up to date."
			} else {
				revision := after
				if before != "" {
					revision = before + ".." + after
				}
				count := firstCommandLine(path, "git", "rev-list", "--count", revision)
				if count == "1" {
					status = "Pulled 1 commit."
				} else if count != "" {
					status = "Pulled " + count + " commits."
				} else {
					status = "Updated."
				}
			}
		}
		color := components.ColorGreen
		if err != nil {
			color = components.ColorRed
		}
		rows = append(rows, []string{relPath(path), remote, branch, colorLines(status, color, styled)})
	}
	writeSimpleTable(w, []string{"Path", "Remote", "Branch", "Pull status"}, rows, styled)
}

func firstCommandLine(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.ReplaceAll(line, "\t", " ")
}
