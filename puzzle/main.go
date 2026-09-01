package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
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
	Help            bool
}

// bindFlags defines the command line on a parser. The help page is written
// from the same parser, so a flag is documented by having been defined.
func bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&options.Style, "style", "default", "render with `STYLE`: default or matrix")
	fs.BoolVar(&options.WholeWordColor, "whole", false, "color per word instead of per character")
	fs.BoolVar(&options.FullPackageName, "full", false, "print full package name")
	fs.IntVar(&options.Width, "width", 120, "terminal `WIDTH`")
	fs.IntVar(&options.Height, "height", 80, "terminal `HEIGHT`")
	fs.BoolVar(&options.Help, "help", false, "print this help")
}

// helpSpec is the page the command prints.
func helpSpec(fs *flag.FlagSet) spec {
	return spec{
		Name:    "puzzle",
		Tagline: "draw the packages of a Go module as a crossword",
		Usage: []string{
			"puzzle [flags]",
		},
		Description: `puzzle reads the module in the current directory, takes its name and the name
of every package under it, and lays the names out as a crossword: the module
name across the middle, and each package crossing a word already on the grid
at a letter they share. A word that finds no crossing is left out.

The names come from "go mod edit -json" and "go list ./...", so the command
runs where a Go module is. The module name is the primary word and is drawn in
the accent colour; the packages around it are shaded.`,
		Flags: fs,
		Examples: []example{
			{"puzzle", "draw the packages of the module in this directory"},
			{"puzzle -style matrix", "draw the same puzzle in green, on a field of noise"},
			{"puzzle -full", "use the full module path as the primary word, not its last element"},
			{"puzzle -whole -width 160 -height 120", "one colour per word, on a larger canvas"},
		},
		Notes: `Placement is random, so two runs over one module draw two puzzles. A run that
left out too much is worth repeating.

-whole is read by the default style alone. The matrix style colours every
letter it draws.

Words are placed inside a grid of 120 by 80, which -width and -height do not
move: they size the canvas that grid is written into, and a size under 120 by
80 is smaller than what placement uses.`,
	}
}

func main() {
	fs := flag.NewFlagSet("puzzle", flag.ContinueOnError)

	// helpSpec is the only help this command writes, so the parser's own usage
	// is discarded rather than printed.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	bindFlags(fs)

	err := fs.Parse(os.Args[1:])
	if options.Help || err == flag.ErrHelp {
		if err := writeHelp(os.Stdout, helpSpec(fs)); err != nil {
			fmt.Fprintln(os.Stderr, "puzzle:", err)
			os.Exit(2)
		}
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "puzzle:", err)
		os.Exit(2)
	}

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
