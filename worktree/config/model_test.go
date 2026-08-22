package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// key builds the key press for a special key such as enter or escape.
func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// text builds the key press for a printable character.
func text(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// typed builds the key presses that type a string.
func typed(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, text(r))
	}
	return msgs
}

// press sends key presses to a model, returning the model and the command the
// last of them left behind.
func press(m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m, cmd
}

// quits reports whether a command closes the form.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// stateOf returns the edited value of the setting with the given key.
func stateOf(t *testing.T, m Model, wantKey string) value {
	t.Helper()
	for i, field := range m.fields {
		if field.Key == wantKey {
			return m.state[i]
		}
	}
	t.Fatalf("no field %q", wantKey)
	return value{}
}

func TestModelTogglesBoolean(t *testing.T) {
	cfg := Default()
	m := New(cfg, "/home/test/.config/worktree.yml")

	m, _ = press(m, key(tea.KeyLeft))
	if stateOf(t, m, "scan.enable_gitignore").flag {
		t.Fatal("scan.enable_gitignore is on, want the left key to have turned it off")
	}
	if !m.dirty() {
		t.Fatal("the form reports no edit after a toggle")
	}
	// Nothing is written into the document until the form saves.
	if !cfg.Scan.EnableGitignore {
		t.Fatal("the toggle reached the document, want it held in the form until it saves")
	}

	m, _ = press(m, key(tea.KeyRight))
	if !stateOf(t, m, "scan.enable_gitignore").flag {
		t.Fatal("scan.enable_gitignore is off, want the right key to have turned it on")
	}
	// Back where it started is not an edit, so there is nothing to save.
	if m.dirty() {
		t.Fatal("the form reports an edit after the toggle was undone")
	}

	m, _ = press(m, text(' '))
	if stateOf(t, m, "scan.enable_gitignore").flag {
		t.Fatal("scan.enable_gitignore is on, want space to have toggled it")
	}
}

func TestModelMovesBetweenRows(t *testing.T) {
	m := New(Default(), "")

	m, _ = press(m, key(tea.KeyUp))
	if m.cursor != 0 {
		t.Fatalf("cursor = %d at the top of the form, want the first setting", m.cursor)
	}

	m, _ = press(m, key(tea.KeyDown), key(tea.KeyDown))
	if m.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", m.cursor)
	}

	// The focus runs on into the buttons and stops at the last of them.
	for range m.rows() + 2 {
		m, _ = press(m, key(tea.KeyDown))
	}
	if m.cursor != m.discardRow() {
		t.Fatalf("cursor = %d at the end of the form, want the discard button", m.cursor)
	}

	// The buttons sit on one row, so they are also reached sideways.
	m, _ = press(m, key(tea.KeyLeft))
	if m.cursor != m.saveRow() {
		t.Fatalf("cursor = %d, want the left key to move to the save button", m.cursor)
	}
	m, _ = press(m, key(tea.KeyRight))
	if m.cursor != m.discardRow() {
		t.Fatalf("cursor = %d, want the right key to move to the discard button", m.cursor)
	}
}

// TestModelEditsListInPlace checks a string list is typed into where it
// stands, with no dialog in the way.
func TestModelEditsListInPlace(t *testing.T) {
	cfg := Default()
	m := New(cfg, "")
	m.focus(2) // scan.ignore_paths, which starts empty

	m, _ = press(m, typed("node_modules, vendorX")...)
	m, _ = press(m, key(tea.KeyBackspace))

	if got, want := stateOf(t, m, "scan.ignore_paths").text, "node_modules, vendor"; got != want {
		t.Fatalf("scan.ignore_paths reads %q, want %q", got, want)
	}
	if got, want := stateOf(t, m, "scan.ignore_paths").entries(), []string{"node_modules", "vendor"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries() = %v, want %v", got, want)
	}
	// The document only takes the edit when the form saves.
	if len(cfg.Scan.IgnorePaths) != 0 {
		t.Fatalf("Scan.IgnorePaths = %v, want the typing held in the form", cfg.Scan.IgnorePaths)
	}

	// The caret moves through the text and edits where it stands.
	m, _ = press(m, key(tea.KeyHome), key(tea.KeyDelete), text('N'))
	if got, want := stateOf(t, m, "scan.ignore_paths").text, "Node_modules, vendor"; got != want {
		t.Fatalf("scan.ignore_paths reads %q, want %q", got, want)
	}
	m, _ = press(m, key(tea.KeyRight), key(tea.KeyRight), key(tea.KeyLeft), key(tea.KeyBackspace))
	if got, want := stateOf(t, m, "scan.ignore_paths").text, "Nde_modules, vendor"; got != want {
		t.Fatalf("scan.ignore_paths reads %q, want %q", got, want)
	}
}

// TestModelListKeepsSpacing checks the keys a boolean is toggled with are
// typed into a list instead, and that respacing one is not an edit.
func TestModelListKeepsSpacing(t *testing.T) {
	m := New(Default(), "")
	m.focus(3) // scan.root_markers

	m, _ = press(m, key(tea.KeyEnd), text(' '), text(' '))
	if got, want := stateOf(t, m, "scan.root_markers").text, "go.work, go.mod, .git  "; got != want {
		t.Fatalf("scan.root_markers reads %q, want the spaces typed into it, %q", got, want)
	}
	if m.dirty() {
		t.Fatal("the form reports an edit after only the spacing of a list changed")
	}
}

