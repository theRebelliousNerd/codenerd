package retain

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T, cfg Config) (*Store, *time.Time) {
	t.Helper()
	s := New(cfg)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	clock := base
	s.SetClock(func() time.Time { return clock })
	return s, &clock
}

// ---------------------------------------------------------------------------
// Content addressing
// ---------------------------------------------------------------------------

func TestSamePayloadYieldsTheSameIDAndIsStoredOnce(t *testing.T) {
	s, _ := testStore(t, Config{})

	first := s.Mint("search", []byte(`{"hits":3}`))
	second := s.Mint("search", []byte(`{"hits":3}`))

	if first != second {
		t.Fatalf("ids differ for identical content: %q vs %q", first, second)
	}
	// The saving is secondary. What matters is that an id quoted back from an
	// earlier turn still resolves while the same result is live -- otherwise an
	// agent is told its own citation expired.
	if got := s.Stats().Entries; got != 1 {
		t.Errorf("entries = %d, want 1", got)
	}
}

func TestKindParticipatesInTheID(t *testing.T) {
	s, _ := testStore(t, Config{})

	a := s.Mint("search", []byte(`"same"`))
	b := s.Mint("read", []byte(`"same"`))

	// Two operations returning identical bytes are still two different
	// observations. Collapsing them would let one expire the other's citation.
	if a == b {
		t.Fatal("different kinds produced the same id")
	}
}

func TestPrefixMarksIDsRedeemable(t *testing.T) {
	s, _ := testStore(t, Config{Prefix: "custom:"})
	id := s.Mint("k", []byte(`1`))
	if len(id) <= len("custom:") || id[:len("custom:")] != "custom:" {
		t.Fatalf("id = %q, want the configured prefix", id)
	}

	// An id from one store must be visibly not an id from another, so a caller
	// cannot redeem a handle against the wrong retention.
	other, _ := testStore(t, Config{Prefix: "elsewhere:"})
	if _, _, err := other.Get(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("a foreign id resolved: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Retention guarantees
// ---------------------------------------------------------------------------

func TestPayloadIsCopiedNotAliased(t *testing.T) {
	s, _ := testStore(t, Config{})

	buf := []byte(`{"v":1}`)
	id := s.Mint("k", buf)

	// The caller's buffer is routinely reused or truncated after the call. A
	// handle that silently rots is worse than no handle at all.
	for i := range buf {
		buf[i] = 'x'
	}

	_, got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != `{"v":1}` {
		t.Fatalf("retained payload = %q; the store aliased the caller's buffer", got)
	}
}

func TestOversizedPayloadIsRefusedRatherThanEvictingEverything(t *testing.T) {
	s, _ := testStore(t, Config{MaxBytes: 100})

	keep := s.Mint("small", []byte("a useful earlier result"))
	if keep == "" {
		t.Fatal("setup: the small payload was not stored")
	}

	huge := make([]byte, 200)
	for i := range huge {
		huge[i] = 'z'
	}
	// Evicting every other handle to hold one oversized blob trades many
	// useful citations for one, and the caller still has its shaped result.
	if got := s.Mint("huge", huge); got != "" {
		t.Errorf("an over-budget payload was stored as %q", got)
	}
	if _, _, err := s.Get(keep); err != nil {
		t.Errorf("the oversized mint evicted an unrelated entry: %v", err)
	}
}

func TestEmptyPayloadMintsNothing(t *testing.T) {
	s, _ := testStore(t, Config{})
	if got := s.Mint("k", nil); got != "" {
		t.Errorf("Mint(nil) = %q, want empty", got)
	}
	if got := s.Mint("k", []byte{}); got != "" {
		t.Errorf("Mint(empty) = %q, want empty", got)
	}
	if got := s.Stats().Entries; got != 0 {
		t.Errorf("entries = %d, want 0", got)
	}
}

func TestZeroLimitsFallBackToDefaultsNotUnbounded(t *testing.T) {
	// "Unbounded" is a memory leak with a friendly name.
	s := New(Config{MaxEntries: 0, MaxBytes: -1, TTL: 0})
	st := s.Stats()
	def := DefaultConfig()
	if st.MaxEntries != def.MaxEntries || st.MaxBytes != def.MaxBytes {
		t.Fatalf("limits = %d entries / %d bytes, want the defaults", st.MaxEntries, st.MaxBytes)
	}
}

// ---------------------------------------------------------------------------
// Eviction
// ---------------------------------------------------------------------------

func TestTTLExpiryFiresTheHook(t *testing.T) {
	s, clock := testStore(t, Config{TTL: time.Minute})

	var evicted []string
	s.SetEvictionHook(func(id string) { evicted = append(evicted, id) })

	id := s.Mint("k", []byte("payload"))
	*clock = clock.Add(2 * time.Minute)

	if _, _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an expired entry resolved: %v", err)
	}
	// The hook is what stops a published handle becoming a lie: without it the
	// caller goes on offering an expansion of bytes that no longer exist.
	if len(evicted) != 1 || evicted[0] != id {
		t.Errorf("eviction hook fired %v, want [%s]", evicted, id)
	}
}

func TestEvictionIsLeastRecentlyUsed(t *testing.T) {
	s, clock := testStore(t, Config{MaxEntries: 3})

	ids := make([]string, 3)
	for i := range ids {
		ids[i] = s.Mint("k", []byte(fmt.Sprintf("payload-%d", i)))
		*clock = clock.Add(time.Second)
	}

	// Touch the oldest. A read must promote, or a frequently reopened result is
	// evicted by a burst of one-off ones.
	if _, _, err := s.Get(ids[0]); err != nil {
		t.Fatalf("setup: %v", err)
	}
	*clock = clock.Add(time.Second)

	s.Mint("k", []byte("newcomer"))

	if _, _, err := s.Get(ids[0]); err != nil {
		t.Error("a just-read entry was evicted; reads do not promote")
	}
	if _, _, err := s.Get(ids[1]); !errors.Is(err, ErrNotFound) {
		t.Error("the true least-recently-used entry survived")
	}
}

func TestTTLIsEnforcedBeforeTheSizeCeilings(t *testing.T) {
	s, clock := testStore(t, Config{MaxEntries: 2, TTL: time.Minute})

	stale := s.Mint("k", []byte("stale"))
	*clock = clock.Add(2 * time.Minute)

	// Dropping the expired entry first may make the entry ceiling moot, so a
	// live entry is not evicted to make room when a worthless one is present.
	fresh1 := s.Mint("k", []byte("fresh-1"))
	fresh2 := s.Mint("k", []byte("fresh-2"))

	if _, _, err := s.Get(stale); !errors.Is(err, ErrNotFound) {
		t.Error("the expired entry survived")
	}
	for _, id := range []string{fresh1, fresh2} {
		if _, _, err := s.Get(id); err != nil {
			t.Errorf("a fresh entry was evicted in favour of an expired one: %v", err)
		}
	}
}

func TestByteCeilingEvicts(t *testing.T) {
	s, clock := testStore(t, Config{MaxBytes: 50, MaxEntries: 100})

	first := s.Mint("k", []byte("0123456789012345678901234"))
	*clock = clock.Add(time.Second)
	second := s.Mint("k", []byte("abcdefghijabcdefghijabcde"))
	*clock = clock.Add(time.Second)
	third := s.Mint("k", []byte("zyxwvutsrqzyxwvutsrqzyxwv"))

	if _, _, err := s.Get(first); !errors.Is(err, ErrNotFound) {
		t.Error("the byte ceiling did not evict the oldest entry")
	}
	if _, _, err := s.Get(third); err != nil {
		t.Errorf("the newest entry was evicted: %v", err)
	}
	_ = second
	if got := s.Stats().Bytes; got > 50 {
		t.Errorf("retained bytes = %d, over the 50 ceiling", got)
	}
}

// TestEvictionHookRunsOutsideTheLock is the deadlock guard.
//
// The hook reaches arbitrary caller code -- in MCP it reaches the Mangle kernel
// to retract a fact. Firing it while holding the store mutex is how a deadlock
// gets built, and the failure mode is a hung agent with no error anywhere.
func TestEvictionHookRunsOutsideTheLock(t *testing.T) {
	s, clock := testStore(t, Config{MaxEntries: 1})

	done := make(chan struct{})
	s.SetEvictionHook(func(string) {
		// Re-entering the store from inside the hook deadlocks if the hook is
		// called under the lock.
		_ = s.Stats()
		close(done)
	})

	s.Mint("k", []byte("first"))
	*clock = clock.Add(time.Second)
	s.Mint("k", []byte("second"))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the eviction hook deadlocked; it is being called under the store lock")
	}
}

// ---------------------------------------------------------------------------
// Lookup
// ---------------------------------------------------------------------------

func TestGetReturnsTheKindItWasMintedUnder(t *testing.T) {
	s, _ := testStore(t, Config{})
	id := s.Mint("code-search", []byte("payload"))

	kind, payload, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if kind != "code-search" {
		t.Errorf("kind = %q, want code-search", kind)
	}
	if string(payload) != "payload" {
		t.Errorf("payload = %q", payload)
	}
}

func TestGetRejectsUnknownAndEmptyIDs(t *testing.T) {
	s, _ := testStore(t, Config{})
	for _, id := range []string{"", "   ", "h:doesnotexist"} {
		if _, _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", id, err)
		}
	}
}

