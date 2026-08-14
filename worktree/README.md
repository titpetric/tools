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

Run `worktree` anywhere in your source workspace. It uses the nearest current
or parent directory containing `go.work`, `go.mod`, or `.git` as the scan root.
If no parent contains one of those markers, it recursively scans the current
directory. Go modules and Git repositories are both included; when they share
a directory they appear as one row. An optional path argument filters the
output to projects matching that path:

```bash
worktree .           # show only the module in the current folder
worktree ./tools     # show all modules under the tools folder
worktree /abs/path   # show modules matching an absolute path
```

Several flags invoke tool functionality:

- `-v` gives a detailed verbose view with extra data; with `-u`, it also prints
  each `go get` and `go mod tidy` command and marks successful commands with a green check,
- `-u` updates every dependency in each selected Go module with `go get -u ./...`,
  updates workspace dependencies to their latest tags, and runs `go mod tidy`.
  Use `worktree -u ./...` to update every Go module under the workspace root,
- `--pull` pulls new changes for every Git repository in the workspace,
- `-t` outputs a dependency matrix, with a green `▲` for current and
  yellow `▲*` for outdated dependencies. Project names show dark-grey `(+N)`
  for commits ahead and a dark-orange `*` for local Git changes; empty rows and
  columns are omitted, except that projects with local changes are always shown.
  A footer summarizes these workspace states,
- `-puml` will render a plantuml representation of the workspace,
- `-d2` will render a d2 representation of the workspace.

You can create a symlink to `git-st`.

```bash
cd /usr/local/bin
ln -s /usr/local/bin/worktree git-st
```

Creating the symlink enables running `git st` and `git st -v`.

## Information summarized

The tool scans and displays information about:

- Go module name
- Go module versions in use (for updates)
- Go module dependencies in workspace
- README.md title is read for the description
- Latest git version tag
- Git commits since version tag
- Git branch in source tree
- Unpushed git commits
- Local changes to source tree
- Untracked changes to source tree
- GitHub issues (gh issue list)

It's focused on summarizing of Go workspaces, or git checkouts of standalone
Go modules. Git support may be extended to better account for custom remotes
and checkouts that aren't a go module source tree.

## Examples

### Summary workspace view

The following screenshots show standard output, workspace filtering and
verbose output for the complete workspace.

![Worktree dependency summary](./examples/worktree-matrix.png)

![Worktree status summary](./examples/worktree.png)

![Worktree for Platform](./examples/worktree-platform.png)

![Worktree for LessGo](./examples/worktree-lessgo.png)

### D2 Diagram

![D2 workspace diagram](./examples/workspace-d2.svg)

### PlantUML Diagram

![PlantUML workspace diagram](./examples/workspace.svg)

## Why?

Using a go workspace is a relatively smooth experience, but most
software still gets built and delivered outside a workspace.

This process requires updating the go.mod dependencies as a new version
gets tagged. For each module in a workspace I'm interested in:

- using the latest release across the workspace in go.mod
- seeing any local changes not yet commited or pushed
- updating dependencies in the correct order

## Alternatives considered

For years now, I've been using `git st`, to get a recursive view of a
git source tree. I maintain a bash version of it in my dotfiles, as well
as had a php version eons ago. Let's consider this something like a v3
for the approach.

Git source trees don't give enough dependency information, so I wanted
something that reads in go.mod go.work files and provides relevant
information to you.
