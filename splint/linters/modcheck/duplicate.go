package modcheck

import (
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// duplicate is one module the build graph offered at more than one version.
//
// The two majors of a module are one entry: they are different module paths
// and the same library, and a build requiring both links both.
type duplicate struct {
	// Base is the module path with its major version suffix removed, which is
	// what the versions of one library are counted under.
	Base string

	// Versions is how many versions the graph offered, and Linked are the ones
	// the build downloads: a version go.sum records by its go.mod alone was
	// read for what it requires and passed over.
	Versions int
	Linked   []string

	// Size is what the linked copies weigh together, as far as the proxy
	// answered, and Overhead is what the copies past the largest weigh. A
	// module linked once has none.
	Size     int64
	Overhead int64

	// sizes is what each linked copy weighs, which the two above are added up
	// from once every version has been seen.
	sizes []int64
}

// duplicates groups what go.sum records by module and returns the modules
// recorded at more than one version, in path order.
//
// The sizes are what the proxy answered, keyed "path@version", and are added
// up over the linked versions: a version that is not downloaded occupies
// nothing.
func duplicates(sums []model.Sum, sizes map[string]int64) []duplicate {
	byBase := map[string]*duplicate{}

	for _, sum := range sums {
		base := gomod.Base(sum.Path)

		entry, seen := byBase[base]
		if !seen {
			entry = &duplicate{Base: base}
			byBase[base] = entry
		}

		entry.Versions++
		if !sum.Zip {
			continue
		}

		entry.Linked = append(entry.Linked, linked(base, sum))
		if size := sizes[sum.Path+"@"+sum.Version]; size > 0 {
			entry.sizes = append(entry.sizes, size)
		}
	}

	out := make([]duplicate, 0, len(byBase))
	for _, entry := range byBase {
		if entry.Versions < 2 {
			continue
		}

		// The largest copy is the one a build carrying the module once would
		// carry. Everything after it is the same library again.
		sort.Slice(entry.sizes, func(i, j int) bool { return entry.sizes[i] > entry.sizes[j] })
		for i, size := range entry.sizes {
			entry.Size += size
			if i > 0 {
				entry.Overhead += size
			}
		}

		out = append(out, *entry)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Base < out[j].Base })

	return out
}

// linked names one downloaded version under the module it is counted with. The
// path is the base for every version but a major, which is a path of its own:
// "v1.4.0" and "v2 v2.0.1" are two copies of one library.
func linked(base string, sum model.Sum) string {
	if suffix := strings.TrimPrefix(sum.Path, base+"/"); suffix != sum.Path {
		return suffix + " " + sum.Version
	}
	return sum.Version
}
