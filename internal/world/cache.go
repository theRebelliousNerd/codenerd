package world

import (
	"codenerd/internal/logging"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// CacheEntry represents cached metadata for a single file.
type CacheEntry struct {
	Hash    string `json:"hash"`
	ModTime int64  `json:"mod_time"`
	Size    int64  `json:"size"`
}

// FileCache manages file metadata caching to avoid re-hashing unchanged files.
type FileCache struct {
	mu      sync.RWMutex
	path    string
	Entries map[string]CacheEntry `json:"entries"`
	Dirty   bool                  `json:"-"`
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
func (c *FileCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.Dirty {
		logging.WorldDebug("FileCache: no changes to save")
		return nil
	}

	logging.WorldDebug("FileCache: saving %d entries to disk", len(c.Entries))

	// Ensure directory exists
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

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		logging.Get(logging.CategoryWorld).Error("FileCache: failed to write cache file: %v", err)
		return err
	}

	c.Dirty = false
	logging.World("FileCache saved: %d entries", len(c.Entries))
	return nil
}

// Get returns the hash if the file hasn't changed.
func (c *FileCache) Get(path string, info os.FileInfo) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Entries[path]
	if !ok {
		return "", false
	}

	// Check if file matches cache
	if entry.ModTime == info.ModTime().Unix() && entry.Size == info.Size() {
		return entry.Hash, true
	}

	return "", false
}

// Update updates the cache with a new hash.
func (c *FileCache) Update(path string, info os.FileInfo, hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Entries[path] = CacheEntry{
		Hash:    hash,
		ModTime: info.ModTime().Unix(),
		Size:    info.Size(),
	}
	c.Dirty = true
}
