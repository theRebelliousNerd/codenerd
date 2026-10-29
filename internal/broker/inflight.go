package broker

import (
	"context"
	"sync"
)

// inflight counts the calls the broker admitted and has not yet settled.
//
// Observing a stream's terminal proves that call's accounting finished, but a
// consumer that walks away (cancellation, shutdown) never observes it, and the
// settling goroutine keeps running after its channels close. A task-cost
// exporter or a graceful shutdown that reads the receipt log must therefore
// wait for accounting, not for channel closure. Drain is that wait.
//
// A call is in flight from a successful admit until settle emits its receipt.
// A refused call is never in flight: it costs nothing and settles at once.
var inflight tracker

type tracker struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // closed when n returns to zero; replaced on the next begin
}

func (t *tracker) begin() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n == 0 {
		t.idle = make(chan struct{})
	}
	t.n++
}

func (t *tracker) end() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n == 0 {
		return
	}
	t.n--
	if t.n == 0 {
		close(t.idle)
	}
}

// Pending is how many admitted calls have not yet settled.
func Pending() int {
	inflight.mu.Lock()
	defer inflight.mu.Unlock()
	return inflight.n
}

// Drain blocks until every call admitted so far has settled its receipt, or
// ctx ends. It returns ctx's error when accounting was still pending; the
// caller then holds observed totals that are lower bounds, and must say so
// rather than report the task as cheap. Streaming stays responsive: nothing
// waits on persistence per chunk, only this explicit barrier does.
func Drain(ctx context.Context) error {
	for {
		inflight.mu.Lock()
		if inflight.n == 0 {
			inflight.mu.Unlock()
			return nil
		}
		idle := inflight.idle
		inflight.mu.Unlock()
		select {
		case <-idle:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
