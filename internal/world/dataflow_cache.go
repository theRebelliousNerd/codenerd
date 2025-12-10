// Package world provides data flow caching with hash-based invalidation.
package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// cacheVersion is incremented when the cache format changes.
// Old cache entries with different versions are invalidated on load.
const cacheVersion = 1

// DataFlowCache manages cached data flow facts with hash-based invalidation.
// It persists per-file data flow analysis results to disk, avoiding expensive
// re-analysis of unchanged files on startup.
type DataFlowCache struct {
	cacheDir string
	mu       sync.RWMutex
	entries  map[string]*CacheDataFlowEntry // file path -> cached entry
	dirty    map[string]bool                // files needing persistence

	// Statistics (atomic for lock-free reads)
	hits   atomic.Int64
	misses atomic.Int64
}

// CacheDataFlowEntry represents cached data flow facts for a single file.
type CacheDataFlowEntry struct {
	FilePath  string             `json:"file_path"`
	FileHash  string             `json:"file_hash"` // SHA256 of file content
	Facts     []SerializedFact   `json:"facts"`     // Cached data flow facts (JSON-safe)
	Timestamp time.Time          `json:"timestamp"`
	Version   int                `json:"version"` // Cache format version
}

// SerializedFact is a JSON-serializable representation of core.Fact.
// The Args field in core.Fact contains interface{} values that may not
// round-trip correctly through JSON (e.g., int64 becomes float64).
// This struct preserves type information explicitly.
type SerializedFact struct {
	Predicate string           `json:"predicate"`
	Args      []SerializedArg  `json:"args"`
}

// SerializedArg preserves the type of each argument through JSON serialization.
type SerializedArg struct {
	Type  string      `json:"type"`  // "string", "int64", "float64", "bool", "atom"
	Value interface{} `json:"value"` // The actual value
}

