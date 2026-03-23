package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func renderPUML(w io.Writer, modules []moduleInfo) {
	fmt.Fprintln(w, "@startuml")
	fmt.Fprintln(w, "top to bottom direction")
	fmt.Fprintln(w, "set namespaceSeparator none")
	fmt.Fprintln(w, "skinparam linetype ortho")
	fmt.Fprintln(w, "skinparam packageStyle rectangle")
	fmt.Fprintln(w, "skinparam component {")
	fmt.Fprintln(w, "  ArrowColor #666666")
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	// Group by directory path (all segments except last) as flat packages
	groups := make(map[string][]pumlComponent)
	var groupOrder []string
	for _, m := range modules {
		short := strings.TrimPrefix(m.Name, "github.com/")
		idx := strings.LastIndex(short, "/")
		pkg, label := "local", short
		if idx != -1 {
			pkg, label = short[:idx], short[idx+1:]
		}
		if _, seen := groups[pkg]; !seen {
			groupOrder = append(groupOrder, pkg)
		}
		comp := pumlComponent{
			label: label,
			alias: pumlAlias(m.Name),
			link:  moduleLink(m.Name),
		}
		if strings.Contains(m.Description, " ") {
			if before, after, ok := strings.Cut(m.Description, " - "); ok {
				comp.label = "<b>" + before + "</b>\\n" + after
			} else {
				comp.label = m.Description
			}
		}
		groups[pkg] = append(groups[pkg], comp)
	}

	// Sort groups by module count descending
	sort.Slice(groupOrder, func(i, j int) bool {
		return len(groups[groupOrder[i]]) > len(groups[groupOrder[j]])
	})

	for _, pkg := range groupOrder {
		fmt.Fprintf(w, "package \"%s\" {\n", pkg)
		comps := groups[pkg]
		for _, c := range comps {
			fmt.Fprintf(w, "  component \"%s\" as %s [[%s]]\n", c.label, c.alias, c.link)
		}
		for i := 0; i < len(comps)-1; i++ {
			fmt.Fprintf(w, "  %s --> %s\n", comps[i].alias, comps[i+1].alias)
		}
		fmt.Fprintln(w, "}")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w)

	// Build module-to-package lookup and track cross-package imports
	modPkg := make(map[string]string)
	modName := make(map[string]string)
	for _, m := range modules {
		short := strings.TrimPrefix(m.Name, "github.com/")
		idx := strings.LastIndex(short, "/")
		pkg, name := "local", short
		if idx != -1 {
			pkg, name = short[:idx], short[idx+1:]
		}
		modPkg[m.Name] = pkg
		modName[m.Name] = name
	}

	crossPkgImports := make(map[string][]string)
	for _, m := range modules {
		for _, dep := range m.Uses {
			if modPkg[m.Name] != modPkg[dep] {
				crossPkgImports[dep] = append(crossPkgImports[dep], modName[m.Name])
			}
		}
	}

	// Add notes for cross-package imports
	for mod, importers := range crossPkgImports {
		alias := pumlAlias(mod)
		label := "Used by " + strings.Join(importers, ", ")
		fmt.Fprintf(w, "note right of %s : %s\n", alias, label)
	}
	fmt.Fprintln(w)

	// Render relationships; only same-package links
	for _, m := range modules {
		from := pumlAlias(m.Name)
		for _, dep := range m.Uses {
			to := pumlAlias(dep)
			if modPkg[m.Name] == modPkg[dep] {
				fmt.Fprintf(w, "%s <-- %s\n", from, to)
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "@enduml")
}

type pumlComponent struct {
	label string
	alias string
	link  string
}

func pumlAlias(modPath string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return r.Replace(modPath)
}
