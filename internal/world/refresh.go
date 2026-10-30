package world

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

// RefreshStats describes the production scan/apply/publish path. LastDuration
// includes publication and replay, but excludes waiting for another refresh.
type RefreshStats struct {
	FullScans           uint64
	DeltaScans          uint64
	UnchangedScans      uint64
	PublicationFailures uint64
	PublicationPending  bool
	LastDuration        time.Duration
}

type pendingRefresh struct {
	result *IncrementalResult
	kernel types.Kernel
}

// SetRefreshPublisher installs the session's projection before its first scan.
// The publisher runs under the scanner's ownership, after kernel application.
// It must be idempotent and must not call Refresh, WithRefreshLock,
// RefreshStats or SetRefreshPublisher recursively. All refresh consumers use
// this same publisher; none may publish the returned delta again after return.
func (s *Scanner) SetRefreshPublisher(publish func(context.Context, *IncrementalResult) error) error {
	_ = s.lockRefresh(context.Background())
	defer s.unlockRefresh()
	if s.pendingRefresh != nil {
		return fmt.Errorf("cannot replace world publisher while a refresh is pending")
	}
	s.refreshPublisher = publish
	return nil
}

func (s *Scanner) lockRefresh(ctx context.Context) error {
	s.refreshOnce.Do(func() { s.refreshGate = make(chan struct{}, 1) })
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.refreshGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			s.unlockRefresh()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scanner) unlockRefresh() { <-s.refreshGate }

// WithRefreshLock admits a non-incremental world update (deep or partial scan)
// under the same observation-to-publication ownership. The callback must cover
// observation, cache/kernel updates and publication, and must not reenter this
// scanner. An older failed refresh is replayed before the callback can run.
func (s *Scanner) WithRefreshLock(ctx context.Context, update func() error) error {
	if err := s.lockRefresh(ctx); err != nil {
		return err
	}
	defer s.unlockRefresh()
	if err := s.finishPendingRefresh(ctx); err != nil {
		return err
	}
	return update()
}

// Refresh serializes observation, kernel application and publication. A delayed
// older graph transaction cannot follow a newer scan's retirement. At most one
// failed result is retained; replay must succeed before another scan advances
// the cache, even if the source would otherwise be unchanged. This receipt is
// in memory, not a crash-durable outbox or a database/kernel transaction.
func (s *Scanner) Refresh(ctx context.Context, root string, db *store.LocalStore, kernel types.Kernel, opts IncrementalOptions) (*IncrementalResult, error) {
	if err := s.lockRefresh(ctx); err != nil {
		return nil, err
	}
	defer s.unlockRefresh()
	start := time.Now()
	defer func() { s.refreshStats.LastDuration = time.Since(start) }()
	if err := s.finishPendingRefresh(ctx); err != nil {
		return nil, err
	}
	result, err := s.ScanWorkspaceIncremental(ctx, root, db, opts)
	if err != nil || result == nil {
		return result, err
	}
	switch {
	case result.Full:
		s.refreshStats.FullScans++
	case result.Unchanged:
		s.refreshStats.UnchangedScans++
	default:
		s.refreshStats.DeltaScans++
	}
	s.pendingRefresh = &pendingRefresh{result: result, kernel: kernel}
	if err := s.finishPendingRefresh(ctx); err != nil {
		// The pending result remains scanner-owned; callers receive no mutable
		// delta to publish independently while its retry is outstanding.
		return nil, err
	}
	return result, nil
}

func (s *Scanner) finishPendingRefresh(ctx context.Context) error {
	pending := s.pendingRefresh
	if pending == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	res := pending.result
	if !res.WorldApplied {
		if err := ApplyIncrementalResult(pending.kernel, res); err != nil {
			return fmt.Errorf("apply world refresh: %w", err)
		}
		res.WorldApplied = true
	}
	if s.refreshPublisher != nil && !res.Unchanged {
		if err := s.refreshPublisher(ctx, res); err != nil {
			s.refreshStats.PublicationFailures++
			return fmt.Errorf("publish world refresh (world applied; publication pending): %w", err)
		}
		res.Published = true
	}
	s.pendingRefresh = nil
	return nil
}

func (s *Scanner) RefreshStats() RefreshStats {
	_ = s.lockRefresh(context.Background())
	defer s.unlockRefresh()
	stats := s.refreshStats
	stats.PublicationPending = s.pendingRefresh != nil && s.pendingRefresh.result.WorldApplied
	return stats
}
