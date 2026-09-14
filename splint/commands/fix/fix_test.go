package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpliceRefusesARangeThatIsNotAnImport is the guard that bounds what a
// parser bug can cost.
//
// The line range comes from a parse. One that read the range wrong had this
// replace whatever was actually there: a block whose closing paren is indented
// was read as ending at the end of the file, and the rewrite replaced the file
// with an import block. Nothing is written unless the lines about to go really
// are an import declaration.
func TestSpliceRefusesARangeThatIsNotAnImport(t *testing.T) {
	source := "package x\n\nimport (\n\t\"fmt\"\n)\n\nfunc F() { fmt.Println() }\n"

	tests := []struct {
		name string
		edit Edit
	}{
		{"a range running past the declaration", Edit{Line: 3, EndLine: 7, Text: "import (\n\t\"fmt\"\n)"}},
		{"a range that is not an import at all", Edit{Line: 7, EndLine: 7, Text: "import (\n\t\"fmt\"\n)"}},
		{"a range past the end of the file", Edit{Line: 3, EndLine: 99, Text: "import ()"}},
		{"a range that ends before it starts", Edit{Line: 5, EndLine: 3, Text: "import ()"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, ok := Splice(source, []Edit{test.edit})
			assert.False(t, ok, "the edit was accepted")
			assert.Equal(t, source, out, "the file was changed by a refused edit")
		})
	}
}

// TestSpliceReplacesTheDeclaration covers the edit that is accepted.
func TestSpliceReplacesTheDeclaration(t *testing.T) {
	source := "package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n\nfunc F() { fmt.Println(sort.IntsAreSorted(nil)) }\n"

	out, ok := Splice(source, []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}})
	require.True(t, ok)
	assert.Equal(t, "package x\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n)\n\nfunc F() { fmt.Println(sort.IntsAreSorted(nil)) }\n", out)
}

// TestSpliceDeletesADeclarationAndItsBlankLine covers a second import
// declaration merging into the first.
func TestSpliceDeletesADeclarationAndItsBlankLine(t *testing.T) {
	source := "package x\n\nimport \"fmt\"\n\nimport \"sort\"\n\nfunc F() {}\n"

	out, ok := Splice(source, []Edit{
		{Line: 3, EndLine: 3, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"},
		{Line: 5, EndLine: 5},
	})
	require.True(t, ok)
	assert.Equal(t, "package x\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n)\n\nfunc F() {}\n", out)
}

// TestApplyKeepsCRLF covers a file written under the other line ending
// convention. Writing a block composed with newlines into one leaves a file
// with both, which reads as a change to every line of it.
func TestApplyKeepsCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.go")

	source := "package x\r\n\r\nimport (\r\n\t\"sort\"\r\n\t\"fmt\"\r\n)\r\n\r\nfunc F() {}\r\n"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	plan := &Plan{Files: []FileFix{{
		Path:  path,
		Name:  "crlf.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}}

	changed, err := Apply(plan)
	require.NoError(t, err)
	require.Equal(t, []string{"crlf.go"}, changed)

	out := read(t, path)
	assert.NotContains(t, strings.ReplaceAll(out, "\r\n", ""), "\n", "the file came out with both line endings")
	assert.Equal(t, "package x\r\n\r\nimport (\r\n\t\"fmt\"\r\n\t\"sort\"\r\n)\r\n\r\nfunc F() {}\r\n", out)
}

// TestApplyKeepsAFileWithNoTrailingNewline covers a file that ends without
// one. A formatter that added one would be changing a line nobody asked it to.
func TestApplyKeepsAFileWithNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bare.go")

	require.NoError(t, os.WriteFile(path, []byte("package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n\nfunc F() {}"), 0o644))

	_, err := Apply(&Plan{Files: []FileFix{{
		Path:  path,
		Name:  "bare.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}})
	require.NoError(t, err)

	assert.False(t, strings.HasSuffix(read(t, path), "\n"), "a trailing newline was added")
}

// TestApplyKeepsTheFileMode covers a file that is not 0644.
func TestApplyKeepsTheFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mode.go")

	require.NoError(t, os.WriteFile(path, []byte("package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n"), 0o600))

	_, err := Apply(&Plan{Files: []FileFix{{
		Path:  path,
		Name:  "mode.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestApplyLeavesNoTemporary covers the write: the file is replaced through a
// temporary beside it, and the temporary is not left behind.
func TestApplyLeavesNoTemporary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.go")

	require.NoError(t, os.WriteFile(path, []byte("package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n"), 0o644))

	_, err := Apply(&Plan{Files: []FileFix{{
		Path:  path,
		Name:  "one.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}})
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "one.go", entries[0].Name())
}

// TestInsertPutsTheBlockUnderThePackageClause covers a file that imports
// nothing and needs to.
func TestInsertPutsTheBlockUnderThePackageClause(t *testing.T) {
	source := "// Package x does a thing.\npackage x\n\nfunc F() string { return fmt.Sprint(1) }\n"

	out, ok := Splice(source, []Edit{{Text: "import (\n\t\"fmt\"\n)"}})
	require.True(t, ok)
	assert.Equal(t, "// Package x does a thing.\npackage x\n\nimport (\n\t\"fmt\"\n)\n\nfunc F() string { return fmt.Sprint(1) }\n", out)
}

// read returns a file as it was written, line endings and all.
func read(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