// NewDataFlowCache creates a new cache with the specified directory.
// If the directory does not exist, it will be created.
// Existing cache entries are loaded from disk during initialization.
func NewDataFlowCache(cacheDir string) (*DataFlowCache, error) {
	if cacheDir == "" {
		return nil, fmt.Errorf("cache directory path required")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	cache := &DataFlowCache{
		cacheDir: cacheDir,
		entries:  make(map[string]*CacheDataFlowEntry),
		dirty:    make(map[string]bool),
	}

	// Load existing cache entries from disk
	cache.loadFromDisk()

	return cache, nil
}

// GetOrCompute returns cached facts if the file content hash matches,
// otherwise computes new facts using the provided function and caches them.
func (c *DataFlowCache) GetOrCompute(file string, content []byte, compute func() []core.Fact) []core.Fact {
	hash := c.computeHash(content)

	// Fast path: check cache with read lock
	c.mu.RLock()
	entry, ok := c.entries[file]
	if ok && entry.FileHash == hash && entry.Version == cacheVersion {
		c.mu.RUnlock()
		c.hits.Add(1)
		logging.WorldDebug("DataFlowCache HIT: %s (hash: %s)", filepath.Base(file), hash[:8])
		return c.deserializeFacts(entry.Facts)
	}
	c.mu.RUnlock()

	// Cache miss - compute facts
	c.misses.Add(1)
	logging.WorldDebug("DataFlowCache MISS: %s (computing...)", filepath.Base(file))

	facts := compute()

	// Store in cache with write lock
	c.mu.Lock()
	c.entries[file] = &CacheDataFlowEntry{
		FilePath:  file,
		FileHash:  hash,
		Facts:     c.serializeFacts(facts),
		Timestamp: time.Now(),
		Version:   cacheVersion,
	}
	c.dirty[file] = true
	c.mu.Unlock()

	logging.WorldDebug("DataFlowCache STORED: %s (%d facts)", filepath.Base(file), len(facts))
	return facts
}

// computeHash returns the SHA256 hash of content as a hex string.
func (c *DataFlowCache) computeHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// serializeFacts converts core.Fact slice to JSON-serializable form.
func (c *DataFlowCache) serializeFacts(facts []core.Fact) []SerializedFact {
	result := make([]SerializedFact, 0, len(facts))
	for _, f := range facts {
		sf := SerializedFact{
			Predicate: f.Predicate,
			Args:      make([]SerializedArg, 0, len(f.Args)),
		}
		for _, arg := range f.Args {
			sf.Args = append(sf.Args, c.serializeArg(arg))
		}
		result = append(result, sf)
	}
	return result
}

// serializeArg converts a single argument to serializable form with type info.
func (c *DataFlowCache) serializeArg(arg interface{}) SerializedArg {
	switch v := arg.(type) {
	case core.MangleAtom:
		return SerializedArg{Type: "atom", Value: string(v)}
	case string:
		return SerializedArg{Type: "string", Value: v}
	case int:
		return SerializedArg{Type: "int64", Value: int64(v)}
	case int64:
		return SerializedArg{Type: "int64", Value: v}
	case float64:
		return SerializedArg{Type: "float64", Value: v}
	case bool:
		return SerializedArg{Type: "bool", Value: v}
	default:
		// Fallback: convert to string representation
		return SerializedArg{Type: "string", Value: fmt.Sprintf("%v", v)}
	}
}

// deserializeFacts converts serialized facts back to core.Fact slice.
func (c *DataFlowCache) deserializeFacts(serialized []SerializedFact) []core.Fact {
	result := make([]core.Fact, 0, len(serialized))
	for _, sf := range serialized {
		f := core.Fact{
			Predicate: sf.Predicate,
			Args:      make([]interface{}, 0, len(sf.Args)),
		}
		for _, arg := range sf.Args {
			f.Args = append(f.Args, c.deserializeArg(arg))
		}
		result = append(result, f)
	}
	return result
}

// deserializeArg converts a serialized argument back to its original type.
func (c *DataFlowCache) deserializeArg(arg SerializedArg) interface{} {
	switch arg.Type {
	case "atom":
		if s, ok := arg.Value.(string); ok {
			return core.MangleAtom(s)
		}
		return core.MangleAtom(fmt.Sprintf("%v", arg.Value))
	case "string":
		if s, ok := arg.Value.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", arg.Value)
	case "int64":
		// JSON unmarshals numbers as float64
		switch v := arg.Value.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		default:
			return int64(0)
		}
	case "float64":
		if f, ok := arg.Value.(float64); ok {
			return f
		}
		return float64(0)
	case "bool":
		if b, ok := arg.Value.(bool); ok {
			return b
		}
		return false
	default:
		return arg.Value
	}
}

// Invalidate removes a file from the cache (both memory and disk).
func (c *DataFlowCache) Invalidate(file string) {
	c.mu.Lock()
	delete(c.entries, file)
	delete(c.dirty, file)
	c.mu.Unlock()

	// Remove from disk
	cacheFile := c.cacheFilePath(file)
	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		logging.Get(logging.CategoryWorld).Warn("DataFlowCache: failed to remove cache file %s: %v",
			cacheFile, err)
	}

	logging.WorldDebug("DataFlowCache INVALIDATED: %s", filepath.Base(file))
}

// InvalidateAll clears the entire cache (both memory and disk).
func (c *DataFlowCache) InvalidateAll() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheDataFlowEntry)
	c.dirty = make(map[string]bool)
	c.hits.Store(0)
	c.misses.Store(0)
	c.mu.Unlock()

	// Clear cache directory
	if err := os.RemoveAll(c.cacheDir); err != nil {
		logging.Get(logging.CategoryWorld).Warn("DataFlowCache: failed to remove cache directory: %v", err)
	}
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		logging.Get(logging.CategoryWorld).Error("DataFlowCache: failed to recreate cache directory: %v", err)
	}

	logging.World("DataFlowCache: invalidated all entries")
}

