package main

import (
	"io"
	"strconv"

	"github.com/titpetric/tools/worktree/components"
)

// renderVerdictStats writes the whole run as one table of counts, a row per
// release, newest first. It is the report with the analysis collapsed: what
// each release did to the API and to the data model, without the symbols
// behind it.
func renderVerdictStats(w io.Writer, verdicts []verdict, styled bool) {
	if len(verdicts) == 0 {
		return
	}

	writeTitle(w, verdicts[0].Module, styled)

	headers := []string{
		"Version", "Since", "Commits",
		"Symbols +", "Symbols ~", "Symbols -",
		"Fields +", "Fields ~", "Fields -",
	}

	rows := make([][]string, 0, len(verdicts))
	for _, v := range verdicts {
		added, changed, removed := v.API.FieldCounts()
		rows = append(rows, []string{
			v.Version,
			v.Since,
			strconv.Itoa(len(v.Commits)),
			count(len(v.API.Added), components.ColorGreen, styled),
			count(len(v.API.Changed), components.ColorAmber, styled),
			count(len(v.API.Removed), components.ColorRed, styled),
			count(added, components.ColorGreen, styled),
			count(changed, components.ColorAmber, styled),
			count(removed, components.ColorRed, styled),
		})
	}
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// count renders one cell of the stats table. A zero is left grey, so the
// releases that did something stand out from the ones that did not.
func count(n int, color string, styled bool) string {
	if n == 0 {
		return colorLines("0", components.ColorSeparator, styled)
	}
	return colorLines(strconv.Itoa(n), color, styled)
}
