package prose

import (
	"strings"
	"testing"
)

func TestDensityFindingsLeaveRoom(t *testing.T) {
	baseline := Density{BlockWords: 10, SentenceWords: 10, P90Words: 20, Subordinators: 10, Stacked: 4}

	within := Density{BlockWords: 14, SentenceWords: 14, P90Words: 29, Subordinators: 14, Stacked: 5}
	if found := densityFindings(within, baseline); len(found) != 0 {
		t.Errorf("a quarter over the baseline reported %d findings: %+v", len(found), found)
	}

	over := Density{BlockWords: 30, SentenceWords: 30, P90Words: 60, Subordinators: 30, Stacked: 20}
	if found := densityFindings(over, baseline); len(found) != 5 {
		t.Errorf("three times the baseline reported %d findings, want 5: %+v", len(found), found)
	}
}

func TestDensityFindingsNeedABaseline(t *testing.T) {
	loud := Density{BlockWords: 40, SentenceWords: 40, P90Words: 80, Subordinators: 40, Stacked: 40}

	if found := densityFindings(loud, Density{}); len(found) != 0 {
		t.Errorf("a measure against nothing reported %d findings: %+v", len(found), found)
	}
}

func TestDensityMessageNamesBothNumbers(t *testing.T) {
	found := densityFindings(
		Density{SentenceWords: 30},
		Density{SentenceWords: 10},
	)

	if len(found) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(found), found)
	}

	message := found[0].Message
	for _, want := range []string{"30 words", "baseline's 10", "3.0x", "Break it"} {
		if !strings.Contains(message, want) {
			t.Errorf("the message is missing %q: %q", want, message)
		}
	}
	if found[0].Rule != RuleDensity {
		t.Errorf("the finding is under %q, want %q", found[0].Rule, RuleDensity)
	}
}

func TestTrim(t *testing.T) {
	for value, want := range map[float64]string{9: "9", 0: "0", 12.5: "12.50", 17.139: "17.14"} {
		if got := trim(value); got != want {
			t.Errorf("trim(%v) = %q, want %q", value, got, want)
		}
	}
}
