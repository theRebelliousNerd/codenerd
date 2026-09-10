// Package retain holds recent observation payloads so an elided result can be
// reopened exactly, without re-running the operation that produced it.
//
// It exists as its own package because the MCP control plane already solved
// this problem well and the answer is not MCP-specific. A code-search result
// shown as symbols and dependency edges, a file read shown as the lines around
// an edit, a subagent's return shown as findings rather than a transcript --
// each wants to hand back a shaped view and keep the rest reachable. Building a
// second retention layer beside this one is how two mechanisms end up with two
// eviction policies and two ways to expire a citation.
//
// The payload is RETAINED rather than re-derived, and that is the whole point.
// Re-running an observation to expand it is defensible only when the operation
// is a pure read of live state. It is not defensible in general: the call
// behind a handle may have created a pull request, charged a card, or deleted a
// branch. "Show me the rest of what you already told me" must never be a second
// side effect, and the only way to guarantee that is to keep the bytes.
//
// Re-running is also wrong for a subtler reason that survives even for pure
// reads: the expansion would show a different world from the one the reasoning
// was built on. An agent that cited line 40 of a file and then expanded to find
// different content has been handed a contradiction it cannot diagnose.
package retain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config bounds retention. All three limits apply at once, because they fail
// differently: entries bound a flood of tiny results, bytes bound one enormous
// result, and TTL bounds a long session that touches neither ceiling.
type Config struct {
	MaxEntries int
	MaxBytes   int
	TTL        time.Duration

	// Prefix marks a minted id as redeemable, so a model can recognise one in a
	// result without being told what it is. Each caller uses its own, which
	// also means an id from one store is visibly not an id from another.
	Prefix string
}

// DefaultConfig sizes retention for a working session.
//
// 8 MB is roughly the largest total a long agent session plausibly needs to
// hold open at once, and is small enough that it never competes with the
// process for memory that matters.
func DefaultConfig() Config {
	return Config{
		MaxEntries: 64,
		MaxBytes:   8 << 20,
		TTL:        30 * time.Minute,
		Prefix:     "h:",
	}
}

// ErrNotFound is returned for an unknown or expired id. It is distinct because
// the caller's correct response differs from every other failure: re-run the
// original operation, do not retry the expansion.
var ErrNotFound = errors.New("retained payload not found or expired")

type entry struct {
	id       string
	kind     string
	payload  []byte
	bytes    int
	created  time.Time
	lastUsed time.Time
}

// Store retains recent payloads under content-addressed ids.
type Store struct {
	mu       sync.Mutex
	cfg      Config
	entries  map[string]*entry
	curBytes int

	// onEvict is called for every id the store drops. Eviction is the one
	// moment a published handle becomes a lie: a caller would go on offering an
	// expansion of bytes that no longer exist, and the agent would spend a turn
	// discovering that.
	onEvict func(id string)

	// now is injectable so TTL behaviour is testable without sleeping.
	now func() time.Time
}

// New creates a store. A zero or negative limit falls back to the default for
// that axis rather than meaning "unbounded": an unbounded retention store is a
// memory leak with a friendly name.
func New(cfg Config) *Store {
	def := DefaultConfig()
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = def.MaxEntries
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = def.MaxBytes
	}
	if cfg.TTL <= 0 {
		cfg.TTL = def.TTL
	}
	if cfg.Prefix == "" {
		cfg.Prefix = def.Prefix
	}
	return &Store{cfg: cfg, entries: make(map[string]*entry), now: time.Now}
}

