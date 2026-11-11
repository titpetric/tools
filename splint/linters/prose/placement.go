package prose

import (
	"fmt"

	"github.com/titpetric/tools/splint/model"
)

// Reads the depth the YAML walk recorded, so a markdown paragraph is never one.
func placementFindings(files []*model.ProseFile) []Result {
	var found []Result

	for _, file := range files {
		for _, block := range file.Blocks {
			if !block.Comment || block.Depth == 0 {
				continue
			}
			found = append(found, Result{
				Rule:     RulePlacement,
				Position: model.Position{File: file.Path, Line: block.Line},
				Message:  placementMessage(block.Depth),
			})
		}
	}

	return found
}

func placementMessage(depth int) string {
	return fmt.Sprintf("a comment %s into the tree; one line above a top-level key is the whole budget, and a nested key, a list item and a value take none. Put the prose in the document beside the file",
		plural(depth, "level", "levels"))
}
