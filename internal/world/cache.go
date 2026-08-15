package world

import (
	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// CacheEntry represents cached metadata for a single file.
type CacheEntry struct {
	Hash    string `json:"hash"`
	ModTime int64  `json:"mod_time"`
	Size    int64  `json:"size"`
}

// FileCache manages file metadata caching to avoid re-hashing unchanged files.
//
// Keys are absolute filesystem paths, deliberately unlike the facts, whose
// identities are workspace-relative. This file is a machine-local artifact
// under .nerd/cache and never travels; keying it by the path the walker
// actually visits keeps lookup allocation-free on the hot path. Moving the
// checkout invalidates the whole cache, which costs one rehash pass and is
// self-healing.
type FileCache struct {
	mu      sync.RWMutex
	path    string
	Entries map[string]CacheEntry `json:"entries"`
	Dirty   bool                  `json:"-"`

	// Hit/miss counters. The data-flow cache has reported its hit rate for a
	// while; the file cache — the one that decides whether the scanner rehashes
	// every file in the repo — reported nothing, so a cache that had silently
	// stopped working (a mtime granularity change, a key format change, a
	// cache file that never saved) was invisible.
	hits   atomic.Int64
	misses atomic.Int64
}

// NewFileCache creates or loads a file cache.
func NewFileCache(workspaceRoot string) *FileCache {
	cachePath := filepath.Join(workspaceRoot, ".nerd", "cache", "manifest.json")
	logging.WorldDebug("Creating FileCache at: %s", cachePath)
	cache := &FileCache{
		path:    cachePath,
		Entries: make(map[string]CacheEntry),
	}
	cache.load()
	logging.WorldDebug("FileCache loaded with %d entries", len(cache.Entries))
	return cache
}

// load reads the cache from disk.
func (c *FileCache) load() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			logging.WorldDebug("FileCache: no existing cache file, starting fresh")
		} else {
			logging.Get(logging.CategoryWorld).Warn("FileCache: failed to read cache: %v", err)
		}
		return
	}

	if err := json.Unmarshal(data, &c.Entries); err != nil {
		logging.Get(logging.CategoryWorld).Warn("FileCache: corrupt cache, starting fresh: %v", err)
		c.Entries = make(map[string]CacheEntry)
	}
}

// Save writes the cache to disk if dirty.
//
// The write is atomic: unique temp file in the destination directory, fsync,
// rename. A plain os.WriteFile truncates the existing manifest before writing
// the new bytes, so a crash, a full disk, or two scans racing left a truncated
// or interleaved manifest — the only copy of the hash cache — and the next scan
// rehashed the entire repository (or, worse, read a half-written entry as
// truth).
func (c *FileCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.Dirty {
		logging.WorldDebug("FileCache: no changes to save")
		return nil
	}

	logging.WorldDebug("FileCache: saving %d entries to disk", len(c.Entries))

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryWorld).Error("FileCache: failed to create cache directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(c.Entries, "", "  ")
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("FileCache: failed to marshal cache: %v", err)
		return err
	}

	if err := writeFileAtomic(c.path, data, 0644); err != nil {
		logging.Get(logging.CategoryWorld).Error("FileCache: failed to write cache file: %v", err)
		return err
	}

	c.Dirty = false
	logging.World("FileCache saved: %d entries (%s)", len(c.Entries), c.statsLocked())
	return nil
}

// writeFileAtomic writes data to path via a unique temp file + fsync + rename,
// so a reader never observes a partial file and a failed write never destroys
// the previous contents.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	// Unique name: two scanners saving concurrently must not write the same
	// temp file and rename each other's partial bytes into place.
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		// Best effort: on the success path the file is already renamed away.
		_ = os.Remove(tmp)
	}()

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	// fsync before rename: rename is atomic in the directory entry, but without
	// the sync the renamed inode can still be empty after a power loss.
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return atomicfile.Replace(tmp, path)
}

// Get returns the hash if the file hasn't changed.
//
// ModTime is compared at nanosecond resolution. Earlier this used Unix()
// (second resolution) — fast write-write cycles (formatter rewrites on
// IDE save, go-generate loops, test fixtures regenerated in-place)
// landed within the same second and bypassed invalidation, returning
// stale hashes whose facts then drifted from disk content. UnixNano
// gives 1-ns resolution which matches the OS stat granularity on all
// supported platforms.
func (c *FileCache) Get(path string, info os.FileInfo) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Entries[path]
	if !ok {
		c.misses.Add(1)
		return "", false
	}

	// Check if file matches cache
	if entry.ModTime == info.ModTime().UnixNano() && entry.Size == info.Size() {
		c.hits.Add(1)
		return entry.Hash, true
	}

	c.misses.Add(1)
	return "", false
}

// Update updates the cache with a new hash.
func (c *FileCache) Update(path string, info os.FileInfo, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Entries[path] = CacheEntry{
		Hash:    hash,
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
	}
	c.Dirty = true
}

// Stats reports lookup effectiveness, in the same shape the data-flow cache
// reports, so both caches can be logged and compared.
func (c *FileCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.statsLocked()
}

func (c *FileCache) statsLocked() CacheStats {
	dirty := 0
	if c.Dirty {
		dirty = len(c.Entries)
	}
	return CacheStats{
		Hits:    c.hits.Load(),
		Misses:  c.misses.Load(),
		Entries: len(c.Entries),
		Dirty:   dirty,
	}
}

// LogStats emits the cache effectiveness line for a completed scan.
func (c *FileCache) LogStats(scope string) {
	s := c.Stats()
	logging.World("FileCache[%s]: %s", scope, s)
}
