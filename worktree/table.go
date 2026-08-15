package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		rows = append(rows, toRow(len(headers), values))
	}
	if !styled {
		writeMarkdownTable(w, headers, rows)
		return
	}

	widths := headerWidths(headers)
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
	writeMarkdownHeader(w, headers)
	for _, row := range rows {
		writeMarkdownRow(w, len(headers), row)
	}
}

func writeMarkdownHeader(w io.Writer, headers []string) {
	fmt.Fprint(w, "|")
	for _, header := range headers {
		fmt.Fprintf(w, " %s |", markdownCell(header))
	}
	fmt.Fprint(w, "\n|")
	for range headers {
		fmt.Fprint(w, " --- |")
	}
	fmt.Fprintln(w)
}

func writeMarkdownRow(w io.Writer, numCols int, row components.Rows) {
	fmt.Fprint(w, "|")
	for i := 0; i < numCols; i++ {
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

// toRow converts cell values into a row of numCols multi-line cells.
func toRow(numCols int, values []string) components.Rows {
	row := make(components.Rows, numCols)
	for i := 0; i < numCols; i++ {
		if i < len(values) && values[i] != "" {
			row[i] = components.Cell(strings.Split(strings.TrimSpace(values[i]), "\n"))
		}
	}
	return row
}

// headerWidths returns the display width of each header.
func headerWidths(headers []string) []int {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = ansi.StringWidth(header)
	}
	return widths
}

// colorLines wraps each line of value in color when styled is true.
func colorLines(value, color string, styled bool) string {
	if !styled || color == "" || value == "" {
		return value
	}
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = color + line + components.ColorReset
	}
	return strings.Join(lines, "\n")
}

// relPath renders path relative to the working directory, prefixed with "./".
func relPath(path string) string {
	rel, err := filepath.Rel(".", path)
	if err != nil {
		return path
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return rel
	}
	rel = filepath.Join(".", rel)
	if !strings.HasPrefix(rel, "."+string(filepath.Separator)) {
		rel = "." + string(filepath.Separator) + rel
	}
	return rel
}

func markdownCell(value string) string {
	value = ansi.Strip(value)
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	return strings.ReplaceAll(value, "\n", "<br>")
}
