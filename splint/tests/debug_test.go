package tests_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// TestDebugParity prints what two documents disagree on, a few examples per
// field, which is how a divergence in the summary is chased down.
//
// It is driven by the environment rather than by flags so it can be pointed at
// one project and one field without editing anything:
//
//	SPLINT_DEBUG=oida SPLINT_FIELD=References go test ./tests -run Debug -v
func TestDebugParity(t *testing.T) {
	project := os.Getenv("SPLINT_DEBUG")
	if project == "" {
		t.Skip("set SPLINT_DEBUG to a project name")
	}

	root := filepath.Join(workspace, project)
	reference, err := extract(t, root)
	if err != nil {
		t.Fatalf("go-fsck: %v", err)
	}

	options := splint.Options{SourcePath: root, Pattern: "./...", IncludeSources: true}
	parsed, err := simpleparser.New(options).Parse(context.Background())
	if err != nil {
		t.Fatalf("simple parser: %v", err)
	}

	only := os.Getenv("SPLINT_FIELD")
	left, right := index(reference), index(parsed)
	shown := map[string]int{}

	for _, key := range sortedKeys(counted(left)) {
		a := left[key]
		b, ok := right[key]
		if !ok {
			continue
		}
		for _, field := range diff(a, b) {
			if _, ignored := allowed[field]; ignored && only == "" {
				continue
			}
			if only != "" && field != only {
				continue
			}
			if shown[field] >= 3 {
				continue
			}
			shown[field]++
			left, right := show(field, a), show(field, b)
			at := firstDifference(left, right)
			t.Logf("%s  %s  (differ at %d)\n    go-fsck: %s\n    simple : %s", field, key, at, window(left, at), window(right, at))
		}
	}
}

// counted turns the index into a set the sorted key helper can order, so the
// examples come out in the same order every run.
func counted(index map[string]*model.Declaration) map[string]int {
	out := make(map[string]int, len(index))
	for key := range index {
		out[key] = 1
	}
	return out
}

// show renders one field of a declaration for the log.
func show(field string, decl *model.Declaration) string {
	switch field {
	case "Type":
		return decl.Type
	case "Receiver":
		return decl.Receiver
	case "Signature":
		return decl.Signature
	case "Doc":
		return decl.Doc
	case "Source":
		return decl.Source
	case "Arguments":
		return strings.Join(decl.Arguments, " | ")
	case "Returns":
		return strings.Join(decl.Returns, " | ")
	case "Names":
		return strings.Join(decl.Names, " | ")
	case "References":
		return strings.Join(flatten(decl.References), " ")
	case "Fields":
		return strings.Join(fieldStrings(decl), " | ")
	case "SelfContained":
		if decl.SelfContained {
			return "true"
		}
		return "false"
	}
	return ""
}

// firstDifference is the index of the first byte two strings do not share.
func firstDifference(left, right string) int {
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] != right[i] {
			return i
		}
	}
	return min(len(left), len(right))
}

// window is the text around an index, which is where two values part company.
func window(value string, at int) string {
	start := max(0, at-40)
	end := min(len(value), at+60)
	return strconv.Quote(value[start:end])
}

// clip shortens a value to something a log line holds, marking where it was
// cut so a difference past the cut is not mistaken for agreement.
func clip(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	if len(value) > 160 {
		return value[:160] + "[...]"
	}
	return value
}
