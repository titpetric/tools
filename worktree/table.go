package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

func supportsANSI(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func writeSimpleTable(w io.Writer, headers []string, values [][]string, styled bool) {
	rows := make([]components.Rows, 0, len(values))
	for _, values := range values {
		row := make(components.Rows, len(headers))
		for i := range headers {
			if i < len(values) && values[i] != "" {
				row[i] = components.Cell(strings.Split(strings.TrimSpace(values[i]), "\n"))
			}
		}
		rows = append(rows, row)
	}
	if !styled {
		writeMarkdownTable(w, headers, rows)
		return
	}

	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = ansi.StringWidth(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], cell.Width())
		}
	}
	writeBorder(w, boxTopLeft, boxTeeDown, boxTopRight, widths)
	writeHeaderRow(w, headers, widths)
	writeBorder(w, boxTeeRight, boxCross, boxTeeLeft, widths)
	for _, row := range rows {
		writeTableRow(w, row, widths)
	}
	writeBorder(w, boxBottomLeft, boxTeeUp, boxBottomRight, widths)
}

func writeMarkdownTable(w io.Writer, headers []string, rows []components.Rows) {
	fmt.Fprint(w, "|")
	for _, header := range headers {
		fmt.Fprintf(w, " %s |", markdownCell(header))
	}
	fmt.Fprint(w, "\n|")
	for range headers {
		fmt.Fprint(w, " --- |")
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		fmt.Fprint(w, "|")
		for i := range headers {
			var lines []string
			if i < len(row) {
				for _, line := range row[i] {
					if line != components.Separator {
						lines = append(lines, markdownCell(line))
					}
				}
			}
			fmt.Fprintf(w, " %s |", strings.Join(lines, "<br>"))
		}
		fmt.Fprintln(w)
	}
}

func markdownCell(value string) string {
	value = ansi.Strip(value)
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	return strings.ReplaceAll(value, "\n", "<br>")
}
