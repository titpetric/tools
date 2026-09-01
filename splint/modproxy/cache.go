package modproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// CacheName is the file the sizes are kept in, under the user's cache
// directory: "~/.cache/splint/sizes.yml" on a machine that sets nothing.
const CacheName = "splint/sizes.yml"

// Cache is what module versions weigh, kept on disk between runs.
//
// A version is immutable, so its size is a fact that does not expire and is
// worth keeping: a tree of forty dependencies is forty requests the next run
// does not make. Nothing else the proxy answers is cached. A published date is
// immutable too and is read once per report; the latest version is not
// immutable at all.
//
// A module carries the size of every version asked about and one size for the
// module itself, which is what a version the file does not hold is answered
// with. Modules do not change size much from one release to the next, and an
// answer within a few percent is worth more than a round trip.
type Cache struct {
	// Path is the file read and written. A cache with no path answers from
	// memory and writes nothing.
	Path string

	mu      sync.Mutex
	modules map[string]*cachedModule
	dirty   bool
}

// cachedModule is what is known about one module.
type cachedModule struct {
	// Size is the size of a version the file does not hold, which is the mean
	// of the ones it does.
	Size int64 `yaml:"size"`

	// Versions is the size of each version that has been asked about, keyed by
	// the version string.
	Versions map[string]int64 `yaml:"versions,omitempty"`
}

// DefaultCachePath is where the sizes are kept, and is empty when the machine
// has no cache directory to put them in.
func DefaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, CacheName)
}

// OpenCache reads the cache at a path, and returns an empty one for a file
// that is not there or cannot be read.
//
// A cache is an optimisation and never a failure: a file written by a newer
// version, a half written file, a file nobody can read, all of them answer
// nothing and are written over on the next save.
func OpenCache(path string) *Cache {
	cache := &Cache{Path: path, modules: map[string]*cachedModule{}}

	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}

	var modules map[string]*cachedModule
	if err := yaml.Unmarshal(data, &modules); err != nil {
		return cache
	}

	for path, module := range modules {
		if module != nil {
			cache.modules[path] = module
		}
	}

	return cache
}

// Size is what a version weighs, and whether the cache knows.
//
// A version the file holds is answered exactly. One it does not is answered
// with the size of the module, which is the mean of the versions it does hold.
func (c *Cache) Size(path, version string) (int64, bool) {
	if c == nil {
		return 0, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	module, known := c.modules[path]
	if !known {
		return 0, false
	}

	if size, exact := module.Versions[version]; exact && size > 0 {
		return size, true
	}
	if module.Size > 0 {
		return module.Size, true
	}
	return 0, false
}

// Add records what a version weighs. A size of nothing is what the proxy
// answers when it could not be reached, and is not recorded.
func (c *Cache) Add(path, version string, size int64) {
	if c == nil || size <= 0 || path == "" || version == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	module, known := c.modules[path]
	if !known {
		module = &cachedModule{Versions: map[string]int64{}}
		c.modules[path] = module
	}
	if module.Versions == nil {
		module.Versions = map[string]int64{}
	}

	if module.Versions[version] == size {
		return
	}

	module.Versions[version] = size
	module.Size = mean(module.Versions)
	c.dirty = true
}

// Save writes the cache, and writes nothing when nothing was learned.
//
// The file is written beside itself and renamed over the old one, so a reader
// sees one file or the other and never half of a write. What was written is
// read back before the rename: a file that does not parse is a file the next
// run would throw away, and throwing it away here costs nobody a round trip.
func (c *Cache) Save() error {
	if c == nil || c.Path == "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	data, err := yaml.Marshal(c.modules)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		return err
	}

	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	if err := validate(tmp, len(c.modules)); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, c.Path); err != nil {
		os.Remove(tmp)
		return err
	}

	c.dirty = false
	return nil
}

// validate reads a written cache back and checks it says what it was written
// with.
func validate(path string, modules int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var read map[string]*cachedModule
	if err := yaml.Unmarshal(data, &read); err != nil {
		return fmt.Errorf("the cache written to %s does not parse: %w", path, err)
	}
	if len(read) != modules {
		return fmt.Errorf("the cache written to %s holds %d modules, not %d", path, len(read), modules)
	}

	return nil
}

// mean is the average size of the versions of one module, which is what a
// version nobody asked about is answered with.
func mean(versions map[string]int64) int64 {
	var total, count int64

	for _, size := range versions {
		if size <= 0 {
			continue
		}
		total += size
		count++
	}

	if count == 0 {
		return 0
	}
	return total / count
}

// Modules returns the module paths the cache holds, in order. It is what a
// test reads and what a reader of the file sees.
func (c *Cache) Modules() []string {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]string, 0, len(c.modules))
	for path := range c.modules {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
