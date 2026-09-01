# worktree - Show workspace details

![Worktree status](./examples/worktree.png)

This is a tool for developers that:

- Use `git` workspaces
- Use `go` modules or go workspaces
- Use `gh issues` for issue tracking
- Need insight to their workspaces

The tool provides information and overview of the workspace state.

To install the tool:

```bash
go install github.com/titpetric/tools/worktree@main
```

Run `worktree` anywhere in your source workspace. It uses the nearest current or parent directory containing `go.work`, `go.mod`, or `.git` as the scan root. If no parent contains one of those markers, it recursively scans the current directory. Go modules and Git repositories are both included; when they share a directory they appear as one row. An optional path argument filters the output to projects matching that path:

```bash
worktree .           # show only the module in the current folder
worktree ./tools     # show all modules under the tools folder
worktree /abs/path   # show modules matching an absolute path
```

By default the scan honours `.gitignore` files. An ignored folder is not descended into, so Git repositories and Go modules inside it are skipped; this keeps vendored checkouts and build output out of the listing. Only `.gitignore` files are read, not `.git/info/exclude` or the global excludes file, and a pattern applies even to paths that the repository tracks. Set `enable_gitignore: false` in the configuration to turn this off; see [Configuration](#configuration).

Two directory names are never descended into, whatever the configuration says: `testdata`, whose `go.mod` files are test fixtures, and `.git`. A module below either is not listed, and `-u` and `-go` do not rewrite it.

Two commands print the git commands for tagging a new release of the git repository in the current directory. They read the existing tags, detect the latest semver release, ignoring prereleases and tags that aren't semantic versions, and increment it:

```bash
worktree patch   # v1.2.3 -> git tag v1.2.4
worktree minor   # v1.2.3 -> git tag v1.3.0
```

The output is written to stdout so it can be reviewed and then piped into a shell:

```bash
worktree patch | sh -x
```

The `v` prefix of the latest tag is preserved. If the repository has no release tags yet, the version starts at `v0.0.0`, so `patch` proposes `v0.0.1` and `minor` proposes `v0.1.0`, with a shell comment noting it. A go module nested in a larger repository is released as `<subdir>/vX.Y.Z`, which is how the go tool tells the releases of one module in a repository from another's, so the proposed tag carries that prefix.

## Resolving a release chain

`worktree resolve` walks the selected modules from their outside dependencies inward and works out what it takes to release each of them:

```bash
worktree resolve            # render the plan
worktree resolve --apply    # perform it
worktree resolve ./tools    # only the modules under a path
```

Without `--apply` nothing is run and nothing is written. The plan is not a shell script to pipe anywhere, because the run has to read the state of each repository as it reaches it:

| Path    | Module                 | Release              | Resolution                                                                                                                                                                                                                                         |
|---------|------------------------|----------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| ./alpha | example.com/repo/alpha | v0.1.0 (+2) → v0.2.0 | go get -u ./...<br>go mod tidy<br>git add -- go.mod go.sum<br>git commit -m 'alpha: update go.mod, go.sum' -- go.mod go.sum<br>api: 2 added, 0 changed, 1 removed<br>git tag alpha/v0.2.0<br>git push --tags                                       |
| ./beta  | example.com/repo/beta  | v0.2.0 → v0.2.1      | go get -u ./...<br>go get example.com/repo/alpha@v0.2.0<br>go mod tidy<br>git add -- go.mod go.sum<br>git commit -m 'beta: update go.mod, go.sum' -- go.mod go.sum<br>api: 0 added, 0 changed, 0 removed<br>git tag beta/v0.2.1<br>git push --tags |

The `Release` column reads as the version the module is at, the commits it has taken since, and the version it moves to, coloured green for a patch and amber for a minor. No step changes the working directory: every command is run in the module it belongs to, which is the directory the `Path` column names.

Every go module carrying a release tag is offered the update, even one with no commits of its own and nothing in the workspace to move: its dependencies outside the workspace can still have moved, and `go get -u ./...` is the only way to find out. Such a module is released only if the update rewrites `go.mod` or `go.sum`, which the plan says outright and `--apply` finds out for itself. Modules with nothing to do at all, which is anything untagged and any repository that is not a go module, are left out; `--all` asks for them, as it does everywhere else in the tool.

Each `go get` is tidied straight after it rather than once at the end, so the requirement it replaced is out of `go.mod` and its checksums are out of `go.sum` before the next one is asked for.

### Aligning the go version

Every selected module is brought up to the highest `go` directive among them, so a workspace resolved together stays on one language version rather than drifting a release apart:

```
go mod edit -go=1.27
go get -u ./...
go mod tidy
```

A `toolchain` directive older than the new version would leave `go.mod` invalid, so it is dropped first with `go mod edit -toolchain=none`; `go get` and `go mod tidy` add a newer one back when they need it.

Moving to another go release series costs a **minor**, whether the directive was raised by hand since the tag or is being raised by this run: the module stops building for anyone on the older toolchain, which is as breaking as a symbol going away. A point release of the same series, `1.27` to `1.27.1`, changes nothing for a consumer and stays a patch.

The order is a topological sort of the dependencies among the selected modules, so a module is never released before something it requires. A dependency outside the selection keeps the tag it already carries and is never bumped. Modules left in a dependency cycle are reported and skipped.

Each module is pinned to the version its dependency ends up at, which is the tag this run creates for it rather than the tag that exists now: `beta` above requires `alpha@v0.2.0`, a tag the step before it creates. That is the reason for the ordering, and the reason a run that stops does not carry on: every module after the stop would pin a version that never gets created.

`--apply` runs each step and marks it `ok` or `failed`, and a failure stops the run with a non-zero exit status. It performs the release as well, running the same `git tag` and `git push --tags` that `worktree patch` and `worktree minor` print. A repository with no `go.mod` is resolved on its git state alone, without the go steps.

### Choosing between a patch and a minor

The release is a minor when it costs a consumer something, and a patch otherwise. The API side of that is decided by comparing the exported symbols of the working tree against the latest tag with [`go-fsck diff`](https://github.com/titpetric/exp/tree/main/cmd/go-fsck):

- a removed exported symbol, or one whose signature changed, is breaking, and earns a minor,
- so does an exported field a type loses or reshapes, and a method an interface gains, see [Data model changes](#data-model-changes),
- so does moving to another go release series, see [Aligning the go version](#aligning-the-go-version),
- added symbols, a dependency update that only rewrites `go.mod` and `go.sum`, and everything else earn a patch.

The tagged revision is unpacked into a temporary directory rather than checked out, so the working tree is left alone. Parameter names are not part of a signature, so renaming one is not a change; changing its type is. Test packages, commands and internal packages are left out, since none of them are API another module can depend on. `-v` lists the symbols behind the count.

Anything that stops the comparison from running - no `go-fsck` installed, a `go-fsck` without the `diff` command, or a revision that cannot be read - is reported in place and read as non breaking, so a missing tool costs a patch release rather than stopping the run. Note that `go-fsck` skips files carrying build constraints, so symbols behind a `//go:build` line are invisible to both sides of the comparison.

### Stopping on a dirty repository

`go.mod` and `go.sum` are committed by resolve itself. They are staged first, since a `go.sum` the update creates is not yet tracked and a pathspec would not match it, and then committed by pathspec, which leaves every other change in the working tree out of the commit. Only the files that exist are named. Anything else left in the working tree afterwards stops the run:

| Path       | Module         | Release              | Resolution                                                                                                 |
|------------|----------------|----------------------|------------------------------------------------------------------------------------------------------------|
| ./worktree | tools/worktree | v0.3.1 (+1) → v0.3.2 | ...<br>api: 4 added, 0 changed, 0 removed<br>M main.go<br>?? resolve.go<br>Stopped: working tree is dirty. |

Releasing a module in that state would tag work nobody reviewed. Under `--apply` the check reads the working tree at the moment it gets there; without it, the state after the commit is predicted from the working tree as it stands now, with `go.mod` and `go.sum` left out of the reckoning. Every module after the stop is reported as not reached, since each would pin a version that never gets created.

## Reporting a release

`worktree verdict` writes the report of a single repository: which version it is at or moving to, why, the commits behind it, what became of its exported API, and how the packages of the working tree are split between what they export and what they keep.

```bash
worktree verdict                            # the repository of the current directory
worktree verdict ./tools/lessgo             # a repository elsewhere
worktree verdict > NOTES.md                 # markdown, for a release note
worktree verdict --from v0.4.4 --to v0.5.5  # a range of your choosing
worktree verdict --all                      # every release the repository has made
worktree verdict --no-cache                 # read every commit again, cache nothing
```

It draws a table on a terminal and writes markdown when its output goes anywhere else, so the redirected form pastes into GitHub release notes with the commit hashes already linked.

On a terminal each section is a heading with its table directly beneath it, and a blank line after the table. The heading is written in bold yellow, a colour no table is drawn in, so it is not read as another column heading. A report of several releases separates one from the next by two blank lines, so a release stands further from the release below it than its own tables stand from each other.

What it reports depends on where the repository stands:

- **behind its latest tag**, it proposes the next release, comparing the tag against the working tree,
- **level with its latest tag**, it describes the release that was made, comparing the tag before it against it. This is the release-note case: tag first, then ask what went into it,
- **with no release tag**, it proposes a first release, measured from the start of history.

A release with nothing before it is compared against a module holding no packages at all, so everything it exports is listed as added and nothing is listed as removed. That is the same report every other release gets, taken from an empty starting point rather than skipped for want of a tag to compare against.

`--from` and `--to` name the two revisions outright, for a report over a range the repository is no longer standing on. A bare version is resolved to the tag the repository carries for it, including the `<subdir>/` prefix of a nested module, and anything else is passed to git as it stands, so a commit or a branch can be named just as well. Given only `--to`, the range starts at the release below it; given only `--from`, it ends at the working tree. A `--to` naming a release is reported as that release, and anything else as a proposal, since there is no tag to call it by.

The go directive is compared along with the API, so a release that moves to another go release series is a minor and says so, the same rule [resolve](#aligning-the-go-version) applies.

### Every release at once

`worktree verdict --all` reports the whole release chain rather than one range: every tag is read, and one report is written per release bump, newest first. `--from=all`, `--from=0` and `--from=HEAD` all say the same thing.

The chain is drawn between `major.minor` series, not between every tag, so a repository with forty patch releases does not produce forty reports. Given the tags `v0.0.1 v0.0.2 v0.1.0 v0.1.1 v0.2.0 v0.2.1 v0.2.2` and commits on top of the last one, the report holds:

| Range            | What it covers                                                 |
|------------------|----------------------------------------------------------------|
| `since v0.2.2`   | the working tree, and the release it has earned                |
| `v0.2.0..v0.2.2` | the latest release, against the release that opened its series |
| `v0.1.0..v0.2.0` | the move from the 0.1 series to the 0.2 series                 |
| `v0.0.1..v0.1.0` | the move from the 0.0 series to the 0.1 series                 |
| `v0.0.1`         | the first release, measured from the first commit              |

A series is compared against the release that opened the one below it, so `v0.2.0` is measured from `v0.1.0` and not from `v0.1.1`. The opener is the lowest release of the series, which is its `.0` unless that was never tagged. The patch releases in between get no report of their own; their commits are still there, inside the section spanning them. The latest release earns one anyway when it is not the opener of its own series, since nothing else covers it.

Add `-v` to report every tag instead, each against the one before it, which is the same history at the granularity it was tagged at.

`--from` and `--to` bound the chain at either end, naming the releases it runs between. `worktree verdict --all --to v0.5.0` stops at `v0.5.0` and reports nothing after it, including the working tree, which is past its end. `worktree verdict --all --from v0.3.0` starts there rather than at the first commit. A range holding no release at all is an error rather than an empty report.

Each revision is unpacked and modelled once for the whole run, so a release ending one section and opening the next is read once rather than twice.

```
# github.com/go-bridget/mig @ v0.6.0

Released v0.6.0: 33 exported symbols were removed and 2 signatures changed since v0.5.5.

## Commits v0.5.5..v0.6.0

| Commit | External | Internal | Subject |
| --- | --- | --- | --- |
| [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) | +2/~0/-33 | +4/~0/-9 | Restructure mig to drop cmd/ |
| [`ff1fdc2`](https://github.com/go-bridget/mig/commit/ff1fdc2) | +0/~2/-0 |  | migrate: rework api |

## API v0.5.5..v0.6.0

| Change | Package | Exported | Unexported | Commits |
| --- | --- | --- | --- | --- |
| Added | /migrate | type Manager |  | [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) |
|  |  | func NewManager (db *sqlx.DB, migrations fs.FS, project string) (*Manager, error) |  | [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) |
|  |  |  | type loader | [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) |
| Removed | /cmd/mig/gen | type Column |  | [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) |
|  | /migrate | func Load (fsys fs.FS, project string) error |  | [`29097b5`](https://github.com/go-bridget/mig/commit/29097b5) |
```

The category names the first row of its group and the rows below it leave the column empty; the table draws no rule between rows, so a group reads as one block. A module holding more than one package gains the `Package` column, without which `const Name` three times over says nothing. The symbols of a package are gathered together within their category and only the first of them names it, the same way the data model table reads. Everywhere counts and categories are listed, the order is what the release added, what it reshaped, what it took away.

The `Package` column is the import path below the module, written from the module root down: `/model` is the model package of this module and `/frontend/model` is the other one, where a bare `model` twice over says nothing about which is which. The package at the root of the module is `/`.

A symbol is written in the `Exported` column when a consumer can reach it and in `Unexported` when it cannot, so a reader after what the release costs reads one column and a reader after what the refactor moved reads the other. A release that only moved private code fills one column and leaves the other empty.

The `External` and `Internal` columns of the commit table count what one commit did on its own, in the order the rest of the report reads: what it added, what it reshaped, what it took away. External is what a consumer can see and Internal what it cannot, which is what tells a release from a refactor. Each commit is compared against the one under it, so a range of twenty commits is twenty comparisons and the commit behind a removal can be picked out of them. A commit that moved neither half leaves the cells empty rather than writing three zeroes, and a commit that touched no file of the module is not listed at all.

The `Commits` column of the API table names the commits behind each symbol, oldest first, linked the way the commit table links them. A symbol added by one commit and reshaped by another lists both. The data model table carries the same column, for the commits behind each field.

### Reading history once

Reading a range commit by commit means modelling every commit in it, and on a repository with a few hundred of them that is minutes rather than seconds. A commit cannot change, and neither can the model of one, so the models are kept on disk and every later run reads them back:

```
~/.cache/worktree/verdict-<key>.json
```

The directory is the one `os.UserCacheDir` names, which is `$XDG_CACHE_HOME/worktree` or `~/.cache/worktree` on Linux and `~/Library/Caches/worktree` on macOS. The key is the module, the commit the revision resolves to, the go-fsck binary that read it, and the layout of the entry. A tag is resolved to its commit before it is looked up, so moving a tag to another commit reads another entry rather than the one it used to name. Installing another go-fsck does the same. The working tree is never kept: it is the one revision that can change under a run.

`--no-cache` reads every commit again and writes nothing, which is the flag to reach for when go-fsck itself is what changed. A cache directory that cannot be made, or an entry that cannot be written, is a slower run and not a failed one.

### Data model changes

The exported fields of a type are a promise to a consumer as much as a func signature is, so the report covers them too, under a **Data model** section. It is one table for every type the release touched, built the way the API table above it is built: `Change | Package | Type | Field | Commits`, grouped by what the release did, with the cells that repeat the row above them left empty.

```
## Data model v0.0.1..v0.1.0

| Change | Package | Type | Field | Commits |
| --- | --- | --- | --- | --- |
| Added | /commands/docs | DocMeta ▲ | Layout string `yaml:"layout"` | [`4b1c0aa`](https://example.com/commit/4b1c0aa) |
|  |  |  | Title string `yaml:"title"` | [`4b1c0aa`](https://example.com/commit/4b1c0aa) |
|  |  | Tab ▲ | IsCode bool | [`9d70e31`](https://example.com/commit/9d70e31) |
|  | /tour | Lesson | FileOptions map[string][]string | [`0f21c8e`](https://example.com/commit/0f21c8e) |
|  |  | Module | FS fs.FS | [`0f21c8e`](https://example.com/commit/0f21c8e) |
| Changed | /config | Config | Addr string `yaml:"addr"` -> []string `yaml:"addr"` | [`c4a55d2`](https://example.com/commit/c4a55d2) |
| Removed | /tour | Chapter | Slug string | [`0f21c8e`](https://example.com/commit/0f21c8e) |
```

The `▲` marks a type the release introduces, the same mark the [dependency matrix](#information-summarized) uses for something that is there now. Its rows are the shape it declares, so a reader sees what it is rather than only that it exists; unexported fields are left out, since nothing outside the module can reach them. A type without the mark was already there and is written as the fields that moved on it, rather than as the whole declaration again.

The category opens a group and the package and type are restated under it, however far the group above them reached. Within a group the rows read by package, then type, then field.

The `Field` column carries the name and the shape, since the name has no column of its own. A field that moved is written once under its name with the shape on either side of the move, the way the API table writes a changed signature. An interface needs no treatment of its own: its methods sit in the same column, so the type and the field together read as `store.Store.Put`.

What a data model change costs depends on the shape:

| Shape     | field added  | field reshaped | field removed |
|-----------|--------------|----------------|---------------|
| struct    | not breaking | breaking       | breaking      |
| interface | breaking     | breaking       | breaking      |

Adding a method to an interface stops every implementor compiling, where adding a field to a struct costs a consumer nothing. A struct tag is compared along with the field type and counts as a reshape: it is what a document decodes through, so renaming a `json` or `yaml` key breaks every document already written even though the code reading it still compiles. An embedded field is reached by the last identifier of the type it embeds, and counts because it promotes that type's method set.

A breaking data model change earns a minor the same way a removed symbol does, and the verdict says which: `Minor release: v0.2.0, because 1 exported field moved since v0.1.0.`

This needs a [go-fsck](https://github.com/titpetric/exp/tree/main/cmd/go-fsck) that reports the field comparison. An older one reports only the symbols, and the section is left out.

### Visibility

The report ends on what each package of the working tree declares, split into the half a consumer can reach and the half it cannot. It answers a different question from the tables above it: not what the release moved, but where the module keeps its weight as it stands.

```
## Visibility, the working tree

| Package | Types | Funcs | Ratio |
| --- | --- | --- | --- |
| ./ | 14 / 0 | 30 / 6 | 15.1% |
| ./frontend | 0 / 4 | 14 / 20 | 80.6% |
| ./model | 23 / 2 | 70 / 32 | 21.4% |
```

`Types` and `Funcs` read exported over internal, counted by the case of the declared name: a method counts as a func, and `(*Tracer).serveHTTP` is internal the way a free function is. `Ratio` is the code inside internal func bodies over the code of the package, blank lines and comments left out of both. Test files are left out, and so is the `internal` tree: Go scopes it to the module already, whatever it exports.

The counts are reported and not judged. There is no share of internal code a package ought to carry: a parser is mostly private and a data model mostly not, and both are as they should be. What the table is for is reading one package against another, and against what the same package was a release ago.

This needs [gofsck](https://github.com/titpetric/tools/tree/main/gofsck) on the path. Without it the section is left out, the way an unreadable API leaves out the tables above.

### Counts alone

`--stats` collapses the analysis to one table, a row per release, which is the shape to read a long chain in:

```bash
worktree verdict --all --stats
```

```
# github.com/titpetric/vuego-cli

| Version | Since | Commits | Symbols + | Symbols ~ | Symbols - | Fields + | Fields ~ | Fields - |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| v0.3.0 | v0.2.0 | 11 | 1 | 0 | 0 | 0 | 0 | 0 |
| v0.2.0 | v0.1.0 | 5 | 34 | 0 | 0 | 0 | 0 | 0 |
| v0.1.0 | v0.0.1 | 87 | 26 | 2 | 2 | 2 | 0 | 1 |
| v0.0.1 |  | 9 | 43 | 0 | 0 | 0 | 0 | 0 |
```

The module is named once, without a version, since the table holds them. It works on a single verdict too, where it is a table of one row. On a terminal a zero is greyed, so the releases that did something stand out from the ones that did not.

Several flags invoke tool functionality:

- `-v`, or `--verbose`, gives a detailed verbose view with extra data, and names every untracked file instead of the folder standing in for it, as `--all` also does; with `-u`, the update status also lists each `go get` and `go mod tidy` command that ran and marks successful commands with a green check,
- `-u` updates the dependencies of each selected Go module that are known to be stale, meaning the workspace modules it requires at a version below their latest tag, with `go get <module>@<tag>`, and then runs `go mod tidy`. Dependencies outside the workspace and workspace modules already at their latest tag are left alone; a module with nothing stale is reported as `Already up to date.` without running the go tool. It displays each module's path, module name, and the resulting `go.mod` changes (`dep v1.0.0 → v1.1.0`, `+ dep`, `- dep`, or `Already up to date.`). Results print line by line as each module finishes, so progress is visible while the remaining modules are still updating; the path and module name of the module being worked on appear before its results. Version changes to an existing requirement are orange, new requirements green, dropped ones grey, and failing commands are reported in red. Use `worktree -u ./...` to update every Go module under the workspace root,
- `-U` updates every dependency of each selected Go module with `go get -u ./...`, including ones outside the workspace, before applying the workspace tag updates and `go mod tidy` that `-u` performs. It implies `-u`,
- `--go=<version>` sets the `go` directive of every `go.mod` and `go.work` in the workspace to that version and then performs the same update as `-u`. The version is given as `1.27`, `1.27.1` or `go1.27`. A `toolchain` directive older than the new version is dropped, since it would leave the file invalid; `go get` and `go mod tidy` add a newer one back when they need it. Changed `go.work` files are reported before the update table, each module's go directive change (`go 1.25 → 1.27`) appears in its update status. A module whose `go.mod` already declares the version is reported as `Already up to date.` and skipped without running the go tool, so a repeated run over an updated workspace returns immediately. Combine it with `-u` to update the stale dependencies of every module regardless of its go directive,
- `--pull` pulls new changes for every Git repository in the workspace and displays each repository's path, first remote, branch, and `git pull` output as a table,
- `-t` outputs a dependency matrix, with a green `▲` for current and yellow `▲*` for outdated dependencies. Project names show dark-grey `(+N)` for commits ahead and a dark-orange `*` for local Git changes; empty rows and columns are omitted, except that projects with local changes are always shown. A footer summarizes these workspace states,
- `-puml` will render a plantuml representation of the workspace,
- `-d2` will render a d2 representation of the workspace,
- `--apply` makes `worktree resolve` perform the plan it would otherwise only render; see [Resolving a release chain](#resolving-a-release-chain).

Table output uses the rounded, colored terminal format when stdout is an ANSI terminal and falls back to Markdown when redirected or piped.

The `Git State` column lists the untracked paths of a module. A folder holding nothing tracked stands in for everything below it, named in orange with what it holds, `demos/common/ +17 dirs, +91 files, +7921 SLOC`, so a new subtree costs one line rather than one per file. A new file in a folder that is otherwise tracked is still named, with the lines it adds. `-v` and `--all` name every file instead.

The `Go` column holds each module's go directive. The versions are compared as semantic versions, where a missing patch reads as `.0` and a release candidate such as `1.27rc1` sorts below `1.27`. Every module below the highest version the workspace declares is colored orange, the rest teal. Module import paths lose their `github.com/` prefix, so the module column stays narrow.

## Configuration

Command line flags select what to display and what to update. How the workspace is scanned is configured instead, in `~/.config/worktree.yml`. Run `worktree config` to edit it in a form, printed inline in the same frame the tables use:

```bash
worktree config
```

![Worktree config form](./examples/worktree-config.png)

The form shows every setting at once, each on its own row with its value and a short description of what it does; the file it writes is captioned in the bottom border. Nothing is hidden behind a dialog.

Arrow keys move between rows. A flag is toggled where it stands with `←`, `→` or `Space`. A list setting is typed into where it stands, its entries separated by commas. `Enter` on a setting changes nothing and moves the focus to the `Save` button below the settings, where `Enter` writes the file; `Discard` beside it leaves the file alone. Saving a form with nothing changed writes nothing. `F10` saves from any row, `Esc` leaves, or moves to `Discard` first when there are unsaved edits.

The settings are:

| Key                     | Default                     | Meaning                                                                                                                                                  |
|-------------------------|-----------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| `scan.enable_gitignore` | `true`                      | Honour `.gitignore` files while walking.                                                                                                                 |
| `scan.enable_git_repos` | `true`                      | List Git repositories that are not also Go modules.                                                                                                      |
| `scan.ignore_paths`     | empty                       | Directory names never descended into, whether or not a `.gitignore` mentions them. Matched against the directory name alone, at any depth.               |
| `scan.root_markers`     | `go.work`, `go.mod`, `.git` | Files marking the workspace root. The nearest parent directory holding one of them becomes the scan root; with no markers the current directory is used. |

Turn `enable_gitignore` off when a repository consolidates further Git checkouts below it and gitignores those folders to keep them out of its own index. With the setting on, those checkouts are never descended into, so they do not appear at all:

```yaml
scan:
  enable_gitignore: false
```

`ignore_paths` is the way to keep such a listing clean. It is empty by default because it also overrides a negation that re-includes the name, such as `!vendor`:

```yaml
scan:
  enable_gitignore: false
  ignore_paths:
    - node_modules
    - vendor
```

The file is the complete configuration. The built-in defaults are not applied underneath it, so a setting the file does not name reads as off, and every setting is named so that off is what a missing key means. `worktree config` always writes every key back, so editing through the setup screen cannot drop one. Where a flag and a setting ever cover the same behaviour, the flag given on the command line wins.

When no file exists the built-in defaults apply, which are the behaviour the tool had before it was configurable. A file that cannot be parsed is reported rather than ignored; `worktree config` still opens on it, starting from the defaults, so it can be fixed.

You can create a symlink to `git-st`.

```bash
cd /usr/local/bin
ln -s /usr/local/bin/worktree git-st
```

Creating the symlink enables running `git st` and `git st -v`.

## Information summarized

The tool scans and displays information about:

- Go module name
- Go version from the go.mod go directive
- Go module versions in use (for updates)
- Go module dependencies in workspace
- README.md title is read for the description
- Latest git version tag
- Git commits since version tag
- Git branch in source tree
- Unpushed git commits
- Local changes to source tree
- Untracked changes to source tree, collapsed to a folder
- GitHub issues (gh issue list)

It's focused on summarizing of Go workspaces, or git checkouts of standalone Go modules. Git support may be extended to better account for custom remotes and checkouts that aren't a go module source tree.

## Examples

### Summary workspace view

The following screenshots show standard output, workspace filtering and verbose output for the complete workspace.

![Worktree dependency summary](./examples/worktree-matrix.png)

![Worktree status summary](./examples/worktree.png)

![Worktree for Platform](./examples/worktree-platform.png)

![Worktree for LessGo](./examples/worktree-lessgo.png)

### D2 Diagram

![D2 workspace diagram](./examples/workspace-d2.svg)

### PlantUML Diagram

![PlantUML workspace diagram](./examples/workspace.svg)

## Why?

Using a go workspace is a relatively smooth experience, but most software still gets built and delivered outside a workspace.

This process requires updating the go.mod dependencies as a new version gets tagged. For each module in a workspace I'm interested in:

- using the latest release across the workspace in go.mod
- seeing any local changes not yet commited or pushed
- updating dependencies in the correct order

## Alternatives considered

For years now, I've been using `git st`, to get a recursive view of a git source tree. I maintain a bash version of it in my dotfiles, as well as had a php version eons ago. Let's consider this something like a v3 for the approach.

Git source trees don't give enough dependency information, so I wanted something that reads in go.mod go.work files and provides relevant information to you.
