package config

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Model is the bubbletea model of the setup form.
//
// The form shows every setting at once: a boolean is toggled where it stands
// and a string list is typed into where it stands, so nothing is hidden behind
// a dialog. Below the settings are the save and discard buttons the form is
// finished on.
type Model struct {
	config *Config
	path   string

	// fields is the flattened setting list, in the order the form shows it.
	fields []Field

	// state holds the edited value of every setting and initial the value it
	// was loaded with, so an edit that is undone counts as no edit. The
	// document is only written from state when the form saves.
	state   []value
	initial []value

	// cursor is the focused row: a setting, then the save and discard buttons
	// after the last one.
	cursor int

	// caret is the position being typed at in the focused list setting,
	// counted in runes.
	caret int

	// saved records that the document was written, so the caller knows the
	// form was not abandoned.
	saved bool

	// status is the message shown in place of the key legend, cleared on the
	// next key.
	status string

	width  int
	height int
}

// New returns the setup form for a configuration document. The document is
// read into the form and written back only when the form saves.
func New(cfg *Config, path string) Model {
	fields := cfg.Fields()
	state := make([]value, len(fields))
	for i, field := range fields {
		state[i] = field.state()
	}

	m := Model{
		config:  cfg,
		path:    path,
		fields:  fields,
		state:   state,
		initial: slices.Clone(state),
		width:   80,
		height:  24,
	}
	// The form opens on the first setting.
	m.focus(0)
	return m
}

// Saved reports whether the document was written before the form exited.
func (m Model) Saved() bool {
	return m.saved
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// rows returns the number of focusable rows: every setting, then the two
// buttons.
func (m Model) rows() int {
	return len(m.fields) + 2
}

// saveRow and discardRow return the rows of the two buttons.
func (m Model) saveRow() int    { return len(m.fields) }
func (m Model) discardRow() int { return len(m.fields) + 1 }

// onButtons reports whether the focus is on one of the buttons rather than on
// a setting.
func (m Model) onButtons() bool {
	return m.cursor >= len(m.fields)
}

// onList reports whether the focused row is a string list, which is the row
// that takes typing.
func (m Model) onList() bool {
	return !m.onButtons() && m.fields[m.cursor].IsList()
}

// dirty reports whether the form holds an edit the document does not, which is
// what makes saving worth doing.
func (m Model) dirty() bool {
	for i, v := range m.state {
		if !v.equal(m.initial[i], m.fields[i].IsList()) {
			return true
		}
	}
	return false
}

// focus moves the focus to a row, clamped to the form, leaving the caret at
// the end of the text of a list setting.
func (m *Model) focus(row int) {
	m.cursor = min(max(row, 0), m.rows()-1)
	if m.onList() {
		m.caret = len([]rune(m.state[m.cursor].text))
	}
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status = ""

	// The keys that mean the same on every row are read first, so a list
	// setting cannot swallow them as typing.
	switch keyName(msg) {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "shift+tab":
		m.focus(m.cursor - 1)
		return m, nil
	case "down", "tab":
		m.focus(m.cursor + 1)
		return m, nil
	case "enter":
		return m.activate()
	case "f10":
		// The shortcut for the save button, from wherever the focus is.
		return m.save()
	case "esc":
		return m.leave()
	}

	if m.onList() {
		return m.editKey(msg), nil
	}
	return m.chooseKey(msg), nil
}

// chooseKey handles the keys of a boolean setting and of the buttons, the rows
// that are chosen rather than typed into.
func (m Model) chooseKey(msg tea.KeyPressMsg) Model {
	switch keyName(msg) {
	case "left", "-":
		if m.onButtons() {
			m.focus(m.saveRow())
			break
		}
		m.state[m.cursor].flag = false
	case "right", "+":
		if m.onButtons() {
			m.focus(m.discardRow())
			break
		}
		m.state[m.cursor].flag = true
	case " ", "space":
		if !m.onButtons() {
			m.state[m.cursor].flag = !m.state[m.cursor].flag
		}
	case "home":
		m.focus(0)
	case "end":
		m.focus(m.rows() - 1)
	}
	return m
}

// editKey handles the keys of a string list setting, which is typed into where
// it stands. Entries are separated by commas; the blanks that leaves while a
// separator is being typed are dropped when the form saves.
func (m Model) editKey(msg tea.KeyPressMsg) Model {
	runes := []rune(m.state[m.cursor].text)
	m.caret = min(m.caret, len(runes))

	switch keyName(msg) {
	case "left":
		m.caret = max(m.caret-1, 0)
	case "right":
		m.caret = min(m.caret+1, len(runes))
	case "home":
		m.caret = 0
	case "end":
		m.caret = len(runes)
	case "backspace":
		if m.caret > 0 {
			m.setText(string(slices.Delete(runes, m.caret-1, m.caret)))
			m.caret--
		}
	case "delete":
		if m.caret < len(runes) {
			m.setText(string(slices.Delete(runes, m.caret, m.caret+1)))
		}
	default:
		// Text carries what the key typed, so modifiers and named keys leave
		// the setting alone.
		typed := []rune(msg.Key().Text)
		if len(typed) == 0 {
			break
		}
		m.setText(string(slices.Insert(runes, m.caret, typed...)))
		m.caret += len(typed)
	}
	return m
}

// setText replaces the text of the focused list setting.
func (m *Model) setText(text string) {
	m.state[m.cursor].text = text
}

// activate is what enter does. On a setting it changes nothing and moves on to
// the save button, which is where the form is finished; on a button it presses
// it.
func (m Model) activate() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case m.saveRow():
		return m.save()
	case m.discardRow():
		return m, tea.Quit
	default:
		m.focus(m.saveRow())
		return m, nil
	}
}

// leave is what escape does. With edits in hand it moves to the discard
// button rather than dropping them, so leaving with unsaved edits is always
// something the form was told twice.
func (m Model) leave() (tea.Model, tea.Cmd) {
	if !m.dirty() || m.cursor == m.discardRow() {
		return m, tea.Quit
	}
	m.focus(m.discardRow())
	m.status = "Unsaved changes: ENTER discards them, ↑ goes back to the settings"
	return m, nil
}

// save writes the document and exits. A form with nothing edited writes
// nothing, so saving an untouched document cannot rewrite it. A write that
// fails keeps the form open with the reason, so the edits are not lost.
func (m Model) save() (tea.Model, tea.Cmd) {
	if !m.dirty() {
		return m, tea.Quit
	}
	for i, field := range m.fields {
		field.apply(m.state[i])
	}
	if err := SaveFile(m.path, m.config); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.saved = true
	return m, tea.Quit
}

// keyName renders a key press as the lowercase name the handlers switch on.
func keyName(msg tea.KeyPressMsg) string {
	return strings.ToLower(msg.String())
}
