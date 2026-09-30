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

	rewritten, formatted, err := Apply(plan)
	require.NoError(t, err)
	require.Equal(t, []string{"crlf.go"}, rewritten)
	require.Empty(t, formatted, "a file whose block was rewritten is not counted twice")

	out := read(t, path)
	assert.NotContains(t, strings.ReplaceAll(out, "\r\n", ""), "\n", "the file came out with both line endings")
	assert.Equal(t, "package x\r\n\r\nimport (\r\n\t\"fmt\"\r\n\t\"sort\"\r\n)\r\n\r\nfunc F() {}\r\n", out)
}

// TestApplyEndsAFileWithANewline covers a file that ends without one, which
// gofmt gives a newline and so does this: the formatting pass is gofmt's, and a
// file this one wrote is a file gofmt would leave alone.
func TestApplyEndsAFileWithANewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bare.go")

	require.NoError(t, os.WriteFile(path, []byte("package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n\nfunc F() {}"), 0o644))

	_, _, err := Apply(&Plan{Files: []FileFix{{
		Path:  path,
		Name:  "bare.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}})
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(read(t, path), "\n"), "the file came out without a trailing newline")
}

// TestApplyFormatsAFileWithNoEdits covers the file the fixer never used to
// open: its import block is what the rules describe, and everything below it is
// gofmt's business. A plan entry with no edits is the format-only case.
func TestApplyFormatsAFileWithNoEdits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fields.go")

	// The block is already right and the struct fields are not aligned, which
	// is what gofmt does and splint fix did not.
	source := "package x\n\nimport (\n\t\"fmt\"\n)\n\ntype T struct {\n\tName string\n\tID int\n}\n\nfunc F(t T) { fmt.Println(t.Name) }\n"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	rewritten, formatted, err := Apply(&Plan{Files: []FileFix{{Path: path, Name: "fields.go"}}})
	require.NoError(t, err)
	assert.Empty(t, rewritten, "no import block changed")
	assert.Equal(t, []string{"fields.go"}, formatted)

	assert.Contains(t, read(t, path), "Name string\n\tID   int", "the fields came out unaligned")
}

// TestApplyLeavesAFileThatDoesNotParse covers the file gofmt cannot read: a
// syntax error is the compiler's finding and not this command's, and the import
// block is still written.
func TestApplyLeavesAFileThatDoesNotParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")

	source := "package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n\nfunc F( {}\n"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	rewritten, _, err := Apply(&Plan{Files: []FileFix{{
		Path:  path,
		Name:  "broken.go",
		Edits: []Edit{{Line: 3, EndLine: 6, Text: "import (\n\t\"fmt\"\n\t\"sort\"\n)"}},
	}}})
	require.NoError(t, err)
	require.Equal(t, []string{"broken.go"}, rewritten)

	out := read(t, path)
	assert.Contains(t, out, "import (\n\t\"fmt\"\n\t\"sort\"\n)", "the block was not written")
	assert.Contains(t, out, "func F( {}", "the unparsable half was rewritten")
}

// TestApplyKeepsTheFileMode covers a file that is not 0644.
func TestApplyKeepsTheFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mode.go")

	require.NoError(t, os.WriteFile(path, []byte("package x\n\nimport (\n\t\"sort\"\n\t\"fmt\"\n)\n"), 0o600))

	_, _, err := Apply(&Plan{Files: []FileFix{{
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

	_, _, err := Apply(&Plan{Files: []FileFix{{
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
