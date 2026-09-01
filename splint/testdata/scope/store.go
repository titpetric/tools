// The self contained check: this file declares what the file beside it
// reaches, and neither of them can be built without the other.
package scope

// Store is what the handler beside this file is built on.
type Store struct {
	Name string
}

// Get returns nothing, which is enough to be reached.
func (s *Store) Get(key string) string {
	return s.Name
}

// open is the package level helper the other file calls.
func open(name string) *Store {
	return &Store{Name: name}
}
