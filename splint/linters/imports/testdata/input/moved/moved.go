// Package moved imports storage at the path it sat at before. One package of
// the tree is called storage, under service2.
package moved

import (
	"example.com/imports/storage"
)

// New returns an empty store.
func New() *storage.Store {
	return &storage.Store{}
}
