package model

import (
	"log/slog"
	"sort"
)

// Severity is the weight of an issue, which is an slog level so a linter says
// how much it means in the vocabulary the rest of a Go program already uses.
type Severity = slog.Level

// The severities a linter reports at.
const (
	SeverityDebug = slog.LevelDebug
	SeverityInfo  = slog.LevelInfo
	SeverityWarn  = slog.LevelWarn
	SeverityError = slog.LevelError
)

// Issue is one finding of a linter.
type Issue struct {
	// Linter is the linter that reported it, and Rule the rule within that
	// linter. A linter with one rule leaves Rule empty.
	Linter string `json:"Linter" yaml:"Linter"`
	Rule   string `json:"Rule,omitempty" yaml:"Rule,omitempty"`

	// Severity is how much the issue means.
	Severity Severity `json:"Severity" yaml:"Severity"`

	// Position is the location of the issue.
	Position Position `json:"Position" yaml:"Position"`

	// Symbol is the declaration the issue is about, receiver and name, and is
	// empty for an issue about a file or a package.
	Symbol string `json:"Symbol,omitempty" yaml:"Symbol,omitempty"`

	// Message is the issue in a sentence.
	Message string `json:"Message" yaml:"Message"`

	// Attrs carries whatever else the linter has to say, so a rule reports a
	// count or a suggestion without the schema growing a field per rule.
	Attrs map[string]string `json:"Attrs,omitempty" yaml:"Attrs,omitempty"`
}

// RuleName is the linter for an issue without a rule,
// and linter/rule otherwise.
func (i Issue) RuleName() string {
	if i.Rule == "" {
		return i.Linter
	}
	return i.Linter + "/" + i.Rule
}

// Attr returns one attribute, and whether the issue sets it.
func (i Issue) Attr(key string) (string, bool) {
	value, ok := i.Attrs[key]
	return value, ok
}

// AttrKeys returns the attribute names, sorted for a stable rendering.
func (i Issue) AttrKeys() []string {
	keys := make([]string, 0, len(i.Attrs))
	for key := range i.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
