package analyzer

import (
	"os"
)

// ListPackages returns the local packages under rootPath. The pattern is
// either "." or "./..." for a recursive listing.
func ListPackages(rootPath string, pattern string) (TargetList, error) {
	if err := os.Chdir(rootPath); err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	packages, err := listPackages(pattern)
	if err != nil || len(packages) == 0 {
		return nil, err
	}

	result := cleanPackages(packages, cwd)

	return result, nil
}
