package retain

import (
	"errors"
	"testing"
	"time"
)

// Brutal retention probes: isolation, dedup, eviction, expiry, nil-safety.

func brutalStore() (*Store, *time.Time) {
	now := time.Now()
	s := New(Config{MaxEntries: 4, MaxBytes: 1024, TTL: time.Minute, Prefix: "t:"})
	s.SetClock(func() time.Time { return now })
	return s, &now
}

// Mint copies on the way in; Get must copy on the way out, or a caller that
// mutates the returned slice silently rots every future expansion.
func TestGetMutationIsolation(t *testing.T) {
	s, _ := brutalStore()
	id := s.Mint("file", []byte("line40-original"))
	if id == "" {
		t.Fatal("Mint stored nothing")
	}
	_, p1, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	for i := range p1 {
		p1[i] = 'X' // hostile caller mutates the returned buffer
	}
	_, p2, err := s.Get(id)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if string(p2) != "line40-original" {
		t.Fatalf("retained bytes rotted through Get aliasing: %q", p2)
	}
}

// Same (kind, payload) mints the same id once; different kinds differ.
func TestContentAddressedDedup(t *testing.T) {
	s, _ := brutalStore()
	a := s.Mint("file", []byte("same"))
	b := s.Mint("file", []byte("same"))
	c := s.Mint("search", []byte("same"))
	if a == "" || a != b {
		t.Fatalf("dedup failed: %q vs %q", a, b)
	}
	if a == c {
		t.Fatal("kind is not part of the address: file and search collide")
	}
	if got := s.Stats().Entries; got != 2 {
		t.Fatalf("expected 2 stored entries, got %d", got)
	}
}

// Overflowing MaxEntries evicts least-recently-used first and fires the hook.
func TestLRUEvictionOrderAndHook(t *testing.T) {
	s, now := brutalStore()
	var evicted []string
	s.SetEvictionHook(func(id string) { evicted = append(evicted, id) })
	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		*now = (*now).Add(time.Second) // distinct lastUsed per entry
		ids = append(ids, s.Mint("k", []byte{byte('a' + i)}))
	}
	if got := s.Stats().Entries; got != 4 {
		t.Fatalf("expected 4 entries after overflow, got %d", got)
	}
	if _, _, err := s.Get(ids[0]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("oldest entry should be evicted, Get err=%v", err)
	}
	for _, id := range ids[1:] {
		if _, _, err := s.Get(id); err != nil {
			t.Fatalf("recent entry %s missing: %v", id, err)
		}
	}
	if len(evicted) != 1 || evicted[0] != ids[0] {
		t.Fatalf("hook fired for %v, want [%s]", evicted, ids[0])
	}
}

// A read promotes the entry: a reopened result survives a burst of one-offs.
func TestReadPromotesAgainstEviction(t *testing.T) {
	s, now := brutalStore()
	keep := s.Mint("k", []byte("keep"))
	for i := 0; i < 3; i++ {
		*now = (*now).Add(time.Second)
		s.Mint("k", []byte{byte('x' + i)})
	}
	*now = (*now).Add(time.Second)
	if _, _, err := s.Get(keep); err != nil { // promote
		t.Fatalf("Get failed: %v", err)
	}
	*now = (*now).Add(time.Second)
	s.Mint("k", []byte("overflow")) // forces one eviction
	if _, _, err := s.Get(keep); err != nil {
		t.Fatalf("promoted entry was evicted: %v", err)
	}
}

// TTL expiry deletes on read, adjusts bytes, and fires the hook once.
func TestTTLExpiryShape(t *testing.T) {
	s, now := brutalStore()
	var hooks int
	s.SetEvictionHook(func(id string) { hooks++ })
	id := s.Mint("k", []byte("temp"))
	before := s.Stats().Bytes
	if before == 0 {
		t.Fatal("expected bytes accounted")
	}
	*now = (*now).Add(2 * time.Minute)
	_, _, err := s.Get(id)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired entry Get err=%v, want ErrNotFound", err)
	}
	if got := s.Stats().Bytes; got != 0 {
		t.Fatalf("expired bytes not released: %d", got)
	}
	if hooks != 1 {
		t.Fatalf("hook fired %d times, want 1", hooks)
	}
}

// Oversized and empty payloads store nothing and return "".
func TestUnstorablePayloads(t *testing.T) {
	s, _ := brutalStore()
	if id := s.Mint("k", make([]byte, 2048)); id != "" {
		t.Fatalf("oversized payload stored as %q", id)
	}
	if id := s.Mint("k", nil); id != "" {
		t.Fatalf("empty payload stored as %q", id)
	}
	if id := s.Mint("k", []byte{}); id != "" {
		t.Fatalf("empty payload stored as %q", id)
	}
	if got := s.Stats().Entries; got != 0 {
		t.Fatalf("expected 0 entries, got %d", got)
	}
}

// Nil store is safe on every method.
func TestNilStoreSafe(t *testing.T) {
	var s *Store
	if id := s.Mint("k", []byte("x")); id != "" {
		t.Fatalf("nil Mint returned %q", id)
	}
	if _, _, err := s.Get("t:abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("nil Get err=%v", err)
	}
	if st := s.Stats(); st != (Stats{}) {
		t.Fatalf("nil Stats=%+v", st)
	}
	s.SetEvictionHook(func(string) {}) // must not panic
	s.SetClock(time.Now)               // must not panic
}
