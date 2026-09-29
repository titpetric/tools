package main

import (
	"strings"
	"testing"
)

// chainString renders a chain as "from..to" per section, so a table test reads
// as the report it describes. The start of history and the working tree have no
// version to name, and are left empty on their side of the range.
func chainString(ranges []versionRange) []string {
	out := make([]string, 0, len(ranges))
	for _, r := range ranges {
		out = append(out, r.From+".."+r.To)
	}
	return out
}

func TestSeriesOpeners(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{{
		name: "one opener per series",
		tags: []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1", "v0.2.0"},
		want: []string{"v0.0.1", "v0.1.0", "v0.2.0"},
	}, {
		name: "tags out of order",
		tags: []string{"v0.2.0", "v0.1.1", "v0.0.2", "v0.1.0", "v0.0.1"},
		want: []string{"v0.0.1", "v0.1.0", "v0.2.0"},
	}, {
		name: "a series that never tagged its zero",
		tags: []string{"v0.1.1", "v0.1.2", "v0.2.3"},
		want: []string{"v0.1.1", "v0.2.3"},
	}, {
		name: "a major bump opens a series of its own",
		tags: []string{"v0.9.0", "v0.9.1", "v1.0.0", "v1.0.1"},
		want: []string{"v0.9.0", "v1.0.0"},
	}, {
		name: "prereleases and other tags are not releases",
		tags: []string{"v0.1.0-rc.1", "v0.1.0", "nightly", "v0.2.0"},
		want: []string{"v0.1.0", "v0.2.0"},
	}, {
		name: "no tags at all",
		tags: nil,
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, opener := range seriesOpeners(test.tags) {
				got = append(got, opener.String())
			}
			if strings.Join(got, " ") != strings.Join(test.want, " ") {
				t.Errorf("seriesOpeners(%v) = %v, want %v", test.tags, got, test.want)
			}
		})
	}
}

func TestReleaseChain(t *testing.T) {
	full := []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1", "v0.2.0", "v0.2.1", "v0.2.2"}

	tests := []struct {
		name    string
		tags    []string
		ahead   bool
		verbose bool
		from    string
		upTo    string
		want    []string
	}{{
		name:  "series bumps, the latest release, and the working tree",
		tags:  full,
		ahead: true,
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "level with the latest tag, so there is nothing pending",
		tags: full,
		want: []string{
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "the latest release opens its own series",
		tags: []string{"v0.1.0", "v0.1.1", "v0.2.0"},
		want: []string{
			"v0.1.0..v0.2.0",
			"..v0.1.0",
		},
	}, {
		name:    "verbose reports every release against the one before it",
		tags:    []string{"v0.0.1", "v0.0.2", "v0.1.0"},
		verbose: true,
		want: []string{
			"v0.0.2..v0.1.0",
			"v0.0.1..v0.0.2",
			"..v0.0.1",
		},
	}, {
		name: "a series whose zero was never tagged",
		tags: []string{"v0.1.1", "v0.1.2", "v0.2.3"},
		want: []string{
			"v0.1.1..v0.2.3",
			"..v0.1.1",
		},
	}, {
		name: "a major bump is a series of its own",
		tags: []string{"v0.9.0", "v1.0.0", "v1.0.1"},
		want: []string{
			"v1.0.0..v1.0.1",
			"v0.9.0..v1.0.0",
			"..v0.9.0",
		},
	}, {
		name: "prereleases are not releases",
		tags: []string{"v0.1.0-rc.1", "v0.1.0", "v0.2.0-rc.1", "v0.2.0"},
		want: []string{
			"v0.1.0..v0.2.0",
			"..v0.1.0",
		},
	}, {
		name: "one release",
		tags: []string{"v0.1.0"},
		want: []string{"..v0.1.0"},
	}, {
		name:  "one release with work on top of it",
		tags:  []string{"v0.1.0"},
		ahead: true,
		want: []string{
			"v0.1.0..",
			"..v0.1.0",
		},
	}, {
		name:  "no releases at all is one range over the whole history",
		tags:  nil,
		ahead: true,
		want:  []string{".."},
	}, {
		name:  "bounded by a release, which is where the chain stops",
		tags:  full,
		ahead: true,
		upTo:  "v0.1.1",
		want: []string{
			"v0.1.0..v0.1.1",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name:  "bounded by the release opening a series",
		tags:  full,
		ahead: true,
		upTo:  "v0.1.0",
		want: []string{
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "bounded above every release",
		tags: full,
		upTo: "v0.0.0",
		want: []string{"..v0.0.0"},
	}, {
		name:  "bounded below by a release, which is where the chain starts",
		tags:  full,
		ahead: true,
		from:  "v0.1.0",
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
		},
	}, {
		name:  "bounded below inside the latest series",
		tags:  full,
		ahead: true,
		from:  "v0.2.0",
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
		},
	}, {
		name: "bounded at both ends",
		tags: full,
		from: "v0.0.1",
		upTo: "v0.2.0",
		want: []string{
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
		},
	}, {
		name: "bounded above every release there is",
		tags: full,
		from: "v9.0.0",
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := chainString(releaseChain(test.tags, test.ahead, test.verbose, test.from, test.upTo))
			if strings.Join(got, " | ") != strings.Join(test.want, " | ") {
				t.Errorf("releaseChain() =\n\t%v\nwant\n\t%v", got, test.want)
			}
		})
	}
}
