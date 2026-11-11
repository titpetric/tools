// The blank import check: a package imported for its init alone decides what
// the binary does. This is neither main.go nor main_test.go.
package fixture

// pprof registers its handlers on the default mux when it is linked in, which
// is the whole reason it is imported this way. The whole reason a library
// file has no business doing it.
import _ "net/http/pprof"
