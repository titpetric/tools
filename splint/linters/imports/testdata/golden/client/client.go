// Package client spells out the testify import, which is what teaches the
// test file beside it what assert means.
package client

import (
	"github.com/stretchr/testify/assert"
)

// Check is the one thing in the tree that names the assert package outright.
var Check = assert.New
