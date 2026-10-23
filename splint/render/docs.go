// Package render writes what the linters found and what they measured.
//
// The reader decides what the package writes. A terminal gets a summary of
// what each linter found and then the findings, in colour. Anything else gets
// a GitHub Actions workflow command per finding, and GitHub turns each one into
// an annotation on the file and the line of a pull request review. A compiler
// shaped line becomes an annotation only where a workflow registers a problem
// matcher for it, and a workflow command needs no matcher.
//
// GitHub shows ten warnings and ten errors per step and fifty annotations per
// job, and drops the rest without saying so, so a gate reads the exit code and
// the statistics and ignores the count of annotations. The command format is
// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands
// and the escaping is what @actions/core does in command.ts.
//
// The measurements are tables: a terminal gets a drawn table, and anything
// else gets markdown.
package render
