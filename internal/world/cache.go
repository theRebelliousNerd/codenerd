package world

import (
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
	cache := &FileCache{
		path:    cachePath,
		Entries: make(map[string]CacheEntry),
	}
	cache.load()
	return cache
}

// load reads the cache from disk.
func (c *FileCache) load() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		// Cache doesn't exist or readable, start fresh
		return
	}

	if err := json.Unmarshal(data, &c.Entries); err != nil {
		// Corrupt cache, start fresh
		c.Entries = make(map[string]CacheEntry)
	}
}

// Save writes the cache to disk if dirty.
func (c *FileCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.Dirty {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c.Entries, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return err
	}

	c.Dirty = false
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
