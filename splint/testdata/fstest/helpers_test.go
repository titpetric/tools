// This test has no helpers.go beside it and is not reported: the package is
// called fstest, so its tests are the package rather than a check of one.
package fstest

import "testing"

func TestHelpers(t *testing.T) {
	t.Skip("helpers.go is not here, which is allowed in a test package")
}
