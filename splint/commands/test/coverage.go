package test

import (
	"fmt"
	"go/ast"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"golang.org/x/tools/cover"
)

// coverBlock is one block of a coverage profile, by line range.
type coverBlock struct {
	start int
	end   int
}

// coverageData merges the profiles of every go test invocation.
type coverageData struct {
	files map[string]map[coverBlock]bool
}

// newCoverageData returns an empty merged profile.
func newCoverageData() *coverageData {
	return &coverageData{files: map[string]map[coverBlock]bool{}}
}

// add folds the profile at filename into the merged profile.
//
// A block that ran in one invocation is covered, so a repeated block keeps
// its covered verdict rather than adding the counts together.
func (c *coverageData) add(filename string) error {
	profiles, err := cover.ParseProfiles(filename)
	if err != nil {
		return fmt.Errorf("read coverage profile: %w", err)
	}

	for _, profile := range profiles {
		if profile == nil {
			continue
		}

		blocks := c.files[profile.FileName]
		if blocks == nil {
			blocks = map[coverBlock]bool{}
			c.files[profile.FileName] = blocks
		}

		for _, block := range profile.Blocks {
			key := coverBlock{start: block.StartLine, end: block.EndLine}
			blocks[key] = blocks[key] || block.Count > 0
		}
	}
	return nil
}

// lineCoverage is the lines of a declaration that ran, and the lines measured.
type lineCoverage struct {
	covered int
	total   int
}

// lines counts the covered and measured lines of one declaration.
//
// A block belongs to a declaration when it starts inside the declaration's
// lines: go test emits a block for the body and for each branch, and all of
// them start within the func keyword and the closing brace.
func (c *coverageData) lines(one changedFunc) lineCoverage {
	key := filepath.Base(one.File)
	if one.ImportPath != "" {
		key = one.ImportPath + "/" + key
	}

	var out lineCoverage
	for block, covered := range c.files[key] {
		if block.start < one.StartLine || block.start > one.EndLine {
			continue
		}
		n := block.end - block.start + 1
		out.total += n
		if covered {
			out.covered += n
		}
	}
	return out
}

// percent is the share of the measured lines that ran.
func (c lineCoverage) percent() float64 {
	if c.total == 0 {
		return 0
	}
	return 100 * float64(c.covered) / float64(c.total)
}

// changedRow is one row of the scan: a changed symbol and its coverage.
type changedRow struct {
	symbol   string
	indirect bool
	via      []string
	lines    lineCoverage
}

// writeScan writes the changed symbols and their coverage, and the statistics
// of the tests that ran against the tests the tree holds.
func (s *selection) writeScan(w io.Writer, cov *coverageData) error {
	rows := make([]changedRow, 0, len(s.Changed))
	for _, one := range s.Changed {
		rows = append(rows, changedRow{
			symbol:   symbolName(one),
			indirect: !ast.IsExported(one.Name),
			via:      one.Via,
			lines:    cov.lines(one),
		})
	}

	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "CHANGED SYMBOL\tMODE\tLINES (COV/TOTAL)\tCOVERAGE")
	fmt.Fprintln(tw, "--------------\t----\t-----------------\t--------")
	for _, row := range rows {
		mode := "direct"
		if row.indirect {
			mode = "indirect"
			if len(row.via) > 0 {
				mode += " (via " + strings.Join(row.via, ", ") + ")"
			}
		}

		lines := "-"
		coverage := "-"
		if row.lines.total > 0 {
			lines = fmt.Sprintf("%d/%d", row.lines.covered, row.lines.total)
			coverage = fmt.Sprintf("%.1f%%", row.lines.percent())
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.symbol, mode, lines, coverage)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	var covered, total, indirect int
	for _, row := range rows {
		covered += row.lines.covered
		total += row.lines.total
		if row.indirect {
			indirect++
		}
	}

	ran := len(s.Tests)
	ranPercent := 0.0
	if s.TotalTests > 0 {
		ranPercent = 100 * float64(ran) / float64(s.TotalTests)
	}

	fmt.Fprintf(w, "\nTests: %d/%d ran (%.1f%%)", ran, s.TotalTests, ranPercent)
	if len(s.Benches) > 0 {
		fmt.Fprintf(w, ", benchmarks: %d/%d ran", len(s.Benches), s.TotalBenches)
	}
	fmt.Fprintf(w, "\nChanged symbols: %d (%d direct, %d indirect)\n", len(rows), len(rows)-indirect, indirect)
	if total > 0 {
		fmt.Fprintf(w, "Changed lines covered: %d/%d (%.1f%%)\n", covered, total, 100*float64(covered)/float64(total))
	}

	return nil
}

// symbolName names a changed symbol with its directory when it is not the
// source root, so two packages with the same function name stay apart.
func symbolName(one changedFunc) string {
	if one.Dir == "." || one.Dir == "" {
		return one.Key
	}
	return filepath.ToSlash(one.Dir) + "." + one.Key
}