func TestNilStoreIsInert(t *testing.T) {
	// Retention is optional wiring; a nil store must be safe so disabling it
	// never becomes a crash.
	var s *Store
	if got := s.Mint("k", []byte("x")); got != "" {
		t.Errorf("nil Mint = %q", got)
	}
	if _, _, err := s.Get("anything"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nil Get error = %v", err)
	}
	s.SetEvictionHook(func(string) {})
	s.SetClock(time.Now)
	if got := s.Stats(); got != (Stats{}) {
		t.Errorf("nil Stats = %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestStoreIsSafeUnderConcurrency(t *testing.T) {
	s := New(Config{MaxEntries: 32, MaxBytes: 1 << 20})
	s.SetEvictionHook(func(string) { _ = s.Stats() })

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := s.Mint("k", []byte(fmt.Sprintf("w%d-i%d", worker, i)))
				if id != "" {
					_, _, _ = s.Get(id)
				}
				_ = s.Stats()
			}
		}(w)
	}
	wg.Wait()

	st := s.Stats()
	if st.Entries > st.MaxEntries {
		t.Errorf("entries = %d, over the %d ceiling", st.Entries, st.MaxEntries)
	}
	if st.Bytes > st.MaxBytes {
		t.Errorf("bytes = %d, over the %d ceiling", st.Bytes, st.MaxBytes)
	}
	if st.Bytes < 0 {
		t.Errorf("byte accounting went negative: %d", st.Bytes)
	}
}
