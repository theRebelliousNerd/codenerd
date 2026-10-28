package prompt

import (
	"container/list"
	"strconv"
	"sync"
)

// lruCache is a size-bounded, mutex-guarded LRU. The compiler keeps three:
// compiled results, dynamic-snapshot preflights, and typed-atom catalogs are
// not (catalogs are one entry per source, see compiler_generation.go).
type lruCache[K comparable, V any] struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[K]*list.Element
}

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

func newLRUCache[K comparable, V any](limit int) *lruCache[K, V] {
	if limit < 1 {
		limit = 1
	}
	return &lruCache[K, V]{limit: limit, order: list.New(), items: make(map[K]*list.Element)}
}

func (l *lruCache[K, V]) Get(key K) (V, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.items[key]; ok {
		l.order.MoveToFront(elem)
		return elem.Value.(*lruEntry[K, V]).value, true
	}
	var zero V
	return zero, false
}

func (l *lruCache[K, V]) Put(key K, value V) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if elem, ok := l.items[key]; ok {
		l.order.MoveToFront(elem)
		elem.Value.(*lruEntry[K, V]).value = value
		return
	}
	l.items[key] = l.order.PushFront(&lruEntry[K, V]{key: key, value: value})
	for l.order.Len() > l.limit {
		oldest := l.order.Back()
		l.order.Remove(oldest)
		delete(l.items, oldest.Value.(*lruEntry[K, V]).key)
	}
}

func (l *lruCache[K, V]) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = make(map[K]*list.Element)
	l.order.Init()
}

func (l *lruCache[K, V]) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.order.Len()
}

// promptCacheLimit bounds compiled results and, separately, preflight
// snapshots. It is a memory ceiling, not a tuning knob: an evicted entry is
// recompiled, never served wrong.
const promptCacheLimit = 1000

type promptCompilationOutcome struct {
	result   *CompilationResult
	cacheHit bool
}

// promptCacheKey is the complete identity of a compiled result: the caller
// context (which carries the kernel snapshot fingerprint), the compiler
// configuration revision, and the source vector naming the exact generation
// of every non-kernel source the compile read.
func promptCacheKey(contextHash string, configRevision uint64, sources string) string {
	return HashContent(contextHash +
		"\x00compiler_config_revision=" + strconv.FormatUint(configRevision, 10) +
		"\x00sources=" + sources)
}

// preflightKey names one dynamic-snapshot computation: the kernel state it
// was cloned from, the exact compile_context facts asserted on top, the shard
// identity the injection filter matches, and the configuration revision that
// sets the injection bounds. Equal keys over a reusable kernel identity yield
// byte-identical dynamic atoms and fingerprints, so the clone, the context
// assertion and the full QueryAll walk can be skipped.
type preflightKey struct {
	kernelInstance   uint64
	kernelGeneration uint64
	contextFacts     string
	shardID          string
	shardInstanceID  string
	configRevision   uint64
}

type preflightEntry struct {
	dynamic     []*PromptAtom
	fingerprint string
}

func newPreflightKey(identity KernelSnapshotIdentity, cc *CompilationContext, configRevision uint64) preflightKey {
	facts := cc.ToContextFacts()
	encoded := make([]string, 0, len(facts))
	for _, fact := range facts {
		if text, ok := fact.(string); ok {
			encoded = append(encoded, text)
		}
	}
	return preflightKey{
		kernelInstance:   identity.Instance,
		kernelGeneration: identity.Generation,
		contextFacts:     HashContent(joinLengthPrefixed(canonicalStringSet(encoded))),
		shardID:          cc.ShardID,
		shardInstanceID:  cc.ShardInstanceID,
		configRevision:   configRevision,
	}
}

func joinLengthPrefixed(values []string) string {
	n := 0
	for _, v := range values {
		n += len(v) + 8
	}
	buf := make([]byte, 0, n)
	for _, v := range values {
		buf = strconv.AppendInt(buf, int64(len(v)), 10)
		buf = append(buf, ':')
		buf = append(buf, v...)
	}
	return string(buf)
}

// clearPromptCache drops compiled results. Preflight snapshots are keyed by
// kernel identity and configuration revision, never by source state, so they
// stay valid across source registration.
func (c *JITPromptCompiler) clearPromptCache(reason string) {
	c.cache.Clear()
	c.counters.resetCacheCounters()
	if reason != "" {
		logPromptCacheCleared(reason)
	}
}
