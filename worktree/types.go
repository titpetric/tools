package main

// xterm256 color helpers - dark orange with green accents
const (
	colorReset     = "\033[0m"
	colorBorder    = "\033[38;5;60m"  // borders, structure - dusty slate
	colorSeparator = "\033[38;5;238m" // row separators - dark slate
	colorHeader    = "\033[38;5;146m" // headers - bright lavender/slate
	colorAmber     = "\033[38;5;214m" // paths, secondary text
	colorGreen     = "\033[38;5;114m" // up-to-date, accents
	colorGreenLt   = "\033[38;5;156m" // module names
	colorTeal      = "\033[38;5;72m"  // git hash, branch
	colorWhite     = "\033[38;5;255m" // primary values
	colorYellow    = "\033[38;5;220m" // warnings, commits ahead
	colorRed       = "\033[38;5;167m" // alerts
)

type gitStatus struct {
	Unpushed  int
	Modified  int
	DiffLines []string // git diff --stat output lines
}

type moduleInfo struct {
	Name        string
	Path        string
	Description string
	Latest      string
	Ahead       int
	GitBranch   string
	Git         string
	GitMsgs     []string
	Uses        []string
	UsedBy      []string
}

type requireInfo struct {
	path    string
	version string
}
