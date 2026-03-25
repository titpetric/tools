package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func renderD2(w io.Writer, modules []moduleInfo) {
	fmt.Fprintln(w, "direction: down")
	fmt.Fprintln(w, "grid-columns: 1")
	fmt.Fprintln(w)

	// Group by directory path (all segments except last) as flat containers
	type d2Component struct {
		key  string
		name string
		desc string
		mod  string
		pkg  string
		uses []string
	}

	groups := make(map[string][]d2Component)
	modToPkg := make(map[string]string)
	var groupOrder []string
	for _, m := range modules {
		short := strings.TrimPrefix(m.Name, "github.com/")
		idx := strings.LastIndex(short, "/")
		pkg, name := "local", short
		if idx != -1 {
			pkg, name = short[:idx], short[idx+1:]
		}
		if _, seen := groups[pkg]; !seen {
			groupOrder = append(groupOrder, pkg)
		}
		modToPkg[m.Name] = pkg
		comp := d2Component{
			key:  d2Key(short[idx+1:]),
			name: name,
			mod:  m.Name,
			pkg:  pkg,
			uses: m.Uses,
		}
		if strings.Contains(m.Description, " ") {
			if _, after, ok := strings.Cut(m.Description, " - "); ok {
				comp.desc = after
			}
		}
		groups[pkg] = append(groups[pkg], comp)
	}

	// Sort groups by module count descending
	sort.Slice(groupOrder, func(i, j int) bool {
		return len(groups[groupOrder[i]]) > len(groups[groupOrder[j]])
	})

	// Build a map from module name to container.key path
	modToPath := make(map[string]string)
	for _, pkg := range groupOrder {
		containerKey := d2Key(pkg)
		for _, c := range groups[pkg] {
			modToPath[c.mod] = containerKey + "." + c.key
		}
	}

	// Track cross-package imports for each module (who imports it)
	crossPkgImports := make(map[string][]string)
	for _, pkg := range groupOrder {
		for _, c := range groups[pkg] {
			for _, dep := range c.uses {
				if depPkg, ok := modToPkg[dep]; ok && depPkg != pkg {
					crossPkgImports[dep] = append(crossPkgImports[dep], c.name)
				}
			}
		}
	}

	// Render containers and components
	for _, pkg := range groupOrder {
		containerKey := d2Key(pkg)
		fmt.Fprintf(w, "%s: %s {\n", containerKey, pkg)
		fmt.Fprintln(w, "  style.fill: \"#f8f9fa\"")
		for _, c := range groups[pkg] {
			if c.desc != "" {
				fmt.Fprintf(w, "  %s: \"%s\\n%s\"\n", c.key, c.name, c.desc)
			} else {
				fmt.Fprintf(w, "  %s: %s\n", c.key, c.name)
			}
			fmt.Fprintf(w, "  %s.link: %s\n", c.key, moduleLink(c.mod))
			if importers := crossPkgImports[c.mod]; len(importers) > 0 {
				noteKey := c.key + "-imports"
				label := "Used by " + strings.Join(importers, ", ")
				fmt.Fprintf(w, "  %s: \"%s\" {\n", noteKey, label)
				fmt.Fprintln(w, "    shape: text")
				fmt.Fprintln(w, "    style.font-size: 12")
				fmt.Fprintln(w, "    style.font-color: \"#666\"")
				fmt.Fprintln(w, "  }")
				fmt.Fprintf(w, "  %s -- %s: {\n", c.key, noteKey)
				fmt.Fprintln(w, "    style.stroke-dash: 3")
				fmt.Fprintln(w, "    style.stroke: \"#ccc\"")
				fmt.Fprintln(w, "  }")
			}
		}
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	// Render only same-package connections
	fmt.Fprintln(w, "**.style.border-radius: 8")
	fmt.Fprintln(w)
	for _, pkg := range groupOrder {
		containerKey := d2Key(pkg)
		for _, c := range groups[pkg] {
			fromPath := containerKey + "." + c.key
			for _, dep := range c.uses {
				depPkg := modToPkg[dep]
				if depPkg == pkg {
					if toPath, ok := modToPath[dep]; ok {
						fmt.Fprintf(w, "%s <- %s\n", fromPath, toPath)
					}
				}
			}
		}
	}
}

func d2Key(s string) string {
	s = strings.TrimPrefix(s, "github.com/")
	r := strings.NewReplacer("/", "-", ".", "-")
	return r.Replace(s)
}
