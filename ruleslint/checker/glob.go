package checker

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// match is one path a pattern resolved to, and the placeholders it bound.
type match struct {
	path  string
	binds map[string]string
	dir   bool
}

var placeholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// free reports whether a pattern holds a placeholder no binding fills.
func free(pattern string, binds map[string]string) bool {
	for _, one := range placeholder.FindAllStringSubmatch(pattern, -1) {
		if _, ok := binds[one[1]]; !ok {
			return true
		}
	}
	return false
}

// subst fills the placeholders a binding names, and keeps the rest as written.
func subst(text string, binds map[string]string) string {
	return placeholder.ReplaceAllStringFunc(text, func(one string) string {
		if value, ok := binds[placeholder.FindStringSubmatch(one)[1]]; ok {
			return value
		}
		return one
	})
}

// expand resolves pattern under base, directories alone for a trailing slash.
func expand(root, base, pattern string, binds map[string]string) []match {
	trimmed := strings.Trim(pattern, "/")
	out := []match{{path: base, binds: binds, dir: true}}
	if trimmed == "" {
		return out
	}
	for _, seg := range strings.Split(trimmed, "/") {
		var next []match
		for _, one := range out {
			next = append(next, step(root, one, seg)...)
		}
		out = next
	}
	if strings.HasSuffix(pattern, "/") {
		out = slices.DeleteFunc(out, func(one match) bool { return !one.dir })
	}
	return out
}

// step resolves one segment inside the directory a match names.
func step(root string, parent match, seg string) []match {
	seg = subst(seg, parent.binds)
	if !strings.ContainsAny(seg, "*{") {
		info, err := os.Stat(filepath.Join(root, parent.path, seg))
		if err != nil {
			return nil
		}
		return []match{{path: path.Join(parent.path, seg), binds: parent.binds, dir: info.IsDir()}}
	}

	re, names := segRE(seg)
	entries, err := os.ReadDir(filepath.Join(root, parent.path))
	if err != nil || re == nil {
		return nil
	}

	var out []match
	for _, entry := range entries {
		got := re.FindStringSubmatch(entry.Name())
		if got == nil {
			continue
		}
		out = append(out, match{
			path:  path.Join(parent.path, entry.Name()),
			binds: bind(parent.binds, names, got[1:]),
			dir:   entry.IsDir(),
		})
	}
	return out
}

// bind copies a binding and adds under it the names a segment captured.
func bind(binds map[string]string, names, values []string) map[string]string {
	if len(names) == 0 {
		return binds
	}
	out := make(map[string]string, len(binds)+len(names))
	maps.Copy(out, binds)
	for i, name := range names {
		out[name] = values[i]
	}
	return out
}

// segRE compiles one segment, with {name} as a capture of that name.
func segRE(seg string) (*regexp.Regexp, []string) {
	var names []string
	var out strings.Builder

	out.WriteString("^")
	last := 0
	for _, loc := range placeholder.FindAllStringSubmatchIndex(seg, -1) {
		out.WriteString(wildcard(seg[last:loc[0]], `[^/]*`))
		out.WriteString(`([^/]+)`)
		names = append(names, seg[loc[2]:loc[3]])
		last = loc[1]
	}
	out.WriteString(wildcard(seg[last:], `[^/]*`))
	out.WriteString("$")

	re, err := regexp.Compile(out.String())
	if err != nil {
		return nil, nil
	}
	return re, names
}

// holds reports whether content holds text, with "*" as a run inside one line.
func holds(content, text string) bool {
	if !strings.Contains(text, "*") {
		return strings.Contains(content, text)
	}
	re, err := regexp.Compile(wildcard(text, `[^\n]*`))
	return err == nil && re.MatchString(content)
}

// wildcard quotes the literal pieces of text, with "*" as the run any names.
func wildcard(text, any string) string {
	parts := strings.Split(text, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	return strings.Join(parts, any)
}
