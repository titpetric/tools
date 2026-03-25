package filecheck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
	"golang.org/x/tools/go/packages"
)

// bucketThresholds defines power-of-two KB boundaries for the histogram.
// Index 0: ≤1KB, 1: ≤2KB, 2: ≤4KB, 3: ≤8KB, 4: ≤16KB, 5: ≤32KB, 6: ≤64KB, 7: >64KB
var bucketThresholds = []int64{1024, 2048, 4096, 8192, 16384, 32768, 65536}

const numBuckets = 8

// ratingThreshold is the byte size (16KB) above which files are considered "very high" complexity.
const ratingThreshold = 16384

// Analyzer performs file size statistics analysis on a set of packages.
type Analyzer struct{}

// New creates a new filecheck analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze examines packages and returns file size statistics.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	moduleRoot := findModuleRoot(pkgs)

	// Collect unique file paths grouped by extension.
	filesByExt := make(map[string][]string)
	seen := make(map[string]bool)

	for _, pkg := range pkgs {
		for _, f := range pkg.GoFiles {
			if !isUnderRoot(f, moduleRoot) {
				continue
			}
			if seen[f] {
				continue
			}
			seen[f] = true
			ext := filepath.Ext(f)
			filesByExt[ext] = append(filesByExt[ext], f)
		}
		for _, f := range pkg.OtherFiles {
			if !isUnderRoot(f, moduleRoot) {
				continue
			}
			if seen[f] {
				continue
			}
			seen[f] = true
			ext := filepath.Ext(f)
			filesByExt[ext] = append(filesByExt[ext], f)
		}
	}

	// Recursively scan the entire module root for markdown files.
	if moduleRoot != "" {
		scanMarkdownRecursive(moduleRoot, seen, filesByExt)
	}

	var totalSize int64
	var overThresholdSize int64
	var scanned []ScannedGroup

	// Sort extensions for deterministic output.
	exts := make([]string, 0, len(filesByExt))
	for ext := range filesByExt {
		exts = append(exts, ext)
	}
	sort.Strings(exts)

	for _, ext := range exts {
		// Only process .go and .md files per README spec
		if ext != ".go" && ext != ".md" {
			continue
		}

		files := filesByExt[ext]
		histogram := make([]int, numBuckets)

		var groupSize int64
		var groupOverThreshold int64

		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			size := info.Size()
			totalSize += size
			groupSize += size
			if size >= ratingThreshold {
				overThresholdSize += size
				groupOverThreshold += size
			}
			bucket := fileBucket(size)
			histogram[bucket]++
		}

		var groupScore float64
		if groupSize > 0 {
			groupScore = 100.0 - (float64(groupOverThreshold)/float64(groupSize))*100.0
		}

		scanned = append(scanned, ScannedGroup{
			Ext:       ext,
			Files:     len(files),
			Histogram: histogram,
			Score:     groupScore,
		})
	}

	var rating float64
	if totalSize > 0 {
		rating = 100.0 - (float64(overThresholdSize)/float64(totalSize))*100.0
	}

	return &Report{
		Scanned: scanned,
		Rating:  rating,
	}, nil
}

// fileBucket returns the histogram bucket index for a given file size.
func fileBucket(size int64) int {
	for i, threshold := range bucketThresholds {
		if size <= threshold {
			return i
		}
	}
	return numBuckets - 1
}

// findModuleRoot determines the common root directory from the first non-test package's files.
func findModuleRoot(pkgs []*packages.Package) string {
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.PkgPath, ".test") || strings.HasSuffix(pkg.PkgPath, "_test") {
			continue
		}
		for _, f := range pkg.GoFiles {
			return findGoModDir(filepath.Dir(f))
		}
	}
	return ""
}

// findGoModDir walks up from dir looking for go.mod to find the module root.
func findGoModDir(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// isUnderRoot returns true if the file path is under the given root directory.
func isUnderRoot(path, root string) bool {
	if root == "" {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator)) || path == root
}

// loadGitignore loads and compiles a .gitignore file from the given root directory.
// Returns nil if no .gitignore exists or if there's an error reading it.
func loadGitignore(root string) *gitignore.GitIgnore {
	gitignorePath := filepath.Join(root, ".gitignore")
	gi, err := gitignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		return nil
	}
	return gi
}

// scanMarkdownRecursive walks the directory tree from root and collects all markdown files.
func scanMarkdownRecursive(root string, seen map[string]bool, filesByExt map[string][]string) {
	gi := loadGitignore(root)

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Get relative path for gitignore matching
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relPath = path
		}

		// Check gitignore patterns
		if gi != nil {
			// For directories, append slash for proper matching
			matchPath := relPath
			if d.IsDir() {
				matchPath = relPath + "/"
			}
			if gi.MatchesPath(matchPath) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		// Skip vendor directory
		if d.IsDir() && d.Name() == "vendor" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		ext := filepath.Ext(name)
		// Only scan markdown files
		if ext != ".md" {
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		if seen[path] {
			return nil
		}
		seen[path] = true
		filesByExt[ext] = append(filesByExt[ext], path)
		return nil
	})
}
