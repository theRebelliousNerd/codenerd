package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// A handle is the promise that makes truncation safe: the caller is shown a
// shaped result and told, precisely, where the rest of it went and how to ask
// for it. Without that promise, shaping is just lossy — the agent learns that
// results are unreliable and starts asking for full every time, which costs
// more than never having shaped anything.
//
// This store RETAINS the payload rather than re-deriving it, and that is a
// deliberate departure from the browser-side pattern, where expanding a handle
// re-runs the observation and projects a different slice out of it. Re-running
// is defensible when the underlying operation is a read of a live page. It is
// not defensible here: this control plane fronts arbitrary MCP servers, so the
// call behind a handle may have created a pull request, charged a card, or
// deleted a branch. "Expand what you already told me" must never be a second
// side effect, and the only way to guarantee that is to keep the bytes.

// HandleStoreConfig bounds retention. All three limits apply at once, because
// they fail differently: entries bound a flood of tiny results, bytes bound one
// enormous result, and TTL bounds a long session that touches neither ceiling.
type HandleStoreConfig struct {
	MaxEntries int
	MaxBytes   int
	TTL        time.Duration
}

// DefaultHandleStoreConfig sizes retention for a working session.
//
// 8 MB is roughly the largest total a long agent session plausibly needs to
// hold open at once, and is small enough that it never competes with the
// process for memory that matters.
func DefaultHandleStoreConfig() HandleStoreConfig {
	return HandleStoreConfig{
		MaxEntries: 64,
		MaxBytes:   8 << 20,
		TTL:        30 * time.Minute,
	}
}

type handleEntry struct {
	id       string
	toolID   string
	payload  json.RawMessage
	bytes    int
	created  time.Time
	lastUsed time.Time
}

// HandleStore retains recent tool payloads so an elided result can be reopened
// exactly, without a second call to the server.
type HandleStore struct {
	mu       sync.Mutex
	cfg      HandleStoreConfig
	entries  map[string]*handleEntry
	curBytes int

	// onEvict is called for every handle the store drops. Eviction is the one
	// moment a published mcp_result_handle fact becomes a lie: the kernel would
	// go on offering an expansion of bytes that no longer exist, and the agent
	// would spend a turn discovering that.
	onEvict func(handle string)

	// now is injectable so TTL behaviour is testable without sleeping.
	now func() time.Time
}

// SetEvictionHook installs the callback fired when a handle is dropped.
func (s *HandleStore) SetEvictionHook(fn func(handle string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEvict = fn
}

// NewHandleStore creates a store. A zero or negative limit falls back to the
// default for that axis rather than meaning "unbounded": an unbounded handle
// store is a memory leak with a friendly name.
func NewHandleStore(cfg HandleStoreConfig) *HandleStore {
	def := DefaultHandleStoreConfig()
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = def.MaxEntries
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = def.MaxBytes
	}
	if cfg.TTL <= 0 {
		cfg.TTL = def.TTL
	}
	return &HandleStore{
		cfg:     cfg,
		entries: make(map[string]*handleEntry),
		now:     time.Now,
	}
}

// handlePrefix marks a string as redeemable so a model can recognise one in a
// result without being told what it is.
const handlePrefix = "mcp:h:"

// Mint stores a payload and returns its handle.
//
// The id is content-addressed over (toolID, payload), so a tool called twice
// with the same answer yields the same handle and is stored once. That is not
// only a saving: it means a handle quoted back from an earlier turn still
// resolves if the same result is still live, which is exactly the case where an
// agent would otherwise be told its own citation had expired.
func (s *HandleStore) Mint(toolID string, payload json.RawMessage) string {
	if s == nil || len(payload) == 0 {
		return ""
	}

	sum := sha256.New()
	sum.Write([]byte(toolID))
	sum.Write([]byte{0})
	sum.Write(payload)
	id := handlePrefix + hex.EncodeToString(sum.Sum(nil))[:12]

	// The eviction hook reaches the kernel, so it is collected under the lock
	// and fired after it. Calling out to the kernel while holding a store mutex
	// is how a deadlock gets built.
	minted, evicted, hook := s.mintLocked(toolID, payload, id)
	s.notifyEvicted(hook, evicted)
	return minted
}

