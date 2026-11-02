package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// run starts the selected tests and benchmarks.
//
// Every result goes to w as one PASS or FAIL line, and the output of an
// invocation that failed goes to progress, where a reader looks only when a
// line says FAIL.
func (s *selection) run(ctx context.Context, src *source, opts Options, w, progress io.Writer) (int, error) {
	status := map[testID]bool{}
	var failed []string
	cov := newCoverageData()

	start := func(dir string, ids []testID, benchmark bool) error {
		pattern := regexpPattern(ids)
		args := append([]string(nil), opts.GoFlags...)
		if !hasFlag(args, "count") {
			// A cached pass describes an earlier build of the same source,
			// which is not what this run set out to check.
			args = append(args, "-count=1")
		}
		if benchmark {
			args = withFlags(args, "-run", "^$", "-bench", pattern, "-benchtime", "10s")
		} else {
			args = withFlags(args, "-run", pattern)
		}
		args = withFlags(args, "-v")

		module, rel := src.module(dir)
		profile, err := os.CreateTemp("", "splint-test-cover-*.out")
		if err != nil {
			return err
		}
		profilePath := profile.Name()
		profile.Close()
		defer os.Remove(profilePath)

		args = append(args, "-coverprofile="+profilePath)
		if pkgs := s.coverPackages(src, module); pkgs != "" {
			args = append(args, "-coverpkg="+pkgs)
		}

		cmdArgs := append([]string{"test"}, args...)
		cmdArgs = append(cmdArgs, packageArg(rel))

		if opts.Verbose {
			fmt.Fprintf(progress, "splint test: go %s\n", strings.Join(cmdArgs, " "))
		}

		cmd := exec.CommandContext(ctx, "go", cmdArgs...)
		cmd.Dir = module
		out, err := cmd.CombinedOutput()

		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				return err
			}
		}

		if info, statErr := os.Stat(profilePath); statErr == nil && info.Size() > 0 {
			if err := cov.add(profilePath); err != nil {
				return err
			}
		}

		if opts.Verbose && len(out) > 0 {
			progress.Write(out)
		}

		if benchmark {
			for _, id := range ids {
				status[id] = code == 0
			}
			if code != 0 {
				failed = append(failed, string(out))
			}
			return nil
		}

		reported := statusLines(out)
		for _, id := range ids {
			pass, found := reported[id.Name]
			if !found {
				// A build failure reports no test, and every test the
				// invocation would have run failed with it.
				pass = code == 0
			}
			status[id] = pass
		}
		if code != 0 {
			failed = append(failed, string(out))
		}
		return nil
	}

	if opts.Split {
		for _, id := range s.ids() {
			benchmark := s.Benches[id]
			if err := start(id.Dir, []testID{id}, benchmark); err != nil {
				return 0, err
			}
		}
	} else {
		for dir, ids := range group(s.Tests) {
			if err := start(dir, ids, false); err != nil {
				return 0, err
			}
		}
		for dir, ids := range group(s.Benches) {
			if err := start(dir, ids, true); err != nil {
				return 0, err
			}
		}
	}

	exit := 0
	for _, id := range s.ids() {
		pass := status[id]
		if !pass {
			exit = 1
			fmt.Fprintf(w, "FAIL %s\n", id.Name)
			continue
		}
		fmt.Fprintf(w, "PASS %s\n", id.Name)
	}

	// The verdict lines are all a passing run says. A failing run carries
	// what go test wrote, which is where the reason is.
	if exit != 0 && !opts.Verbose {
		for _, out := range failed {
			io.WriteString(progress, out)
			if !strings.HasSuffix(out, "\n") {
				io.WriteString(progress, "\n")
			}
		}
	}

	// The scan merges the coverage of every invocation and maps it onto the
	// symbols the change touched.
	if err := s.writeScan(w, cov); err != nil {
		return 0, err
	}

	return exit, nil
}

// coverPackages names the changed packages in module for -coverpkg, so a
// changed package is measured even when its tests live in another package.
func (s *selection) coverPackages(src *source, module string) string {
	seen := map[string]bool{}
	var out []string

	for _, one := range s.Changed {
		if one.ImportPath == "" || seen[one.ImportPath] {
			continue
		}
		if dir, _ := src.module(one.Dir); dir != module {
			continue
		}
		seen[one.ImportPath] = true
		out = append(out, one.ImportPath)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// group collects tests by directory, in a stable order.
func group(set map[testID]bool) map[string][]testID {
	out := map[string][]testID{}
	for id := range set {
		out[id.Dir] = append(out[id.Dir], id)
	}
	for dir := range out {
		sort.Slice(out[dir], func(i, j int) bool { return out[dir][i].Name < out[dir][j].Name })
	}
	return out
}

// regexpPattern selects exactly the names of a set.
func regexpPattern(ids []testID) string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = regexp.QuoteMeta(id.Name)
	}
	return "^(" + strings.Join(names, "|") + ")$"
}

// withFlags appends flags to a forwarded set, but before -args, which hands
// everything after it to the test binary.
func withFlags(flags []string, ours ...string) []string {
	at := len(flags)
	for i, flag := range flags {
		if flag == "-args" || flag == "--args" {
			at = i
			break
		}
	}

	out := make([]string, 0, len(flags)+len(ours))
	out = append(out, flags[:at]...)
	out = append(out, ours...)
	out = append(out, flags[at:]...)
	return out
}

// hasFlag reports whether a forwarded set names a flag.
func hasFlag(flags []string, name string) bool {
	for _, flag := range flags {
		if flag == "-"+name || flag == "--"+name || strings.HasPrefix(flag, "-"+name+"=") {
			return true
		}
	}
	return false
}

// packageArg names a directory as go test takes it.
func packageArg(dir string) string {
	if dir == "." || dir == "" {
		return "."
	}
	return "./" + filepath.ToSlash(dir)
}

// statusLine reads the verdict go test -v writes for one test.
var statusLine = regexp.MustCompile(`(?m)^\s*--- (PASS|FAIL|SKIP): (\S+)`)

// statusLines maps a top-level test name to whether it passed. A subtest is
// part of the test above it and is not a selection of its own.
func statusLines(out []byte) map[string]bool {
	result := map[string]bool{}
	for _, match := range statusLine.FindAllStringSubmatch(string(out), -1) {
		name := match[2]
		if at := strings.IndexByte(name, '/'); at >= 0 {
			name = name[:at]
		}
		switch match[1] {
		case "FAIL":
			result[name] = false
		default:
			// A skipped test did not fail, and the first verdict is the one a
			// subtest cannot take back.
			if _, seen := result[name]; !seen {
				result[name] = true
			}
		}
	}
	return result
}
