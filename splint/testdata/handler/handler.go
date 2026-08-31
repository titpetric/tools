// Package handler has an exported handler with no unexported wrapper, which is
// what the wraphandler check reports.
package handler

import "net/http"

// Serve is an exported handler that wraps nothing, so there is no way to test
// what it does without a server.
func Serve(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Wrapped is the shape the check wants: the handler is a thin wrapper over a
// function that returns an error and can be tested on its own.
func Wrapped(w http.ResponseWriter, r *http.Request) {
	if err := wrapped(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// wrapped is where the work is, and what a test calls.
func wrapped(w http.ResponseWriter, r *http.Request) error {
	_, err := w.Write([]byte("ok"))
	return err
}
