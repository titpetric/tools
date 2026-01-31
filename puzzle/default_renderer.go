package main

import (
	"bytes"
	"fmt"
)

type DefaultRenderer struct{}

func (DefaultRenderer) Render(grid [][]rune, placed []Placement) {
	minX, minY, maxX, maxY := bounds(grid)
	minX--
	minY--
	maxX++
	maxY++

	var buf bytes.Buffer
	width := maxX - minX + 1

	buf.WriteString("┌")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┐\n")

	for y := minY; y <= maxY; y++ {
		buf.WriteString("│")
		for x := minX; x <= maxX; x++ {
			r := grid[y][x]
			if r == ' ' {
				buf.WriteString("   ")
			} else {
				color := Gray
				if isPrimary(placed, x, y) {
					color = Orange
				}
				buf.WriteString(" ")
				buf.WriteString(color)
				buf.WriteRune(toUpper(r))
				buf.WriteString(Reset)
				buf.WriteString(" ")
			}
		}
		buf.WriteString("│\n")
	}

	buf.WriteString("└")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┘\n")

	fmt.Println(buf.String())
}