func (s *HandleStore) mintLocked(toolID string, payload json.RawMessage, id string) (string, []string, func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if existing, ok := s.entries[id]; ok {
		existing.lastUsed = now
		return id, nil, nil
	}

	// A single payload larger than the whole budget is not stored: evicting
	// every other handle to hold one oversized blob trades many useful
	// citations for one, and the caller still gets the shaped result.
	if len(payload) > s.cfg.MaxBytes {
		return "", nil, nil
	}

	// Copy: the caller's buffer may be reused or truncated after this returns,
	// and a handle that silently rots is worse than no handle.
	stored := make(json.RawMessage, len(payload))
	copy(stored, payload)

	s.entries[id] = &handleEntry{
		id: id, toolID: toolID, payload: stored,
		bytes: len(stored), created: now, lastUsed: now,
	}
	s.curBytes += len(stored)
	return id, s.evictLocked(now), s.onEvict
}

// evictLocked enforces TTL first, then the byte and entry ceilings by least
// recently used. TTL first because an expired entry is worthless at any size,
// so dropping it may make the other two ceilings moot.
func (s *HandleStore) evictLocked(now time.Time) []string {
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

	ordered := make([]*handleEntry, 0, len(s.entries))
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

// notifyEvicted fires the hook outside the store lock.
func (s *HandleStore) notifyEvicted(hook func(string), ids []string) {
	if hook == nil {
		return
	}
	for _, id := range ids {
		hook(id)
	}
}

// ErrHandleNotFound is returned for an unknown or expired handle. It is a
// distinct error because the caller's correct response differs from every other
// failure: re-run the original call, do not retry the expansion.
var ErrHandleNotFound = fmt.Errorf("handle not found or expired")

// Expand reopens a stored payload, optionally at a JSON pointer, shaped to a
// view.
//
// Expanding is itself budgeted. It has to be: the reason a result was elided is
// that it was too big, and an expansion that returns all of it has simply moved
// the problem one turn later. Pointer-plus-budget is what turns "show me more"
// into a walk the caller controls.
func (s *HandleStore) Expand(id, pointer string, view View, budget DigestBudget) (*Digest, error) {
	if s == nil {
		return nil, ErrHandleNotFound
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("handle is required")
	}

	s.mu.Lock()
	expired := false
	entry, ok := s.entries[id]
	if ok {
		now := s.now()
		if now.Sub(entry.created) > s.cfg.TTL {
			s.curBytes -= entry.bytes
			delete(s.entries, id)
			expired = true
			ok = false
		} else {
			entry.lastUsed = now
		}
	}
	var payload json.RawMessage
	var toolID string
	if ok {
		payload = entry.payload
		toolID = entry.toolID
	}
	hook := s.onEvict
	s.mu.Unlock()

	if expired {
		s.notifyEvicted(hook, []string{id})
	}

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrHandleNotFound, id)
	}

	if strings.TrimSpace(pointer) == "" {
		digest := DigestJSON(payload, view, budget)
		digest.Handle = id
		return &digest, nil
	}

	var root any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("handle %s (%s) is not JSON, so it has no addressable slices", id, toolID)
	}

	slice, err := ResolvePointer(root, pointer)
	if err != nil {
		return nil, err
	}
	sliceRaw, err := json.Marshal(slice)
	if err != nil {
		return nil, fmt.Errorf("re-encode slice at %s: %w", pointer, err)
	}

	digest := DigestJSON(sliceRaw, view, budget)
	digest.Handle = id
	return &digest, nil
}

// HandleStats reports retention for observability and for the control plane's
// own status surface.
type HandleStats struct {
	Entries    int `json:"entries"`
	Bytes      int `json:"bytes"`
	MaxBytes   int `json:"max_bytes"`
	MaxEntries int `json:"max_entries"`
}

// Stats returns a snapshot of retention.
func (s *HandleStore) Stats() HandleStats {
	if s == nil {
		return HandleStats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return HandleStats{
		Entries:    len(s.entries),
		Bytes:      s.curBytes,
		MaxBytes:   s.cfg.MaxBytes,
		MaxEntries: s.cfg.MaxEntries,
	}
}
