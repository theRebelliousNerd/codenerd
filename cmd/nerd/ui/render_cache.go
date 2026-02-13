// Package ui provides rendering cache for performance optimization.
package ui

import (
	"hash/fnv"
	"math"
	"sync"
)

// RenderCache provides hash-based caching for rendered content.
type RenderCache struct {
	cache   sync.Map
	maxSize int
}

// cacheEntry stores cached render output with metadata.
type cacheEntry struct {
	hash    uint64
	content string
	hits    int
}

// NewRenderCache creates a new render cache with the specified max size.
func NewRenderCache(maxSize int) *RenderCache {
	return &RenderCache{
		cache:   sync.Map{},
		maxSize: maxSize,
	}
}

// DefaultRenderCache is a singleton cache for general UI rendering.
var DefaultRenderCache = NewRenderCache(100)

// computeHash computes a FNV-1a hash for cache keys.
//
// Supported types are intentionally limited to avoid allocations in hot paths.
func computeHash(inputs ...interface{}) uint64 {
	h := fnv.New64a()
	var b [8]byte

	for _, input := range inputs {
		switch v := input.(type) {
		case string:
			h.Write([]byte(v))
		case int:
			u := uint64(v)
			b[0] = byte(u)
			b[1] = byte(u >> 8)
			b[2] = byte(u >> 16)
			b[3] = byte(u >> 24)
			b[4] = byte(u >> 32)
			b[5] = byte(u >> 40)
			b[6] = byte(u >> 48)
			b[7] = byte(u >> 56)
			h.Write(b[:])
		case float64:
			u := math.Float64bits(v)
			b[0] = byte(u)
			b[1] = byte(u >> 8)
			b[2] = byte(u >> 16)
			b[3] = byte(u >> 24)
			b[4] = byte(u >> 32)
			b[5] = byte(u >> 40)
			b[6] = byte(u >> 48)
			b[7] = byte(u >> 56)
			h.Write(b[:])
		case bool:
			if v {
				h.Write([]byte{1})
			} else {
				h.Write([]byte{0})
			}
		}
	}

	return h.Sum64()
}

// Get retrieves cached content if available.
func (rc *RenderCache) Get(key uint64) (string, bool) {
	if val, ok := rc.cache.Load(key); ok {
		entry := val.(*cacheEntry)
		entry.hits++
		return entry.content, true
	}
	return "", false
}

// Set stores rendered content in the cache.
func (rc *RenderCache) Set(key uint64, content string) {
	entry := &cacheEntry{
		hash:    key,
		content: content,
		hits:    1,
	}
	rc.cache.Store(key, entry)
}

// Clear empties the cache.
func (rc *RenderCache) Clear() {
	rc.cache = sync.Map{}
}

// GetOrCompute retrieves from cache or computes if missing.
func (rc *RenderCache) GetOrCompute(key uint64, compute func() string) string {
	if content, ok := rc.Get(key); ok {
		return content
	}

	content := compute()
	rc.Set(key, content)
	return content
}

// ComputeKey generates a cache key from multiple inputs.
func ComputeKey(inputs ...interface{}) uint64 {
	return computeHash(inputs...)
}

// CachedRender wraps a render function with caching.
type CachedRender struct {
	cache      *RenderCache
	lastKey    uint64
	lastResult string
}

// NewCachedRender creates a new cached render wrapper.
func NewCachedRender(cache *RenderCache) *CachedRender {
	if cache == nil {
		cache = DefaultRenderCache
	}
	return &CachedRender{
		cache: cache,
	}
}

// Render executes the render function with caching.
func (cr *CachedRender) Render(keyInputs []interface{}, renderFunc func() string) string {
	key := ComputeKey(keyInputs...)

	// Fast path: same as last render.
	if key == cr.lastKey && cr.lastResult != "" {
		return cr.lastResult
	}

	result := cr.cache.GetOrCompute(key, renderFunc)
	cr.lastKey = key
	cr.lastResult = result
	return result
}

// Invalidate clears the last cached result.
func (cr *CachedRender) Invalidate() {
	cr.lastKey = 0
	cr.lastResult = ""
}
