package retrieval

import (
	"container/list"
	"sync"
	"time"
)

// =============================================================================
// KEYWORD RESULT CACHE
// =============================================================================
//
// A cached keyword result is a claim about a search domain -- every member
// file the scan could visit -- not just about the files it named. An empty
// result says no member holds the keyword; a one-file result says no other
// member does. So the cache is keyed by its domain (Begin), every change to
// the domain's content moves its epoch, and a scan publishes only into the
// epoch it started in (Set): an invalidation that lands while a scan is in
// flight is never undone by that scan's late, pre-change result.

// keywordResult is one keyword's complete result over the domain.
type keywordResult struct {
	hits []KeywordHit
	// overflow is true when the scan stopped at maxHitsPerKeyword: the hits
	// are true, but other members may hold the keyword too.
	overflow bool
}

// KeywordHitCache caches keyword search results for one search domain.
type KeywordHitCache struct {
	entries map[string]*cacheEntry
	list    *list.List
	mu      sync.Mutex
	maxSize int
	ttl     time.Duration

	// domain identifies the member set and scan semantics every entry was
	// computed over; another domain empties the cache (Begin).
	domain string
	// epoch counts the domain changes the cache has absorbed. A result
	// stamped with an older epoch was read before one of them.
	epoch uint64
}

type cacheEntry struct {
	key       string
	result    keywordResult
	timestamp time.Time
	element   *list.Element
}

// NewKeywordHitCache creates a new cache.
func NewKeywordHitCache(maxSize int, ttl time.Duration) *KeywordHitCache {
	return &KeywordHitCache{
		entries: make(map[string]*cacheEntry),
		list:    list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Begin opens a lookup against domain and returns the epoch a scan started
// now must publish into. A domain other than the cached one (a checkout, an
// ignore edit, another backend) empties the cache first.
func (c *KeywordHitCache) Begin(domain string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if domain != c.domain {
		c.clearLocked()
		c.domain = domain
	}
	return c.epoch
}

// Get retrieves a keyword's cached result.
func (c *KeywordHitCache) Get(keyword string) (keywordResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[keyword]
	if !ok {
		return keywordResult{}, false
	}
	if time.Since(entry.timestamp) > c.ttl {
		c.list.Remove(entry.element)
		delete(c.entries, keyword)
		return keywordResult{}, false
	}
	c.list.MoveToFront(entry.element)
	// A copy: callers append to and reorder what they get.
	return keywordResult{hits: append([]KeywordHit(nil), entry.result.hits...), overflow: entry.result.overflow}, true
}

// Set stores a keyword's result if the domain has not changed since epoch,
// and reports whether it did. A result read before a change is not stored.
func (c *KeywordHitCache) Set(keyword string, result keywordResult, epoch uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch != c.epoch {
		return false
	}
	result.hits = append([]KeywordHit(nil), result.hits...)
	if entry, ok := c.entries[keyword]; ok {
		entry.result = result
		entry.timestamp = time.Now()
		c.list.MoveToFront(entry.element)
		return true
	}
	if c.list.Len() >= c.maxSize {
		c.evictOldest()
	}
	entry := &cacheEntry{key: keyword, result: result, timestamp: time.Now()}
	entry.element = c.list.PushFront(entry)
	c.entries[keyword] = entry
	return true
}

// evictOldest removes the oldest cache entry in O(1) time.
func (c *KeywordHitCache) evictOldest() {
	elem := c.list.Back()
	if elem != nil {
		c.list.Remove(elem)
		entry := elem.Value.(*cacheEntry)
		delete(c.entries, entry.key)
	}
}

// Len is the number of cached results.
func (c *KeywordHitCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Clear empties the cache and moves its epoch.
func (c *KeywordHitCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
}

func (c *KeywordHitCache) clearLocked() {
	c.entries = make(map[string]*cacheEntry)
	c.list.Init()
	c.epoch++
}

// beginDelta opens a change to the domain's content: it moves the epoch, so
// no scan already in flight publishes, drops every overflowed entry (a
// change can surface a member the cap hid), and returns the keywords still
// cached, whose results applyDelta brings up to date.
func (c *KeywordHitCache) beginDelta() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	keywords := make([]string, 0, len(c.entries))
	for key, entry := range c.entries {
		if entry.result.overflow {
			c.list.Remove(entry.element)
			delete(c.entries, key)
			continue
		}
		keywords = append(keywords, key)
	}
	return keywords
}

// applyDelta maintains each cached result through a change:
//
//	H_next(q) = (H_prev(q) minus hits in changed files) union Search(q, changed files)
//
// keywords are the ones beginDelta returned: a result stored after it was
// scanned after the change and is left alone. changed says whether a hit is
// in a changed file; added holds each keyword's hits in the changed files as
// they are now. A result the delta
// pushes past maxHitsPerKeyword is dropped, not truncated: which hits a cold
// scan keeps past the cap is walk order, and the entry would claim a
// completeness it does not have. It returns how many entries changed.
func (c *KeywordHitCache) applyDelta(keywords []string, changed func(KeywordHit) bool, added map[string][]KeywordHit) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	touched := 0
	for _, key := range keywords {
		entry, ok := c.entries[key]
		if !ok {
			continue
		}
		kept := entry.result.hits[:0:0]
		removed := 0
		for _, hit := range entry.result.hits {
			if changed(hit) {
				removed++
				continue
			}
			kept = append(kept, hit)
		}
		fresh := added[key]
		if removed == 0 && len(fresh) == 0 {
			continue
		}
		touched++
		kept = append(kept, fresh...)
		if len(kept) >= maxHitsPerKeyword {
			c.list.Remove(entry.element)
			delete(c.entries, key)
			continue
		}
		entry.result.hits = kept
	}
	return touched
}
