package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const (
	GridW = 60
	GridH = 25
)

// ANSI colors
const (
	Reset  = "\033[0m"
	Orange = "\033[38;5;208m"
	Gray   = "\033[38;5;240m"
)

type Placement struct {
	Word      string
	X, Y      int
	DX, DY    int
	IsPrimary bool
}

func main() {
	words := getPackages()
	if len(words) == 0 {
		fmt.Println("no packages found")
		return
	}

	grid := make([][]rune, GridH)
	for y := range grid {
		grid[y] = make([]rune, GridW)
		for x := range grid[y] {
			grid[y][x] = ' '
		}
	}

	sort.Slice(words, func(i, j int) bool {
		return len(words[i]) > len(words[j])
	})

	var placed []Placement

	// Place primary word
	mainWord := words[0]
	startX := GridW/2 - len(mainWord)/2
	startY := GridH / 2

	for i, r := range mainWord {
		grid[startY][startX+i] = r
	}

	placed = append(placed, Placement{
		Word:      mainWord,
		X:         startX,
		Y:         startY,
		DX:        1,
		DY:        0,
		IsPrimary: true,
	})

	// Place remaining words
	for _, w := range words[1:] {
		if tryPlace(grid, &placed, w) {
			continue
		}
	}

	render(grid, placed)
}

func getPackages() []string {
	cmd := exec.Command("go", "list", "./...")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	set := map[string]struct{}{}

	for _, l := range lines {
		parts := strings.Split(l, "/")
		name := strings.ToLower(parts[len(parts)-1])
		if len(name) >= 2 {
			set[name] = struct{}{}
		}
	}

	var words []string
	for w := range set {
		words = append(words, w)
	}
	return words
}

func tryPlace(grid [][]rune, placed *[]Placement, word string) bool {
	for _, p := range *placed {
		for i, wc := range word {
			for j, pc := range p.Word {
				if wc != pc {
					continue
				}

				var x, y, dx, dy int
				if p.DX == 1 {
					x = p.X + j
					y = p.Y - i
					dx, dy = 0, 1
				} else {
					x = p.X - i
					y = p.Y + j
					dx, dy = 1, 0
				}

				if canPlace(grid, word, x, y, dx, dy) {
					for k, r := range word {
						grid[y+k*dy][x+k*dx] = r
					}
					*placed = append(*placed, Placement{
						Word: word,
						X:    x,
						Y:    y,
						DX:   dx,
						DY:   dy,
					})
					return true
				}
			}
		}
	}
	return false
}

func canPlace(grid [][]rune, word string, x, y, dx, dy int) bool {
	for i, r := range word {
		xx := x + i*dx
		yy := y + i*dy

		if xx < 0 || yy < 0 || xx >= GridW || yy >= GridH {
			return false
		}
		cell := grid[yy][xx]
		if cell != ' ' && cell != r {
			return false
		}
	}
	return true
}

func render(grid [][]rune, placed []Placement) {
	var buf bytes.Buffer

	// Find used bounds
	minX, minY := GridW, GridH
	maxX, maxY := 0, 0

	for y := range grid {
		for x := range grid[y] {
			if grid[y][x] != ' ' {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	// Padding
	minX -= 1
	minY -= 1
	maxX += 1
	maxY += 1

	width := maxX - minX + 1

	// Top border
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
				buf.WriteRune(unicodeUpper(r))
				buf.WriteString(Reset)
				buf.WriteString(" ")
			}
		}
		buf.WriteString("│\n")
	}

	// Bottom border
	buf.WriteString("└")
	for i := 0; i < width*3; i++ {
		buf.WriteString("─")
	}
	buf.WriteString("┘\n")

	fmt.Println(buf.String())
}

func isPrimary(placed []Placement, x, y int) bool {
	for _, p := range placed {
		if !p.IsPrimary {
			continue
		}
		for i := range p.Word {
			xx := p.X + i*p.DX
			yy := p.Y + i*p.DY
			if xx == x && yy == y {
				return true
			}
		}
	}
	return false
}

func unicodeUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}
