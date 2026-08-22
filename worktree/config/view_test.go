package config

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// printableWidth returns the width a rendered row takes on screen, with the
// styling stripped out.
func printableWidth(line string) int {
	return ansi.StringWidth(ansi.Strip(line))
}

// renderLines renders the form into its rows, styling stripped.
func renderLines(m Model) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(m.render(), "\n"), "\n") {
		lines = append(lines, ansi.Strip(line))
	}
	return lines
}

func TestPadAndTruncate(t *testing.T) {
	if got, want := pad("ab", 5), "ab   "; got != want {
		t.Fatalf("pad() = %q, want %q", got, want)
	}
	if got, want := pad("abcdef", 3), "abcdef"; got != want {
		t.Fatalf("pad() shorter than its input = %q, want %q", got, want)
	}
	if got := truncPad("abcdef", 3); printableWidth(got) != 3 {
		t.Fatalf("truncPad() = %q, want 3 columns", got)
	}
	if got := truncPad("ab", 5); printableWidth(got) != 5 {
		t.Fatalf("truncPad() = %q, want 5 columns", got)
	}
}

// TestFitMarksACut checks a description too long for its column is shown as
// shortened rather than as written that way.
func TestFitMarksACut(t *testing.T) {
	got := fit("a description longer than its column", 12)
	if printableWidth(got) != 12 {
		t.Fatalf("fit() = %q, want 12 columns", got)
	}
	if !strings.HasSuffix(strings.TrimRight(got, " "), "…") {
		t.Fatalf("fit() = %q, want the cut marked", got)
	}
	if got, want := fit("short", 12), "short       "; got != want {
		t.Fatalf("fit() = %q, want %q", got, want)
	}
}

// TestPadMeasuresPrintableWidth checks styled text is padded by what it shows
// rather than by how many bytes it holds.
func TestPadMeasuresPrintableWidth(t *testing.T) {
	got := pad(styleValue+"ab"+styleReset, 5)
	if printableWidth(got) != 5 {
		t.Fatalf("pad() = %q, want 5 printable columns", got)
	}
}

func TestCaption(t *testing.T) {
	got := caption("title", 20, styleValue)
	if printableWidth(got) != 20 {
		t.Fatalf("caption() = %q, want 20 columns", got)
	}
	if !strings.Contains(ansi.Strip(got), "─ title ─") {
		t.Fatalf("caption() = %q, want the text inset into the rule", ansi.Strip(got))
	}
	if got := caption("a caption longer than its rule", 12, styleValue); printableWidth(got) != 12 {
		t.Fatalf("caption() = %q, want 12 columns", got)
	}
}

// TestCaretText checks the caret is drawn where the typing goes, and that a
// text longer than its column scrolls to keep the caret in view.
func TestCaretText(t *testing.T) {
	if got, want := ansi.Strip(caretText("abc", 1, 10)), pad("a"+caretGlyph+"bc", 10); got != want {
		t.Fatalf("caretText() = %q, want the caret after the first rune", got)
	}
	if got := caretText("abc", 3, 10); printableWidth(got) != 10 {
		t.Fatalf("caretText() = %q, want 10 columns", got)
	}

	long := caretText("abcdefghij", 10, 5)
	if printableWidth(long) != 5 {
		t.Fatalf("caretText() = %q, want 5 columns", long)
	}
	if got, want := ansi.Strip(long), "ghij"+caretGlyph; got != want {
		t.Fatalf("caretText() = %q, want %q, the end being typed at", got, want)
	}
}

func TestShortPath(t *testing.T) {
	t.Setenv("HOME", "/home/test")

	if got, want := shortPath("/home/test/.config/worktree.yml"), "~/.config/worktree.yml"; got != want {
		t.Fatalf("shortPath() = %q, want %q", got, want)
	}
	if got, want := shortPath("/etc/worktree.yml"), "/etc/worktree.yml"; got != want {
		t.Fatalf("shortPath() outside the home directory = %q, want %q", got, want)
	}
}

// TestViewShowsEverySetting checks the form shows every setting, its value and
// what it does at once, with nothing behind a dialog.
func TestViewShowsEverySetting(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := Default()
	m := New(cfg, "/home/test/.config/worktree.yml")

	got := m.render()
	for _, want := range []string{
		"worktree config",
		"Scan",
		"Enable Gitignore",
		"Enable Git Repos",
		"Ignore Paths",
		"Root Markers",
		checkOn,
		"go.work, go.mod, .git",
		listEmpty,
		buttonSave,
		buttonDiscard,
		"~/.config/worktree.yml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("the form does not show %q:\n%s", want, got)
		}
	}
	for _, field := range m.fields {
		if !strings.Contains(ansi.Strip(got), field.Help) {
			t.Fatalf("the form does not describe %q:\n%s", field.Key, got)
		}
	}
}

