package components

// Dependent represents a module that depends on another, with its display name and outdated status.
type Dependent struct {
	Name     string
	Outdated bool
}