// SetEvictionHook installs the callback fired when an id is dropped.
func (s *Store) SetEvictionHook(fn func(id string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEvict = fn
}

// SetClock replaces the time source. For tests only.
func (s *Store) SetClock(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Mint stores a payload and returns its id, or "" when nothing was stored.
//
// The id is content-addressed over (kind, payload), so an operation repeated
// with the same answer yields the same id and is stored once. That is not only
// a saving: an id quoted back from an earlier turn still resolves if the same
// result is still live, which is exactly the case where an agent would
// otherwise be told its own citation had expired.
func (s *Store) Mint(kind string, payload []byte) string {
	if s == nil || len(payload) == 0 {
		return ""
	}

	sum := sha256.New()
	sum.Write([]byte(kind))
	sum.Write([]byte{0})
	sum.Write(payload)
	id := s.cfg.Prefix + hex.EncodeToString(sum.Sum(nil))[:12]

	// The eviction hook can reach arbitrary caller code, so it is collected
	// under the lock and fired after it. Calling out while holding a store
	// mutex is how a deadlock gets built.
	minted, evicted, hook := s.mintLocked(kind, payload, id)
	notify(hook, evicted)
	return minted
}

func (s *Store) mintLocked(kind string, payload []byte, id string) (string, []string, func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if existing, ok := s.entries[id]; ok {
		existing.lastUsed = now
		return id, nil, nil
	}

	// A single payload larger than the whole budget is not stored: evicting
	// every other entry to hold one oversized blob trades many useful citations
	// for one, and the caller still has its shaped result.
	if len(payload) > s.cfg.MaxBytes {
		return "", nil, nil
	}

	// Copy: the caller's buffer may be reused or truncated after this returns,
	// and a handle that silently rots is worse than no handle.
	stored := make([]byte, len(payload))
	copy(stored, payload)

	s.entries[id] = &entry{
		id: id, kind: kind, payload: stored,
		bytes: len(stored), created: now, lastUsed: now,
	}
	s.curBytes += len(stored)
	return id, s.evictLocked(now), s.onEvict
}

// Get returns the retained payload and the kind it was minted under.
//
// A read promotes the entry, so a repeatedly reopened result survives a burst
// of one-off ones -- which is exactly the entry worth keeping.
func (s *Store) Get(id string) (kind string, payload []byte, err error) {
	if s == nil {
		return "", nil, ErrNotFound
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", nil, fmt.Errorf("%w: empty id", ErrNotFound)
	}

	s.mu.Lock()
	expired := false
	e, ok := s.entries[id]
	if ok {
		now := s.now()
		if now.Sub(e.created) > s.cfg.TTL {
			s.curBytes -= e.bytes
			delete(s.entries, id)
			expired = true
			ok = false
		} else {
			e.lastUsed = now
		}
	}
	if ok {
		kind, payload = e.kind, e.payload
	}
	hook := s.onEvict
	s.mu.Unlock()

	if expired {
		notify(hook, []string{id})
	}
	if !ok {
		return "", nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return kind, payload, nil
}

// evictLocked enforces TTL first, then the byte and entry ceilings by least
// recently used. TTL first because an expired entry is worthless at any size,
// so dropping it may make the other two ceilings moot.
func (s *Store) evictLocked(now time.Time) []string {
	var evicted []string
	for id, e := range s.entries {
		if now.Sub(e.created) > s.cfg.TTL {
			s.curBytes -= e.bytes
			delete(s.entries, id)
			evicted = append(evicted, id)
		}
	}
	if len(s.entries) <= s.cfg.MaxEntries && s.curBytes <= s.cfg.MaxBytes {
		return evicted
	}

	ordered := make([]*entry, 0, len(s.entries))
	for _, e := range s.entries {
		ordered = append(ordered, e)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].lastUsed.Equal(ordered[j].lastUsed) {
			// Deterministic tie-break so eviction is reproducible in tests.
			return ordered[i].id < ordered[j].id
		}
		return ordered[i].lastUsed.Before(ordered[j].lastUsed)
	})

	for _, e := range ordered {
		if len(s.entries) <= s.cfg.MaxEntries && s.curBytes <= s.cfg.MaxBytes {
			return evicted
		}
		s.curBytes -= e.bytes
		delete(s.entries, e.id)
		evicted = append(evicted, e.id)
	}
	return evicted
}

func notify(hook func(string), ids []string) {
	if hook == nil {
		return
	}
	for _, id := range ids {
		hook(id)
	}
}

// Stats reports retention, for observability and status surfaces.
type Stats struct {
	Entries    int `json:"entries"`
	Bytes      int `json:"bytes"`
	MaxBytes   int `json:"max_bytes"`
	MaxEntries int `json:"max_entries"`
}

// Stats returns a snapshot of retention.
func (s *Store) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		Entries:    len(s.entries),
		Bytes:      s.curBytes,
		MaxBytes:   s.cfg.MaxBytes,
		MaxEntries: s.cfg.MaxEntries,
	}
}
