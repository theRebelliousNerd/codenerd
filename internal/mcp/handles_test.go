package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHandleStore_ShouldRetainAndExpandExactly(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(DefaultHandleStoreConfig())
	payload, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{"id": "a"}, {"id": "b"}, {"id": "c"}},
	})

	handle := store.Mint("srv/list", payload)
	if handle == "" {
		t.Fatal("Mint returned no handle")
	}

	digest, err := store.Expand(handle, "/items/2", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got, _ := json.Marshal(digest.Data)
	if !strings.Contains(string(got), `"id":"c"`) {
		t.Errorf("expanded /items/2 = %s; want the third item", got)
	}
}

func TestHandleStore_ShouldBeContentAddressed(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(DefaultHandleStoreConfig())
	payload := json.RawMessage(`{"same":true}`)

	// The same answer from the same tool yields the same handle, so a handle an
	// agent quoted from an earlier turn still resolves.
	first := store.Mint("srv/tool", payload)
	second := store.Mint("srv/tool", payload)
	if first != second {
		t.Errorf("identical payloads minted %q and %q; content addressing is broken", first, second)
	}
	if store.Stats().Entries != 1 {
		t.Errorf("entries = %d, want 1; the duplicate was stored twice", store.Stats().Entries)
	}

	// A different tool is a different subject even with the same bytes.
	if other := store.Mint("srv/other", payload); other == first {
		t.Error("different tools produced the same handle")
	}
}

func TestHandleStore_ShouldNotStorePayloadLargerThanWholeBudget(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(HandleStoreConfig{MaxEntries: 8, MaxBytes: 1024, TTL: time.Minute})
	store.Mint("srv/small", json.RawMessage(`{"a":1}`))

	oversized, _ := json.Marshal(map[string]any{"blob": strings.Repeat("x", 4096)})
	if handle := store.Mint("srv/big", oversized); handle != "" {
		t.Error("an oversized payload was stored; it would evict every useful citation to hold one blob")
	}
	if store.Stats().Entries != 1 {
		t.Errorf("entries = %d, want 1; the small handle should have survived", store.Stats().Entries)
	}
}

func TestHandleStore_ShouldEvictLeastRecentlyUsed(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(HandleStoreConfig{MaxEntries: 2, MaxBytes: 1 << 20, TTL: time.Hour})
	base := time.Now()
	store.now = func() time.Time { return base }

	first := store.Mint("srv/a", json.RawMessage(`{"n":1}`))

	store.now = func() time.Time { return base.Add(time.Second) }
	second := store.Mint("srv/b", json.RawMessage(`{"n":2}`))

	// Touch the first so the second becomes least-recently-used.
	store.now = func() time.Time { return base.Add(2 * time.Second) }
	if _, err := store.Expand(first, "", ViewSummary, BudgetFor(ViewSummary)); err != nil {
		t.Fatalf("Expand first: %v", err)
	}

	store.now = func() time.Time { return base.Add(3 * time.Second) }
	store.Mint("srv/c", json.RawMessage(`{"n":3}`))

	if _, err := store.Expand(first, "", ViewSummary, BudgetFor(ViewSummary)); err != nil {
		t.Errorf("the recently-used handle was evicted: %v", err)
	}
	if _, err := store.Expand(second, "", ViewSummary, BudgetFor(ViewSummary)); err == nil {
		t.Error("the least-recently-used handle survived eviction")
	}
}

func TestHandleStore_WhenExpired_ShouldReportNotFoundDistinctly(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(HandleStoreConfig{MaxEntries: 8, MaxBytes: 1 << 20, TTL: time.Minute})
	base := time.Now()
	store.now = func() time.Time { return base }
	handle := store.Mint("srv/a", json.RawMessage(`{"n":1}`))

	store.now = func() time.Time { return base.Add(2 * time.Minute) }
	_, err := store.Expand(handle, "", ViewCompact, BudgetFor(ViewCompact))
	if err == nil {
		t.Fatal("an expired handle expanded")
	}
	// The caller's correct response to an expired handle differs from every
	// other failure — re-run the call, do not retry the expansion — so the
	// error has to be distinguishable.
	if !strings.Contains(err.Error(), ErrHandleNotFound.Error()) {
		t.Errorf("error = %q; want it to wrap ErrHandleNotFound", err)
	}
}

func TestHandleStore_ShouldCopyPayloadDefensively(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(DefaultHandleStoreConfig())
	buf := []byte(`{"v":"original"}`)
	handle := store.Mint("srv/a", buf)

	// The caller's buffer may be reused after Mint returns. A handle that rots
	// when that happens is worse than no handle.
	copy(buf, []byte(`{"v":"MUTATED!"}`))

	digest, err := store.Expand(handle, "/v", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if digest.Data != "original" {
		t.Errorf("stored payload = %v; the store aliased the caller's buffer", digest.Data)
	}
}

func TestHandleStore_ExpandIsBudgetedToo(t *testing.T) {
	t.Parallel()

	store := NewHandleStore(DefaultHandleStoreConfig())
	var items []map[string]any
	for i := 0; i < 500; i++ {
		items = append(items, map[string]any{"id": i})
	}
	payload, _ := json.Marshal(map[string]any{"items": items})
	handle := store.Mint("srv/list", payload)

	// The reason the result was elided is that it was too big; an expansion
	// that returns all of it has only moved the problem one turn later.
	digest, err := store.Expand(handle, "", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if digest.Bytes > BudgetFor(ViewCompact).MaxBytes {
		t.Errorf("expansion returned %d bytes, over the %d budget", digest.Bytes, BudgetFor(ViewCompact).MaxBytes)
	}
	if !digest.Truncated {
		t.Error("a still-truncated expansion must say so")
	}
}

func TestHandleStore_NilSafe(t *testing.T) {
	t.Parallel()

	var store *HandleStore
	if got := store.Mint("a", json.RawMessage(`{}`)); got != "" {
		t.Errorf("nil store minted %q", got)
	}
	if _, err := store.Expand("x", "", ViewCompact, BudgetFor(ViewCompact)); err == nil {
		t.Error("nil store expanded without error")
	}
	if got := store.Stats(); got.Entries != 0 {
		t.Errorf("nil store stats = %+v", got)
	}
}
