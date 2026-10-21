package analyzer

import (
	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// FindModule returns the module governing dir, walking up to its go.mod.
func FindModule(dir string) (*model.Module, error) {
	return gomod.Find(dir)
}
