package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

func TestBrowsableRemote(t *testing.T) {
	tests := map[string]string{
		"git@github.com:titpetric/platform.git":       "https://github.com/titpetric/platform",
		"git@github.com:titpetric/platform":           "https://github.com/titpetric/platform",
		"ssh://git@github.com/titpetric/platform.git": "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform.git":   "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform":       "https://github.com/titpetric/platform",
		"http://git.example.com/team/project.git":     "http://git.example.com/team/project",
		"git@git.example.com:2222/team/project.git":   "https://git.example.com/2222/team/project",
		"  git@github.com:titpetric/platform.git\n":   "https://github.com/titpetric/platform",
		// Nothing a browser opens, so nothing to link to.
		"":                        "",
		"/srv/git/project.git":    "",
		"git://github.com/x/y":    "",
		"file:///srv/git/project": "",
	}

	for remote, want := range tests {
		if got := browsableRemote(remote); got != want {
			t.Errorf("browsableRemote(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestCommitLogCoversTheModuleOnly(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: change")
	writeTestFile(t, filepath.Join(root, "beta", "beta.go"), "package beta\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "beta: change")
	runGit(t, root, "tag", "alpha/v0.2.0")

	alpha := filepath.Join(root, "alpha")

	got := commitLogSinceTag(alpha, "alpha/v0.1.0")
	if len(got) != 1 || got[0].Subject != "alpha: change" {
		t.Fatalf("commitLogSinceTag() = %#v, want the one commit touching alpha", got)
	}
	if got[0].Hash == "" {
		t.Error("commitLogSinceTag() returned a commit without a hash")
	}

	// A range between two tags is what a released module reports on.
	if got := commitLogBetween(alpha, "alpha/v0.1.0", "alpha/v0.2.0"); len(got) != 1 {
		t.Errorf("commitLogBetween() = %d commits, want 1", len(got))
	}

	// Without a tag the whole history of the module is what a first release
	// would cover.
	if got := commitLogSinceTag(alpha, ""); len(got) != 2 {
		t.Errorf("commitLogSinceTag() without a tag = %d commits, want 2", len(got))
	}
}

// verdictRepo builds a module with two releases and returns its directory. The
// second release removes Greet and adds Bye, so it is a breaking one.
func verdictRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: replace Greet with Bye")
	runGit(t, root, "tag", "alpha/v0.2.0")

	return alpha
}

func TestReadVerdictReportsTheLastReleaseWhenLevelWithItsTag(t *testing.T) {
	requireGoFsck(t)

	got, err := readVerdict(verdictRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	if !got.Released {
		t.Error("readVerdict() proposed a release for a module level with its tag")
	}
	if got.Version != "v0.2.0" || got.Since != "v0.1.0" {
		t.Errorf("readVerdict() = {Version: %q, Since: %q}, want {v0.2.0, v0.1.0}", got.Version, got.Since)
	}
	if len(got.Commits) != 1 || got.Commits[0].Subject != "alpha: replace Greet with Bye" {
		t.Errorf("readVerdict() Commits = %#v, want the commit between the two tags", got.Commits)
	}
	// The comparison is between the tags, not against the working tree.
	if len(got.API.Removed) != 1 || got.API.Removed[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Removed = %#v, want Greet", got.API.Removed)
	}
	if want := "Released v0.2.0: 1 exported symbol was removed since v0.1.0."; got.Summary() != want {
		t.Errorf("Summary() = %q, want %q", got.Summary(), want)
	}
}

func TestReadVerdictProposesAReleaseWhenBehind(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n\n// Hi greets.\nfunc Hi() string { return \"hi\" }\n")
	runGit(t, filepath.Dir(alpha), "commit", "--quiet", "-am", "alpha: add Hi")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	if got.Released {
		t.Error("readVerdict() reported a release that has not been made")
	}
	if got.Version != "v0.2.1" || got.Since != "v0.2.0" || got.Release != releasePatch {
		t.Errorf("readVerdict() = {Version: %q, Since: %q, Release: %q}, want {v0.2.1, v0.2.0, patch}", got.Version, got.Since, got.Release)
	}
	if len(got.API.Added) != 1 || got.API.Added[0].Name != "Hi" {
		t.Errorf("readVerdict() API.Added = %#v, want Hi", got.API.Added)
	}
}

// The first release has no earlier one to be compared against, so everything it
// exports is reported as added.
func TestReadVerdictWithOneTagAddsEverythingItExports(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if !got.Released || got.Version != "v0.1.0" || got.Since != "" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {true, v0.1.0, \"\"}", got.Released, got.Version, got.Since)
	}
	if got.API.Skipped != "" {
		t.Fatalf("readVerdict() API.Skipped = %q, want the API read against nothing", got.API.Skipped)
	}
	if len(got.API.Added) != 1 || got.API.Added[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Added = %#v, want Greet", got.API.Added)
	}
	if got.API.Breaking {
		t.Error("readVerdict() called a first release breaking")
	}
}

func TestReadVerdictWithoutAReleaseTag(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n\n// Tag is a release tag.\ntype Tag struct {\n\tName string\n}\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Released || got.Version != "v0.0.1" || got.Since != "" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {false, v0.0.1, \"\"}", got.Released, got.Version, got.Since)
	}
	if len(got.Commits) == 0 {
		t.Error("readVerdict() without a tag reported no commits, want the whole history")
	}

	// Everything the working tree exports is an addition, since the release is
	// measured against a module holding nothing at all.
	var names []string
	for _, symbol := range got.API.Added {
		names = append(names, symbol.Name)
	}
	if want := []string{"Greet", "Tag"}; !reflect.DeepEqual(names, want) {
		t.Errorf("readVerdict() API.Added = %v, want %v", names, want)
	}
	if got.Release != releasePatch {
		t.Errorf("readVerdict() Release = %q, want a patch: a first release takes nothing away", got.Release)
	}

	// The fields of an added type are read as additions of their own, so the
	// first release states the shape it declares.
	if fields := len(dataModelEntries(got)); fields != 1 {
		t.Errorf("dataModelEntries() = %d entries, want the one field of Tag", fields)
	}
}

func TestApiDiffBetweenReadsTwoTagsAndNotTheWorkingTree(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	// A working tree that would swamp the answer if it were being read.
	writeTestFile(t, filepath.Join(alpha, "stray.go"), "package alpha\n\n// Stray is uncommitted.\nfunc Stray() {}\n")

	got := apiDiffBetween(alpha, "alpha/v0.1.0", "alpha/v0.2.0")
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if len(got.Added) != 1 || got.Added[0].Name != "Bye" {
		t.Errorf("apiDiffBetween() Added = %#v, want Bye alone", got.Added)
	}
	if !got.Breaking || len(got.Removed) != 1 {
		t.Errorf("apiDiffBetween() = %#v, want a breaking result removing Greet", got)
	}
}

func TestApiDiffBetweenReadsTheExportedShapeOfAnAddedType(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Grouped names declare a field each, and an unexported one is nobody's
	// promise to keep.
	writeTestFile(t, filepath.Join(alpha, "alpha.go"),
		"package alpha\n\n// Tag is a release tag.\ntype Tag struct {\n\tName string `json:\"name\"`\n\n\tMajor, Minor, Patch uint64\n\n\traw string\n}\n")

	got := apiDiffBetween(alpha, "alpha/v0.1.0", "")
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if len(got.Added) != 1 {
		t.Fatalf("apiDiffBetween() Added = %#v, want Tag alone", got.Added)
	}

	tag := got.Added[0]
	if tag.Underlying != "struct" {
		t.Errorf("Tag.Underlying = %q, want struct", tag.Underlying)
	}

	var names []string
	for _, field := range tag.Fields {
		names = append(names, field.Name)
	}
	want := []string{"Major", "Minor", "Name", "Patch"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Tag.Fields = %#v, want %#v", names, want)
	}

	for _, field := range tag.Fields {
		if field.Name == "Name" && field.Tag != `json:"name"` {
			t.Errorf("Name lost its tag: %#v", field)
		}
	}
}

func TestApiDiffBetweenReportsAFieldThatMoved(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"),
		"package alpha\n\n// Config configures.\ntype Config struct {\n\tAddr string `yaml:\"addr\"`\n\tRetries int\n}\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Config")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"),
		"package alpha\n\n// Config configures.\ntype Config struct {\n\tAddr []string `yaml:\"addr\"`\n\tTimeout int\n}\n")

	got := apiDiffBetween(alpha, "alpha/v0.1.0", "")
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if len(got.Types) != 1 {
		t.Fatalf("apiDiffBetween() Types = %#v, want Config alone", got.Types)
	}

	change := got.Types[0]
	if change.Name != "Config" || change.Underlying != "struct" {
		t.Errorf("Types[0] = {Name: %q, Underlying: %q}, want {Config, struct}", change.Name, change.Underlying)
	}

	moved := make(map[string]string)
	for _, field := range change.Fields {
		moved[field.Name] = field.Change
	}
	want := map[string]string{"Addr": fieldChanged, "Retries": fieldRemoved, "Timeout": fieldAdded}
	if !reflect.DeepEqual(moved, want) {
		t.Errorf("Config fields = %#v, want %#v", moved, want)
	}

	// Taking a field away costs a consumer something, so the release is a
	// minor even though no symbol went away.
	if !got.Breaking {
		t.Error("apiDiffBetween() did not call a removed field breaking")
	}
}

