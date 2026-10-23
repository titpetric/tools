package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Annotations writes every finding as a GitHub Actions workflow command:
//
//	::warning file=frontend/view/page.go,line=42,title=godoc/missing::Page - exported symbol lacks a godoc comment
//
// A report with no findings gets one line saying so.
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

// annotationKind is the command that reports a level. GitHub has three
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

// escapeData is the escaped form of a message: the runner reads a command up to
// the first line ending, and a percent opens an escape.
func escapeData(value string) string {
	return strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	).Replace(value)
}

// escapeProperty is the escaped form of a property value: the same, and the
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
