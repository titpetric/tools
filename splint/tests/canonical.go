package tests

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Canonical renders a package as a tree keyed by what a thing is rather than
// by where it sits in a list.
//
// The declaration lists become maps keyed on the symbol, so a comparison lines
// two documents up by identity: one extra declaration is one difference, where
// a positional walk would shift every declaration after it and report a
// hundred. Everything else is left exactly as it encodes, so a key nobody
// thought about is still walked.
func Canonical(def *model.Definition) map[string]any {
	tree, ok := encode(def).(map[string]any)
	if !ok {
		return map[string]any{"unencodable": fmt.Sprintf("%T", def)}
	}

	for _, list := range []string{"Types", "Consts", "Vars", "Funcs"} {
		declarations, ok := tree[list].([]any)
		if !ok {
			continue
		}
		tree[list] = keyed(declarations)
	}

	return tree
}

// keyed turns a list of declarations into a map from symbol to declaration.
//
// A key that two declarations share, which is a file declaring the same name
// twice, keeps both under numbered keys rather than losing one: a comparison
// that silently dropped a declaration would be reporting agreement it had not
// established.
func keyed(declarations []any) KeyedList {
	out := make(KeyedList, len(declarations))

	for _, entry := range declarations {
		decl, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		key := symbolKey(decl)
		if _, taken := out[key]; taken {
			for i := 2; ; i++ {
				numbered := fmt.Sprintf("%s#%d", key, i)
				if _, taken := out[numbered]; !taken {
					key = numbered
					break
				}
			}
		}
		out[key] = decl
	}

	return out
}

// symbolKey names one declaration the way a reader refers to it: the file it
// is in, the receiver it hangs off, and the name or names it declares.
//
// The line is not part of it. A declaration that moved down a file is the same
// declaration, and keying on the line would report it as one removed and one
// added rather than as a line that changed.
func symbolKey(decl map[string]any) string {
	parts := []string{text(decl["File"])}

	if receiver := text(decl["Receiver"]); receiver != "" {
		parts = append(parts, strings.TrimPrefix(receiver, "*"))
	}

	if name := text(decl["Name"]); name != "" {
		parts = append(parts, name)
	} else if names, ok := decl["Names"].([]any); ok && len(names) > 0 {
		written := make([]string, 0, len(names))
		for _, name := range names {
			written = append(written, text(name))
		}
		sort.Strings(written)
		parts = append(parts, strings.Join(written, ","))
	} else {
		parts = append(parts, "<unnamed>")
	}

	return strings.Join(parts, ".")
}

// text renders a value as the string it is, and as nothing when it is absent.
func text(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}

	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
