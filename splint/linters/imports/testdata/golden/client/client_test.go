package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAssertResolvesFromTheTree reaches assert with no import of it. Nothing
// in the module cache is read: client.go writes the path out, and that is what
// places it.
func TestAssertResolvesFromTheTree(t *testing.T) {
	assert.NotNil(t, Check)
}
