package gofsck

import "go/token"

// AnalyzerSymbol holds information about a symbol: its filename, name, receiver (if any), and if it's a test symbol.
type AnalyzerSymbol struct {
	Filename string
	Symbol   string
	Receiver string
	Type     string
	Package  string
	Default  string

	Pos token.Pos
}

func (s *AnalyzerSymbol) String() string {
	if s.Receiver != "" {
		return s.Receiver + "." + s.Symbol
	}
	return s.Symbol
}
