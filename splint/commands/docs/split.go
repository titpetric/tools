package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// PackageDoc holds one package's rendered documentation and where it goes.
type PackageDoc struct {
	ImportPath string
	Filename   string
	Content    string
}

// splitFilename names the file a package is written to: the import path with
// the first matching strip prefix taken off, or with its first segment
// dropped when no prefix was given, slashes turned to underscores.
func splitFilename(importPath string, strip []string) string {
	path := importPath

	switch {
	case len(strip) > 0:
		for _, prefix := range strip {
			if strings.HasPrefix(importPath, prefix) {
				path = strings.TrimPrefix(importPath, prefix)
				path = strings.TrimPrefix(path, "/")
				break
			}
		}
	default:
		if parts := strings.Split(importPath, "/"); len(parts) > 1 {
			path = strings.Join(parts[1:], "/")
		}
	}

	// The root package strips to nothing and is named after itself.
	if path == "" {
		path = filepath.Base(importPath)
	}

	return strings.ReplaceAll(path, "/", "_") + ".md"
}

// groupDefinitionsByPackage keys the definitions by import path, leaving the
// test packages out.
func groupDefinitionsByPackage(defs model.DefinitionList) map[string]model.DefinitionList {
	groups := make(map[string]model.DefinitionList)
	for _, def := range defs {
		if skipDocs(def) {
			continue
		}
		groups[def.Package.ImportPath] = append(groups[def.Package.ImportPath], def)
	}
	return groups
}

// renderSplit writes one markdown file per package under OutDir, and a
// README.md listing them.
func renderSplit(defs model.DefinitionList, opts Options) error {
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	groups := groupDefinitionsByPackage(defs)
	examples := collectExamples(defs)

	var importPaths []string
	for path := range groups {
		importPaths = append(importPaths, path)
	}
	sort.Strings(importPaths)

	var packageDocs []PackageDoc
	for _, importPath := range importPaths {
		group := groups[importPath]
		if len(group) == 0 {
			continue
		}

		packageDocs = append(packageDocs, PackageDoc{
			ImportPath: importPath,
			Filename:   splitFilename(importPath, opts.StripPrefix),
			Content:    packageMarkdown(group[0], examples[importPath]),
		})
	}

	for _, pkg := range packageDocs {
		name := filepath.Join(opts.OutDir, pkg.Filename)
		if err := os.WriteFile(name, []byte(pkg.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	readmePath := filepath.Join(opts.OutDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme(packageDocs)), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	return nil
}

// readme is the table of contents beside the split files.
func readme(packageDocs []PackageDoc) string {
	var buf strings.Builder

	fmt.Fprint(&buf, "# API Documentation\n\n")
	fmt.Fprint(&buf, "## Table of Contents\n\n")

	for _, pkg := range packageDocs {
		fmt.Fprintf(&buf, "- [%s](./%s)\n", pkg.ImportPath, pkg.Filename)
	}

	return buf.String()
}
