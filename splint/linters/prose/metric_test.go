package prose

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMetric(t *testing.T) {
	body, err := json.Marshal(Metric{
		Blocks:        12,
		Words:         340,
		BlockWords:    19,
		SentenceWords: 16,
		P90Words:      31,
		Subordinators: 17.13,
		Stacked:       8.52,
		Shapes:        4,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	for _, want := range []string{
		`"Blocks":12`, `"Words":340`, `"BlockWords":19`, `"SentenceWords":16`,
		`"P90Words":31`, `"Subordinators":17.13`, `"Stacked":8.52`, `"Shapes":4`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the encoding is missing %s: %s", want, body)
		}
	}

	got := Metric{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Subordinators != 17.13 || got.BlockWords != 19 || got.Shapes != 4 {
		t.Errorf("the round trip answered %+v", got)
	}
}