// TestViewShowsListsWhileTyping checks a list is shown where it stands while
// it is edited, with the rest of the form still readable beside it.
func TestViewShowsListsWhileTyping(t *testing.T) {
	m := New(Default(), "")
	m.focus(2)
	m, _ = press(m, typed("node_modules, vendor")...)

	got := strings.Join(renderLines(m), "\n")
	for _, want := range []string{"node_modules, vendor", caretGlyph, "Enable Gitignore", buttonSave} {
		if !strings.Contains(got, want) {
			t.Fatalf("the form does not show %q while a list is typed into:\n%s", want, got)
		}
	}
}

// TestViewMarksTheFocus checks exactly one row carries the focus marker.
func TestViewMarksTheFocus(t *testing.T) {
	m := New(Default(), "")

	for _, row := range []int{0, 2, 4, 5} {
		m.focus(row)
		if got := strings.Count(strings.Join(renderLines(m), "\n"), markerOn); got != 1 {
			t.Fatalf("row %d: the form marks %d rows as focused, want 1", row, got)
		}
	}
}

// TestViewLegendFollowsTheFocus checks the legend describes the row the focus
// is on, since that is where the keys apply.
func TestViewLegendFollowsTheFocus(t *testing.T) {
	m := New(Default(), "")

	if got := m.legend(); !strings.Contains(got, "Toggle") {
		t.Fatalf("legend on a boolean = %q, want the toggle keys", got)
	}
	m.focus(2)
	if got := m.legend(); !strings.Contains(got, "Type to edit") {
		t.Fatalf("legend on a list = %q, want the typing keys", got)
	}
	m.focus(m.saveRow())
	if got := m.legend(); !strings.Contains(got, "Nothing changed") {
		t.Fatalf("legend on the save button of an unedited form = %q, want it to say so", got)
	}
	m, _ = press(m, key(tea.KeyUp), key(tea.KeyUp), key(tea.KeyUp), key(tea.KeyUp), key(tea.KeyLeft))
	m.focus(m.saveRow())
	if got := m.legend(); !strings.Contains(got, "ENTER Save") {
		t.Fatalf("legend on the save button of an edited form = %q, want the save key", got)
	}
}

// TestRenderIsRectangular checks every row of the frame is the same width in
// every state, so the form lines up where it is printed and does not move
// while it is filled in.
func TestRenderIsRectangular(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(Model) Model
	}{
		{"opened", func(m Model) Model { return m }},
		{"on a list", func(m Model) Model {
			m.focus(3)
			return m
		}},
		{"typing a long list", func(m Model) Model {
			m.focus(2)
			m, _ = press(m, typed(strings.Repeat("some_long_directory_name, ", 4))...)
			return m
		}},
		{"on the buttons", func(m Model) Model {
			m.focus(m.saveRow())
			return m
		}},
		{"leaving with edits", func(m Model) Model {
			m, _ = press(m, key(tea.KeyLeft), key(tea.KeyEscape))
			return m
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := renderLines(test.setup(New(Default(), "/home/test/.config/worktree.yml")))
			width := ansi.StringWidth(lines[0])
			for i, line := range lines {
				if got := ansi.StringWidth(line); got != width {
					t.Fatalf("row %d is %d columns wide, want %d: %q", i, got, width, line)
				}
			}
		})
	}
}

// TestRenderIsInline checks the form is only as tall as it has rows to show,
// rather than filling the terminal.
func TestRenderIsInline(t *testing.T) {
	m := New(Default(), "/home/test/.config/worktree.yml")
	m.height = 50

	if view := m.View(); view.AltScreen {
		t.Fatal("View() asks for the alternate screen, want the form inline")
	}
	// A heading, the settings, a blank row and the buttons, inside the frame.
	want := 1 + len(m.fields) + 1 + 1 + chromeRows
	if got := len(renderLines(m)); got != want {
		t.Fatalf("render() drew %d rows, want %d", got, want)
	}
}

// TestRenderHasNoBackground checks the form only sets foreground colours, so
// it draws over the terminal background rather than painting its own.
func TestRenderHasNoBackground(t *testing.T) {
	m := New(Default(), "/home/test/.config/worktree.yml")

	if strings.Contains(m.render(), "\033[48;") {
		t.Fatal("render() set a background colour, want foreground styling only")
	}
}

// TestRenderCaptions checks the title and the file being edited are carried in
// the frame rather than costing rows of their own.
func TestRenderCaptions(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	m := New(Default(), "/home/test/.config/worktree.yml")

	lines := renderLines(m)
	if !strings.Contains(lines[0], title()) {
		t.Fatalf("top border = %q, want the title in it", lines[0])
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, "~/.config/worktree.yml") {
		t.Fatalf("bottom border = %q, want the path in it", last)
	}
}
