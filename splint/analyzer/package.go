package analyzer

import (
	"fmt"
	"log"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/titpetric/tools/splint/model"
)

func cleanPackages(pkgs []*packages.Package, workDir string) TargetList {
	results := make(TargetList, 0, len(pkgs))

	isDebug := false
	if isDebug {
		// This area is sensitive. ID combines test context and package names.
		for _, pkg := range pkgs {
			fmt.Printf("- %s [%s, %q]\n", pkg.Name, pkg.Dir, pkg.ID)
		}
	}

	for _, pkg := range pkgs {
		if isDebug {
			log.Printf("> %q, %q %q %q", pkg.ID, pkg.Name, pkg.PkgPath, pkg.ForTest)
			for _, f := range pkg.GoFiles {
				log.Println("-", f)
			}
		}

		isTestScope := false
		for _, f := range pkg.GoFiles {
			if strings.HasSuffix(f, "_test.go") {
				isTestScope = true
			}
		}

		results = append(results, &Target{
			Package: &model.Package{
				ID:          pkg.ID,
				Package:     pkg.Name,
				ImportPath:  pkg.PkgPath,
				Path:        "." + strings.TrimPrefix(pkg.Dir, workDir),
				TestPackage: isTestScope,
			},
			Syntax: pkg,
		})
	}

	if isDebug {
		fmt.Println("Done with", len(results))
	}

	return results
}

func listPackages(pattern string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule | packages.LoadSyntax,
		Tests: true,
	}

	return packages.Load(cfg, pattern)
}
