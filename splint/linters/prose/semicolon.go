package prose

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// semicolonFindings reports a semicolon in prose, never in a fence, an indented block or a code span.
func semicolonFindings(files []*model.ProseFile) []Result {
	var found []Result
	for _, file := range files {
		for _, block := range file.Blocks {
			text := link.ReplaceAllString(span.ReplaceAllString(block.Text, " "), "] ")
			if !strings.Contains(text, ";") {
				continue
			}
			found = append(found, Result{
				Rule:     RuleSemicolon,
				Position: model.Position{File: file.Path, Line: block.Line},
				Message:  semicolonMessage,
			})
		}
	}
	return found
}

// semicolonMessage names the rewrite: one sentence, cut to fit.
const semicolonMessage = "a semicolon joins two sentences. Condense them into one, or end the first and start the next."
