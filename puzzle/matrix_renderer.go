package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"
)

// Colors
const (
	MatrixReset       = "\033[0m"
	MatrixLetter      = "\033[1;97m" // bold bright white for main package
	MatrixBrightGreen = "\033[38;5;82m"
	MatrixGreen       = "\033[38;5;34m"
	MatrixDarkGreen   = "\033[38;5;22m"
	MatrixNoise       = "\033[38;5;236m" // dark gray / black for padding
	MatrixBox         = "\033[38;5;28m"  // dim green for bounding box
)

type MatrixRenderer struct{}

func (MatrixRenderer) Render(grid [][]rune, placed []Placement) {
	rand.Seed(time.Now().UnixNano())

	// compute bounds using util.go
	minX, minY, maxX, maxY := bounds(grid)
	minX -= 2
	maxX += 2

	var buf bytes.Buffer

	width := maxX - minX + 1

	// top border
	buf.WriteString(MatrixBox)
	buf.WriteString("┌")
	for i := 0; i < width*3; i++ { // *3 because each cell is " <char> "
		buf.WriteString("─")
	}
	buf.WriteString("┐\n")
	buf.WriteString(MatrixReset)

	for y := minY; y <= maxY; y++ {
		buf.WriteString(MatrixBox)
		buf.WriteString("│")
		buf.WriteString(MatrixReset)

		for x := minX; x <= maxX; x++ {
			r := grid[y][x]

			var color string
			var char rune

			if r == ' ' {
				color = MatrixNoise
				char = ' '
			} else {
				if isPrimary(placed, x, y) {
					color = MatrixLetter
				} else {
					switch rand.Intn(3) {
					case 0:
						color = MatrixBrightGreen
					case 1:
						color = MatrixGreen
					default:
						color = MatrixDarkGreen
					}
				}
				char = toUpper(r)
			}

			// two-space padding around character
			buf.WriteString(color)
			buf.WriteString(" ")
			buf.WriteRune(char)
			buf.WriteString(" ")
			buf.WriteString(MatrixReset)
		}

		buf.WriteString(MatrixBox)
		buf.WriteString("│\n")
		buf.WriteString(MatrixReset)
	}

	// bottom border
	buf.WriteString(MatrixBox)
	buf.WriteString("└")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┘\n")
	buf.WriteString(MatrixReset)

	fmt.Print(buf.String())
}