func TestVerdictSummary(t *testing.T) {
	tests := []struct {
		name string
		in   verdict
		want string
	}{
		{
			name: "first release",
			in: verdict{
				Version: "v0.0.1",
				API:     apiDiff{Added: []apiSymbol{{Key: "x.A", Exported: true}, {Key: "x.B", Exported: true}}},
			},
			want: "First release: v0.0.1, 2 exported symbols are added.",
		},
		{
			name: "first release, the API was not read",
			in:   verdict{Version: "v0.0.1", API: apiDiff{Skipped: "go-fsck is not installed"}},
			want: "First release: v0.0.1, the API was not read, go-fsck is not installed.",
		},
		{
			name: "patch",
			in:   verdict{Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch},
			want: "Patch release: v1.0.1, no exported symbols were removed since v1.0.0.",
		},
		{
			name: "minor, one removal",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A", Exported: true}}, Breaking: true},
			},
			want: "Minor release: v1.1.0, because 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "minor, removals and signature changes",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{
					Removed:  []apiSymbol{{Key: "x.A", Exported: true}, {Key: "x.B", Exported: true}},
					Changed:  []apiChange{{Key: "x.C"}},
					Breaking: true,
				},
			},
			want: "Minor release: v1.1.0, because 2 exported symbols were removed and 1 signature changed since v1.0.0.",
		},
		{
			name: "nothing to compare",
			in: verdict{
				Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch,
				API: apiDiff{Skipped: "go-fsck is not installed"},
			},
			want: "Patch release: v1.0.1, the API was not compared, go-fsck is not installed.",
		},
		{
			name: "a release that was made",
			in:   verdict{Version: "v1.1.0", Since: "v1.0.0", Released: true},
			want: "Released v1.1.0: no exported symbols were removed since v1.0.0.",
		},
		{
			name: "a release that was made, breaking",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Released: true,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A", Exported: true}}, Breaking: true},
			},
			want: "Released v1.1.0: 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "a release with nothing before it",
			in: verdict{
				Version: "v1.0.0", Released: true,
				API: apiDiff{Added: []apiSymbol{{Key: "x.A", Exported: true}}},
			},
			want: "Released v1.0.0: the first release, 1 exported symbol is added.",
		},
	}

	for _, test := range tests {
		if got := test.in.Summary(); got != test.want {
			t.Errorf("%s: Summary() = %q, want %q", test.name, got, test.want)
		}
	}
}

