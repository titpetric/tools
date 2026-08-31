package tests_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/simpleparser"
)

// runs is how many times each parser reads each tree. The fastest of them is
// what is reported: a slow run measured the machine, and the quickest measured
// the parser.
const runs = 3

// TestParserTiming times the two parsers by running the command, which is what
// a caller of it experiences.
//
// A benchmark inside the process measures the parse and nothing else. What a
// reader wants to know is what "splint --parser=simpleparser ./..." costs
// against "splint --parser=astparser ./...", process start and all, so that is
// what this times.
func TestParserTiming(t *testing.T) {
	binary := build(t)

	type row struct {
		project string
		files   int
		ast     time.Duration
		simple  time.Duration
	}

	var rows []row
	for _, project := range projects {
		root := filepath.Join(workspace, project)
		if _, err := os.Stat(root); err != nil {
			continue
		}

		rows = append(rows, row{
			project: project,
			files:   goFiles(root),
			ast:     fastest(t, binary, analyzer.ParserName, root),
			simple:  fastest(t, binary, simpleparser.ParserName, root),
		})
	}

	if len(rows) == 0 {
		t.Skip("no project was read")
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].files < rows[j].files })

	var totalAST, totalSimple time.Duration
	t.Logf("%-16s %6s %10s %10s %8s", "project", "files", "astparser", "simple", "ratio")
	for _, r := range rows {
		totalAST += r.ast
		totalSimple += r.simple
		t.Logf("%-16s %6d %10s %10s %7.1fx", r.project, r.files,
			r.ast.Round(time.Millisecond), r.simple.Round(time.Millisecond), ratio(r.ast, r.simple))
	}
	t.Logf("%-16s %6s %10s %10s %7.1fx", "total", "", totalAST.Round(time.Millisecond),
		totalSimple.Round(time.Millisecond), ratio(totalAST, totalSimple))

	if totalSimple >= totalAST {
		t.Errorf("the simple parser took %s against the ast parser's %s, which is the wrong way round",
			totalSimple, totalAST)
	}
}

// build compiles the command under test, so the timing is of this tree rather
// than of whatever is installed.
func build(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "splint")
	cmd := exec.Command("go", "build", "-o", binary, "../cmd/splint")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	return binary
}

// fastest runs one parser over one tree and returns the quickest of the runs.
func fastest(t *testing.T, binary, parser, root string) time.Duration {
	t.Helper()

	best := time.Duration(0)
	for i := 0; i < runs; i++ {
		cmd := exec.Command(binary, "--parser="+parser, "-i", root, "--output", filepath.Join(t.TempDir(), "out.json"), "./...")
		cmd.Stdout, cmd.Stderr = nil, nil

		start := time.Now()
		err := cmd.Run()
		took := time.Since(start)

		// A linter finding something exits 1, which is not a failure to run.
		if err != nil {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("%s over %s: %v", parser, root, err)
			}
		}
		if best == 0 || took < best {
			best = took
		}
	}

	return best
}

// goFiles counts the Go files of a tree, which is the size the timings are
// read against.
func goFiles(root string) int {
	count := 0

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			count++
		}
		return nil
	})

	return count
}

// ratio is how many times one duration goes into another.
func ratio(slow, fast time.Duration) float64 {
	if fast == 0 {
		return 0
	}
	return float64(slow) / float64(fast)
}
