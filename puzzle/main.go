package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	GridW = 60
	GridH = 25
)

type Placement struct {
	Word      string
	X, Y      int
	DX, DY    int
	IsPrimary bool
	Color string
}

type Renderer interface {
	Render(grid [][]rune, placed []Placement)
}

var options struct {
	Style string
	WholeWordColor bool
}

func main() {
	flag.StringVar(&options.Style, "style", "default", "render style: default | matrix")
        flag.BoolVar(&options.WholeWordColor, "whole", false, "color per word instead of per character")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

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

	for _, w := range words[1:] {
		tryPlace(grid, &placed, w)
	}

	var renderer Renderer
	switch options.Style {
	case "matrix":
		renderer = MatrixRenderer{}
	default:
		renderer = DefaultRenderer{}
	}

	renderer.Render(grid, placed)
}

// ───────────────── puzzle construction ─────────────────

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