// sampleVerdict is a verdict holding one of everything the report can show.
func sampleVerdict() verdict {
	return verdict{
		Module:  "example.com/x",
		Version: "v1.1.0",
		Since:   "v1.0.0",
		Release: releaseMinor,
		RepoURL: "https://github.com/example/x",
		Commits: []commitLog{
			{Hash: "abc1234", Subject: "feat: add Client"},
			{Hash: "def5678", Subject: "refactor: drop Legacy"},
		},
		API: apiDiff{
			Removed: []apiSymbol{{
				Key: "example.com/x.Legacy", Package: "example.com/x",
				Name: "Legacy", Kind: "func", Exported: true, Signature: "func Legacy () error",
			}},
			Added: []apiSymbol{{
				Key: "example.com/x.Client", Package: "example.com/x",
				Name: "Client", Kind: "type", Exported: true, Underlying: "struct",
				Fields: []apiField{{Name: "Name", Type: "string", Tag: `json:"name"`}},
			}},
			Changed: []apiChange{{
				Key: "example.com/x.Open", Package: "example.com/x",
				Name: "Open", Exported: true, Old: "Open ()", New: "Open (string)",
			}},
			Types: []apiTypeChange{{
				Key: "example.com/x.Config", Package: "example.com/x",
				Name: "Config", Underlying: "struct", Breaking: true,
				Fields: []apiFieldChange{
					{
						Name: "Addr", Change: fieldChanged,
						Old: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"addr"`},
						New: &apiField{Name: "Addr", Type: "[]string", Tag: `yaml:"addr"`},
					},
					{Name: "Timeout", Change: fieldAdded, New: &apiField{Name: "Timeout", Type: "int"}},
					{Name: "Retries", Change: fieldRemoved, Old: &apiField{Name: "Retries", Type: "int"}},
				},
			}},
			Breaking: true,
		},
	}
}

// sampleInterfaceChange is a verdict whose data model change is to an
// interface, whose fields read as a method set.
func sampleInterfaceChange() verdict {
	v := sampleVerdict()
	v.API.Added = nil
	v.API.Types = []apiTypeChange{{
		Key: "example.com/x/store.Store", Package: "example.com/x/store",
		Name: "Store", Underlying: "interface", Breaking: true,
		Fields: []apiFieldChange{
			{Name: "Put", Change: fieldAdded, New: &apiField{Name: "Put", Type: "Put (key string) error"}},
		},
	}}
	return v
}

func TestRenderVerdictMarkdown(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)

	got := out.String()
	for _, want := range []string{
		"# example.com/x @ v1.1.0",
		"Minor release: v1.1.0, because 1 exported symbol was removed and 1 signature changed and 2 exported fields moved since v1.0.0.",
		"## Commits since v1.0.0",
		"| [`abc1234`](https://github.com/example/x/commit/abc1234) | feat: add Client |",
		"## API since v1.0.0",
		"| Change | Exported | Unexported |",
		"| Added | type Client struct |  |",
		"| Changed | Before: Open ()<br>After: Open (string) |",
		"| Removed | func Legacy () error |  |",
		// One table, whatever the release did to however many types.
		"## Data model since v1.0.0",
		"| Change | Package | Type | Field |",
		// A type the release adds is written as the shape it declares, and
		// carries the mark that says the type itself is new.
		"| Added | / | type Client struct ▲ | Name string `json:\"name\"` |",
		// A type that was already there is written as what moved on it, and
		// the cells repeating the row above are left empty.
		"|  |  | type Config struct | Timeout int ▲ |",
		"| Changed | / | type Config struct | Addr string `yaml:\"addr\"` -> []string `yaml:\"addr\"` |",
		"| Removed | / | type Config struct | Retries int |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote escape codes into markdown")
	}
	// A module with one package has nothing for the API table to disambiguate.
	// The data model table names the package either way, since it has a type
	// column to keep it apart from.
	if strings.Contains(got, "| Change | Package | Exported | Unexported |") {
		t.Errorf("renderVerdict() added a package column for a single package:\n%s", got)
	}
}

func TestRenderVerdictANSI(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), true)

	got := out.String()
	if !strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote no escape codes to a terminal")
	}
	for _, unwanted := range []string{"<details>", "<summary>", "```", "# example.com/x", "| Change |"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote markup %q to a terminal:\n%s", unwanted, got)
		}
	}

	plain := ansi.Strip(got)
	for _, want := range []string{
		"example.com/x @ v1.1.0",
		"Commits since v1.0.0",
		"abc1234",
		"Removed",
		"func Legacy () error",
		"Data model since v1.0.0",
		"type Client struct ▲",
		"Name string `json:\"name\"`",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "╭") {
		t.Errorf("renderVerdict() drew no table:\n%s", plain)
	}

	// A heading sits directly on top of the table it names, and the blank line
	// falls after the table, so the two read as one block.
	if !strings.Contains(plain, "Commits since v1.0.0\n╭") {
		t.Errorf("renderVerdict() parted a heading from its table:\n%s", plain)
	}
	if !strings.Contains(plain, "╯\n\nAPI since v1.0.0") {
		t.Errorf("renderVerdict() left no blank line between two tables:\n%s", plain)
	}
	if !strings.HasSuffix(plain, "╯\n\n") {
		t.Errorf("renderVerdict() did not end on a blank line:\n%s", plain)
	}

	// The heading is not written in the colour the columns of the table are, or
	// it would read as another one of them.
	if !strings.Contains(got, components.ColorSection+"API since v1.0.0") {
		t.Errorf("renderVerdict() wrote a heading in another colour:\n%q", got)
	}
}

