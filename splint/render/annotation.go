package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Annotations writes every finding as a GitHub Actions workflow command.
//
//	::warning file=frontend/view/page.go,line=42,title=godoc/missing::Page - exported symbol lacks a godoc comment
//
// A log line is read by a person and a workflow command is read by the runner,
// which turns it into an annotation on the file and the line of a pull request
// review. Nothing else in a log does that: a compiler shaped line becomes an
// annotation only where a problem matcher was registered for it, and this is
// what needs no matcher.
//
// GitHub shows ten warnings and ten errors per step and fifty annotations per
// job, and drops the rest without saying so, which is why the exit code and
// the statistics are what a gate reads rather than the count of these.
//
// The reference is
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands
// and the escaping is what @actions/core does in command.ts.
func Annotations(w io.Writer, found *report.Report) error {
	if found.Len() == 0 {
		_, err := io.WriteString(w, empty(found)+"\n")
		return err
	}

	for _, issue := range found.Issues {
		if _, err := io.WriteString(w, Annotation(issue)+"\n"); err != nil {
			return err
		}
	}

	return nil
}

// Annotation is one finding as a workflow command.
func Annotation(issue model.Issue) string {
	properties := []string{"file=" + escapeProperty(issue.Position.Path())}

	if issue.Position.Line > 0 {
		properties = append(properties, "line="+strconv.Itoa(issue.Position.Line))
		if end := issue.Position.Block(); end > issue.Position.Line {
			properties = append(properties, "endLine="+strconv.Itoa(end))
		}
	}
	if rule := issue.RuleName(); rule != "" {
		properties = append(properties, "title="+escapeProperty(rule))
	}

	message := issue.Message
	if issue.Symbol != "" {
		message = issue.Symbol + " - " + message
	}

	return fmt.Sprintf("::%s %s::%s", annotationKind(issue.Severity),
		strings.Join(properties, ","), escapeData(message))
}

// annotationKind is the command a level is reported under. GitHub has three
// and slog has four: debug is not an annotation, and reads as the notice the
// level below a warning reads as.
func annotationKind(severity model.Severity) string {
	switch {
	case severity >= model.SeverityError:
		return "error"
	case severity >= model.SeverityWarn:
		return "warning"
	}
	return "notice"
}

// escapeData is what a message is written as: the runner reads a command up to
// the first line ending, and a percent opens an escape.
func escapeData(value string) string {
	return strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	).Replace(value)
}

// escapeProperty is what a property value is written as: the same, and the
// two characters that separate the properties from each other and from the
// message.
func escapeProperty(value string) string {
	return strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
		":", "%3A",
		",", "%2C",
	).Replace(value)
}
