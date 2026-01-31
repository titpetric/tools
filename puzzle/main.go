package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"
)

const (
	GridW = 120
	GridH = 80
)

type Placement struct {
	Word      string
	X, Y      int
	DX, DY    int
	IsPrimary bool
	Color     string
}

type Renderer interface {
	Render(grid [][]rune, placed []Placement)
}

var options struct {
	Style           string
	FullPackageName bool
	WholeWordColor  bool
	Width, Height   int
}

func main() {
	flag.StringVar(&options.Style, "style", "default", "render style: default | matrix")
	flag.BoolVar(&options.WholeWordColor, "whole", false, "color per word instead of per character")
	flag.BoolVar(&options.FullPackageName, "full", false, "print full package name")
	flag.IntVar(&options.Width, "width", 120, "terminal width")
	flag.IntVar(&options.Height, "height", 80, "terminal height")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	words := getPackages()
	if len(words) == 0 {
		fmt.Println("no packages found")
		return
	}

	width, height := options.Width, options.Height
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		for j := range grid[i] {
			grid[i][j] = ' ' // fill with spaces
		}
	}

	mainWord := words[0]

	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })

	var placed []Placement

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
