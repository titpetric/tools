// Package collide holds two files reaching different modules under the same
// short name, which compiles and reads as though they agree.
package collide

import "example.com/fixture/collide/a/model"

// FromA is what the model in a says.
func FromA() string {
	return model.Name()
}
