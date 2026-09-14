package analyzer

import (
	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// FindModule returns the module governing dir, found by walking up from it
// until a go.mod turns up.
func FindModule(dir string) (*model.Module, error) {
	return gomod.Find(dir)
}
