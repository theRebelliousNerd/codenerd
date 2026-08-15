package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newDiskCache returns a cache backed by dir, bypassing the package singleton
// so tests never fight each other over it.
func newDiskCache(t *testing.T, dir string, maxSize int, ttl time.Duration) *ResearchCache {
	t.Helper()
	c := NewResearchCache(maxSize, ttl)
	if err := c.disk.SetDiskDir(dir); err != nil {
		t.Fatalf("SetDiskDir: %v", err)
	}
	return c
}

func TestResearchCache_WhenDiskBacked_ShouldSurviveProcessRestart(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".nerd", "cache", "research")

	first := newDiskCache(t, dir, 100, time.Hour)
	first.Set("https://example.com/llms.txt", "the docs", "context7")

	// A new cache is a new process: empty map, same directory.
	second := newDiskCache(t, dir, 100, time.Hour)
	entry, ok := second.Get("https://example.com/llms.txt")
	if !ok {
		t.Fatal("entry did not survive: a research cache that dies with the process re-fetches everything on every run")
	}
	if entry.Value != "the docs" || entry.Source != "context7" {
		t.Fatalf("restored entry lost fields: %+v", entry)
	}
}

func TestResearchCache_WhenLoadedFromDisk_ShouldHydrateMemoryTier(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "research")

	first := newDiskCache(t, dir, 100, time.Hour)
	first.Set("a", "1", "web_fetch")
	first.Set("b", "2", "web_fetch")

	second := newDiskCache(t, dir, 100, time.Hour)
	if n := second.disk.loadInto(second); n != 2 {
		t.Fatalf("loadInto restored %d entries, want 2", n)
	}
	if second.Size() != 2 {
		t.Fatalf("memory tier holds %d entries after hydrate, want 2", second.Size())
	}
}

func TestResearchCache_WhenEntryExpired_ShouldNotResurrectFromDisk(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "research")

	c := newDiskCache(t, dir, 100, 10*time.Millisecond)
	c.Set("k", "v", "web_fetch")
	time.Sleep(30 * time.Millisecond)

	if _, ok := c.Get("k"); ok {
		t.Fatal("expired entry served")
	}
	// And a fresh process must not find it either.
	fresh := newDiskCache(t, dir, 100, time.Hour)
	if _, ok := fresh.Get("k"); ok {
		t.Fatal("expired entry resurrected from disk")
	}
}

func TestResearchCache_WhenCleared_ShouldRemoveDiskEntries(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "research")

	c := newDiskCache(t, dir, 100, time.Hour)
	c.Set("k", "v", "web_fetch")
	c.Clear()

	// Clearing memory only would be a lie: Get would restore it from disk.
	if _, ok := c.Get("k"); ok {
		t.Fatal("cleared entry came back from disk")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			t.Fatalf("Clear left %s on disk", e.Name())
		}
	}
}

func TestResearchCache_WhenKeyLooksLikeTraversal_ShouldStayInsideCacheDir(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	dir := filepath.Join(base, "research")

	c := newDiskCache(t, dir, 100, time.Hour)
	// The key is caller-supplied and never sanitized upstream. It must not be
	// able to become a path segment.
	for _, key := range []string{
		"../../escape",
		"/etc/passwd",
		`..\..\escape`,
		strings.Repeat("x", 4096),
	} {
		c.Set(key, "payload", "web_fetch")
		got, ok := c.Get(key)
		if !ok || got.Value != "payload" {
			t.Fatalf("key %q did not round-trip", key)
		}
	}

	// Everything written must live directly in dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries were written at all")
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Fatalf("cache created a subdirectory %q from a caller key", e.Name())
		}
	}
	// And nothing may have appeared beside the cache directory.
	siblings, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("ReadDir base: %v", err)
	}
	for _, s := range siblings {
		if s.Name() != "research" {
			t.Fatalf("a cache key escaped the cache directory and created %q", s.Name())
		}
	}
}

func TestResearchCache_WhenDiskDisabled_ShouldStillWork(t *testing.T) {
	t.Parallel()
	c := NewResearchCache(10, time.Hour) // zero-value diskStore: persistence off

	c.Set("k", "v", "web_fetch")
	if got, ok := c.Get("k"); !ok || got.Value != "v" {
		t.Fatal("memory-only cache broken")
	}
	c.Clear()
	if _, ok := c.Get("k"); ok {
		t.Fatal("Clear did not clear the memory tier")
	}
}

func TestResearchCache_WhenDiskFileCorrupt_ShouldMissNotFail(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "research")

	c := newDiskCache(t, dir, 100, time.Hour)
	c.Set("k", "v", "web_fetch")

	// Truncate the entry file to garbage.
	p := c.disk.path("k")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	fresh := newDiskCache(t, dir, 100, time.Hour)
	if _, ok := fresh.Get("k"); ok {
		t.Fatal("corrupt entry was served")
	}
	// A corrupt cache must not break the cache: writes still work afterwards.
	fresh.Set("k2", "v2", "web_fetch")
	if _, ok := fresh.Get("k2"); !ok {
		t.Fatal("cache stopped working after encountering a corrupt entry")
	}
}

func TestEnableDiskCache_WhenWorkspaceGiven_ShouldUseNerdCacheDir(t *testing.T) {
	// Not parallel: mutates the package-level default cache.
	ws := t.TempDir()
	if err := EnableDiskCache(ws); err != nil {
		t.Fatalf("EnableDiskCache: %v", err)
	}
	t.Cleanup(func() { _ = EnableDiskCache("") })

	want := filepath.Join(ws, ".nerd", "cache", "research")
	if got := getDefaultCache().disk.DiskDir(); got != want {
		t.Fatalf("disk dir = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
}
