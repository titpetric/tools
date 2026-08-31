package gomod

import (
	"os"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// ReadSum parses the go.sum at filename into the versions the model carries.
//
// A line is a module, a version and a hash, and a version written with a
// "/go.mod" suffix hashes the requirements of that version rather than its
// source. The two lines are one version here, and the suffix is what says
// whether the build downloads it: the module graph reads the go.mod of every
// version it considers, and downloads the source of the one it selects.
//
// The hashes are not kept. What they secure is the download, and what a report
// is reading go.sum for is which versions there were.
func ReadSum(filename string) ([]model.Sum, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var out []model.Sum
	at := map[string]int{}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		sum := model.Sum{Path: fields[0], Version: fields[1], Zip: true}
		if trimmed, cut := strings.CutSuffix(sum.Version, "/go.mod"); cut {
			sum.Version, sum.Zip = trimmed, false
		}

		key := sum.Path + "@" + sum.Version
		if index, seen := at[key]; seen {
			out[index].Zip = out[index].Zip || sum.Zip
			continue
		}

		at[key] = len(out)
		out = append(out, sum)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Version < out[j].Version
	})

	return out, nil
}