func TestRenderVerdictNamesThePackageWhenSymbolsSpanMoreThanOne(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added, apiSymbol{
		Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const", Exported: true,
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	for _, want := range []string{"| Change | Package | Exported | Unexported |", "| /inner | const Name |  |", "| / | type Client struct |  |"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictNamesAPackageOncePerRunOfSymbols(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added,
		apiSymbol{
			Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const", Exported: true,
		},
		apiSymbol{
			Key: "example.com/x.Dial", Package: "example.com/x", Name: "Dial", Kind: "func", Signature: "func Dial () error",
		},
		apiSymbol{
			Key: "example.com/x/inner.Other", Package: "example.com/x/inner", Name: "Other", Kind: "const",
		},
	)

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	// The symbols of a package are gathered together, and only the first of
	// them names it. The removal below opens a group of its own, so the package
	// is named again there.
	for _, want := range []string{
		"| Added | / | type Client struct |  |",
		"|  |  | func Dial () error |",
		"|  | /inner | const Name |",
		"|  |  | const Other |",
		"| Removed | / | func Legacy () error |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
	if n := strings.Count(got, "| /inner |"); n != 1 {
		t.Errorf("renderVerdict() named the package %d times, want once:\n%s", n, got)
	}
}

func TestRenderVerdictWithoutARemoteLeavesHashesUnlinked(t *testing.T) {
	v := sampleVerdict()
	v.RepoURL = ""

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	if !strings.Contains(got, "| `abc1234` | feat: add Client |") {
		t.Errorf("renderVerdict() did not fall back to a plain hash:\n%s", got)
	}
	if strings.Contains(got, "](") {
		t.Errorf("renderVerdict() linked a hash without a remote to link into:\n%s", got)
	}
}

func TestRenderVerdictOmitsEmptySections(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, verdict{Module: "example.com/x", Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch}, false)

	got := out.String()
	if strings.Contains(got, "##") {
		t.Errorf("renderVerdict() wrote a heading with nothing under it:\n%s", got)
	}
	if !strings.Contains(got, "Patch release: v1.0.1") {
		t.Errorf("renderVerdict() did not report the release:\n%s", got)
	}
}

func TestParseOptionsVerdict(t *testing.T) {
	tests := []struct {
		args   []string
		filter string
	}{
		{args: []string{"worktree", "verdict"}},
		{args: []string{"worktree", "verdict", "platform"}, filter: "platform"},
	}

	for _, test := range tests {
		func() {
			args, commandLine := os.Args, flag.CommandLine
			t.Cleanup(func() { os.Args, flag.CommandLine = args, commandLine })

			os.Args = test.args
			flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			opts := ParseOptions()
			if !opts.Verdict {
				t.Fatalf("%v: Verdict = false, want true", test.args)
			}
			if opts.FilterArg != test.filter {
				t.Errorf("%v: FilterArg = %q, want %q", test.args, opts.FilterArg, test.filter)
			}
		}()
	}
}

func TestGoSeriesChanged(t *testing.T) {
	tests := []struct {
		before, after string
		want          bool
	}{
		// Another release series is what a consumer feels.
		{"1.26", "1.27", true},
		{"1.26.4", "1.27.0", true},
		{"go1.26", "go1.27", true},
		{"1.27", "1.26", true},
		// A point release of the same series changes nothing for them.
		{"1.27", "1.27", false},
		{"1.27", "1.27.1", false},
		{"1.27.1", "1.27.9", false},
		// Nothing to read on either side is not a change.
		{"", "1.27", false},
		{"1.27", "", false},
		{"nonsense", "1.27", false},
	}

	for _, test := range tests {
		if got := goSeriesChanged(test.before, test.after); got != test.want {
			t.Errorf("goSeriesChanged(%q, %q) = %v, want %v", test.before, test.after, got, test.want)
		}
	}
}

func TestVerdictMovingGoSeriesCostsAMinor(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Nothing of the API moves, only the language version.
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.GoBefore != "1.24" || got.GoAfter != "1.27" {
		t.Errorf("readVerdict() = {GoBefore: %q, GoAfter: %q}, want {1.24, 1.27}", got.GoBefore, got.GoAfter)
	}
	if got.Release != releaseMinor || got.Version != "v0.2.0" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q}, want {minor, v0.2.0}", got.Release, got.Version)
	}
	if want := "Minor release: v0.2.0, because go moved from 1.24 to 1.27 since v0.1.0."; got.Summary() != want {
		t.Errorf("Summary() = %q, want %q", got.Summary(), want)
	}

	// Once that release is made, a point release of the same series is not
	// worth another minor.
	runGit(t, root, "tag", "alpha/v0.2.0")
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27.3\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27.3")

	got, err = readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Release != releasePatch || got.Version != "v0.2.1" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q} for a point release, want {patch, v0.2.1}", got.Release, got.Version)
	}
}

