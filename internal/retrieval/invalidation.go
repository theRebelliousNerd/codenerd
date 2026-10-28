package retrieval

import (
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// CACHE MAINTENANCE
// =============================================================================
//
// KeywordHitCache entries live for five minutes. Within one session an agent
// routinely edits a file and then reasons about it again, and the second pass
// used to be answered from a cache built before the edit.
//
// The change signal is taken from Mangle rather than from a Go callback
// chain. Every write already lands in the EDB as file_written/4 (VirtualStore
// and TransactionManager both assert it), every out-of-band change the
// transaction manager sees as file_modified_externally/1, and every delete as
// file_deleted/1, so the kernel is the single authority on what changed and
// the retriever only has to read it.
//
// A change does not just stale the results that named the changed file. An
// empty cached result says no member holds the keyword; a write that adds it
// to any file falsifies that, and so does one that adds it to a second file
// beside the one the result names. Dropping only the results whose hits name
// a changed file (the rule until 2026-10-27) left both standing. So every
// cached result is maintained through the change (KeywordHitCache.applyDelta):
// the changed files' old hits leave, and the changed files are scanned for the
// keyword as they are now.

// invalidationPredicates are the EDB signals that a workspace file changed.
var invalidationPredicates = []string{"file_written", "file_modified_externally", "file_deleted"}

// membershipFiles change which files are members, not just what one holds:
// a change to one is a new domain, and every cached result goes.
var membershipFiles = map[string]bool{".gitignore": true, ".ignore": true}

// changeMatcher matches a hit in any of the given changed paths. Matching is
// suffix-based on the normalized path because the cache stores the scanner's
// paths (rooted at the retriever's workDir) while the kernel stores whatever
// the writer used; a workspace-relative path and its absolute form must match
// each other.
func changeMatcher(paths []string) func(KeywordHit) bool {
	needles := make([]string, 0, len(paths))
	for _, p := range paths {
		n := strings.ToLower(normalizePathSeparators(filepath.Clean(p)))
		if n != "" && n != "." {
			needles = append(needles, n)
		}
	}
	return func(hit KeywordHit) bool {
		candidate := strings.ToLower(normalizePathSeparators(filepath.Clean(hit.FilePath)))
		for _, n := range needles {
			if candidate == n || strings.HasSuffix(candidate, "/"+n) || strings.HasSuffix(n, "/"+candidate) {
				return true
			}
		}
		return false
	}
}

// scannerPath maps a changed path as the kernel recorded it onto the form the
// native walk produces (rooted at workDir), and reports whether it lies in the
// search domain at all. A path outside the workspace changes no result.
func (r *SparseRetriever) scannerPath(p string) (string, bool) {
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		return filepath.Join(r.workDir, clean), true
	}
	absRoot, err := filepath.Abs(r.workDir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(r.workDir, rel), true
}

// ApplyFileChanges brings every cached keyword result up to date with the
// given changed paths (written, modified or deleted), and returns how many
// cached results it changed or dropped:
//
//	H_next(q) = (H_prev(q) minus hits in changed files) union Search(q, changed files)
//
// Each changed member file is read once for every cached keyword. A change
// that can move membership (an ignore file, git metadata) or that the
// external backend's semantics cannot be maintained under empties the cache
// instead; so does a failure to ask the workspace which files are members.
// An overflowed result is dropped rather than maintained.
func (r *SparseRetriever) ApplyFileChanges(paths ...string) int {
	if len(paths) == 0 {
		return 0
	}
	var changed []string
	wide := r.backend != nil
	for _, p := range paths {
		scanned, inDomain := r.scannerPath(p)
		if !inDomain {
			continue
		}
		base := filepath.Base(scanned)
		slashed := "/" + filepath.ToSlash(scanned) + "/"
		if membershipFiles[base] || strings.Contains(slashed, "/.git/") {
			wide = true
		}
		changed = append(changed, scanned)
	}
	if len(changed) == 0 {
		return 0
	}
	if wide {
		dropped := r.cache.Len()
		r.cache.Clear()
		return dropped
	}

	keywords := r.cache.beginDelta()
	if len(keywords) == 0 {
		return 0
	}
	mem, err := membershipFor(r.workDir)
	if err != nil {
		dropped := r.cache.Len()
		r.cache.Clear()
		logging.Context("SparseRetriever: cache emptied; membership unavailable for a delta: %v", err)
		return dropped
	}
	lowered := make([][]byte, len(keywords))
	for i, kw := range keywords {
		lowered[i] = []byte(strings.ToLower(kw))
	}
	added := make(map[string][]KeywordHit)
	seen := make(map[string]bool, len(changed))
	for _, path := range changed {
		if seen[path] {
			continue
		}
		seen[path] = true
		// A deleted or now-excluded file contributes no hits; its old ones
		// still leave.
		if member, admErr := mem.Admit(path, false); admErr != nil || !member || r.excludedPath(path) {
			continue
		}
		perKeyword, ok := r.scanFile(path, keywords, lowered, nil)
		if !ok {
			continue
		}
		for i, hits := range perKeyword {
			if len(hits) > 0 {
				added[keywords[i]] = append(added[keywords[i]], hits...)
			}
		}
	}
	return r.cache.applyDelta(keywords, changeMatcher(changed), added)
}

// fileWriteEventID identifies one file_written fact, including its path,
// content hash, session, and second-precision timestamp. The kernel stores
// file_written as a set, so an exact replay has the same identity while a
// distinct same-second write remains observable.
func fileWriteEventID(path, hash, session string, timestamp int64) string {
	return fmt.Sprintf("%q\x00%q\x00%q\x00%d", path, hash, session, timestamp)
}

// InvalidateFromKernel reads the change log out of the EDB, applies every
// change it has not applied yet (ApplyFileChanges), and returns how many new
// change events it found.
//
// Writes newer than the cursor are considered, along with unseen writes at the
// cursor timestamp. Writers use Unix-second timestamps, so exact event
// identities for the current timestamp are retained to distinguish a new
// same-second write from an exact replay. file_modified_externally and
// file_deleted carry no timestamp: one is new when it was not in the kernel at
// the previous pass, so a fact retracted and asserted again is seen again.
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
	unstamped := make(map[string]struct{})

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
			id := pred + "\x00" + path
			unstamped[id] = struct{}{}
			if _, before := r.lastUnstamped[id]; before {
				continue
			}
			paths = append(paths, path)
		}
	}
	r.lastUnstamped = unstamped

	if len(paths) == 0 {
		return 0
	}

	changed := r.ApplyFileChanges(paths...)

	if newest > cursor {
		r.lastWriteCursor = newest
		r.lastWriteEvents = newestEvents
	} else if len(newEventsAtCursor) > 0 {
		for eventID := range newEventsAtCursor {
			seenAtCursor[eventID] = struct{}{}
		}
		r.lastWriteEvents = seenAtCursor
	}

	logging.Context("SparseRetriever: %d workspace change(s) applied to the keyword cache, %d cached result(s) changed", len(paths), changed)
	return len(paths)
}
