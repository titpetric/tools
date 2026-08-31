package paired

import "testing"

func TestTested(t *testing.T) {
	if Tested() != 1 {
		t.Error("Tested() is not 1")
	}
}