func TestReadVerdictBetweenNamedRevisions(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	// A working tree the report must not read, since a range was named.
	writeTestFile(t, filepath.Join(alpha, "stray.go"), "package alpha\n\n// Stray is uncommitted.\nfunc Stray() {}\n")

	got, err := readVerdict(alpha, "v0.1.0", "v0.2.0", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if !got.Released || got.Version != "v0.2.0" || got.Since != "v0.1.0" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {true, v0.2.0, v0.1.0}", got.Released, got.Version, got.Since)
	}
	if len(got.API.Removed) != 1 || got.API.Removed[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Removed = %#v, want Greet", got.API.Removed)
	}
	for _, symbol := range got.API.Added {
		if symbol.Name == "Stray" {
			t.Error("readVerdict() read the working tree for a named range")
		}
	}

	// Naming only the newer revision measures from the release below it.
	got, err = readVerdict(alpha, "", "v0.2.0", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Since != "v0.1.0" {
		t.Errorf("readVerdict(--to) Since = %q, want v0.1.0", got.Since)
	}

	// Naming only the older one measures to the working tree, where Stray is.
	got, err = readVerdict(alpha, "v0.1.0", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Released {
		t.Error("readVerdict(--from) reported a release that has not been made")
	}
	var found bool
	for _, symbol := range got.API.Added {
		found = found || symbol.Name == "Stray"
	}
	if !found {
		t.Errorf("readVerdict(--from) did not read the working tree: %#v", got.API.Added)
	}
}

func TestTaggedRef(t *testing.T) {
	tests := []struct {
		ref, prefix, want string
	}{
		{"v0.1.0", "alpha/", "alpha/v0.1.0"},
		{"v0.1.0", "", "v0.1.0"},
		// Anything that is not a version is a commit or a branch, and is left
		// as it was given.
		{"HEAD", "alpha/", "HEAD"},
		{"main", "alpha/", "main"},
		{"29097b5", "alpha/", "29097b5"},
		{"", "alpha/", ""},
	}

	for _, test := range tests {
		if got := taggedRef(test.ref, test.prefix); got != test.want {
			t.Errorf("taggedRef(%q, %q) = %q, want %q", test.ref, test.prefix, got, test.want)
		}
	}
}

func TestParseOptionsVerdictRange(t *testing.T) {
	args, commandLine := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, commandLine })

	os.Args = []string{"worktree", "verdict", "--from", "v0.1.0", "--to=v0.2.0"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.Verdict {
		t.Fatal("Verdict = false, want true")
	}
	if opts.From != "v0.1.0" || opts.To != "v0.2.0" {
		t.Errorf("= {From: %q, To: %q}, want {v0.1.0, v0.2.0}", opts.From, opts.To)
	}
}

func TestShortPackage(t *testing.T) {
	tests := []struct {
		module, pkg string
		want        string
	}{
		// The package at the root of the module is the module root, which is
		// what the leading slash names.
		{module: "example.com/x", pkg: "example.com/x", want: "/"},
		{module: "github.com/titpetric/vuego-cli", pkg: "github.com/titpetric/vuego-cli/config", want: "/config"},
		// A package below the module keeps its whole path under the root, so
		// two packages sharing a name stay apart.
		{module: "example.com/x", pkg: "example.com/x/commands/host", want: "/commands/host"},
		{module: "example.com/x", pkg: "example.com/x/frontend/model", want: "/frontend/model"},
		// A package outside the module, which the model should not hold, is
		// named as it stands rather than mangled.
		{module: "example.com/x", pkg: "example.com/other", want: "example.com/other"},
	}

	for _, test := range tests {
		if got := shortPackage(test.module, test.pkg); got != test.want {
			t.Errorf("shortPackage(%q, %q) = %q, want %q", test.module, test.pkg, got, test.want)
		}
	}
}

func TestRenderVerdictNamesInterfaceMethods(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleInterfaceChange(), false)

	got := out.String()
	// A method needs no treatment of its own: the type column names the
	// interface and the field column carries the signature, so the row reads as
	// store.Store.Put.
	if want := "| Added | /store | type Store interface | Put (key string) error ▲ |"; !strings.Contains(got, want) {
		t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
	}
}

