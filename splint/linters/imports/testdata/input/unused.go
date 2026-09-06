package main

import (
	"errors"
	"net/http"
	"sort"
)

// ErrEmpty is returned for a request with nothing in it.
var ErrEmpty = errors.New("empty")

// Status is the code a handler answers an empty request with.
const Status = http.StatusBadRequest

// Nothing in this file names the third import, so the fixer takes it out and
// the linter reports it under imports/unused.
