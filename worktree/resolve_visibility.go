package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// visibilityReport is what every package of a module declares, split into the
// half a consumer can reach and the half it cannot, as "gofsck" reports it.
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

	// InternalRatio is the code inside internal func bodies over the code of
	// the package, as a percentage.
	InternalRatio float64 `json:"internal_ratio"`
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
	if _, err := exec.LookPath("gofsck"); err != nil {
		return visibilityReport{Skipped: "gofsck is not installed"}
	}

	cmd := exec.Command("gofsck", "-skip-generated", "-format", "json", "./...")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return visibilityReport{Skipped: fmt.Sprintf("gofsck: %v", err)}
	}

	var report struct {
		Analyzers []struct {
			Name string          `json:"name"`
			Data json.RawMessage `json:"data"`
		} `json:"analyzers"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		return visibilityReport{Skipped: fmt.Sprintf("gofsck: %v", err)}
	}

	for _, analyzer := range report.Analyzers {
		if analyzer.Name != "visibility" {
			continue
		}
		var visibility visibilityReport
		if err := json.Unmarshal(analyzer.Data, &visibility); err != nil {
			return visibilityReport{Skipped: fmt.Sprintf("gofsck visibility: %v", err)}
		}
		return visibility
	}
	return visibilityReport{Skipped: "the installed gofsck has no visibility analyzer"}
}
