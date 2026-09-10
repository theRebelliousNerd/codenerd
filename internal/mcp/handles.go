package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/retain"
)

// A handle is the promise that makes truncation safe: the caller is shown a
// shaped result and told, precisely, where the rest of it went and how to ask
// for it. Without that promise, shaping is just lossy — the agent learns that
// results are unreliable and starts asking for full every time, which costs
// more than never having shaped anything.
//
// Retention lives in internal/retain, not here. The problem this store solved
// well is not MCP-specific: a code-search result shown as symbols and
// dependency edges, a file read shown as the lines around an edit, a subagent's
// return shown as findings rather than a transcript — each wants to hand back a
// shaped view and keep the rest reachable. Building a second retention layer
// beside this one is how two mechanisms end up with two eviction policies and
// two ways to expire a citation.
//
// What stays here is the half that genuinely is MCP's: projecting a retained
// JSON payload through a pointer and a view. Retention is general; the shape a
// result takes is not.

// HandleStoreConfig bounds retention. All three limits apply at once, because
// they fail differently: entries bound a flood of tiny results, bytes bound one
// enormous result, and TTL bounds a long session that touches neither ceiling.
type HandleStoreConfig struct {
	MaxEntries int
	MaxBytes   int
	TTL        time.Duration
}

// DefaultHandleStoreConfig sizes retention for a working session.
func DefaultHandleStoreConfig() HandleStoreConfig {
	def := retain.DefaultConfig()
	return HandleStoreConfig{
		MaxEntries: def.MaxEntries,
		MaxBytes:   def.MaxBytes,
		TTL:        def.TTL,
	}
}

// handlePrefix marks a string as redeemable so a model can recognise one in a
// result without being told what it is.
const handlePrefix = "mcp:h:"

// HandleStore retains recent tool payloads so an elided result can be reopened
// exactly, without a second call to the server.
//
// The payload is retained rather than re-derived, and that is deliberate: this
// control plane fronts arbitrary MCP servers, so the call behind a handle may
// have created a pull request, charged a card, or deleted a branch. "Expand
// what you already told me" must never be a second side effect.
type HandleStore struct {
	store *retain.Store
}

// NewHandleStore creates a store. A zero or negative limit falls back to the
// default for that axis rather than meaning "unbounded".
func NewHandleStore(cfg HandleStoreConfig) *HandleStore {
	return &HandleStore{store: retain.New(retain.Config{
		MaxEntries: cfg.MaxEntries,
		MaxBytes:   cfg.MaxBytes,
		TTL:        cfg.TTL,
		Prefix:     handlePrefix,
	})}
}

// SetEvictionHook installs the callback fired when a handle is dropped.
//
// Eviction is the one moment a published mcp_result_handle fact becomes a lie:
// the kernel would go on offering an expansion of bytes that no longer exist,
// and the agent would spend a turn discovering that.
func (s *HandleStore) SetEvictionHook(fn func(handle string)) {
	if s == nil {
		return
	}
	s.store.SetEvictionHook(fn)
}

// Mint stores a payload and returns its handle.
//
// The id is content-addressed over (toolID, payload), so a tool called twice
// with the same answer yields the same handle and is stored once. That is not
// only a saving: it means a handle quoted back from an earlier turn still
// resolves if the same result is still live, which is exactly the case where an
// agent would otherwise be told its own citation had expired.
func (s *HandleStore) Mint(toolID string, payload json.RawMessage) string {
	if s == nil {
		return ""
	}
	return s.store.Mint(toolID, payload)
}

// ErrHandleNotFound is returned for an unknown or expired handle. It is a
// distinct error because the caller's correct response differs from every other
// failure: re-run the original call, do not retry the expansion.
var ErrHandleNotFound = retain.ErrNotFound

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
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("handle is required")
	}

	toolID, payload, err := s.store.Get(id)
	if err != nil {
		return nil, err
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
	st := s.store.Stats()
	return HandleStats{
		Entries:    st.Entries,
		Bytes:      st.Bytes,
		MaxBytes:   st.MaxBytes,
		MaxEntries: st.MaxEntries,
	}
}
