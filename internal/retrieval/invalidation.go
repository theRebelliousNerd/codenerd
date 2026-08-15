package retrieval

import (
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// CACHE INVALIDATION
// =============================================================================
//
// KeywordHitCache entries live for five minutes. Within one session an agent
// routinely edits a file and then reasons about it again, and until now the
// second pass was answered from a cache built before the edit: the retriever
// confidently reported hits at line numbers that no longer existed, or missed a
// symbol the edit had just introduced.
//
// The invalidation signal is taken from Mangle rather than from a Go callback
// chain. Every write already lands in the EDB as file_written/4 (VirtualStore
// and TransactionManager both assert it) and every out-of-band change as
// file_modified_externally/1, so the kernel is the single authority on what
// changed and the retriever only has to read it. Nothing has to be wired into
// the write path, which is exactly why this survives a new writer being added.

// invalidationPredicates are the EDB signals that a workspace file changed.
var invalidationPredicates = []string{"file_written", "file_modified_externally"}

// InvalidateFiles drops every cached keyword result that references any of the
// given paths, and reports how many cache entries were dropped.
//
// Matching is suffix-based on the normalized path because the cache stores the
// scanner's paths (rooted at the retriever's workDir) while the kernel stores
// whatever the writer used; a workspace-relative path and its absolute form must
// invalidate each other.
func (r *SparseRetriever) InvalidateFiles(paths ...string) int {
	if len(paths) == 0 {
		return 0
	}
	needles := make([]string, 0, len(paths))
	for _, p := range paths {
		n := strings.ToLower(normalizePathSeparators(filepath.Clean(p)))
		if n != "" && n != "." {
			needles = append(needles, n)
		}
	}
	if len(needles) == 0 {
		return 0
	}

	return r.cache.InvalidateWhere(func(hit KeywordHit) bool {
		candidate := strings.ToLower(normalizePathSeparators(filepath.Clean(hit.FilePath)))
		for _, n := range needles {
			if candidate == n || strings.HasSuffix(candidate, "/"+n) || strings.HasSuffix(n, "/"+candidate) {
				return true
			}
		}
		return false
	})
}

// InvalidateAll empties the keyword hit cache. Use it when the workspace moved
// under the retriever wholesale (branch switch, checkout, bulk patch).
func (r *SparseRetriever) InvalidateAll() {
	r.cache.Clear()
	r.mu.Lock()
	r.lastWriteCursor = 0
	r.mu.Unlock()
}

// InvalidateFromKernel reads the write log out of the EDB and drops the cache
// entries it affects, returning the number of entries dropped.
//
// Only writes newer than the last call are considered: the cursor is the
// file_written timestamp (unix seconds), kept under r.mu so two concurrent seeds
// cannot both claim the same window and leave the other half of the writes
// unapplied. file_modified_externally carries no timestamp, so it is always
// replayed — it is rare and dropping a stale entry twice costs nothing.
func (r *SparseRetriever) InvalidateFromKernel(src FactSource) int {
	if src == nil {
		return 0
	}

	r.mu.RLock()
	cursor := r.lastWriteCursor
	r.mu.RUnlock()

	var paths []string
	newest := cursor

	for _, pred := range invalidationPredicates {
		facts, err := src.Query(pred)
		if err != nil || len(facts) == 0 {
			continue
		}
		for _, f := range facts {
			if len(f.Args) == 0 {
				continue
			}
			// Names and strings both read back as plain Go strings; asserting
			// types.MangleAtom here would be a branch that never runs.
			path := types.ExtractString(f.Args[0])
			if path == "" {
				continue
			}
			if pred == "file_written" && len(f.Args) >= 4 {
				ts, ok := types.ExtractInt64(f.Args[3])
				if ok {
					if ts <= cursor {
						continue
					}
					if ts > newest {
						newest = ts
					}
				}
			}
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return 0
	}

	dropped := r.InvalidateFiles(paths...)

	r.mu.Lock()
	if newest > r.lastWriteCursor {
		r.lastWriteCursor = newest
	}
	r.mu.Unlock()

	if dropped > 0 {
		logging.Context("SparseRetriever: invalidated %d cached keyword results after %d workspace writes", dropped, len(paths))
	}
	return dropped
}

// InvalidateWhere removes every cache entry holding a hit the predicate accepts,
// returning the number of entries removed. A keyword's result set is all-or-
// nothing: a stale file inside it makes the ranking for that keyword wrong, not
// just that one row.
func (c *KeywordHitCache) InvalidateWhere(match func(KeywordHit) bool) int {
	if match == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	for key, entry := range c.entries {
		affected := false
		for _, hit := range entry.hits {
			if match(hit) {
				affected = true
				break
			}
		}
		if !affected {
			continue
		}
		c.list.Remove(entry.element)
		delete(c.entries, key)
		removed++
	}
	return removed
}
