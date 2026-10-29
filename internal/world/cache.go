package world

import (
	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// CacheEntry represents cached metadata for a single file.
type CacheEntry struct {
	Hash    string `json:"hash"`
	ModTime int64  `json:"mod_time"`
	Size    int64  `json:"size"`
	// Gen is the content generation observed with Hash (NTFS ChangeTime,
	// inode ctime elsewhere). GenOK false is a legacy entry, or a generation
	// that could not be read: the next lookup hashes instead of trusting
	// size and mtime.
	Gen        int64 `json:"gen,omitempty"`
	GenOK      bool  `json:"gen_ok,omitempty"`
	GenIsClock bool  `json:"gen_is_clock,omitempty"`
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

	// gens is this object's directory-query snapshot. It is not saved. The
	// next scan constructs a new FileCache and reads generations again; a
	// snapshot kept across a later rewrite would miss a restored mtime.
	gens dirGenSnapshot
}

// cacheOwnerDir resolves the workspace that owns the file-cache manifest for a
// scan rooted at scanRoot. It walks up from scanRoot (inclusive) to the nearest
// ancestor containing a .nerd/config.json file, bounded by the filesystem root.
// If no such ancestor exists, it returns scanRoot unchanged, preserving the
// previous behaviour for tests and non-workspace scans. A stray .nerd/cache
// directory without a config.json never captures the cache.
func cacheOwnerDir(scanRoot string) string {
	abs, err := filepath.Abs(scanRoot)
	if err != nil {
		return scanRoot
	}
	dir := filepath.Clean(abs)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".nerd", "config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return scanRoot
		}
		dir = parent
	}
}

// NewFileCache creates or loads a file cache.
//
// scanRoot is the directory being scanned, which may be a subdirectory of the
// workspace rather than the workspace root itself. The manifest lives under the
// owning workspace resolved by cacheOwnerDir. Cache keys are already absolute
// paths and Save never prunes entries, so a subtree scan sharing the workspace
// manifest cannot evict other entries.
func NewFileCache(scanRoot string) *FileCache {
	owner := cacheOwnerDir(scanRoot)
	cachePath := filepath.Join(owner, ".nerd", "cache", "manifest.json")
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

// Get returns the stored hash when it still names the file's bytes.
//
// A hit is the contentStamp pre-check: size, mtime, and content generation
// unchanged and older than symbolStampQuantum, so the file is not read. A
// miss hashes. The hash is still returned when it equals the stored one — a
// touch that did not change bytes — and the stamp is refreshed so the next
// lookup can hit. A same-size rewrite whose mtime was restored does not hit:
// the generation moved, the hash differs, and Get returns false.
//
// Hits count pre-check trusts only. A hash that happened to match is a miss
// in the effectiveness counters; the scan did read the file.
func (c *FileCache) Get(path string, info os.FileInfo) (string, bool) {
	c.mu.RLock()
	entry, ok := c.Entries[path]
	c.mu.RUnlock()
	if !ok {
		c.misses.Add(1)
		return "", false
	}

	gen, genOK, isClock := c.observeGen(path, info)
	cur := contentStamp{
		size:       info.Size(),
		mtime:      info.ModTime().UnixNano(),
		gen:        gen,
		genOK:      genOK,
		genIsClock: isClock,
	}
	stored := stampFromEntry(entry)
	now := time.Now().UnixNano()
	if stored.trusts(cur, now) {
		c.hits.Add(1)
		return entry.Hash, true
	}

	hash, err := calculateHash(path)
	if err != nil || hash != entry.Hash {
		c.misses.Add(1)
		return "", false
	}
	c.misses.Add(1)
	c.storeStamp(path, info, hash, gen, genOK, isClock)
	return hash, true
}

// Update updates the cache with a new hash and the generation observed now.
func (c *FileCache) Update(path string, info os.FileInfo, hash string) {
	gen, genOK, isClock := c.observeGen(path, info)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Entries[path] = CacheEntry{
		Hash:       hash,
		ModTime:    info.ModTime().UnixNano(),
		Size:       info.Size(),
		Gen:        gen,
		GenOK:      genOK,
		GenIsClock: isClock,
	}
	c.Dirty = true
}

func (c *FileCache) observeGen(path string, info os.FileInfo) (int64, bool, bool) {
	return contentGeneration(path, info, c.gens.of(filepath.Dir(path)))
}

// storeStamp records hash as the bytes at info's stamp. A concurrent Update
// that stored a different hash wins; refreshing metadata must not clobber a
// newer content identity. An unchanged entry is left clean so a young file,
// re-read because its timestamp is still inside the tick, does not rewrite
// the manifest on every scan.
func (c *FileCache) storeStamp(path string, info os.FileInfo, hash string, gen int64, genOK, genIsClock bool) {
	next := CacheEntry{
		Hash:       hash,
		ModTime:    info.ModTime().UnixNano(),
		Size:       info.Size(),
		Gen:        gen,
		GenOK:      genOK,
		GenIsClock: genIsClock,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.Entries[path]; ok {
		if prev.Hash != hash || prev == next {
			return
		}
	}
	c.Entries[path] = next
	c.Dirty = true
}

// Stats reports lookup effectiveness.
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

// CacheStats contains cache performance statistics.
type CacheStats struct {
	Hits    int64 // Number of cache hits
	Misses  int64 // Number of cache misses
	Entries int   // Number of cached entries
	Dirty   int   // Number of entries pending persistence
}

// HitRate returns the cache hit rate as a percentage (0-100).
// Returns 0 if no lookups have been performed.
func (s CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// String returns a human-readable summary of cache statistics.
func (s CacheStats) String() string {
	return fmt.Sprintf("hits=%d misses=%d entries=%d dirty=%d hitRate=%.1f%%",
		s.Hits, s.Misses, s.Entries, s.Dirty, s.HitRate())
}
