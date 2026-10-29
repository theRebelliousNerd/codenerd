package core

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

// orientationProjectionCache memoizes parsing a projection file that does not
// change between the many workspace kernels one boot constructs. The key is
// the file's stat identity; Refresh rewrites the file atomically, which
// changes it. Callers receive clones so no kernel can mutate another's rows.
type orientationProjectionCacheEntry struct {
	size  int64
	mod   int64
	decls string
	facts []Fact
}

var (
	orientationProjectionCacheMu sync.Mutex
	orientationProjectionCache   = map[string]orientationProjectionCacheEntry{}
)

func rememberOrientationProjection(path string, before os.FileInfo, statErr error, bodyLen int, decls string, facts []Fact) {
	if statErr != nil || before == nil || before.Size() != int64(bodyLen) {
		return
	}
	after, err := os.Stat(path)
	if err != nil || after.Size() != before.Size() || after.ModTime().UnixNano() != before.ModTime().UnixNano() {
		return
	}
	orientationProjectionCacheMu.Lock()
	orientationProjectionCache[path] = orientationProjectionCacheEntry{
		size: before.Size(), mod: before.ModTime().UnixNano(), decls: decls, facts: cloneOrientationFacts(facts),
	}
	orientationProjectionCacheMu.Unlock()
}

func cloneOrientationFacts(rows []Fact) []Fact {
	if rows == nil {
		return nil
	}
	out := make([]Fact, len(rows))
	for i, row := range rows {
		args := make([]any, len(row.Args))
		copy(args, row.Args)
		out[i] = Fact{Predicate: row.Predicate, Args: args}
	}
	return out
}

// loadOrientationProjection splits the orientation projection (.nerd/
// orientation/orientation.mg) into its declarations and its rows. The rows are
// returned as EDB facts, not program text: an incremental scan refreshes them
// with an exact retract (orient.RefreshResult.Apply), and a row compiled into
// the program is a clause no retract can remove, so every refresh would stack
// a new head, document digest and role on top of the stale ones.
func loadOrientationProjection(path string) (string, []Fact, error) {
	before, statErr := os.Stat(path)
	if statErr == nil {
		orientationProjectionCacheMu.Lock()
		if hit, ok := orientationProjectionCache[path]; ok &&
			hit.size == before.Size() && hit.mod == before.ModTime().UnixNano() {
			decls, facts := hit.decls, cloneOrientationFacts(hit.facts)
			orientationProjectionCacheMu.Unlock()
			return decls, facts, nil
		}
		orientationProjectionCacheMu.Unlock()
	}
	// The seven shards of a domain cortex boot at once and all miss together:
	// one parse serves them (measured 2026-10-27: seven concurrent parses of
	// the same projection, 7.9 s of CPU). Each caller gets its own clone.
	v, err, _ := orientationProjectionFlight.Do(path, func() (any, error) {
		decls, facts, err := parseOrientationProjection(path, before, statErr)
		if err != nil {
			return nil, err
		}
		return orientationProjectionCacheEntry{decls: decls, facts: facts}, nil
	})
	if err != nil {
		return "", nil, err
	}
	entry := v.(orientationProjectionCacheEntry)
	return entry.decls, cloneOrientationFacts(entry.facts), nil
}

// orientationProjectionFlight coalesces concurrent parses of one projection.
var orientationProjectionFlight singleflight.Group

// parseOrientationProjection reads and parses the projection and remembers it.
func parseOrientationProjection(path string, before os.FileInfo, statErr error) (string, []Fact, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var decls, rows strings.Builder
	rowCount := 0
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "Decl "):
			decls.WriteString(trimmed)
			decls.WriteString("\n")
		default:
			rows.WriteString(trimmed)
			rows.WriteString("\n")
			rowCount++
		}
	}
	if rowCount == 0 {
		rememberOrientationProjection(path, before, statErr, len(body), decls.String(), nil)
		return decls.String(), nil, nil
	}
	facts, err := ParseFactsFromString(rows.String())
	if err != nil {
		return "", nil, fmt.Errorf("orientation projection %s: %w", path, err)
	}
	if len(facts) != rowCount {
		return "", nil, fmt.Errorf("orientation projection %s: %d of %d rows are ground facts; the projection carries rows only", path, len(facts), rowCount)
	}
	rememberOrientationProjection(path, before, statErr, len(body), decls.String(), facts)
	return decls.String(), facts, nil
}
