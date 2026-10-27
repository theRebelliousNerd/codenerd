package retrieval

import (
	"fmt"
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

// fileWriteEventID identifies one file_written fact, including its path,
// content hash, session, and second-precision timestamp. The kernel stores
// file_written as a set, so an exact replay has the same identity while a
// distinct same-second write remains observable.
func fileWriteEventID(path, hash, session string, timestamp int64) string {
	return fmt.Sprintf("%q\x00%q\x00%q\x00%d", path, hash, session, timestamp)
}

// InvalidateAll empties the keyword hit cache. Use it when the workspace moved
// under the retriever wholesale (branch switch, checkout, bulk patch).
func (r *SparseRetriever) InvalidateAll() {
	r.cache.Clear()
	r.mu.Lock()
	r.lastWriteCursor = 0
	r.lastWriteEvents = nil
	r.mu.Unlock()
}

// InvalidateFromKernel reads the write log out of the EDB and drops the cache
// entries it affects, returning the number of entries dropped.
//
// Writes newer than the cursor are considered, along with unseen writes at the
// cursor timestamp. Writers use Unix-second timestamps, so exact event
// identities for the current timestamp are retained to distinguish a new
// same-second write from an exact replay. file_modified_externally carries no
// timestamp, so it is always replayed — it is rare and dropping a stale entry
// twice costs nothing.
func (r *SparseRetriever) InvalidateFromKernel(src FactSource) int {
	if src == nil {
		return 0
	}

	// Serialize query, invalidation, and cursor advancement. Otherwise parallel
	// seeds could both observe the same new event before either records it.
	r.mu.Lock()
	defer r.mu.Unlock()
	cursor := r.lastWriteCursor
	seenAtCursor := r.lastWriteEvents
	if seenAtCursor == nil {
		seenAtCursor = make(map[string]struct{})
	}

	var paths []string
	newest := cursor
	newestEvents := make(map[string]struct{})
	newEventsAtCursor := make(map[string]struct{})

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
					if ts < cursor {
						continue
					}
					eventID := fileWriteEventID(
						path,
						types.ExtractString(f.Args[1]),
						types.ExtractString(f.Args[2]),
						ts,
					)
					if ts == cursor {
						if _, alreadySeen := seenAtCursor[eventID]; alreadySeen {
							continue
						}
						if _, alreadyObserved := newEventsAtCursor[eventID]; alreadyObserved {
							continue
						}
						newEventsAtCursor[eventID] = struct{}{}
						paths = append(paths, path)
						continue
					}
					if ts > newest {
						newest = ts
						newestEvents = make(map[string]struct{})
					}
					if ts == newest {
						if _, alreadyObserved := newestEvents[eventID]; alreadyObserved {
							continue
						}
						newestEvents[eventID] = struct{}{}
					}
					paths = append(paths, path)
					continue
				}
			}
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return 0
	}

	dropped := r.InvalidateFiles(paths...)

	if newest > cursor {
		r.lastWriteCursor = newest
		r.lastWriteEvents = newestEvents
	} else if len(newEventsAtCursor) > 0 {
		for eventID := range newEventsAtCursor {
			seenAtCursor[eventID] = struct{}{}
		}
		r.lastWriteEvents = seenAtCursor
	}

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
