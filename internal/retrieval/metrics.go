package retrieval

import (
	"fmt"
	"sync/atomic"
	"time"
)

// =============================================================================
// METRICS
// =============================================================================
//
// The retriever's cost was invisible: a keyword sweep reads every non-excluded
// file in the workspace, and the only evidence it left behind was a "found N
// total hits" line. When a session felt slow there was no way to tell a cold
// cache from a hot one, or a repository of 3,000 files from one of 300,000.
// These counters are process-cheap (one atomic add per file) and are surfaced by
// `nerd retrieve --stats` and the seed path's context log.

type retrieverMetrics struct {
	searches     atomic.Int64
	cacheHits    atomic.Int64
	cacheMisses  atomic.Int64
	filesWalked  atomic.Int64
	filesScanned atomic.Int64
	filesSkipped atomic.Int64
	hits         atomic.Int64
	timeouts     atomic.Int64
	errors       atomic.Int64
	searchNanos  atomic.Int64
}

// RetrieverMetrics is a point-in-time snapshot of retriever activity.
type RetrieverMetrics struct {
	// Searches is the number of single-keyword sweeps started.
	Searches int64
	// CacheHits / CacheMisses count keyword lookups served from the hit cache.
	CacheHits   int64
	CacheMisses int64
	// FilesWalked is directory entries offered to the workers; FilesScanned is
	// those actually read; FilesSkipped is those rejected as oversized, binary
	// or unreadable.
	FilesWalked  int64
	FilesScanned int64
	FilesSkipped int64
	// Hits is the total KeywordHit records produced.
	Hits int64
	// Timeouts and Errors count sweeps that ended early.
	Timeouts int64
	Errors   int64
	// TotalSearchTime is wall time summed across sweeps; concurrent sweeps
	// overlap, so it exceeds elapsed time under parallelism.
	TotalSearchTime time.Duration
}

// CacheHitRate returns the fraction of keyword lookups served from cache, or 0
// when nothing has been looked up.
func (m RetrieverMetrics) CacheHitRate() float64 {
	total := m.CacheHits + m.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(m.CacheHits) / float64(total)
}

// MeanSearchTime returns the average duration of one keyword sweep.
func (m RetrieverMetrics) MeanSearchTime() time.Duration {
	if m.Searches == 0 {
		return 0
	}
	return m.TotalSearchTime / time.Duration(m.Searches)
}

// String renders the snapshot as a single log/CLI line.
func (m RetrieverMetrics) String() string {
	return fmt.Sprintf(
		"searches=%d cache_hit_rate=%.0f%% mean_search=%s walked=%d scanned=%d skipped=%d hits=%d timeouts=%d errors=%d",
		m.Searches, m.CacheHitRate()*100, m.MeanSearchTime().Round(time.Millisecond),
		m.FilesWalked, m.FilesScanned, m.FilesSkipped, m.Hits, m.Timeouts, m.Errors)
}

// Metrics returns a snapshot of retriever activity since construction.
func (r *SparseRetriever) Metrics() RetrieverMetrics {
	return RetrieverMetrics{
		Searches:        r.metrics.searches.Load(),
		CacheHits:       r.metrics.cacheHits.Load(),
		CacheMisses:     r.metrics.cacheMisses.Load(),
		FilesWalked:     r.metrics.filesWalked.Load(),
		FilesScanned:    r.metrics.filesScanned.Load(),
		FilesSkipped:    r.metrics.filesSkipped.Load(),
		Hits:            r.metrics.hits.Load(),
		Timeouts:        r.metrics.timeouts.Load(),
		Errors:          r.metrics.errors.Load(),
		TotalSearchTime: time.Duration(r.metrics.searchNanos.Load()),
	}
}
