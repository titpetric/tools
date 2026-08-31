package render

// The xterm256 palette, carried over from worktree so a report from either
// tool reads the same on the same terminal.
//
// The colours are written as escape codes rather than built through a styling
// library: the output is static, and a table that is drawn once needs nothing
// a constant cannot give it.
const (
	colorReset     = "\033[0m"
	colorSeparator = "\033[38;5;238m"
	colorHeader    = "\033[38;5;146m"
	colorAmber     = "\033[38;5;214m"
	colorGreen     = "\033[38;5;114m"
	colorTeal      = "\033[38;5;72m"
	colorGrey      = "\033[38;5;245m"
	colorRed       = "\033[38;5;167m"

	// colorSection opens a section of a report. It is bold, and no table is
	// drawn in it, so a heading is told apart from the column headings under
	// it.
	colorSection = "\033[1;38;5;220m"
)

// paint wraps a value in a colour, and leaves it alone when there is no colour
// to paint it or nothing to paint.
func paint(value, color string) string {
	if color == "" || value == "" {
		return value
	}
	return color + value + colorReset
}
