// Package handler is service1's HTTP half. It reaches model with no import of
// one, and the model it means is service1's.
package handler

import (
	"net/http"
)

// Serve writes a user.
func Serve(w http.ResponseWriter, name string) {
	user := model.User{Name: name}
	_, _ = w.Write([]byte(user.Name))
}
