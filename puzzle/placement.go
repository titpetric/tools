package main

import "math/rand"

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

					var color string
					if options.WholeWordColor {
						color = DefaultColors[rand.Intn(len(DefaultColors))]
					}

					*placed = append(*placed, Placement{
						Word:  word,
						X:     x,
						Y:     y,
						DX:    dx,
						DY:    dy,
						Color: color,
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
