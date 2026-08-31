// The argument and return order checks, which want a context first, a duration
// last, and an error after the value it qualifies.
package fixture

import (
	"context"
	"time"
)

// User is what the functions below return.
type User struct {
	Name string
}

// Wait takes its duration before the thing it applies to, which is the reverse
// of how it reads.
func Wait(d time.Duration, name string) error {
	time.Sleep(d)
	return nil
}

// Find returns its error before the value the error is about.
func Find(ctx context.Context, id string) (error, *User) {
	return nil, &User{Name: id}
}

// Good is what both checks want: the context first, the error last.
func Good(ctx context.Context, id string) (*User, error) {
	return &User{Name: id}, nil
}