// TestModelCaretFollowsTheFocus checks the caret lands at the end of a list
// when the focus reaches it, which is where typing carries on from.
func TestModelCaretFollowsTheFocus(t *testing.T) {
	m := New(Default(), "")

	m, _ = press(m, key(tea.KeyDown), key(tea.KeyDown), key(tea.KeyDown))
	if want := len([]rune(m.state[3].text)); m.caret != want {
		t.Fatalf("caret = %d on reaching a list, want %d, the end of its text", m.caret, want)
	}
}

// TestModelEnterGoesToSave checks enter on a setting changes nothing and moves
// on to the save button, which is where the form is finished.
func TestModelEnterGoesToSave(t *testing.T) {
	cfg := Default()
	m := New(cfg, "")

	m, cmd := press(m, key(tea.KeyEnter))
	if m.cursor != m.saveRow() {
		t.Fatalf("cursor = %d after enter on a setting, want the save button", m.cursor)
	}
	if m.dirty() {
		t.Fatal("enter on a setting changed a value, want it to change nothing")
	}
	if quits(cmd) {
		t.Fatal("enter on a setting closed the form")
	}
}

// TestModelSaveWithNothingChanged checks the save button writes nothing when
// there is nothing to write, so an untouched document is not rewritten.
func TestModelSaveWithNothingChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	m := New(Default(), path)

	m, cmd := press(m, key(tea.KeyEnter), key(tea.KeyEnter))
	if !quits(cmd) {
		t.Fatal("the save button did not close the form")
	}
	if m.saved {
		t.Fatal("the form reports it saved, want nothing written when nothing changed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) = %v, want no file written", path, err)
	}
}

func TestModelSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	cfg := Default()
	m := New(cfg, path)

	// Turn a flag off, type a list entry, then finish on the save button.
	m, _ = press(m, key(tea.KeyLeft), key(tea.KeyDown), key(tea.KeyDown))
	m, _ = press(m, typed("node_modules")...)
	m, cmd := press(m, key(tea.KeyEnter), key(tea.KeyEnter))

	if !m.saved {
		t.Fatal("the form reports it did not save")
	}
	if !quits(cmd) {
		t.Fatal("the save button did not close the form")
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}
	if got.Scan.EnableGitignore {
		t.Fatal("saved scan.enable_gitignore is on, want the edit written")
	}
	if !got.Scan.EnableGitRepos {
		t.Fatal("saved scan.enable_git_repos is off, want the untouched setting written too")
	}
	if want := []string{"node_modules"}; !reflect.DeepEqual(got.Scan.IgnorePaths, want) {
		t.Fatalf("saved scan.ignore_paths = %v, want %v", got.Scan.IgnorePaths, want)
	}
	if want := []string{"go.work", "go.mod", ".git"}; !reflect.DeepEqual(got.Scan.RootMarkers, want) {
		t.Fatalf("saved scan.root_markers = %v, want the untouched list written back", got.Scan.RootMarkers)
	}
}

// TestModelDiscards checks the discard button leaves the document as it was.
func TestModelDiscards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	cfg := Default()
	m := New(cfg, path)

	m, cmd := press(m, key(tea.KeyLeft), key(tea.KeyEnd), key(tea.KeyEnter))
	if !quits(cmd) {
		t.Fatal("the discard button did not close the form")
	}
	if m.saved {
		t.Fatal("the form reports it saved, want the edit discarded")
	}
	if !cfg.Scan.EnableGitignore {
		t.Fatal("the discarded edit reached the document")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) = %v, want no file written", path, err)
	}
}

// TestModelEscapeKeepsEdits checks escape with unsaved edits moves to the
// discard button rather than dropping them, so leaving is always answered for.
func TestModelEscapeKeepsEdits(t *testing.T) {
	m := New(Default(), "")

	// With nothing edited, escape leaves straight away.
	if _, cmd := press(m, key(tea.KeyEscape)); !quits(cmd) {
		t.Fatal("escape on an unedited form did not close it")
	}

	m, cmd := press(m, key(tea.KeyLeft), key(tea.KeyEscape))
	if quits(cmd) {
		t.Fatal("escape closed a form holding an edit, want it to ask first")
	}
	if m.cursor != m.discardRow() {
		t.Fatalf("cursor = %d after escape, want the discard button", m.cursor)
	}
	if m.status == "" {
		t.Fatal("no message about the unsaved edits")
	}

	// The settings are still there to go back to.
	m, _ = press(m, key(tea.KeyUp))
	if m.cursor != m.saveRow() {
		t.Fatalf("cursor = %d, want the save button back up the form", m.cursor)
	}

	// Escape on the discard button leaves.
	m.focus(m.discardRow())
	if _, cmd := press(m, key(tea.KeyEscape)); !quits(cmd) {
		t.Fatal("escape on the discard button did not close the form")
	}
}

// TestModelReportsSaveFailure checks a write that fails keeps the form open
// with the reason, rather than losing the edits.
func TestModelReportsSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(Default(), filepath.Join(blocker, "worktree.yml"))
	m, cmd := press(m, key(tea.KeyLeft), key(tea.KeyF10))

	if m.saved {
		t.Fatal("the form reports it saved, want the failure recorded")
	}
	if quits(cmd) {
		t.Fatal("the form closed on a failed save, want it kept open with the edits")
	}
	if m.status == "" {
		t.Fatal("no status message after a failed save")
	}
	// The status replaces the key legend. A message longer than the screen is
	// cut to fit, so only its start is looked for.
	got := m.render()
	if !strings.Contains(got, "create") {
		t.Fatalf("the form does not show the failure %q:\n%s", m.status, got)
	}
	if strings.Contains(got, m.legend()) {
		t.Fatalf("the form still shows the key legend instead of the failure:\n%s", got)
	}
}
