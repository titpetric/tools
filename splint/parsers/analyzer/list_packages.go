package analyzer

import (
	"os"
)

// ListPackages returns the local packages under rootPath for "." or "./...".
// It moves the process into rootPath and leaves it there for the rest of a
// parse; Parser.Parse puts it back.
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
