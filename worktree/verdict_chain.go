package main

import (
	"fmt"
	"io"
)

// readVerdicts reports on every release the module in dir has made, one verdict
// per section of its release chain, newest first.
//
// Every revision is read through one model cache, so a release that ends one
// section and starts the next is unpacked and modelled once rather than twice.
// The cached flag is whether that cache is backed by the one on disk, which
// carries the commits of a run into the next one, and --no-cache turns off.
func readVerdicts(dir string, verbose bool, from, upTo string, cached bool) ([]verdict, error) {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return nil, err
	}

	ahead := false
	if latest, found := LatestRelease(tags); found {
		ahead = commitsSinceTag(dir, prefix+latest.String()) > 0
	}

	chain := releaseChain(tags, ahead, verbose, from, upTo)
	if len(chain) == 0 {
		return nil, fmt.Errorf("no release falls in the range asked for")
	}

	models, err := newAPIModels(cached)
	if err != nil {
		return nil, err
	}
	defer func() { _ = models.Close() }()

	base := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	var verdicts []verdict
	for _, r := range chain {
		v, err := base.report(dir, tags, prefix, r.From, r.To, models)
		if err != nil {
			return nil, err
		}
		verdicts = append(verdicts, v)
	}
	return verdicts, nil
}

// renderVerdicts writes every section of a release chain, newest first, each
// the report a single verdict writes, separated by a blank line. On a terminal
// that lands under the blank line the report before it ends on, so one release
// stands further from the next than the tables inside it stand from each other.
func renderVerdicts(w io.Writer, verdicts []verdict, styled bool) {
	for i, v := range verdicts {
		if i > 0 {
			fmt.Fprintln(w)
		}
		renderVerdict(w, v, styled)
	}
}
