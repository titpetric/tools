package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"
)

type DefaultRenderer struct{}

// Colors for non-primary letters (gradient gray → white)
var DefaultColors = []string{
	"\033[38;5;240m", // gray (#555)
	"\033[38;5;245m", // medium gray (#777)
	"\033[38;5;250m", // light gray (#aaa)
	"\033[38;5;254m", // very light gray (#ddd)
	"\033[38;5;231m", // bright white (#fff)
}

var Orange = "\033[38;5;208m" // primary word
var Reset = "\033[0m"

func (DefaultRenderer) Render(grid [][]rune, placed []Placement) {
	rand.Seed(time.Now().UnixNano())

	minX, minY, maxX, maxY := bounds(grid)
	minX--
	minY--
	maxX++
	maxY++

	var buf bytes.Buffer
	width := maxX - minX + 1

	// top border
	buf.WriteString("┌")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┐\n")

	// content
	for y := minY; y <= maxY; y++ {
		buf.WriteString("│")
		for x := minX; x <= maxX; x++ {
			r := grid[y][x]
			if r == ' ' {
				buf.WriteString("   ")
				continue
			}

			var color string

			if isPrimary(placed, x, y) {
				color = Orange
			} else if options.WholeWordColor {
				// lookup placement covering this cell
				for _, p := range placed {
					for i := range p.Word {
						xx := p.X + i*p.DX
						yy := p.Y + i*p.DY
						if xx == x && yy == y {
							color = p.Color
							break
						}
					}
					if color != "" {
						break
					}
				}
				// fallback to random shade if no placement color
				if color == "" {
					color = DefaultColors[rand.Intn(len(DefaultColors))]
				}
			} else {
				// per-character random shading
				color = DefaultColors[rand.Intn(len(DefaultColors))]
			}

			// render with padding
			buf.WriteString(" ")
			buf.WriteString(color)
			buf.WriteRune(toUpper(r))
			buf.WriteString(Reset)
			buf.WriteString(" ")
		}
		buf.WriteString("│\n")
	}

	// bottom border
	buf.WriteString("└")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┘\n")

	fmt.Println(buf.String())
}
