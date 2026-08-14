package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

func findScanRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		for _, marker := range []string{"go.work", "go.mod", ".git"} {
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

func findProjects(root string) ([]projectDir, error) {
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

	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
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
		if !p.GoModule && !p.GitRepo {
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

func main() {
	opts := ParseOptions()

	root, err := findScanRoot(".")
	if err != nil {
		log.Fatalf("failed to find scan root: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		log.Fatalf("failed to chdir to %s: %v", root, err)
	}
	projects, err := findProjects(".")
	if err != nil {
		log.Fatalf("failed to scan projects: %v", err)
	}
	if len(projects) == 0 {
		log.Fatalf("no go.work, go.mod, or .git directory found")
	}

	if opts.Pull {
		var dirs []string
		for _, project := range projects {
			dirs = append(dirs, project.Path)
		}
		pullRepos(dirs)
		return
	}

	// Map: module path -> dir, short name -> module path
	modPaths := make(map[string]string)
	goModPaths := make(map[string]string)
	shortNames := make(map[string]string)
	for _, project := range projects {
		modPath := filepath.ToSlash(strings.TrimPrefix(project.Path, "./"))
		if project.GoModule {
			modPath, err = readModulePath(project.Path)
			if err != nil {
				log.Fatalf("failed to read module in %s: %v", project.Path, err)
			}
			goModPaths[modPath] = project.Path
		}
		modPaths[modPath] = project.Path
		shortNames[components.ShortName(modPath)] = modPath
	}

	// Build dependency map (uses) and version map
	uses := make(map[string][]string)
	versionRefs := make(versionRefs)
	if len(goModPaths) > 0 {
		for modPath, dir := range goModPaths {
			reqs, err := readRequiresVersioned(dir)
			if err != nil {
				log.Fatalf("failed to read requires for %s: %v", modPath, err)
			}
			for _, r := range reqs {
				if _, ok := goModPaths[r.path]; ok {
					uses[modPath] = append(uses[modPath], r.path)
					if versionRefs[modPath] == nil {
						versionRefs[modPath] = make(map[string]string)
					}
					versionRefs[modPath][r.path] = r.version
				}
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

	// Filter modules if a path argument was given
	if opts.FilterPath != "" {
		var matched []string

		// Exact short name match
		if mod, ok := shortNames[opts.FilterArg]; ok {
			matched = append(matched, mod)
		}

		// Path-based match
		if len(matched) == 0 {
			workRoot, _ := os.Getwd()
			for _, mod := range sortedMods {
				dir := modPaths[mod]
				absDir := filepath.Join(workRoot, dir)
				if isSubpath(absDir, opts.FilterPath) || isSubpath(opts.FilterPath, absDir) {
					matched = append(matched, mod)
				}
			}
		}

		// Substring match against dir or module name
		if len(matched) == 0 {
			for _, mod := range sortedMods {
				dir := modPaths[mod]
				if strings.Contains(dir, opts.FilterArg) || strings.Contains(mod, opts.FilterArg) {
					matched = append(matched, mod)
				}
			}
		}

		if len(matched) == 0 {
			log.Fatalf("no module found matching %s", opts.FilterArg)
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
			LatestTag:  info.Latest,
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
		if opts.Verbose {
			g.Issues = getGitHubIssues(dir)
		}
		info.GitState = g

		// Build usage
		info.Usage, info.Outdated = buildUsage(versionRefs, latestTags, info)

		modules = append(modules, info)
	}

	if opts.Update {
		if len(goModPaths) == 0 {
			log.Fatalf("dependency updates require a go.work or go.mod")
		}
		updateDeps(goModPaths, latestTags, opts.Verbose)
		return
	}

	if opts.PUML {
		renderPUML(os.Stdout, modules)
		return
	}

	if opts.D2 {
		renderD2(os.Stdout, modules)
		return
	}

	renderTables(modules, opts)
}

// isSubpath reports whether child is equal to or under parent.
func isSubpath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func updateDeps(modPaths map[string]string, tags latestTags, verbose bool) {
	for modPath, dir := range modPaths {
		modShort := filepath.Base(modPath)

		fmt.Printf("Updating %s (go get -u ./...)\n", modShort)
		cmd := exec.Command("go", "get", "-u", "./...")
		cmd.Dir = dir
		if err := runCommand(cmd, verbose, os.Stdout, os.Stderr); err != nil {
			log.Printf("  go get -u failed in %s: %v", modPath, err)
		}

		// Update workspace dependencies to their latest tags
		reqs, err := readRequiresVersioned(dir)
		if err == nil {
			for _, r := range reqs {
				if tag, ok := tags[r.path]; ok && tag != "" && r.version != tag {
					fmt.Printf("Updating %s: %s %s -> %s\n", modShort, filepath.Base(r.path), r.version, tag)
					cmd := exec.Command("go", "get", r.path+"@"+tag)
					cmd.Dir = dir
					if err := runCommand(cmd, verbose, os.Stdout, os.Stderr); err != nil {
						log.Printf("  go get %s@%s failed: %v", r.path, tag, err)
					}
				}
			}
		}

		fmt.Printf("Tidying %s\n", modShort)
		cmd = exec.Command("go", "mod", "tidy")
		cmd.Dir = dir
		if err := runCommand(cmd, verbose, os.Stdout, os.Stderr); err != nil {
			log.Printf("  go mod tidy failed in %s: %v", modPath, err)
		}
	}
}

func runCommand(cmd *exec.Cmd, verbose bool, stdout, stderr io.Writer) error {
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if verbose {
		fmt.Fprintf(stdout, "$ %s", strings.Join(cmd.Args, " "))
		if err == nil {
			fmt.Fprintf(stdout, " %s✓%s", components.ColorGreen, components.ColorReset)
		}
		fmt.Fprintln(stdout)
	}
	return err
}

func pullRepos(dirs []string) {
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

	for _, path := range paths {
		fmt.Printf("Pulling %s\n", path)
		cmd := exec.Command("git", "pull")
		cmd.Dir = path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("  git pull failed in %s: %v", path, err)
		}
	}
}
