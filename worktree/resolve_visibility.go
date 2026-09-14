package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// visibilityReport is what every package of a module declares, split into the
// half a consumer can reach and the half it cannot, as "splint" measures it.
//
// It describes the working tree and no revision before it. A release is read
// from the API tables; this is read to see where the module keeps its weight,
// which is a question about the code as it stands.
type visibilityReport struct {
	// Packages are the counted packages, in path order.
	Packages []visibilityPackage `json:"packages"`

	// Skipped is why the report was not read, and is empty when it was.
	Skipped string `json:"skipped,omitempty"`
}

// visibilityPackage counts one package.
type visibilityPackage struct {
	// Package is the path relative to the module: "./" for the module root,
	// "./frontend" for a package below it.
	Package string `json:"package"`

	// ExportedTypes and InternalTypes count declared types by the case of
	// their name, and the two Funcs counts do the same for funcs and methods.
	ExportedTypes int `json:"exported_types"`
	InternalTypes int `json:"internal_types"`
	ExportedFuncs int `json:"exported_funcs"`
	InternalFuncs int `json:"internal_funcs"`

	// InternalRatio is the code inside internal func bodies over the func
	// code of the package, as a percentage.
	InternalRatio float64 `json:"internal_ratio"`
}

// splintMetric is one package's counts as the splint visibility linter
// reports them under --stats --json.
type splintMetric struct {
	ExportedTypes int `json:"ExportedTypes"`
	InternalTypes int `json:"InternalTypes"`
	ExportedFuncs int `json:"ExportedFuncs"`
	InternalFuncs int `json:"InternalFuncs"`
	InternalLines int `json:"InternalLines"`
	Lines         int `json:"Lines"`
}

// readVisibility counts the packages of the module in dir.
//
// Every reason the count cannot run comes back as a Skipped report rather than
// an error, the same way an unreadable API does: a module without the tool
// installed reports one section fewer, and the rest of the report stands.
func readVisibility(dir string) visibilityReport {
	if !isGoModule(dir) {
		return visibilityReport{Skipped: "not a go module"}
	}
	if _, err := exec.LookPath("splint"); err != nil {
		return visibilityReport{Skipped: "splint is not installed"}
	}

	cmd := exec.Command("splint", "--linters", "visibility", "--stats", "--json", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return visibilityReport{Skipped: fmt.Sprintf("splint: %v", err)}
	}

	var measurements []struct {
		Linter  string `json:"Linter"`
		Metrics struct {
			Packages map[string]splintMetric `json:"Packages"`
		} `json:"Metrics"`
	}
	if err := json.Unmarshal(out, &measurements); err != nil {
		return visibilityReport{Skipped: fmt.Sprintf("splint: %v", err)}
	}

	module, _ := readModulePath(dir)
	for _, m := range measurements {
		if m.Linter != "visibility" {
			continue
		}

		var visibility visibilityReport
		for path, metric := range m.Metrics.Packages {
			ratio := 0.0
			if metric.Lines > 0 {
				ratio = float64(metric.InternalLines) / float64(metric.Lines) * 100
			}
			visibility.Packages = append(visibility.Packages, visibilityPackage{
				Package:       packageLabel(module, path),
				ExportedTypes: metric.ExportedTypes,
				InternalTypes: metric.InternalTypes,
				ExportedFuncs: metric.ExportedFuncs,
				InternalFuncs: metric.InternalFuncs,
				InternalRatio: ratio,
			})
		}
		sort.Slice(visibility.Packages, func(i, j int) bool {
			return visibility.Packages[i].Package < visibility.Packages[j].Package
		})
		return visibility
	}
	return visibilityReport{Skipped: "the installed splint has no visibility linter"}
}

// packageLabel renders an import path relative to the module: "./" for the
// module root, "./frontend" for a package below it. A path outside the module
// is kept as it is.
func packageLabel(module, path string) string {
	if module == "" {
		return path
	}
	if path == module {
		return "./"
	}
	if rest, ok := strings.CutPrefix(path, module+"/"); ok {
		return "./" + rest
	}
	return path
}
