// Package storage is service2's store. It reaches model with no import of one,
// from a directory below the one holding it, so the name resolves to
// service2/model and never to service1/model.
package storage

import (
	"strings"

	"example.com/imports/service2/model"
)

// Store holds users.
type Store struct {
	users []model.User
}

// Add records a user.
func (s *Store) Add(email string) {
	s.users = append(s.users, model.User{Email: strings.ToLower(email)})
}