// Persist writes all dirty entries to disk.
// Call this periodically or before shutdown to ensure cache durability.
func (c *DataFlowCache) Persist() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.dirty) == 0 {
		logging.WorldDebug("DataFlowCache: no dirty entries to persist")
		return nil
	}

	persistCount := 0
	var lastErr error

	for file := range c.dirty {
		entry, ok := c.entries[file]
		if !ok {
			continue
		}
		if err := c.persistEntry(entry); err != nil {
			logging.Get(logging.CategoryWorld).Error("DataFlowCache: failed to persist %s: %v",
				filepath.Base(file), err)
			lastErr = err
			continue
		}
		delete(c.dirty, file)
		persistCount++
	}

	if persistCount > 0 {
		logging.World("DataFlowCache: persisted %d entries to disk", persistCount)
	}

	return lastErr
}

// persistEntry writes a single cache entry to disk.
// Caller must hold the write lock.
func (c *DataFlowCache) persistEntry(entry *CacheDataFlowEntry) error {
	cacheFile := c.cacheFilePath(entry.FilePath)

	// Ensure parent directory exists
	dir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// cacheFilePath returns the cache file path for a source file.
// Uses SHA256 of the source path to avoid filesystem issues with special characters.
func (c *DataFlowCache) cacheFilePath(sourceFile string) string {
	// Use hash of path to avoid filesystem issues with long paths or special chars
	hash := sha256.Sum256([]byte(sourceFile))
	return filepath.Join(c.cacheDir, hex.EncodeToString(hash[:8])+".json")
}

// loadFromDisk loads all cache entries from disk.
func (c *DataFlowCache) loadFromDisk() {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Get(logging.CategoryWorld).Warn("DataFlowCache: failed to read cache directory: %v", err)
		}
		return
	}

	loadedCount := 0
	skippedCount := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		cacheEntry, err := c.loadEntry(filepath.Join(c.cacheDir, entry.Name()))
		if err != nil {
			logging.WorldDebug("DataFlowCache: skipping invalid entry %s: %v", entry.Name(), err)
			skippedCount++
			continue
		}

		// Skip old cache versions
		if cacheEntry.Version != cacheVersion {
			logging.WorldDebug("DataFlowCache: skipping outdated entry %s (version %d, expected %d)",
				entry.Name(), cacheEntry.Version, cacheVersion)
			skippedCount++
			continue
		}

		c.entries[cacheEntry.FilePath] = cacheEntry
		loadedCount++
	}

	if loadedCount > 0 || skippedCount > 0 {
		logging.World("DataFlowCache: loaded %d entries from disk (%d skipped)", loadedCount, skippedCount)
	}
}

// loadEntry loads a single cache entry from disk.
func (c *DataFlowCache) loadEntry(path string) (*CacheDataFlowEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheDataFlowEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	return &entry, nil
}

// Stats returns cache statistics.
func (c *DataFlowCache) Stats() CacheStats {
	c.mu.RLock()
	entryCount := len(c.entries)
	dirtyCount := len(c.dirty)
	c.mu.RUnlock()

	return CacheStats{
		Hits:       c.hits.Load(),
		Misses:     c.misses.Load(),
		Entries:    entryCount,
		Dirty:      dirtyCount,
	}
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

// Lookup checks if a file is in the cache without computing.
// Returns the cached facts and true if found, nil and false otherwise.
func (c *DataFlowCache) Lookup(file string, content []byte) ([]core.Fact, bool) {
	hash := c.computeHash(content)

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[file]
	if !ok || entry.FileHash != hash || entry.Version != cacheVersion {
		return nil, false
	}

	return c.deserializeFacts(entry.Facts), true
}

// Store adds facts to the cache without computing.
// Useful when facts are computed elsewhere and need to be cached.
func (c *DataFlowCache) Store(file string, content []byte, facts []core.Fact) {
	hash := c.computeHash(content)

	c.mu.Lock()
	c.entries[file] = &CacheDataFlowEntry{
		FilePath:  file,
		FileHash:  hash,
		Facts:     c.serializeFacts(facts),
		Timestamp: time.Now(),
		Version:   cacheVersion,
	}
	c.dirty[file] = true
	c.mu.Unlock()

	logging.WorldDebug("DataFlowCache STORED (direct): %s (%d facts)", filepath.Base(file), len(facts))
}

// Size returns the number of cached entries.
func (c *DataFlowCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// CacheDir returns the cache directory path.
func (c *DataFlowCache) CacheDir() string {
	return c.cacheDir
}
