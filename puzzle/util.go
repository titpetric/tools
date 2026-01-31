package main

func bounds(grid [][]rune) (minX, minY, maxX, maxY int) {
	minX, minY = GridW, GridH
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
	return
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

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}
