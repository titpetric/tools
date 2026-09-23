package fstest

import "testing"

func TestFake(t *testing.T) {
	if Fake() != "fake" {
		t.Error("Fake() is not fake")
	}
}
