package yamlprose_test

import (
	"testing"

	"github.com/titpetric/tools/splint/yamlprose"
)

func TestSentence(t *testing.T) {
	for _, value := range []string{
		"Write the markdown in one format",
		"Check the markdown formatting and language",
		"Bring up the docker env",
		"Formats and checks markdown files.",
		"Runs the tasks defined in Taskfile.yml.",
		"Parses Go into a data model, lints it, and renders docs off it.",
		"Install splint, which formats the markdown",
		"Report the constructions this tree leans on",
		"a comment explaining a decision",
		"Testing and coverage",
		"Documentation and diagrams",
	} {
		if !yamlprose.Sentence(value) {
			t.Errorf("Sentence(%q) = false, want a sentence", value)
		}
	}

	for _, value := range []string{
		// Commands, which is most of what a cmds list holds.
		"go install github.com/titpetric/tools/splint@latest",
		"splint fix --markdown ./...",
		"docker run --rm -v $PWD:/src -w /src jdkato/vale --output=line .",
		"go test -count=1 ./...",
		"find ./docs -name '*.md'",
		"task: mdox:fmt",
		"plantuml -tsvg modules/*/schema.puml",
		"phpscript run tools/docs.php",
		"cat git-tags.txt | semver | jq . > git-tags.json",
		// Data: paths, modules, versions, identifiers, globs.
		"github.com/titpetric/tools/splint",
		"https://github.com/bwplotka/mdox",
		"example.com/app/storage",
		"~/.atkins/skills/mdox.yml",
		"github/tools/docs/atkins.yml",
		"v1.19.2",
		"golang:1.27",
		"/usr/local/bin",
		"the one",
		"a thing",
		"Security",
		"",
		"   ",
		"Database schema migrations",
		"markdown formatting linting",
		"Shell helpers",
	} {
		if yamlprose.Sentence(value) {
			t.Errorf("Sentence(%q) = true, want data", value)
		}
	}
}

// The test knows no schema: a key named desc proves nothing. A command
// under a key named desc is still a command.
func TestSentenceReadsTheValueAndNotTheKey(t *testing.T) {
	if yamlprose.Sentence("go build -o bin/ ./cmd/yamlexpr") {
		t.Error("Sentence() took a command")
	}
	if !yamlprose.Sentence("Build the binary into the bin directory") {
		t.Error("Sentence() refused a sentence")
	}
}
