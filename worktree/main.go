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

func main() {
	opts := ParseOptions()

	// The setup screen runs before the configuration is read for the scan,
	// so a document that fails to parse can still be fixed from it.
	if opts.Configure {
		if err := config.Run(os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Release subcommands work on the git repository of the current
	// directory, not on the workspace scan root.
	if opts.Release != "" {
		tags, prefix, err := moduleTags(".")
		if err != nil {
			log.Fatalf("failed to list git tags: %v", err)
		}
		lines, err := releaseCommands(tags, opts.Release, prefix)
		if err != nil {
			log.Fatal(err)
		}
		for _, line := range lines {
			fmt.Fprintln(os.Stdout, line)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	root, err := findScanRoot(".", cfg.Scan.RootMarkers)
	if err != nil {
		log.Fatalf("failed to find scan root: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		log.Fatalf("failed to chdir to %s: %v", root, err)
	}
	projects, err := findProjects(".", cfg.Scan)
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
		pullRepos(os.Stdout, dirs, supportsANSI(os.Stdout))
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
			GoVersion:   readGoVersion(dir),
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

	if opts.Update || opts.GoVersion != "" {
		if len(goModPaths) == 0 {
			log.Fatalf("dependency updates require a go.work or go.mod")
		}
		styled := supportsANSI(os.Stdout)
		if opts.GoVersion != "" {
			if err := updateGoWorkVersions(os.Stdout, ".", opts.GoVersion, cfg.Scan, styled); err != nil {
				log.Fatal(err)
			}
		}
		updateDeps(os.Stdout, goModPaths, latestTags, opts, styled)
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

	if opts.Matrix {
		renderDependencyMatrix(os.Stdout, modules, versionRefs, latestTags, supportsANSI(os.Stdout))
		return
	}

	renderTables(os.Stdout, modules, opts, supportsANSI(os.Stdout))
}

// isSubpath reports whether child is equal to or under parent.
func isSubpath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
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
