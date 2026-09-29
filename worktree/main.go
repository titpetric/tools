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

	// The verdict is a report on one repository, the one the current
	// directory is in unless another was named, so it does not scan the
	// workspace either.
	if opts.Verdict {
		dir := opts.FilterPath
		if dir == "" {
			dir = "."
		}
		// A chain reports on every release; a single verdict is the one range
		// it was asked for, which the stats table takes as a run of one.
		var verdicts []verdict
		if opts.Chain {
			chain, err := readVerdicts(dir, opts.Verbose, opts.From, opts.To, !opts.NoCache)
			if err != nil {
				log.Fatalf("failed to read the release chain: %v", err)
			}
			verdicts = chain
		} else {
			v, err := readVerdict(dir, opts.From, opts.To, !opts.NoCache)
			if err != nil {
				log.Fatalf("failed to read the release verdict: %v", err)
			}
			verdicts = []verdict{v}
		}

		// A path of "." narrows the verdict to the root package, the way the
		// go tool reads the pattern. "./..." is the whole module, and is what
		// no path at all already means.
		if rootScopeArg(opts.FilterArg) {
			for i := range verdicts {
				scoped, err := scopeVerdict(verdicts[i], dir)
				if err != nil {
					log.Fatalf("failed to narrow the verdict to the root package: %v", err)
				}
				verdicts[i] = scoped
			}
		}

		if opts.Stats {
			renderVerdictStats(os.Stdout, verdicts, supportsANSI(os.Stdout))
			return
		}
		renderVerdicts(os.Stdout, verdicts, supportsANSI(os.Stdout))
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
		g.UntrackedFiles = getUntrackedFiles(dir, opts.Verbose || opts.All)
		if opts.Verbose {
			g.Issues = getGitHubIssues(dir)
		}
		info.GitState = g

		// Build usage
		info.Usage, info.Outdated = buildUsage(versionRefs, latestTags, info)

		modules = append(modules, info)
	}

	if opts.Resolve {
		// The reason a run stopped has already been rendered in place, so
		// only the exit status is left to set.
		if err := resolve(os.Stdout, modules, versionRefs, opts, supportsANSI(os.Stdout)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if opts.Exec != "" {
		if len(goModPaths) == 0 {
			log.Fatalf("--exec requires a go.work or go.mod")
		}
		if execModules(os.Stdout, goModPaths, opts.Exec, opts.Verbose, supportsANSI(os.Stdout)) > 0 {
			os.Exit(1)
		}
		return
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
