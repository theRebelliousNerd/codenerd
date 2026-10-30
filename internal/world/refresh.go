package world

import (
	"context"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

// RefreshStats describes completed refreshes through the production scan/apply
// path. Durations include applying the result, and exclude time waiting on a
// concurrent refresh. Counters make duplicate full scans observable.
type RefreshStats struct {
	FullScans      uint64
	DeltaScans     uint64
	UnchangedScans uint64
	LastDuration   time.Duration
}

// Refresh serializes the scan and application together: two consumers must not
// advance the shared file cache and then apply their generations out of order.
// It does not create a watcher, agent, or independent parser/cache.
func (s *Scanner) Refresh(ctx context.Context, root string, db *store.LocalStore, kernel types.Kernel, opts IncrementalOptions) (*IncrementalResult, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	start := time.Now()
	result, err := s.ScanWorkspaceIncremental(ctx, root, db, opts)
	if err != nil {
		return nil, err
	}
	if err := ApplyIncrementalResult(kernel, result); err != nil {
		return nil, err
	}
	if result != nil {
		switch {
		case result.Full:
			s.refreshStats.FullScans++
		case result.Unchanged:
			s.refreshStats.UnchangedScans++
		default:
			s.refreshStats.DeltaScans++
		}
	}
	s.refreshStats.LastDuration = time.Since(start)
	return result, nil
}

func (s *Scanner) RefreshStats() RefreshStats {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.refreshStats
}
