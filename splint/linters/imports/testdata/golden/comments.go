package main

import (
	"bufio"
	"bytes" // the buffer the reader is filled from

	// Postgres registers itself with database/sql.
	_ "example.com/imports/service2/storage"
)

// Reader reads a buffer a line at a time.
func Reader(buf *bytes.Buffer) *bufio.Reader {
	return bufio.NewReader(buf)
}
