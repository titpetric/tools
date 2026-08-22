package config

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

// Light rounded box drawing, the same frame the worktree tables are drawn
// with, so the form reads as one more table in the output.
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxTeeRight    = "├"
	boxTeeLeft     = "┤"
)

// The form palette, foreground only. The form paints no background of its own,
// so it sits on the terminal background the tables printed before it use.
const (
	styleReset    = components.ColorReset
	styleFrame    = components.ColorSeparator
	styleHeading  = components.ColorAmber
	styleLabel    = components.ColorHeader
	styleValue    = components.ColorGreenLt
	styleSelected = components.ColorWhite
	styleMarked   = components.ColorYellow
	styleHelp     = components.ColorHeader
	styleDim      = components.ColorBorder
	styleLegend   = components.ColorBorder
	styleAlert    = components.ColorAmber
)

// Form geometry. The form is only as wide as its settings need and only as
// tall as it has rows, so it prints inline rather than taking the screen; a
// wide terminal caps the description column rather than stretching it.
const (
	maxFormWidth = 96
	// columnGap is the space between the label, value and description
	// columns.
	columnGap = 2
	// minValueWidth is the room a string list is typed into before it starts
	// to scroll, and maxValueWidth the most a long one may claim. The
	// descriptions give way first on a narrow terminal, since a value is
	// edited and a description is only read.
	minValueWidth = 16
	maxValueWidth = 40
	// chromeRows counts the rows the frame costs: two borders, the rule above
	// the legend, and the legend.
	chromeRows = 4
)

// marker is the glyph in front of the focused row. Focus is shown by the
// marker and the text colour rather than by a highlight bar, which would need
// a background.
const (
	markerOn  = "› "
	markerOff = "  "
)

// The value column glyphs: a checkbox for a boolean, a bar for the position a
// string list is being typed at, and the placeholder of an empty list.
const (
	checkOn    = "[x] Enabled"
	checkOff   = "[ ] Disabled"
	caretGlyph = "▏"
	listEmpty  = "(none)"
)

// The buttons the form is finished on.
const (
	buttonSave    = "[ Save ]"
	buttonDiscard = "[ Discard ]"
)

// View implements tea.Model.
func (m Model) View() tea.View {
	return tea.NewView(strings.TrimSuffix(m.render(), "\n"))
}

// layout is the column geometry of the form, measured from the settings it
// shows so the value and description columns line up without fixing a screen
// width.
type layout struct {
	// label, value and desc are the widths of the three columns of a setting
	// row: what it is called, what it is set to, and what it does.
	label int
	value int
	desc  int
}

// inner returns the width between the frame borders, which every row of the
// form is rendered to.
func (l layout) inner() int {
	return l.label + columnGap + l.value + columnGap + l.desc
}

// layout measures the form against the settings it shows, so it is only as
// wide as they need rather than stretched to the terminal. The value column is
// measured from the document rather than from the text being typed, so a row
// does not move while it is edited; text past the column scrolls instead.
func (m Model) layout() layout {
	var l layout
	for i, field := range m.fields {
		l.label = max(l.label, ansi.StringWidth(markerOff+field.Title))
		l.value = max(l.value, ansi.StringWidth(valueText(field, m.initial[i])))
		l.desc = max(l.desc, ansi.StringWidth(field.Help))
	}
	l.label = max(l.label, ansi.StringWidth(markerOff+buttonSave))
	l.value = min(max(l.value, minValueWidth), maxValueWidth)

	width := m.width
	if width <= 0 {
		width = 80
	}
	// Two borders and the space inside each of them are not the form's to
	// hand out. What is left over is taken off the descriptions first.
	budget := min(width, maxFormWidth) - 4
	l.desc = max(min(l.desc, budget-l.label-l.value-2*columnGap), 0)
	l.value = min(l.value, max(budget-l.label-l.desc-2*columnGap, minValueWidth))
	return l
}

// render draws the whole form.
func (m Model) render() string {
	l := m.layout()

	var b strings.Builder
	b.WriteString(topBorder(l))
	for _, line := range m.body(l) {
		b.WriteString(styleFrame + boxVertical + styleReset + " " + line + " " +
			styleFrame + boxVertical + styleReset + "\n")
	}
	b.WriteString(splitBorder(l))
	b.WriteString(legendLine(m, l))
	b.WriteString(bottomBorder(m, l))
	return b.String()
}

// body renders the rows between the borders: the settings, grouped by section,
// and the buttons below them. The form is as tall as it has rows, so it does
// not reserve space it has nothing to put in; a short terminal cuts it back.
func (m Model) body(l layout) []string {
	var lines []string
	index := 0
	for i, section := range m.config.Sections() {
		if i > 0 {
			lines = append(lines, pad("", l.inner()))
		}
		lines = append(lines, styleHeading+truncPad(section.Title, l.inner())+styleReset)
		for range section.Fields {
			lines = append(lines, m.settingLine(index, l))
			index++
		}
	}
	lines = append(lines, pad("", l.inner()), m.buttonLine(l))

	if rows := m.height - chromeRows; m.height > chromeRows+1 && len(lines) > rows {
		lines = lines[:rows]
	}
	return lines
}

// settingLine renders one setting: what it is called, what it is set to, and
// what it does. The columns are padded to the measured widths so they line up
// down the form.
func (m Model) settingLine(index int, l layout) string {
	field := m.fields[index]
	focused := index == m.cursor

	labelStyle, marker := styleLabel, markerOff
	if focused {
		labelStyle, marker = styleSelected, markerOn
	}
	return labelStyle + truncPad(marker+field.Title, l.label) + styleReset +
		strings.Repeat(" ", columnGap) +
		m.valueCell(index, l.value) +
		strings.Repeat(" ", columnGap) +
		styleHelp + fit(field.Help, l.desc) + styleReset
}

