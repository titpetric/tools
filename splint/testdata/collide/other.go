package collide

import "example.com/fixture/collide/b/model"

// FromB is what the model in b says, reached under the same name the file
// beside this one reaches a different model under.
func FromB() string {
	return model.Name()
}