func TestRenderVerdictOmitsTheDataModelWhenNothingMoved(t *testing.T) {
	v := sampleVerdict()
	v.API.Added, v.API.Types = nil, nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	if got := out.String(); strings.Contains(got, "Data model") {
		t.Errorf("renderVerdict() wrote a data model section with nothing in it:\n%s", got)
	}
}

func TestRenderVerdictWritesNoShapeForATypeWithoutFields(t *testing.T) {
	v := sampleVerdict()
	// A func type declares no field, so its row in the API table says all
	// there is to say about it.
	v.API.Added = []apiSymbol{{
		Key: "example.com/x.Option", Package: "example.com/x",
		Name: "Option", Kind: "type", Exported: true, Underlying: "func(*Client)",
	}}
	v.API.Types = nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	// The API table says the type is there; there is no shape to write for it,
	// so the section is left out entirely.
	got := out.String()
	if !strings.Contains(got, "| Added | type Option func(*Client) |  |") {
		t.Errorf("renderVerdict() lost the type from the API table:\n%s", got)
	}
	if strings.Contains(got, "Data model") {
		t.Errorf("renderVerdict() wrote a data model section for a type declaring no field:\n%s", got)
	}
}

func TestRenderVerdictDataModelIsOneTable(t *testing.T) {
	v := sampleVerdict()
	// A second package, so the table has something to keep apart.
	v.API.Types = append(v.API.Types, apiTypeChange{
		Key: "example.com/x/inner.Store", Package: "example.com/x/inner",
		Name: "Store", Underlying: "struct", Breaking: true,
		Fields: []apiFieldChange{
			{Name: "Bucket", Change: fieldAdded, New: &apiField{Name: "Bucket", Type: "string"}},
			{Name: "Region", Change: fieldRemoved, Old: &apiField{Name: "Region", Type: "string"}},
		},
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// One table for every type the release touched, and no heading per type.
	if n := strings.Count(got, "| Change | Package | Type | Field |"); n != 1 {
		t.Errorf("renderVerdict() wrote %d data model tables, want 1:\n%s", n, got)
	}
	if strings.Contains(got, "###") {
		t.Errorf("renderVerdict() wrote a heading per type:\n%s", got)
	}

	// The rows the table holds, in order: added first, taken away last, and
	// within a category by package, then type, then field.
	// The category names only the first row of its group, as it does in the API
	// table above.
	want := []string{
		"| Added | / | type Client struct ▲ | Name string `json:\"name\"` |",
		"|  |  | type Config struct | Timeout int ▲ |",
		"|  | /inner | type Store struct | Bucket string ▲ |",
		"| Changed | / | type Config struct | Addr string `yaml:\"addr\"` -> []string `yaml:\"addr\"` |",
		"| Removed | / | type Config struct | Retries int |",
		"|  | /inner | type Store struct | Region string |",
	}
	at := -1
	for _, row := range want {
		next := strings.Index(got, row)
		if next < 0 {
			t.Fatalf("renderVerdict() output missing %q:\n%s", row, got)
		}
		if next < at {
			t.Errorf("renderVerdict() wrote %q out of order:\n%s", row, got)
		}
		at = next
	}

	// A category opens a group, and restates the package and type under it
	// however far the group above them reached.
	if strings.Contains(got, "| Removed |  |") {
		t.Errorf("renderVerdict() opened a category on an empty package cell:\n%s", got)
	}
	// The added type carries the mark once, and each field added to a type
	// that already existed carries it on the field; fields of the new type
	// do not repeat it.
	if n := strings.Count(got, newTypeMark); n != 3 {
		t.Errorf("renderVerdict() wrote the mark %d times, want 3:\n%s", n, got)
	}
}

func TestFieldText(t *testing.T) {
	tests := []struct {
		title  string
		change apiFieldChange
		want   string
	}{{
		title:  "a field that was added has the shape it arrived with",
		change: apiFieldChange{Name: "Retries", Change: fieldAdded, New: &apiField{Name: "Retries", Type: "int"}},
		want:   "Retries int",
	}, {
		title:  "a field that was removed has the shape it left with",
		change: apiFieldChange{Name: "Retries", Change: fieldRemoved, Old: &apiField{Name: "Retries", Type: "int"}},
		want:   "Retries int",
	}, {
		// The name is written once: a field is matched to the one it was by
		// name, so the name is the one thing that cannot have changed.
		title: "a field that moved has both shapes under one name",
		change: apiFieldChange{
			Name: "Addr", Change: fieldChanged,
			Old: &apiField{Name: "Addr", Type: "string"},
			New: &apiField{Name: "Addr", Type: "[]string"},
		},
		want: "Addr string -> []string",
	}, {
		title: "a tag is part of the shape, since a document decodes through it",
		change: apiFieldChange{
			Name: "Addr", Change: fieldChanged,
			Old: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"addr"`},
			New: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"address"`},
		},
		want: "Addr string `yaml:\"addr\"` -> string `yaml:\"address\"`",
	}, {
		// An interface method carries its name in the signature it is recorded
		// under, so the name is not written in front of it twice.
		title: "an interface method is not named twice",
		change: apiFieldChange{
			Name: "Put", Change: fieldAdded,
			New: &apiField{Name: "Put", Type: "Put (key string) error"},
		},
		want: "Put (key string) error",
	}, {
		// A field typed after itself is not a name written twice, so it keeps
		// both: the parameter list is what tells a method signature apart.
		title: "a field typed after itself keeps its name",
		change: apiFieldChange{
			Name: "Mode", Change: fieldAdded,
			New: &apiField{Name: "Mode", Type: "Mode", Tag: `yaml:"mode"`},
		},
		want: "Mode Mode `yaml:\"mode\"`",
	}}

	for _, test := range tests {
		if got := fieldText(test.change); got != test.want {
			t.Errorf("%s: fieldText() = %q, want %q", test.title, got, test.want)
		}
	}
}

func TestVerdictBreakageNamesTheDataModel(t *testing.T) {
	tests := []struct {
		title string
		types []apiTypeChange
		want  string
	}{{
		title: "a field a struct lost costs a consumer something",
		types: []apiTypeChange{{
			Name: "Config", Underlying: "struct", Breaking: true,
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldRemoved}},
		}},
		want: "Minor release: v1.1.0, because 1 exported field moved since v1.0.0.",
	}, {
		title: "a field a struct gained costs nothing",
		types: []apiTypeChange{{
			Name: "Config", Underlying: "struct",
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldAdded}},
		}},
		want: "Patch release: v1.1.0, no exported symbols were removed since v1.0.0.",
	}, {
		title: "a method an interface gained stops every implementor compiling",
		types: []apiTypeChange{{
			Name: "Store", Underlying: "interface", Breaking: true,
			Fields: []apiFieldChange{{Name: "Put", Change: fieldAdded}},
		}},
		want: "Minor release: v1.1.0, because 1 exported field moved since v1.0.0.",
	}}

	for _, test := range tests {
		v := verdict{Version: "v1.1.0", Since: "v1.0.0", Release: releasePatch}
		v.API.Types = test.types
		for _, change := range test.types {
			v.API.Breaking = v.API.Breaking || change.Breaking
		}
		if v.API.Breaking {
			v.Release = releaseMinor
		}

		if got := v.Summary(); got != test.want {
			t.Errorf("%s: Summary() = %q, want %q", test.title, got, test.want)
		}
	}
}

