package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// verdictCacheSchema is the layout of what a cache entry holds. Raise it when
// the content changes shape, so entries written by an older worktree are read
// under another name rather than misread under this one.
const verdictCacheSchema = 1

// verdictCache keeps the extracted model of a commit on disk, so history is
// modelled once and every later run reads it back.
//
// A commit is immutable, and so is the model of one: the same commit read by
// the same go-fsck is the same model, whichever range asked for it. That is
// what makes the entry safe to keep between runs, and what the key is built
// from. The working tree is the one revision that can change under a run, and
// is never cached.
//
// Everything about the cache degrades to doing without it. A cache directory
// that cannot be made, an entry that cannot be written, a revision that cannot
// be resolved: each of them means the model is extracted the slow way, and
// none of them is an error a report stops for.
type verdictCache struct {
	// dir is the directory entries live in, empty when there is no cache to
	// read or write.
	dir string

	// tool identifies the go-fsck binary the entries were written by, so a
	// newer one does not read models the older one wrote.
	tool string

	// refs remembers the commit a revision resolved to, since a chain names
	// the same tag at both ends of two ranges.
	refs map[string]string
}

// openVerdictCache opens the cache below the user cache directory, or returns
// a cache that holds nothing when it is turned off or cannot be opened.
func openVerdictCache(enabled bool) *verdictCache {
	cache := &verdictCache{refs: map[string]string{}}
	if !enabled {
		return cache
	}

	root, err := os.UserCacheDir()
	if err != nil {
		return cache
	}
	dir := filepath.Join(root, "worktree")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return cache
	}

	cache.dir, cache.tool = dir, goFsckIdentity()
	return cache
}

// goFsckIdentity names the extraction tool as its path, size and modification
// time, which is what a reinstall changes. A model is only as stable as the
// tool that wrote it, so an entry written by another one is not read back.
func goFsckIdentity() string {
	path, err := exec.LookPath("go-fsck")
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return path
	}
	return path + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// path returns the file the model of one revision of the module in dir is kept
// under, or an empty string when there is nothing to keep it in.
//
// The key is the material that decides what the model holds: the module, the
// commit the revision resolves to, the tool that reads it and the layout of
// the entry. A tag is resolved first, since a tag can be moved and a commit
// cannot.
func (c *verdictCache) path(dir, ref string) string {
	if c.dir == "" || ref == "" {
		return ""
	}

	commit := c.resolve(dir, ref)
	if commit == "" {
		return ""
	}
	module, err := readModulePath(dir)
	if err != nil {
		return ""
	}

	key := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s", verdictCacheSchema, module, commit, c.tool)))
	return filepath.Join(c.dir, "verdict-"+hex.EncodeToString(key[:])[:16]+".json")
}

// resolve returns the commit a revision names, remembering it for the rest of
// the run. A revision the repository does not carry resolves to nothing, and
// is read without the cache.
func (c *verdictCache) resolve(dir, ref string) string {
	key := dir + "\x00" + ref
	if commit, ok := c.refs[key]; ok {
		return commit
	}

	commit := firstCommandLine(dir, "git", "rev-parse", ref+"^{commit}")
	c.refs[key] = commit
	return commit
}

// load copies a cached model to dest and reports whether there was one.
func (c *verdictCache) load(path, dest string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return copyFile(path, dest) == nil
}

// store keeps a model under path, and says nothing when it cannot: a cache
// that is full, read only or gone is a slower run and not a failed one.
//
// The entry is written beside itself and renamed into place, so a run reading
// the cache while another writes it sees the whole model or none of it.
func (c *verdictCache) store(path, model string) {
	if path == "" {
		return
	}

	temp := path + "." + strconv.Itoa(os.Getpid()) + ".tmp"
	if err := copyFile(model, temp); err != nil {
		_ = os.Remove(temp)
		return
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
	}
}

// copyFile writes the contents of src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
