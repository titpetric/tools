package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

func findGoWork() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, "go.work")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func findGoModDirs(root string) []string {
	var dirs []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "go.mod" && !info.IsDir() {
			dirs = append(dirs, "./"+filepath.Dir(path))
		}
		return nil
	})
	return dirs
}

func main() {
	update := flag.Bool("u", false, "update workspace dependencies to latest tags")
	puml := flag.Bool("puml", false, "output PlantUML dependency diagram to stdout")
	d2 := flag.Bool("d2", false, "output D2 dependency diagram to stdout")
	verbose := flag.Bool("v", false, "verbose output: show module column and dependency lists")
	flag.Parse()

	// Resolve optional path filter before chdir
	var filterPath string
	if flag.NArg() > 0 {
		abs, err := filepath.Abs(flag.Arg(0))
		if err != nil {
			log.Fatalf("failed to resolve path %s: %v", flag.Arg(0), err)
		}
		filterPath = abs
	}

	var modDirs []string
	goWorkPath, err := findGoWork()
	if err == nil {
		if err := os.Chdir(filepath.Dir(goWorkPath)); err != nil {
			log.Fatalf("failed to chdir to %s: %v", filepath.Dir(goWorkPath), err)
		}
		modDirs, err = parseGoWork("go.work")
		if err != nil {
			log.Fatalf("failed to parse go.work: %v", err)
		}
	} else {
		// Fallback: find all go.mod files in current directory and subfolders
		modDirs = findGoModDirs(".")
		if len(modDirs) == 0 {
			log.Fatalf("no go.work or go.mod found")
		}
	}

	// Map: module path -> dir
	modPaths := make(map[string]string)
	for _, dir := range modDirs {
		modPath, err := readModulePath(dir)
		if err != nil {
			log.Fatalf("failed to read module in %s: %v", dir, err)
		}
		modPaths[modPath] = dir
	}

	// Build dependency map (uses) and version map
	uses := make(map[string][]string)
	versionRefs := make(versionRefs)
	for modPath, dir := range modPaths {
		reqs, err := readRequiresVersioned(dir)
		if err != nil {
			log.Fatalf("failed to read requires for %s: %v", modPath, err)
		}
		for _, r := range reqs {
			if _, ok := modPaths[r.path]; ok {
				uses[modPath] = append(uses[modPath], r.path)
				if versionRefs[modPath] == nil {
					versionRefs[modPath] = make(map[string]string)
				}
				versionRefs[modPath][r.path] = r.version
			}
		}
	}

	// Build reverse map (used_by)
	usedBy := make(map[string][]string)
	for mod, deps := range uses {
		for _, dep := range deps {
			usedBy[dep] = append(usedBy[dep], mod)
		}
	}

	// Get latest git tag for each module
	latestTags := make(latestTags)
	for modPath, dir := range modPaths {
		tag := latestGitTag(dir)
		if tag != "" {
			latestTags[modPath] = tag
		}
	}

	// Build sorted output: order by count(used_by) desc, count(uses) asc, name asc
	var sortedMods []string
	for mod := range modPaths {
		sortedMods = append(sortedMods, mod)
	}
	sort.Slice(sortedMods, func(i, j int) bool {
		ubi, ubj := len(usedBy[sortedMods[i]]), len(usedBy[sortedMods[j]])
		if ubi != ubj {
			return ubi > ubj
		}
		ui, uj := len(uses[sortedMods[i]]), len(uses[sortedMods[j]])
		if ui != uj {
			return ui < uj
		}
		return sortedMods[i] < sortedMods[j]
	})

	// Filter to a single module if a path argument was given
	if filterPath != "" {
		workRoot, _ := os.Getwd()
		var matched []string
		for _, mod := range sortedMods {
			dir := modPaths[mod]
			absDir := filepath.Join(workRoot, dir)
			// Match if filterPath is inside the module dir, or module dir is inside filterPath
			if isSubpath(absDir, filterPath) || isSubpath(filterPath, absDir) {
				matched = append(matched, mod)
			}
		}
		if len(matched) == 0 {
			log.Fatalf("no module found containing %s", filterPath)
		}
		sortedMods = matched
	}

	// Build module info list
	var modules []moduleInfo
	for _, mod := range sortedMods {
		dir := modPaths[mod]

		info := moduleInfo{
			Name:        mod,
			Path:        dir,
			Description: readReadmeTitle(dir),
		}

		if tag, ok := latestTags[mod]; ok {
			info.Latest = tag
		}

		if deps, ok := uses[mod]; ok {
			sort.Strings(deps)
			info.Uses = deps
		}
		if revs, ok := usedBy[mod]; ok {
			sort.Strings(revs)
			info.UsedBy = revs
		}

		// Build git state
		g := &components.Git{
			BranchName: getGitBranch(dir),
		}
		if info.Latest != "" {
			g.Ahead = commitsSinceTag(dir, info.Latest)
		}
		if st := getGitStatus(dir); st != nil {
			g.Unpushed = st.Unpushed
			g.DiffLines = st.DiffLines
		}
		if g.Ahead > 0 {
			g.Msgs = commitMessagesSinceTag(dir, info.Latest)
		}
		g.UntrackedFiles = getUntrackedFiles(dir)
		if *verbose {
			g.Issues = getGitHubIssues(dir)
		}
		info.GitState = g

		// Build usage
		info.Usage, info.Outdated = buildUsage(versionRefs, latestTags, info)

		modules = append(modules, info)
	}

	if *update {
		updateDeps(versionRefs, modPaths, latestTags)
		return
	}

	if *puml {
		renderPUML(os.Stdout, modules)
		return
	}

	if *d2 {
		renderD2(os.Stdout, modules)
		return
	}

	renderTables(modules, *verbose)
}

// isSubpath reports whether child is equal to or under parent.
func isSubpath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func updateDeps(refs versionRefs, modPaths map[string]string, tags latestTags) {
	for modPath, modRefs := range refs {
		dir := modPaths[modPath]
		modShort := filepath.Base(modPath)
		updated := false

		for dep, ver := range modRefs {
			latest := tags[dep]
			if latest == "" || ver == latest {
				continue
			}
			depShort := filepath.Base(dep)
			fmt.Printf("Updated %s %s@%s to %s@%s\n", modShort, depShort, ver, depShort, latest)

			cmd := exec.Command("go", "get", dep+"@"+latest)
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Printf("  go get failed for %s in %s: %v", dep, modPath, err)
			}
			updated = true
		}

		if updated {
			cmd := exec.Command("go", "get", "-u", "./...")
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Printf("  go get -u failed in %s: %v", modPath, err)
			}

			cmd = exec.Command("go", "mod", "tidy")
			cmd.Dir = dir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Printf("  go mod tidy failed in %s: %v", modPath, err)
			}
		}
	}
}
