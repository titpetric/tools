# semver - A semver tag filter for git repositories

Semver reads git tags (from stdin or the local repository) and outputs
the latest patch version for each minor release as JSON.

## Installation

```bash
go install github.com/titpetric/tools/semver@latest
```

## Usage

Run in a git repository to list the latest tags:

```bash
semver
```

Or pipe `git ls-remote` output:

```bash
git ls-remote --tags https://github.com/owner/repo | semver
```

## How it works

1. Parses `refs/tags/` lines from stdin or runs `git tag -l` locally
2. Filters for valid semver tags (skipping pre-releases and annotated refs)
3. Groups tags by major and minor version
4. Keeps only the latest patch for each minor release
5. Retains the last two major versions
6. Outputs a JSON array of matching tags with commit, name, and version fields

## Output

```json
[
  {
    "Commit": "abc123",
    "Name": "1.2.3",
    "Ref": "v1.2.3",
    "Major": 1,
    "Minor": 2,
    "Patch": 3
  }
]
```

## Examples

Filter tags from a remote repository:

```bash
git ls-remote --tags https://github.com/golang/go | semver | jq .
```

List local tags with semver filtering:

```bash
semver | jq '.[].Name'
```
