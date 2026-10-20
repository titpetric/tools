package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/report"
)

// Summary draws what each linter found: how many findings at each level, and
// which of its rules they came under.
//
// An operator reads it first. A tree of any size answers with more
// findings than a screen holds, and which linter is talking and about what is
// the question a reader has before they read any of them.
func Summary(w io.Writer, found *report.Report) error {
	breakdowns := found.Breakdowns()
	if len(breakdowns) == 0 {
		return nil
	}

	rows := make([][]string, 0, len(breakdowns))
	for _, one := range breakdowns {
		rows = append(rows, []string{
			one.Linter,
			count(one.Errors),
			count(one.Warnings),
			count(one.Notices),
			strconv.Itoa(one.Total),
			rulesOf(one),
		})
	}

	table(w, []string{"Linter", "Error", "Warn", "Info", "Total", "Rules"}, rows,
		[]string{"", colorRed, colorAmber, colorTeal, "", colorGrey})

	if quiet := found.Quiet(); len(quiet) > 0 {
		_, err := fmt.Fprintln(w, paint(strings.Join(quiet, ", ")+" found nothing.", colorGrey))
		return err
	}

	return nil
}

// count reads a column of levels: a level with no findings shows a blank and
// no zero, so a reader sees only what is there.
func count(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// rulesOf is what a linter reported under, loudest first.
func rulesOf(one report.Breakdown) string {
	parts := make([]string, 0, len(one.Rules))
	for _, rule := range one.Rules {
		parts = append(parts, fmt.Sprintf("%s %d", rule.Rule, rule.Count))
	}
	return strings.Join(parts, ", ")
}
