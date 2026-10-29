package test

import (
	"context"
	"path/filepath"
	"sort"
)

// changedFunc is a function the change added or modified.
type changedFunc struct {
	Dir        string
	Key        string
	Name       string
	Receiver   string
	Kind       testKind
	ImportPath string
}

// changedFuncs compares every changed file with the revision and returns the
// functions whose signature or body moved.
func (s *source) changedFuncs(ctx context.Context, changes []change) ([]changedFunc, error) {
	var out []changedFunc

	for _, one := range changes {
		rel := filepath.Clean(one.Path)
		if rel == "." || one.Status == 'D' {
			continue
		}
		dir := filepath.Dir(rel)

		next, err := parseFile(filepath.Join(s.root, rel), rel, dir)
		if err != nil {
			return nil, err
		}

		var before *fileDecls
		oldPath := rel
		if one.OldPath != "" {
			oldPath = filepath.Clean(one.OldPath)
		}
		if data, found, err := s.oldFile(ctx, oldPath); err != nil {
			return nil, err
		} else if found {
			if before, err = parseSource(data, oldPath, rel, dir); err != nil {
				return nil, err
			}
		}

		old := map[string]*funcDecl{}
		if before != nil {
			for _, fn := range before.Funcs {
				old[fn.Key] = fn
			}
		}

		for _, fn := range next.Funcs {
			was, found := old[fn.Key]
			if found && was.Signature == fn.Signature && was.Body == fn.Body {
				continue
			}
			out = append(out, changedFunc{
				Dir:        dir,
				Key:        fn.Key,
				Name:       fn.Name,
				Receiver:   fn.Receiver,
				Kind:       fn.Kind,
				ImportPath: s.importPath(dir),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir < out[j].Dir
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// changedDirs is every directory a change touched, which is where the
// inventory reads production files as well as tests.
func changedDirs(changes []change, changed []changedFunc) map[string]bool {
	out := map[string]bool{}
	for _, one := range changes {
		if one.Status != 'D' {
			out[filepath.Dir(filepath.Clean(one.Path))] = true
		}
	}
	for _, one := range changed {
		out[one.Dir] = true
	}
	return out
}
