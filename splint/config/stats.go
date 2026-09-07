package config

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// StatsFilename is the file the counters live in, under the directory the
// operating system keeps a user's configuration in.
//
// They are not kept beside the tree. A tree's own file states the rules its
// source is formatted under, which is a decision its authors made and commit;
// a counter is what one person's runs have done and belongs to that person's
// machine. Keeping it in the tree meant every run that rewrote a file left the
// repository dirty and every branch carried a different number.
const StatsFilename = "splint.yml"

// Stats is what runs of splint have counted.
type Stats struct {
	Imports StatsImports `yaml:"imports"`
}

// StatsImports is what the import fixer has counted.
type StatsImports struct {
	// Fixed is how many times a .go file has had its import block rewritten,
	// across every run this user has made, against any tree.
	Fixed int `yaml:"fixed"`
}

// StatsPath is the file the counters are kept in.
func StatsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StatsFilename), nil
}

// LoadStats reads the counters. A machine that has none reads zeroes and no
// error.
func LoadStats() (Stats, error) {
	var stats Stats

	path, err := StatsPath()
	if err != nil {
		return stats, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return stats, err
	}

	if err := yaml.Unmarshal(data, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

// AddFixed adds n to imports.fixed and writes the file back.
//
// The file is edited rather than re-encoded from the struct: somebody who
// wrote a comment beside a counter keeps it. The whole document is kept as
// nodes and one scalar is changed.
//
// A machine with no file gets one holding the count and nothing else. Adding
// nothing writes nothing.
func AddFixed(n int) error {
	if n <= 0 {
		return nil
	}

	path, err := StatsPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		data = nil
	} else if err != nil {
		return err
	}

	var document yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &document); err != nil {
			return err
		}
	}

	counter := entry(documentRoot(&document), "imports")

	value := field(counter, "fixed")
	count, err := strconv.Atoi(value.Value)
	if err != nil {
		count = 0
	}
	value.Kind, value.Tag, value.Style = yaml.ScalarNode, "!!int", 0
	value.Content = nil
	value.Value = strconv.Itoa(count + n)

	out, err := marshal(&document)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return write(path, out)
}

// documentRoot is the mapping at the top of a document, added when the
// document is empty.
func documentRoot(document *yaml.Node) *yaml.Node {
	if document.Kind == 0 {
		document.Kind = yaml.DocumentNode
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}

	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
		root.Value = ""
		root.Content = nil
	}

	return root
}

// entry is the mapping under a key of a mapping, added when the key is not
// there and replaced when what is there is not a mapping.
func entry(parent *yaml.Node, key string) *yaml.Node {
	found := field(parent, key)
	if found.Kind != yaml.MappingNode {
		found.Kind = yaml.MappingNode
		found.Tag = "!!map"
		found.Value = ""
		found.Content = nil
	}
	return found
}

// field is the value node under a key of a mapping, added when the key is not
// there.
func field(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}

	name := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"}
	parent.Content = append(parent.Content, name, value)

	return value
}

// marshal writes the document at two spaces, which is what the rest of the
// YAML in this workspace is written at. yaml.Marshal writes four.
func marshal(document *yaml.Node) ([]byte, error) {
	if len(document.Content) == 0 {
		return nil, nil
	}

	out := &bytes.Buffer{}
	encoder := yaml.NewEncoder(out)
	encoder.SetIndent(2)

	if err := encoder.Encode(document.Content[0]); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// write replaces the file, through a temporary beside it so a run interrupted
// halfway leaves the file it found rather than half of one.
func write(path string, data []byte) error {
	temp := path + ".tmp"

	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}

	return nil
}
