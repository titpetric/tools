package model

import "strings"

// File is one file of a package, as the parse read it.
//
// A file that declares nothing was invisible to the model before this: a
// declaration records the file it is in and the import set is keyed by
// filename, and neither says anything about a file holding neither. A check
// that counts files, or pairs one with its test, has to be able to see them
// all.
type File struct {
	// Name is the base filename, which is what a declaration records.
	Name string `json:"Name" yaml:"Name"`

	// Lines is the code in the file, blanks and comments left out, and Size
	// is the bytes it takes on disk.
	Lines int `json:"Lines" yaml:"Lines"`
	Size  int `json:"Size" yaml:"Size"`

	// Generated reports the "Code generated ... DO NOT EDIT." marker, which
	// is a file nobody wrote and no check should judge.
	Generated bool `json:"Generated,omitempty" yaml:"Generated,omitempty"`

	// Test reports a file the toolchain compiles into the test binary.
	Test bool `json:"Test,omitempty" yaml:"Test,omitempty"`
}

// Base is the filename with the .go suffix and the _test marker taken off,
// which is what pairs a file with the one that tests it.
func (f File) Base() string {
	name := strings.TrimSuffix(f.Name, ".go")
	return strings.TrimSuffix(name, "_test")
}

// FileList is the files of one package.
type FileList []File

// Find returns the file of a name, and whether the package holds one.
func (l FileList) Find(name string) (File, bool) {
	for _, file := range l {
		if file.Name == name {
			return file, true
		}
	}
	return File{}, false
}

// Lines is the code of every file together.
func (l FileList) Lines() int {
	total := 0
	for _, file := range l {
		total += file.Lines
	}
	return total
}

// Filter returns the files a test accepts.
func (l FileList) Filter(match func(File) bool) FileList {
	var kept FileList
	for _, file := range l {
		if match(file) {
			kept = append(kept, file)
		}
	}
	return kept
}
