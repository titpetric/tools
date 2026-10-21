package render

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Line returns a formatted issue line.
func Line(issue model.Issue) string {
	return compose(issue, false)
}

// compose writes one issue, painted when the output goes to a terminal.
//
// The parts carry the colour and the punctuation between them does not, so a
// line reads the same with the escapes stripped out of it as without: what a
// pipe gets and what a terminal gets are one string with one shape.
func compose(issue model.Issue, colour bool) string {
	paintIf := func(value, color string) string {
		if !colour {
			return value
		}
		return paint(value, color)
	}

	parts := []string{
		paintIf(severityName(issue.Severity), severityColor(issue.Severity)),
		paintIf(issue.Position.Ref(), colorTeal),
		paintIf(issue.RuleName(), colorGrey),
	}

	message := issue.Message
	if issue.Symbol != "" {
		message = paintIf(issue.Symbol, colorSymbol) + " - " + message
	}

	return strings.Join(append(parts, message), ": ")
}
