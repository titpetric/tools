package main

import (
	"reflect"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		tag  string
		want Version
		ok   bool
	}{
		{"v1.2.3", Version{Prefix: "v", Major: 1, Minor: 2, Patch: 3}, true},
		{"1.2.3", Version{Major: 1, Minor: 2, Patch: 3}, true},
		{" v0.10.0 ", Version{Prefix: "v", Minor: 10}, true},
		{"v1.2.3-rc.1", Version{Prefix: "v", Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1"}, true},
		{"v1.2.3+build-7", Version{Prefix: "v", Major: 1, Minor: 2, Patch: 3, Build: "build-7"}, true},
		{"v1.2", Version{}, false},
		{"v1.2.3.4", Version{}, false},
		{"v1.02.3", Version{}, false},
		{"latest", Version{}, false},
		{"v1.2.x", Version{}, false},
		{"", Version{}, false},
	}

	for _, test := range tests {
		got, ok := ParseVersion(test.tag)
		if ok != test.ok {
			t.Errorf("ParseVersion(%q) ok = %v, want %v", test.tag, ok, test.ok)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("ParseVersion(%q) = %#v, want %#v", test.tag, got, test.want)
		}
	}
}

func TestParseGoDirective(t *testing.T) {
	tests := []struct {
		directive string
		want      Version
		ok        bool
	}{
		{"1.27", Version{Major: 1, Minor: 27}, true},
		{"1.27.1", Version{Major: 1, Minor: 27, Patch: 1}, true},
		{"go1.27", Version{Major: 1, Minor: 27}, true},
		{" 1.9 ", Version{Major: 1, Minor: 9}, true},
		{"1.21rc1", Version{Major: 1, Minor: 21, Prerelease: "rc1"}, true},
		{"1.27.1beta2", Version{Major: 1, Minor: 27, Patch: 1, Prerelease: "beta2"}, true},
		{"1", Version{}, false},
		{"", Version{}, false},
	}

	for _, test := range tests {
		got, ok := ParseGoDirective(test.directive)
		if ok != test.ok {
			t.Errorf("ParseGoDirective(%q) ok = %v, want %v", test.directive, ok, test.ok)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("ParseGoDirective(%q) = %#v, want %#v", test.directive, got, test.want)
		}
	}
}

func TestGoDirectiveOrder(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.27", "1.27.0", 0},
		{"1.9", "1.27", -1},
		{"1.27.1", "1.27", 1},
		{"1.27rc1", "1.27", -1},
		{"1.27rc1", "1.26", 1},
	}

	for _, test := range tests {
		a, aOK := ParseGoDirective(test.a)
		b, bOK := ParseGoDirective(test.b)
		if !aOK || !bOK {
			t.Fatalf("ParseGoDirective(%q, %q) failed", test.a, test.b)
		}
		if got := Compare(a, b); got != test.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	tests := []string{"v1.2.3", "1.2.3", "v0.0.1", "v1.2.3-rc.1", "v1.2.3+meta"}
	for _, tag := range tests {
		v, ok := ParseVersion(tag)
		if !ok {
			t.Fatalf("ParseVersion(%q) failed", tag)
		}
		if got := v.String(); got != tag {
			t.Errorf("ParseVersion(%q).String() = %q", tag, got)
		}
	}
}

func TestSortVersions(t *testing.T) {
	versions := ParseVersions([]string{
		"v1.10.0", "v1.2.0", "v0.9.9", "v2.0.0", "v1.2.0-rc.2", "v1.2.0-rc.1", "v1.2.0-alpha", "not-a-tag",
	})
	SortVersions(versions)

	var got []string
	for _, v := range versions {
		got = append(got, v.String())
	}
	want := []string{"v0.9.9", "v1.2.0-alpha", "v1.2.0-rc.1", "v1.2.0-rc.2", "v1.2.0", "v1.10.0", "v2.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SortVersions() = %v, want %v", got, want)
	}
}

func TestLatestRelease(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
		ok   bool
	}{
		{"picks highest", []string{"v1.2.0", "v1.10.0", "v1.9.3"}, "v1.10.0", true},
		{"skips prereleases", []string{"v1.2.0", "v1.3.0-rc.1"}, "v1.2.0", true},
		{"skips other tags", []string{"latest", "release-1", "v0.1.0"}, "v0.1.0", true},
		{"keeps prefix style", []string{"1.2.3", "1.2.4"}, "1.2.4", true},
		{"no tags", nil, "", false},
		{"no releases", []string{"v1.0.0-rc.1", "nightly"}, "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := LatestRelease(test.tags)
			if ok != test.ok {
				t.Fatalf("LatestRelease(%v) ok = %v, want %v", test.tags, ok, test.ok)
			}
			if ok && got.String() != test.want {
				t.Errorf("LatestRelease(%v) = %q, want %q", test.tags, got, test.want)
			}
		})
	}
}

func TestBump(t *testing.T) {
	v, ok := ParseVersion("v1.2.3-rc.1+meta")
	if !ok {
		t.Fatal("ParseVersion() failed")
	}
	if got := v.BumpPatch().String(); got != "v1.2.4" {
		t.Errorf("BumpPatch() = %q, want v1.2.4", got)
	}
	if got := v.BumpMinor().String(); got != "v1.3.0" {
		t.Errorf("BumpMinor() = %q, want v1.3.0", got)
	}
}

func TestReleases(t *testing.T) {
	tags := []string{"v0.2.0", "not-a-version", "v0.10.0", "v0.2.1-rc.1", "v0.1.0", "v0.2.1"}

	var got []string
	for _, v := range Releases(tags) {
		got = append(got, v.String())
	}
	// Ascending, prereleases and tags that aren't versions dropped, and 0.10
	// above 0.2 because the numbers are compared rather than the text.
	want := []string{"v0.1.0", "v0.2.0", "v0.2.1", "v0.10.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Releases() = %#v, want %#v", got, want)
	}
}

func TestPreviousRelease(t *testing.T) {
	tests := []struct {
		name  string
		tags  []string
		want  string
		found bool
	}{
		{name: "no tags", tags: nil},
		{name: "one release has nothing before it", tags: []string{"v1.0.0"}},
		{name: "a prerelease is not a release", tags: []string{"v1.0.0", "v1.1.0-rc.1"}},
		{name: "the release below the latest", tags: []string{"v1.0.0", "v0.9.0", "v1.1.0"}, want: "v1.0.0", found: true},
	}

	for _, test := range tests {
		got, found := PreviousRelease(test.tags)
		if found != test.found {
			t.Errorf("%s: PreviousRelease() found = %v, want %v", test.name, found, test.found)
			continue
		}
		if found && got.String() != test.want {
			t.Errorf("%s: PreviousRelease() = %q, want %q", test.name, got, test.want)
		}
	}
}
