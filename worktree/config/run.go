package config

import (
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
)

// Run opens the setup screen on the configuration document, writing it back
// when the screen saves.
//
// A document that fails to parse still opens the screen, since that is where
// it gets fixed, but it opens on the built-in defaults with the parse error in
// the status line. Run writes nothing until the user saves on the screen, so a
// hand written file never gets quietly replaced.
func Run(w io.Writer) error {
	path, err := Path()
	if err != nil {
		return err
	}

	model, loadErr := newModelFor(path)

	final, err := tea.NewProgram(model).Run()
	if err != nil {
		return fmt.Errorf("run setup screen: %w", err)
	}
	if saved, ok := final.(Model); ok && saved.Saved() {
		fmt.Fprintf(w, "wrote %s\n", path)
		return nil
	}
	return loadErr
}

// newModelFor builds the setup screen for the document at path, falling back
// to the built-in defaults when the document fails to parse. newModelFor
// returns the parse error and puts it in the status line, so a run that does
// not save can report it.
func newModelFor(path string) (Model, error) {
	cfg, err := LoadFile(path)
	if err != nil {
		model := New(Default(), path)
		model.status = err.Error()
		return model, err
	}
	return New(cfg, path), nil
}