// valueCell renders the value column of a setting. The focused string list is
// rendered with the caret it is typed at, windowed to the column so a long
// list scrolls rather than widening the form.
func (m Model) valueCell(index, width int) string {
	field, v := m.fields[index], m.state[index]
	focused := index == m.cursor

	if focused && field.IsList() {
		return styleSelected + caretText(v.text, m.caret, width) + styleReset
	}

	style := styleValue
	switch {
	case focused:
		style = styleMarked
	case field.IsList() && v.text == "":
		style = styleDim
	}
	return style + fit(valueText(field, v), width) + styleReset
}

// valueText renders a value as the form shows it when it is not being typed
// into.
func valueText(field Field, v value) string {
	switch {
	case field.IsList() && v.text == "":
		return listEmpty
	case field.IsList():
		return v.text
	case v.flag:
		return checkOn
	default:
		return checkOff
	}
}

// caretText renders text with the caret marked at a rune position, windowed to
// width. The window follows the caret, so the end being typed at stays on
// screen however long the text grows.
func caretText(text string, caret, width int) string {
	runes := []rune(text)
	caret = min(max(caret, 0), len(runes))

	// The caret glyph takes a column of its own.
	start := max(caret-(width-1), 0)
	end := min(start+width-1, len(runes))

	return pad(string(runes[start:caret])+styleMarked+caretGlyph+styleSelected+
		string(runes[caret:end]), width)
}

// buttonLine renders the buttons the form is finished on. Save is dim while
// there is nothing to write, since pressing it then does nothing but close the
// form.
func (m Model) buttonLine(l layout) string {
	save := styleValue
	if !m.dirty() {
		save = styleDim
	}
	return truncPad(button(buttonSave, m.cursor == m.saveRow(), save)+
		strings.Repeat(" ", columnGap)+
		button(buttonDiscard, m.cursor == m.discardRow(), styleLabel), l.inner())
}

// button renders one button, marked when focused the way a setting row is.
func button(label string, focused bool, style string) string {
	if focused {
		return styleSelected + markerOn + label + styleReset
	}
	return style + markerOff + label + styleReset
}

// legendLine renders the key legend of the focused row, or the status message
// when there is one.
func legendLine(m Model, l layout) string {
	text, style := m.legend(), styleLegend
	if m.status != "" {
		text, style = m.status, styleAlert
	}
	return styleFrame + boxVertical + styleReset + " " +
		style + truncPad(text, l.inner()) + styleReset + " " +
		styleFrame + boxVertical + styleReset + "\n"
}

// legend returns the keys the focused row answers to.
func (m Model) legend() string {
	switch {
	case m.cursor == m.saveRow() && !m.dirty():
		return "Nothing changed   ↑↓ Move   ENTER Close   ESC Close"
	case m.cursor == m.saveRow():
		return "↑↓ Move   ENTER Save   ESC Close"
	case m.cursor == m.discardRow():
		return "↑↓ Move   ENTER Discard   ESC Close"
	case m.onList():
		return "↑↓ Move   Type to edit, comma separated   ENTER Go to Save"
	default:
		return "↑↓ Move   ←→ or SPACE Toggle   ENTER Go to Save"
	}
}

// title returns the caption of the top border.
func title() string {
	return fmt.Sprintf("worktree config v%d", Version)
}

// topBorder renders the top frame, captioned with the form title.
func topBorder(l layout) string {
	return styleFrame + boxTopLeft + caption(title(), l.inner()+2, styleHeading) +
		styleFrame + boxTopRight + styleReset + "\n"
}

// splitBorder renders the rule between the settings and the legend.
func splitBorder(l layout) string {
	return styleFrame + boxTeeRight + strings.Repeat(boxHorizontal, l.inner()+2) +
		boxTeeLeft + styleReset + "\n"
}

// bottomBorder renders the bottom frame, captioned with the file the form
// writes so the settings can be read back to a path.
func bottomBorder(m Model, l layout) string {
	return styleFrame + boxBottomLeft + caption(shortPath(m.path), l.inner()+2, styleDim) +
		styleFrame + boxBottomRight + styleReset + "\n"
}

// caption renders text inset into a horizontal rule of width, the way a table
// carries a heading in its frame. Text too long for the rule is cut.
func caption(text string, width int, style string) string {
	text = " " + ansi.Truncate(text, max(width-3, 0), "") + " "
	rule := width - 1 - ansi.StringWidth(text)
	return styleFrame + boxHorizontal + style + text +
		styleFrame + strings.Repeat(boxHorizontal, max(rule, 0))
}

// pad extends text to width with spaces, measuring the printable width so
// styled text lines up.
func pad(text string, width int) string {
	gap := width - ansi.StringWidth(text)
	if gap <= 0 {
		return text
	}
	return text + strings.Repeat(" ", gap)
}

// truncPad fits text to exactly width.
func truncPad(text string, width int) string {
	if ansi.StringWidth(text) > width {
		return ansi.Truncate(text, width, "")
	}
	return pad(text, width)
}

// fit is truncPad for prose, marking a cut with an ellipsis so a description
// that does not fit reads as shortened rather than as written that way.
func fit(text string, width int) string {
	if ansi.StringWidth(text) > width {
		return pad(ansi.Truncate(text, width, "…"), width)
	}
	return pad(text, width)
}

// shortPath renders a path below the home directory as ~/...
func shortPath(path string) string {
	if home, err := homeDir(); err == nil && strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
