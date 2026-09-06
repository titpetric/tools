package main

import (
	"bufio"

	// Postgres registers itself with database/sql.
	_ "example.com/imports/service2/storage"
	"bytes" // the buffer the reader is filled from
)

// Reader reads a buffer a line at a time.
func Reader(buf *bytes.Buffer) *bufio.Reader {
	return bufio.NewReader(buf)
}
