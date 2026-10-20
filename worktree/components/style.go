package components

// xterm256 color constants.
const (
	ColorReset      = "\033[0m"
	ColorBorder     = "\033[38;5;60m"
	ColorSeparator  = "\033[38;5;238m"
	ColorHeader     = "\033[38;5;146m"
	ColorAmber      = "\033[38;5;214m"
	ColorDarkOrange = "\033[38;5;166m"
	ColorGreen      = "\033[38;5;114m"
	ColorGreenLt    = "\033[38;5;156m"
	ColorTeal       = "\033[38;5;72m"
	ColorWhite      = "\033[38;5;255m"
	ColorYellow     = "\033[38;5;220m"
	ColorRed        = "\033[38;5;167m"

	// ColorSection is the heading that opens a section of a report. The heading
	// prints bold and outside any table, so it stands apart from the column
	// headings of the table below it.
	ColorSection = "\033[1;38;5;220m"

	// ColorTitle is the line a report opens on, which names the subject of the
	// whole report and stands out as the brightest thing in it.
	ColorTitle = "\033[1;38;5;255m"
)
