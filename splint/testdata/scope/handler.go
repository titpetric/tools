// The self contained check: every symbol here reaches the file beside it, so
// the file is coupled to it and neither is extractable on its own.
package scope

// Handler answers with what the store holds.
type Handler struct {
	Store *Store
}

// Serve reads the store the package declares elsewhere.
func (h *Handler) Serve(key string) string {
	return open(key).Get(key)
}
