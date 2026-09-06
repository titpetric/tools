// Package model is service2's model, and the other of the two packages in this
// tree called model.
package model

// User is a person service2 knows about, and is not service1's User.
type User struct {
	Email string
}
