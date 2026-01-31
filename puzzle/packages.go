package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

func getPackages() []string {
	set := map[string]struct{}{}
	words := []string{}

	modCmd := exec.Command("go", "mod", "edit", "-json")
	if modOut, err := modCmd.Output(); err == nil {
		var mod struct {
			Module struct {
				Path string
			}
		}
		if err := json.Unmarshal(modOut, &mod); err == nil {
			name := mod.Module.Path
			if !options.FullPackageName {
				parts := strings.Split(mod.Module.Path, "/")
				name = strings.ToLower(parts[len(parts)-1])
			}
			if _, exists := set[name]; !exists {
				set[filepath.Base(mod.Module.Path)] = struct{}{}
				set[name] = struct{}{}
				words = append(words, name)
			}
		}
	}

	cmd := exec.Command("go", "list", "./...")
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			parts := strings.Split(l, "/")
			name := strings.ToLower(parts[len(parts)-1])
			if _, exists := set[name]; !exists {
				set[name] = struct{}{}
				words = append(words, name)
			}
		}
	}

	return words
}
