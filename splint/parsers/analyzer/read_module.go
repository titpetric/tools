package analyzer

import (
	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// ReadModule parses the go.mod at filename into the module facts the model
// carries.
func ReadModule(filename string) (*model.Module, error) {
	return gomod.Read(filename)
}
