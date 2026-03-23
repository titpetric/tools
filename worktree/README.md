# worktree - Show workspace details

This is a tool for people that:

- Use `git` with git tags
- Use `go` modules or go workspaces
- Need insight to their workspaces

The tool provides information and overview of the workspace state.

To install the tool:

```bash
go install github.com/titpetric/tools/worktree@main
```

Run `worktree` or `worktree -v` in your workspace.

Pass `-puml` or `-d2` if you want a PlantUML or D2 diagram of your workspace.

## Information summarized

The tool scans and displays information about:

- Go module versions
- Go module dependencies in workspace
- Latest git version tag
- Git commits since version tag
- Git branch in source tree
- Unpushed git commits
- Local changes to source tree
- README.md title for description

## Functionality

Several flags invoke tool functionality:

- `-u` flag will update stale workspace dependencies, use latest go module versions,
- `-puml` will render a plantuml representation of the workspace,
- `-d2` will render a d2 representation of the workspace.

## Examples

### Basic workspace view

![Worktree status](./examples/worktree.png)

### Verbose workspace view

![Worktree status - verbose](./examples/worktree.png)

### D2 Diagram

![D2 workspace diagram](./examples/workspace-d2.svg)

### PlantUML Diagram

![PlantUML workspace diagram](./examples/workspace.svg).

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