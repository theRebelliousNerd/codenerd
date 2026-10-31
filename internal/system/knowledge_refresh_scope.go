package system

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type withoutKnowledgeRefreshKey struct{}

// WithoutKnowledgeRefresh suppresses acquisition before a bounded mission's
// Cortex boots. It only removes background work; it grants no tool permission.
// The factory includes this boot option in cache identity.
func WithoutKnowledgeRefresh(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, withoutKnowledgeRefreshKey{}, true)
}

func knowledgeRefreshDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, _ := ctx.Value(withoutKnowledgeRefreshKey{}).(bool)
	return disabled
}

var errKnowledgeRefreshSuspended = errors.New("knowledge refresh suspended by an active mission")

// refreshAdmission belongs to one Cortex, not to the process or a request's
// observer. A suspension closes admission before canceling and joining a run.
type refreshAdmission struct {
	once   sync.Once
	slot   chan struct{}
	wake   chan struct{}
	mu     sync.Mutex
	pauses int
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (a *refreshAdmission) init() {
	a.once.Do(func() {
		a.slot = make(chan struct{}, 1)
		a.wake = make(chan struct{}, 1)
	})
}

func (a *refreshAdmission) enter(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.init()
	select {
	case a.slot <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	a.mu.Lock()
	if err := ctx.Err(); err != nil {
		a.mu.Unlock()
		<-a.slot
		return nil, nil, err
	}
	if a.pauses != 0 {
		a.mu.Unlock()
		<-a.slot
		return nil, nil, errKnowledgeRefreshSuspended
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	a.cancel, a.done = cancel, done
	a.mu.Unlock()
	return ctx, func() {
		cancel()
		a.mu.Lock()
		a.cancel, a.done = nil, nil
		close(done)
		a.mu.Unlock()
		<-a.slot
	}, nil
}

func (a *refreshAdmission) suspend(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.init()
	a.mu.Lock()
	a.pauses++
	cancel, done := a.cancel, a.done
	a.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			a.mu.Lock()
			a.pauses--
			if a.pauses == 0 {
				select {
				case a.wake <- struct{}{}:
				default:
				}
			}
			a.mu.Unlock()
		})
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// SuspendKnowledgeRefresh is the host's mission admission barrier. It cancels
// and joins this Cortex's active refresh (scheduled or explicit), and refuses
// new refreshes until every returned lease is released. It affects no other
// Cortex or non-refresh work. Cancellation while waiting refuses the mission.
func (c *Cortex) SuspendKnowledgeRefresh(ctx context.Context) (func(), error) {
	if c == nil {
		return nil, fmt.Errorf("mission needs a Cortex knowledge-refresh owner")
	}
	if ctx == nil {
		return nil, fmt.Errorf("mission needs a cancellation context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.knowledgeRefresher == nil {
		return func() {}, nil
	}
	return c.knowledgeRefresher.admission.suspend(ctx)
}
