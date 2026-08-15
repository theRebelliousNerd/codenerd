package diff

import (
	"container/list"
	"sync"
)

// Cache bounds and defaults.
//
// The diff cache used to be an unbounded sync.Map. Two things were wrong with
// that: a long-lived chat session diffing thousands of file revisions grew the
// map without limit, and ClearCache reassigned the map wholesale, which races
// against concurrent Load/Store on the same field. Both are fixed here by an
// explicit LRU behind a single mutex.
const (
	// defaultMaxCacheEntries bounds how many distinct (oldHash,newHash) pairs
	// the engine remembers.
	defaultMaxCacheEntries = 512

	// defaultMaxCacheBytes bounds the total retained hunk payload. Entry count
	// alone is a poor proxy: one diff of a 50k-line file dwarfs 500 one-line
	// diffs, so we evict on whichever bound trips first.
	defaultMaxCacheBytes = 32 << 20 // 32 MiB
)

// Stats reports diff engine cache counters. Values are cumulative since the
// engine was created; ClearCache does not reset them.
type Stats struct {
	Hits     uint64 // cache lookups served from memory
	Misses   uint64 // cache lookups that required a computation
	Computes uint64 // diffs actually computed by diffmatchpatch
	Binary   uint64 // inputs short-circuited as binary
	Evicted  uint64 // entries dropped to stay within bounds
	Entries  int    // entries currently resident
	Bytes    int64  // approximate resident payload size

	// Collisions counts hits rejected by content verification — a key match
	// whose stored content did not equal the requested content. Always zero
	// unless Options.VerifyCacheContent is set. A nonzero value means the key
	// space is being hit harder than its width supports and is worth alerting
	// on, not merely logging.
	Collisions uint64
}

// cacheEntry is the value stored in the LRU. diff holds the canonical result;
// callers never receive this pointer, only a deep copy.
type cacheEntry struct {
	key  cacheKey
	diff *FileDiff
	size int64

	// oldContent/newContent are retained only under Options.VerifyCacheContent
	// so a hit can be proven rather than trusted.
	verify     bool
	oldContent string
	newContent string
}

// diffCache is a bounded LRU keyed by content hash pairs. All access is guarded
// by mu, which also makes Clear safe against concurrent readers.
type diffCache struct {
	mu         sync.Mutex
	entries    map[cacheKey]*list.Element
	order      *list.List // front = most recently used
	bytes      int64
	maxEntries int
	maxBytes   int64

	hits       uint64
	misses     uint64
	computes   uint64
	binary     uint64
	evicted    uint64
	collisions uint64
}

func newDiffCache(maxEntries int, maxBytes int64) *diffCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxCacheEntries
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxCacheBytes
	}
	return &diffCache{
		entries:    make(map[cacheKey]*list.Element),
		order:      list.New(),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

// get returns a deep copy of the cached diff, or nil on miss. Returning a copy
// is the whole point: the previous shallow `result := *cachedDiff` shared the
// Hunks backing array and every Hunk.Lines slice with the cache, so a caller
// that edited a returned hunk silently corrupted what every later caller saw.
func (c *diffCache) get(key cacheKey, oldContent, newContent string) *FileDiff {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil
	}
	entry := elem.Value.(*cacheEntry)

	// A verified entry must prove itself: serving the wrong diff is worse than
	// recomputing one. The entry is dropped so the miss below repopulates it.
	if entry.verify && (entry.oldContent != oldContent || entry.newContent != newContent) {
		c.collisions++
		c.misses++
		c.order.Remove(elem)
		delete(c.entries, key)
		c.bytes -= entry.size
		return nil
	}

	c.order.MoveToFront(elem)
	c.hits++
	return entry.diff.Clone()
}

// put stores a deep copy of fd, evicting least-recently-used entries until both
// bounds hold. Storing a copy keeps the cache immune to later caller mutation
// of the same FileDiff value.
func (c *diffCache) put(key cacheKey, fd *FileDiff, oldContent, newContent string, verify bool) {
	if fd == nil {
		return
	}
	stored := fd.Clone()
	size := stored.approxSize()
	if verify {
		// Retained content is real resident memory and must be charged to the
		// byte budget, or verification would silently double the cache.
		size += int64(len(oldContent) + len(newContent))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.bytes -= entry.size
		entry.diff = stored
		entry.size = size
		entry.verify = verify
		if verify {
			entry.oldContent = oldContent
			entry.newContent = newContent
		}
		c.bytes += size
		c.order.MoveToFront(elem)
		c.evictLocked()
		return
	}

	// A single diff larger than the whole budget would evict everything and
	// then still not fit. Skip caching it rather than thrashing the LRU.
	if size > c.maxBytes {
		return
	}

	entry := &cacheEntry{key: key, diff: stored, size: size, verify: verify}
	if verify {
		entry.oldContent = oldContent
		entry.newContent = newContent
	}
	elem := c.order.PushFront(entry)
	c.entries[key] = elem
	c.bytes += size
	c.evictLocked()
}

// evictLocked drops LRU entries until both bounds are satisfied. Caller holds mu.
func (c *diffCache) evictLocked() {
	for (len(c.entries) > c.maxEntries || c.bytes > c.maxBytes) && c.order.Len() > 0 {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}
		entry := oldest.Value.(*cacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, entry.key)
		c.bytes -= entry.size
		c.evicted++
	}
}

// clear drops every entry but preserves cumulative counters, so Stats stays a
// lifetime view of engine behavior.
func (c *diffCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[cacheKey]*list.Element)
	c.order.Init()
	c.bytes = 0
}

func (c *diffCache) markCompute() {
	c.mu.Lock()
	c.computes++
	c.mu.Unlock()
}

func (c *diffCache) markBinary() {
	c.mu.Lock()
	c.binary++
	c.mu.Unlock()
}

func (c *diffCache) stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Stats{
		Hits:       c.hits,
		Misses:     c.misses,
		Computes:   c.computes,
		Binary:     c.binary,
		Evicted:    c.evicted,
		Entries:    len(c.entries),
		Bytes:      c.bytes,
		Collisions: c.collisions,
	}
}

// Clone returns a deep copy of fd: the Hunks slice, every Hunk.Lines slice, and
// the Line values are all freshly allocated, so the result shares no mutable
// state with the receiver.
func (fd *FileDiff) Clone() *FileDiff {
	if fd == nil {
		return nil
	}
	out := *fd
	if fd.Hunks != nil {
		out.Hunks = make([]Hunk, len(fd.Hunks))
		for i, h := range fd.Hunks {
			hunk := h
			if h.Lines != nil {
				hunk.Lines = make([]Line, len(h.Lines))
				copy(hunk.Lines, h.Lines)
			}
			out.Hunks[i] = hunk
		}
	}
	return &out
}

// approxSize estimates the retained bytes of fd for cache accounting. It counts
// line content plus a fixed per-line and per-hunk struct overhead; exactness is
// not required, only monotonicity with real memory use.
func (fd *FileDiff) approxSize() int64 {
	const (
		perLineOverhead = 40 // LineNum + Type + string header
		perHunkOverhead = 64 // four ints + slice header
	)
	size := int64(len(fd.OldPath) + len(fd.NewPath))
	for _, h := range fd.Hunks {
		size += perHunkOverhead
		for _, l := range h.Lines {
			size += perLineOverhead + int64(len(l.Content))
		}
	}
	return size
}
