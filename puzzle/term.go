package main

import (
	"os"

	"golang.org/x/term"
)

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 80 {
		return 80 // fallback default width
	}
	return width
}

func calculateGridSize(words []string) (width, height int) {
	// width: fit the target width
	width = getTerminalWidth()
	if width < 10 {
		width = 10
	}

	// height: roughly enough to place all letters without overlap
	longest := 0
	totalLetters := 0
	for _, w := range words {
		if len(w) > longest {
			longest = len(w)
		}
		totalLetters += len(w)
	}

	// height formula: enough rows to spread letters
	height = totalLetters/width + longest
	if height < 100 {
		height = 100
	}

	return
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
