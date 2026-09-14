package analyzer

import (
	"os"
)

// ListPackages returns the local packages under rootPath. The pattern is
// either "." or "./..." for a recursive listing.
//
// It moves the process into rootPath and leaves it there. The toolchain
// resolves a pattern from the working directory, and Load reads a file from a
// path relative to the same place, so the move belongs to the whole of a parse
// rather than to the listing. Parser.Parse is what puts the process back.
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
