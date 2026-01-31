package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"time"
)

type DefaultRenderer struct{}

var DefaultColors = []string{
//	"\033[38;5;236m", // dark gray (#333)
	"\033[38;5;240m", // gray (#555)
	"\033[38;5;245m", // medium gray (#777)
	"\033[38;5;250m", // light gray (#aaa)
	"\033[38;5;254m", // very light gray (#ddd)
	"\033[38;5;231m", // bright white (#fff)
}

var (
	Gray1       = "\033[38;5;245m" // medium gray
	Gray2       = "\033[38;5;250m" // light gray
	White       = "\033[97m"       // bright white
	BrightWhite = "\033[38;5;231m" // bright white, not bold
	Orange      = "\033[38;5;208m" // primary
	Reset       = "\033[0m"
)

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

	for y := minY; y <= maxY; y++ {
		buf.WriteString("│")
		for x := minX; x <= maxX; x++ {
			r := grid[y][x]
			if r == ' ' {
				buf.WriteString("   ")
			} else {
				color := DefaultColors[rand.Intn(len(DefaultColors))]
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

	// bottom border
	buf.WriteString("└")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┘\n")

	fmt.Println(buf.String())
}
