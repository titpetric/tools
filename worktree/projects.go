package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/worktree/config"
)

// findScanRoot returns the nearest current or parent directory holding one of
// the configured root markers. With no markers, or none found, the walk starts
// where it was asked to.
func findScanRoot(start string, markers []string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Abs(start)
		}
		dir = parent
	}
}

func findProjects(root string, scan config.Scan) ([]projectDir, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	projects := make(map[string]*projectDir)
	project := func(dir string) (*projectDir, error) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		if projects[dir] == nil {
			projects[dir] = &projectDir{}
		}
		return projects[dir], nil
	}

	s := newScanner(scan, root)
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if s.skip(path, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		switch info.Name() {
		case ".git":
			p, err := project(filepath.Dir(path))
			if err != nil {
				return err
			}
			p.GitRepo = true
			if info.IsDir() {
				return filepath.SkipDir
			}
		case "go.mod":
			if !info.IsDir() {
				p, err := project(filepath.Dir(path))
				if err != nil {
					return err
				}
				p.GoModule = true
			}
		case "go.work":
			if info.IsDir() {
				break
			}
			dirs, err := parseGoWork(path)
			if err != nil {
				return err
			}
			for _, dir := range dirs {
				p, err := project(filepath.Join(filepath.Dir(path), dir))
				if err != nil {
					return err
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), dir, "go.mod")); err == nil {
					p.GoModule = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]projectDir, 0, len(projects))
	for dir, p := range projects {
		// A git repository that holds no go module is only listed when the
		// configuration asks for one.
		if !p.GoModule && !(p.GitRepo && scan.EnableGitRepos) {
			continue
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		if rel == "." {
			p.Path = "."
		} else {
			p.Path = filepath.Join(".", rel)
			if !strings.HasPrefix(p.Path, "."+string(filepath.Separator)) && !filepath.IsAbs(p.Path) {
				p.Path = "." + string(filepath.Separator) + p.Path
			}
		}
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}
