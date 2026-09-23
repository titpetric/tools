// This test has no gone.go beside it, which the pairing check reports as an
// error. It is in the external test package, which is where a file compiles
// and not what the directory is for.
package paired_test

import "testing"

func TestGone(t *testing.T) {
	t.Skip("gone.go is not here, which is the point")
}