// commitScanRepo builds a module whose history holds one commit of each kind
// the commit table has a cell for: one that adds an exported func, one that
// only touches the docs, and one that reshapes the func added before it.
func commitScanRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")

	writeTestFile(t, filepath.Join(alpha, "README.md"), "# alpha\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: document the package")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string, times int) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: greet a number of times")

	return alpha
}

func TestReadVerdictCountsTheAPIOfEachCommit(t *testing.T) {
	requireGoFsck(t)

	got, err := readVerdict(commitScanRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if len(got.Commits) != 3 {
		t.Fatalf("readVerdict() Commits = %#v, want the three commits since the tag", got.Commits)
	}

	// The commits are newest first, so the range reads bottom up: Greet is
	// added, the docs are written, and the signature moves.
	want := []string{"+0/~1/-0", "", "+1/~0/-0"}
	for i, commit := range got.Commits {
		diff, ok := got.CommitAPI[commit.Hash]
		if !ok {
			t.Fatalf("readVerdict() scanned no API for %s %q", commit.Hash, commit.Subject)
		}
		if diff.Skipped != "" {
			t.Fatalf("readVerdict() skipped %s: %s", commit.Hash, diff.Skipped)
		}

		counts := ""
		if len(diff.Added)+len(diff.Changed)+len(diff.Removed) > 0 {
			counts = symbolCounts(diff, false)
		}
		if counts != want[i] {
			t.Errorf("readVerdict() %q = %q, want %q", commit.Subject, counts, want[i])
		}
	}
}

func TestRenderVerdictWritesTheAPIColumnOfEachCommit(t *testing.T) {
	requireGoFsck(t)

	v, err := readVerdict(commitScanRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	for _, want := range []string{
		"| Commit | API | Subject |",
		"| +1/~0/-0 | alpha: add Greet |",
		// A commit that moved nothing exported leaves the cell empty rather
		// than writing three zeroes.
		"|  | alpha: document the package |",
		"| +0/~1/-0 | alpha: greet a number of times |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictNamesTheCommitsBehindASymbol(t *testing.T) {
	requireGoFsck(t)

	alpha := commitScanRepo(t)
	v, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	// Greet was added by the first commit of the range and reshaped by the
	// last, so the row naming it names both, in the order they were made.
	added, changed := v.Commits[2].Hash, v.Commits[0].Hash

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	if !strings.Contains(got, "| Change | Exported | Unexported | Commits |") {
		t.Fatalf("renderVerdict() wrote no commits column:\n%s", got)
	}
	if want := "`" + added + "`, `" + changed + "`"; !strings.Contains(got, want) {
		t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
	}
}

func TestRenderVerdictLinksTheCommitsBehindASymbol(t *testing.T) {
	v := sampleVerdict()
	v.CommitAPI = map[string]apiDiff{
		"abc1234": {Added: []apiSymbol{{Key: "example.com/x.Client"}}},
		"def5678": {Removed: []apiSymbol{{Key: "example.com/x.Legacy"}}},
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// A commit is linked in the API table the way the commit table links it.
	for _, want := range []string{
		"[`abc1234`](https://github.com/example/x/commit/abc1234) |",
		"[`def5678`](https://github.com/example/x/commit/def5678) |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

// twoModelsRepo builds a module holding two packages named model, one below
// the root and one below the front end, which is what the package column has
// to tell apart.
func twoModelsRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "model", "model.go"), "package model\n\n// Trace is a recorded trace.\ntype Trace struct{ ID string }\n")
	writeTestFile(t, filepath.Join(alpha, "frontend", "model", "model.go"), "package model\n\n// Page is a rendered page.\ntype Page struct{ Title string }\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: add the two models")

	return alpha
}

func TestRenderVerdictNamesAPackageByItsPathBelowTheModule(t *testing.T) {
	requireGoFsck(t)

	v, err := readVerdict(twoModelsRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// Two packages named model, kept apart by the path they sit at rather than
	// by the name they share.
	for _, want := range []string{"| /model | type Trace struct |  |", "| /frontend/model | type Page struct |  |"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "| model | type") {
		t.Errorf("renderVerdict() named a package by its name alone:\n%s", got)
	}
}

func TestRenderVerdictNamesTheCommitsBehindAField(t *testing.T) {
	v := sampleVerdict()
	v.CommitAPI = map[string]apiDiff{
		// The commit that added Client carries every field it declares, and
		// the one that reshaped Config carries the field it moved.
		"abc1234": {Added: []apiSymbol{{
			Key: "example.com/x.Client", Name: "Client", Kind: "type", Underlying: "struct",
			Fields: []apiField{{Name: "Name", Type: "string"}},
		}}},
		"def5678": {Types: []apiTypeChange{{
			Key:    "example.com/x.Config",
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldChanged}},
		}}},
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	if !strings.Contains(got, "| Change | Package | Type | Field | Commits |") {
		t.Fatalf("renderVerdict() wrote no commits column on the data model:\n%s", got)
	}
	for _, want := range []string{
		"| Name string `json:\"name\"` | [`abc1234`](https://github.com/example/x/commit/abc1234) |",
		"`yaml:\"addr\"` | [`def5678`](https://github.com/example/x/commit/def5678) |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictLeavesTheCommitColumnsOutWhenNothingWasScanned(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)
	got := out.String()

	// A range read as one comparison has no commit to attribute a symbol to,
	// and gets no column of empty cells.
	for _, unwanted := range []string{"| Commits |", "| Commit | API | Subject |"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote %q for a range that was not scanned:\n%s", unwanted, got)
		}
	}
}

func TestCollapseRemovedMethods(t *testing.T) {
	symbols := []apiSymbol{
		{Key: "m.Disk", Package: "m", Name: "Disk", Kind: "type"},
		{Key: "m.Disk.Save", Package: "m", Name: "Disk.Save", Kind: "func"},
		{Key: "m.Disk.Len", Package: "m", Name: "Disk.Len", Kind: "func"},
		{Key: "m.Orphan.Close", Package: "m", Name: "Orphan.Close", Kind: "func"},
		{Key: "m.ValidID", Package: "m", Name: "ValidID", Kind: "func"},
		{Key: "other.Disk.Save", Package: "other", Name: "Disk.Save", Kind: "func"},
	}

	kept := collapseRemovedMethods(symbols)

	want := []string{"Disk", "Orphan.Close", "ValidID", "Disk.Save"}
	if len(kept) != len(want) {
		t.Fatalf("kept %d symbols, want %d: %+v", len(kept), len(want), kept)
	}
	for i, name := range want {
		if kept[i].Name != name {
			t.Fatalf("kept[%d] = %q, want %q", i, kept[i].Name, name)
		}
	}
}

func TestFieldReadsEmbedded(t *testing.T) {
	embedded := apiField{Name: "UnimplementedStorage", Type: "*UnimplementedStorage", Embedded: true}
	if got := fieldReads(embedded); got != "embeds *UnimplementedStorage" {
		t.Errorf("fieldReads(embedded) = %q", got)
	}
	// An embed is read as the type it is declared with, pointer and all: the
	// name go-fsck reaches it by drops the star and says less.
	value := apiField{Name: "Base", Type: "platform.Base", Embedded: true}
	if got := fieldReads(value); got != "embeds platform.Base" {
		t.Errorf("fieldReads(value embed) = %q", got)
	}
	named := apiField{Name: "Addr", Type: "string"}
	if got := fieldReads(named); got != "Addr string" {
		t.Errorf("fieldReads(named) = %q", got)
	}
}
