package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/titpetric/tools/worktree/components"
)

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
