package mdfmt

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Prose reads the paragraphs of every markdown file under a root.
//
// Each path comes back relative to the root. A document then reads the same
// from any directory.
func Prose(root string, recursive bool) ([]*model.ProseFile, error) {
	files, err := Files(root, recursive)
	if err != nil {
		return nil, err
	}

	var found []*model.ProseFile
	for _, path := range files {
		file, err := ProseFile(root, path)
		if err != nil {
			return nil, err
		}
		if file != nil && len(file.Blocks) > 0 {
			found = append(found, file)
		}
	}

	return found, nil
}

// ProseFile reads one file's paragraphs.
//
// A fence, a table and a heading come out. The rest is running text, one
// block per paragraph, each joined into the sentence a reader reads.
func ProseFile(root, path string) (*model.ProseFile, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	name := path
	if rel, err := filepath.Rel(root, path); err == nil {
		name = filepath.ToSlash(rel)
	}

	var (
		file    = &model.ProseFile{Path: name}
		scanner = bufio.NewScanner(handle)
		lines   []string
		start   int
		number  int
		fenced  bool
	)

	flush := func() {
		if len(lines) > 0 {
			file.Blocks = append(file.Blocks, model.ProseBlock{
				Line: start,
				Text: strings.Join(strings.Fields(strings.Join(lines, " ")), " "),
			})
			lines = nil
		}
	}

	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		number++
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			flush()
			fenced = !fenced
			continue
		}
		if fenced || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
			flush()
			continue
		}

		if len(lines) == 0 {
			start = number
		}
		lines = append(lines, line)
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return file, nil
}
