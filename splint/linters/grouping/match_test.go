package grouping

import (
	"slices"
	"testing"
)

// TestSymbol covers the filenames a symbol belonging to no type is accepted
// in, which is a type declaration and a function that constructs nothing.
func TestSymbol(t *testing.T) {
	tests := []struct {
		title    string
		name     string
		receiver string
		want     []string
	}{
		{
			title:    "a type is its own name",
			receiver: "ServiceDiscoveryConnectionError",
			want: []string{
				"service_discovery_connection_error.go",
				"service_discovery_connection.go",
				"service_discovery.go",
				"service*.go",
				"errors.go",
				"error.go",
				"default.go",
			},
		},
		{
			title: "a function on its own",
			name:  "Get",
			want: []string{
				"get.go",
				"default.go",
			},
		},
		{
			title: "a compound function name walks back a word",
			name:  "LimiterFunc",
			want: []string{
				"limiter_func.go",
				"limiter*.go",
				"limit*.go",
				"default.go",
			},
		},
		{
			title: "a constructor loses its New",
			name:  "NewSchedulerContextTimeout",
			want: []string{
				"scheduler_context_timeout.go",
				"scheduler_context.go",
				"scheduler*.go",
				"schedul*.go",
				"default.go",
			},
		},
	}

	for _, test := range tests {
		want := append(slices.Clone(test.want), allowlist...)
		got := matchFilenames(test.name, test.receiver, "default.go")
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", test.title, got, want)
		}
	}
}

// TestSymbol_Method covers the filenames a symbol belonging to a type is
// accepted in, where the receiver leads and the name follows it.
func TestSymbol_Method(t *testing.T) {
	tests := []struct {
		title    string
		name     string
		receiver string
		want     []string
	}{
		{
			title:    "a method under its receiver",
			name:     "Get",
			receiver: "ServiceDiscovery",
			want: []string{
				"service_discovery_get.go",
				"service_discovery.go",
				"service*.go",
				"discovery.go",
				"default.go",
			},
		},
		{
			title:    "many of a thing are also one of it",
			name:     "Get",
			receiver: "Assets",
			want: []string{
				"assets_get.go",
				"assets*.go",
				"asset*.go",
				"default.go",
			},
		},
		{
			title:    "a doer is also what it does",
			name:     "Do",
			receiver: "Checker",
			want: []string{
				"checker_do.go",
				"checker*.go",
				"check*.go",
				"default.go",
			},
		},
		{
			title:    "an acronym is one word",
			name:     "Request",
			receiver: "HTTPClient",
			want: []string{
				"http_client_request.go",
				"http_client.go",
				"http*.go",
				"client.go",
				"default.go",
			},
		},
	}

	for _, test := range tests {
		want := append(slices.Clone(test.want), allowlist...)
		got := matchFilenames(test.name, test.receiver, "default.go")
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", test.title, got, want)
		}
	}
}

// TestSymbolMatch covers what the walk over a compound name buys: a symbol is
// at home under either half of its receiver and under its own name alone.
func TestSymbolMatch(t *testing.T) {
	tests := []struct {
		file string
		want bool
	}{
		{file: "service_discovery_start.go", want: true},
		{file: "service_discovery.go", want: true},
		{file: "discovery_start.go", want: true},
		{file: "start.go", want: true},
		{file: "fixture.go", want: true},
		{file: "model.go", want: true},
		{file: "elsewhere.go", want: false},
	}

	for _, test := range tests {
		sym := symbol{
			kind:     "func",
			name:     "Start",
			receiver: "ServiceDiscovery",
			file:     test.file,
			fallback: "fixture*.go",
		}

		expected, total, matched := sym.match()
		if matched != test.want {
			t.Errorf("%s: matched = %v, want %v", test.file, matched, test.want)
		}
		if want := []string{"service_discovery_start.go", "service_discovery.go", "start.go"}; !slices.Equal(expected, want) {
			t.Errorf("%s: expected = %v, want %v", test.file, expected, want)
		}
		if total < len(expected) {
			t.Errorf("%s: total = %d, fewer than the %d named", test.file, total, len(expected))
		}
	}
}
