package research

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// Disk backing for the research cache.
//
// The cache was in-memory only, so every `nerd` invocation started cold: the
// same llms.txt, the same API reference page and the same search result were
// re-fetched on every run, at network latency and (for metered providers) at
// cost. A research cache whose lifetime is one process is a memoization of a
// single conversation, not a cache.
//
// Design constraints that shaped this:
//   - Entries are keyed by an opaque caller-supplied string (often a URL), so
//     the key cannot be a filename. Each entry is a separate JSON file named
//     by the key's hash, which also bounds filename length and keeps a
//     traversal-shaped key ("../../etc/passwd") from ever reaching the path.
//   - Persistence is best-effort. A cache that fails a request because the
//     disk is full or read-only is worse than no cache, so every disk error is
//     logged and swallowed; the in-memory tier keeps serving.
//   - The store lives under <workspace>/.nerd/cache/research, so it is
//     workspace-scoped like the rest of .nerd, and is discarded with it.

const (
	// diskCacheDirName is the subdirectory of .nerd holding cached entries.
	diskCacheDirName = "research"

	// diskEntrySchema versions the on-disk record. A file whose schema does not
	// match is ignored rather than migrated: it is a cache.
	diskEntrySchema = 1
)

// diskEntry is the on-disk form of a CacheEntry. Key is stored alongside the
// value because the filename is a hash and cannot be reversed.
type diskEntry struct {
	Schema    int       `json:"schema"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// diskStore persists cache entries under a directory. A zero value is disabled
// and every method is a no-op, which is what keeps the cache usable when no
// workspace is configured.
type diskStore struct {
	mu  sync.RWMutex
	dir string
}

// SetDiskDir points the store at dir, creating it if needed. An empty dir
// disables persistence. Returns an error only for a genuinely unusable
// directory, so callers can surface a misconfiguration; runtime read/write
// failures afterwards are swallowed by design.
func (d *diskStore) SetDiskDir(dir string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dir = strings.TrimSpace(dir)
	if dir == "" {
		d.dir = ""
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("research cache dir %q unusable: %w", dir, err)
	}
	d.dir = dir
	return nil
}

// DiskDir returns the active directory, or "" when persistence is off.
func (d *diskStore) DiskDir() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.dir
}

// path returns the file backing key, or "" when persistence is off.
//
// hashKey is what makes this safe: the caller's key never becomes a path
// segment, so no key shape — traversal, absolute, backslash-laden — can move
// the write out of dir.
func (d *diskStore) path(key string) string {
	dir := d.DiskDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, hashKey(key)+".json")
}

func (d *diskStore) write(e *CacheEntry) {
	p := d.path(e.Key)
	if p == "" {
		return
	}
	data, err := json.Marshal(diskEntry{
		Schema:    diskEntrySchema,
		Key:       e.Key,
		Value:     e.Value,
		Source:    e.Source,
		CreatedAt: e.CreatedAt,
		ExpiresAt: e.ExpiresAt,
	})
	if err != nil {
		logging.ResearcherDebug("research cache: marshal failed for %s: %v", e.Key, err)
		return
	}
	// Write to a temp file and rename so a crash mid-write cannot leave a
	// half-written entry that later parses as a truncated value.
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		logging.ResearcherDebug("research cache: write failed for %s: %v", e.Key, err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		logging.ResearcherDebug("research cache: rename failed for %s: %v", e.Key, err)
		_ = os.Remove(tmp)
	}
}

func (d *diskStore) read(key string) (*CacheEntry, bool) {
	p := d.path(key)
	if p == "" {
		return nil, false
	}
	data, err := os.ReadFile(p) // #nosec G304 -- p is dir + hash, never caller text
	if err != nil {
		return nil, false
	}
	var de diskEntry
	if err := json.Unmarshal(data, &de); err != nil || de.Schema != diskEntrySchema {
		return nil, false
	}
	// A hash collision would serve the wrong document; compare the stored key.
	if de.Key != key {
		return nil, false
	}
	if !time.Now().Before(de.ExpiresAt) {
		_ = os.Remove(p)
		return nil, false
	}
	return &CacheEntry{
		Key:       de.Key,
		Value:     de.Value,
		CreatedAt: de.CreatedAt,
		ExpiresAt: de.ExpiresAt,
		Source:    de.Source,
	}, true
}

func (d *diskStore) remove(key string) {
	if p := d.path(key); p != "" {
		_ = os.Remove(p)
	}
}

// clear removes every entry file, returning how many were removed.
func (d *diskStore) clear() int {
	dir := d.DiskDir()
	if dir == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed
}

// loadInto hydrates unexpired disk entries into the in-memory tier, newest
// first so that an over-full store fills the memory cache with the most
// recently written entries rather than an arbitrary directory order.
func (d *diskStore) loadInto(c *ResearchCache) int {
	dir := d.DiskDir()
	if dir == "" {
		return 0
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	type loaded struct {
		entry *CacheEntry
	}
	var all []loaded
	for _, n := range names {
		if n.IsDir() || !strings.HasSuffix(n.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, n.Name())) // #nosec G304 -- directory listing
		if err != nil {
			continue
		}
		var de diskEntry
		if err := json.Unmarshal(data, &de); err != nil || de.Schema != diskEntrySchema {
			continue
		}
		if !time.Now().Before(de.ExpiresAt) {
			_ = os.Remove(filepath.Join(dir, n.Name()))
			continue
		}
		all = append(all, loaded{&CacheEntry{
			Key:       de.Key,
			Value:     de.Value,
			CreatedAt: de.CreatedAt,
			ExpiresAt: de.ExpiresAt,
			Source:    de.Source,
		}})
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].entry.CreatedAt.After(all[j].entry.CreatedAt)
	})

	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, l := range all {
		if len(c.entries) >= c.maxSize {
			break
		}
		if _, exists := c.entries[l.entry.Key]; exists {
			continue
		}
		c.entries[l.entry.Key] = l.entry
		count++
	}
	return count
}

// EnableDiskCache points the shared research cache at
// <workspaceRoot>/.nerd/cache/research and hydrates it from whatever survived
// the last run. Passing an empty root disables persistence.
//
// Safe to call more than once; the last call wins.
func EnableDiskCache(workspaceRoot string) error {
	cache := getDefaultCache()
	if strings.TrimSpace(workspaceRoot) == "" {
		return cache.disk.SetDiskDir("")
	}
	dir := filepath.Join(workspaceRoot, ".nerd", "cache", diskCacheDirName)
	if err := cache.disk.SetDiskDir(dir); err != nil {
		return err
	}
	n := cache.disk.loadInto(cache)
	logging.Researcher("Research cache: %d entries restored from %s", n, dir)
	return nil
}

// EnableDiskCacheFromContext resolves the workspace root the same way the file
// tools do and enables persistence under it.
func EnableDiskCacheFromContext(ctx context.Context) error {
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return err
	}
	return EnableDiskCache(root)
}
