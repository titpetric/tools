// Package sizes renders the files of a document as a size report: every file
// with its byte size, the per directory totals, and a histogram over the
// files.
//
// The JSON is the contract go-ddd-stats wrote, so a consumer storing these
// payloads reads old and new rows the same way: Files carry Name, Path,
// Package and Size, Packages carry the per directory Name, Path, Size, Count
// and Average, and the Histogram buckets run from "< 1 KB" to "> 256 KB".
package sizes

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Stats is the whole report.
type Stats struct {
	Files     []*File
	Packages  []*Package
	Histogram []*Bucket
}

// File is one file with its size in bytes.
type File struct {
	Name    string
	Path    string
	Package string
	Size    int64
}

// Package is the files of one directory added up.
type Package struct {
	Name    string
	Path    string
	Size    int64
	Count   int64
	Average int64
}

// Bucket is one histogram bar.
type Bucket struct {
	Size  string
	Count int64
}

// Write renders the report for a document as JSON.
func Write(w io.Writer, defs model.DefinitionList) error {
	encoded, err := json.MarshalIndent(Data(defs), "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

// Data measures the document: every file it carries, whatever scope declared
// it, so a document written with tests counts the test files too.
func Data(defs model.DefinitionList) *Stats {
	seen := map[string]bool{}
	stats := &Stats{Files: []*File{}, Packages: []*Package{}, Histogram: histogram(nil)}

	for _, def := range defs {
		dir := packageDir(def.Package.Path)
		for _, file := range def.Files {
			name := file.Name
			if dir != "" {
				name = path.Join(dir, file.Name)
			}
			if seen[name] {
				continue
			}
			seen[name] = true

			stats.Files = append(stats.Files, &File{
				Name:    name,
				Path:    dir,
				Package: path.Base(path.Dir(name)),
				Size:    int64(file.Size),
			})
		}
	}

	sort.Slice(stats.Files, func(i, j int) bool { return stats.Files[i].Name < stats.Files[j].Name })

	byDir := map[string]*Package{}
	for _, file := range stats.Files {
		pkg, ok := byDir[file.Path]
		if !ok {
			pkg = &Package{Name: file.Package, Path: file.Path}
			byDir[file.Path] = pkg
			stats.Packages = append(stats.Packages, pkg)
		}
		pkg.Size += file.Size
		pkg.Count++
		pkg.Average = pkg.Size / pkg.Count
	}

	stats.Histogram = histogram(stats.Files)
	return stats
}

// packageDir turns a package path into the directory the report names: "" for
// the root, "frontend" for "./frontend", and "generic" for the ".generic" a
// nested module is recorded under.
func packageDir(pkgPath string) string {
	return strings.TrimPrefix(strings.TrimPrefix(pkgPath, "."), "/")
}

// histogram buckets the files by size, in powers of two from 1 KB to 256 KB
// with one bucket past the end.
func histogram(files []*File) []*Bucket {
	limits := []int64{1 << 10, 2 << 10, 4 << 10, 8 << 10, 16 << 10, 32 << 10, 64 << 10, 128 << 10, 256 << 10}

	buckets := make([]*Bucket, len(limits)+1)
	for i, limit := range limits {
		buckets[i] = &Bucket{Size: fmt.Sprintf("< %d KB", limit>>10)}
	}
	buckets[len(limits)] = &Bucket{Size: "> 256 KB"}

	for _, file := range files {
		placed := false
		for i, limit := range limits {
			if file.Size < limit {
				buckets[i].Count++
				placed = true
				break
			}
		}
		if !placed {
			buckets[len(limits)].Count++
		}
	}

	return buckets
}

// WriteD2 renders the histogram as a d2 source document, one rectangle per
// non-empty bucket holding the count of files in it, chained in size order. A
// bucket with no files is a gap in the range and not a measurement, so it is
// left out.
//
// The output goes through the d2 binary, which is what turns it into the
// diagram a README embeds:
//
//	splint sizes --render d2 ./... | d2 --layout elk - docs/assets/size.svg
func WriteD2(w io.Writer, defs model.DefinitionList) error {
	stats := Data(defs)

	title := fmt.Sprintf("File size distribution (*.go, %d files)", len(stats.Files))
	if _, err := fmt.Fprintf(w, "title: |md\n  # %s\n| {near: top-center}\n\n", title); err != nil {
		return err
	}

	var previous string
	for i, bucket := range stats.Histogram {
		if bucket.Count == 0 {
			continue
		}
		id := fmt.Sprintf("b%d", i)
		label := strings.ReplaceAll(bucket.Size, `"`, "")
		if _, err := fmt.Fprintf(w, "%s: %q {\n  shape: rectangle\n  count: %d\n}\n", id, label, bucket.Count); err != nil {
			return err
		}
		if previous != "" {
			if _, err := fmt.Fprintf(w, "%s -> %s\n", previous, id); err != nil {
				return err
			}
		}
		previous = id
	}
	return nil
}
