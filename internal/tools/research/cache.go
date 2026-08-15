package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// CacheEntry holds a cached research result.
type CacheEntry struct {
	Key       string
	Value     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Source    string // e.g., "context7", "web_fetch", "browser"
}

// ResearchCache provides in-memory caching for research results.
// This reduces redundant API calls and speeds up repeated queries.
type ResearchCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	maxSize int
	ttl     time.Duration

	// disk is the optional durable tier. Zero value = memory only, which is
	// the behavior when no workspace has been configured. See cache_disk.go.
	disk diskStore
}

// DefaultCache is the shared research cache instance.
var (
	defaultCache     *ResearchCache
	defaultCacheOnce sync.Once
)

// getDefaultCache returns the shared cache instance.
func getDefaultCache() *ResearchCache {
	defaultCacheOnce.Do(func() {
		defaultCache = NewResearchCache(1000, 30*time.Minute)
	})
	return defaultCache
}

// NewResearchCache creates a new cache with the given size limit and TTL.
func NewResearchCache(maxSize int, ttl time.Duration) *ResearchCache {
	return &ResearchCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached entry by key, falling back to the durable tier when
// the in-memory one misses (a fresh process starts with an empty map but a
// populated .nerd/cache/research).
func (c *ResearchCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if ok {
		if time.Now().After(entry.ExpiresAt) {
			// Expired in memory means expired everywhere; drop both copies so
			// the next Get does not resurrect it from disk.
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
			c.disk.remove(key)
			return nil, false
		}
		return entry, true
	}

	fromDisk, ok := c.disk.read(key)
	if !ok {
		return nil, false
	}
	c.mu.Lock()
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[key] = fromDisk
	c.mu.Unlock()
	return fromDisk, true
}

// Set stores a value in the cache.
func (c *ResearchCache) Set(key, value, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	now := time.Now()
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		ExpiresAt: now.Add(c.ttl),
		Source:    source,
	}
	c.entries[key] = entry
	c.disk.write(entry)
}

// Delete removes an entry from both tiers.
func (c *ResearchCache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
	c.disk.remove(key)
}

// Clear removes all entries from both tiers. Clearing memory alone would be a
// lie: the next Get would restore the entry from disk.
func (c *ResearchCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.mu.Unlock()
	c.disk.clear()
}

// Size returns the number of entries in the cache.
func (c *ResearchCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the oldest entry (by creation time).
func (c *ResearchCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// hashKey creates a cache key from arbitrary inputs.
func hashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CacheGetTool returns a tool for retrieving cached research results.
func CacheGetTool() *tools.Tool {
	return &tools.Tool{
		Name:        "research_cache_get",
		Description: "Retrieve a cached research result by key",
		Category:    tools.CategoryResearch,
		Priority:    90, // High priority - check cache before live fetch
		Execute:     executeCacheGet,
		Schema: tools.ToolSchema{
			Required: []string{"key"},
			Properties: map[string]tools.Property{
				"key": {
					Type:        "string",
					Description: "The cache key (typically a URL or topic identifier)",
				},
			},
		},
	}
}

func executeCacheGet(ctx context.Context, args map[string]any) (string, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	cache := getDefaultCache()
	entry, ok := cache.Get(key)
	if !ok {
		logging.ResearcherDebug("Cache miss: %s", key)
		return "", fmt.Errorf("cache miss: %s", key)
	}

	logging.Researcher("Cache hit: %s (source=%s, age=%v)", key, entry.Source, time.Since(entry.CreatedAt))
	return entry.Value, nil
}

// CacheSetTool returns a tool for storing research results in cache.
func CacheSetTool() *tools.Tool {
	return &tools.Tool{
		Name:        "research_cache_set",
		Description: "Store a research result in the cache",
		Category:    tools.CategoryResearch,
		Priority:    40, // Lower priority - happens after research
		Execute:     executeCacheSet,
		Schema: tools.ToolSchema{
			Required: []string{"key", "value"},
			Properties: map[string]tools.Property{
				"key": {
					Type:        "string",
					Description: "The cache key (typically a URL or topic identifier)",
				},
				"value": {
					Type:        "string",
					Description: "The content to cache",
				},
				"source": {
					Type:        "string",
					Description: "The source of this content (e.g., context7, web_fetch)",
					Default:     "unknown",
				},
			},
		},
	}
}

func executeCacheSet(ctx context.Context, args map[string]any) (string, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	value, _ := args["value"].(string)
	if value == "" {
		return "", fmt.Errorf("value is required")
	}

	source := "unknown"
	if s, ok := args["source"].(string); ok && s != "" {
		source = s
	}

	cache := getDefaultCache()
	cache.Set(key, value, source)

	logging.Researcher("Cached: %s (source=%s, size=%d bytes)", key, source, len(value))
	return fmt.Sprintf("Cached %d bytes for key: %s", len(value), key), nil
}

// CacheClearTool returns a tool for clearing the research cache.
func CacheClearTool() *tools.Tool {
	return &tools.Tool{
		Name:        "research_cache_clear",
		Description: "Clear all entries from the research cache",
		Category:    tools.CategoryResearch,
		Priority:    30,
		Execute:     executeCacheClear,
		Schema: tools.ToolSchema{
			Required:   []string{},
			Properties: map[string]tools.Property{},
		},
	}
}

func executeCacheClear(ctx context.Context, args map[string]any) (string, error) {
	cache := getDefaultCache()
	size := cache.Size()
	cache.Clear()

	logging.Researcher("Cache cleared: %d entries removed", size)
	return fmt.Sprintf("Cleared %d cache entries", size), nil
}

// CacheStatsTool returns a tool for getting cache statistics.
func CacheStatsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "research_cache_stats",
		Description: "Get statistics about the research cache",
		Category:    tools.CategoryResearch,
		Priority:    30,
		Execute:     executeCacheStats,
		Schema: tools.ToolSchema{
			Required:   []string{},
			Properties: map[string]tools.Property{},
		},
	}
}

func executeCacheStats(ctx context.Context, args map[string]any) (string, error) {
	cache := getDefaultCache()

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	// Count by source
	sources := make(map[string]int)
	var totalSize int
	var validCount int

	now := time.Now()
	for _, entry := range cache.entries {
		if now.Before(entry.ExpiresAt) {
			validCount++
			sources[entry.Source]++
			totalSize += len(entry.Value)
		}
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Cache Statistics:\n"))
	result.WriteString(fmt.Sprintf("  Total entries: %d\n", len(cache.entries)))
	result.WriteString(fmt.Sprintf("  Valid entries: %d\n", validCount))
	result.WriteString(fmt.Sprintf("  Total size: %d bytes\n", totalSize))
	result.WriteString(fmt.Sprintf("  Max size: %d entries\n", cache.maxSize))
	result.WriteString(fmt.Sprintf("  TTL: %v\n", cache.ttl))
	if dir := cache.disk.DiskDir(); dir != "" {
		result.WriteString(fmt.Sprintf("  Persistent store: %s\n", dir))
	} else {
		result.WriteString("  Persistent store: disabled (in-memory only)\n")
	}
	result.WriteString(fmt.Sprintf("\nBy source:\n"))
	for source, count := range sources {
		result.WriteString(fmt.Sprintf("  %s: %d\n", source, count))
	}

	return result.String(), nil
}
