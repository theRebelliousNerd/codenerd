package broker

import (
	"sync"

	"codenerd/internal/logging"
)

// ReceiptSink consumes receipts. Implementations must not block: they are
// called on the request's completion path.
type ReceiptSink interface {
	Record(r Receipt)
}

// RingSink keeps the most recent receipts in memory for observability.
//
// Bounded on purpose. An unbounded receipt log in a long agent session is a
// memory leak that only shows up in the sessions you most want to keep alive.
type RingSink struct {
	mu       sync.Mutex
	receipts []Receipt
	next     int
	full     bool
}

// NewRingSink returns a sink holding the last size receipts. A non-positive
// size gets a small default rather than a zero-length buffer that silently
// discards everything.
func NewRingSink(size int) *RingSink {
	if size <= 0 {
		size = 256
	}
	return &RingSink{receipts: make([]Receipt, size)}
}

// Record implements ReceiptSink.
func (r *RingSink) Record(rec Receipt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.receipts[r.next] = rec
	r.next = (r.next + 1) % len(r.receipts)
	if r.next == 0 {
		r.full = true
	}
}

// Receipts returns the buffered receipts, oldest first.
func (r *RingSink) Receipts() []Receipt {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.full {
		out := make([]Receipt, r.next)
		copy(out, r.receipts[:r.next])
		return out
	}

	out := make([]Receipt, 0, len(r.receipts))
	out = append(out, r.receipts[r.next:]...)
	out = append(out, r.receipts[:r.next]...)
	return out
}

// Len returns how many receipts are buffered.
func (r *RingSink) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.full {
		return len(r.receipts)
	}
	return r.next
}

// MultiSink fans a receipt out to several sinks.
type MultiSink []ReceiptSink

// Record implements ReceiptSink.
func (m MultiSink) Record(r Receipt) {
	for _, sink := range m {
		if sink != nil {
			sink.Record(r)
		}
	}
}

// LogSink writes a one-line summary of every receipt at debug level, and
// escalates refusals to warn.
//
// A refusal is the one receipt an operator must not have to go looking for: it
// means a turn did not happen, and the reason needs to be in the log next to
// whatever the user saw instead.
type LogSink struct{}

// Record implements ReceiptSink.
func (LogSink) Record(r Receipt) {
	log := logging.Get(logging.CategoryAPI)

	if !r.Decision.Allowed {
		log.Warn("broker REFUSED %s/%s %s (%s): %s | counted=%d conf=%s window=%d headroom=%d",
			r.Purpose, r.Provider, r.Method, r.Decision.Code, r.Decision.Reason,
			r.Estimated.Tokens, r.Estimated.Confidence, r.Decision.Window, r.Decision.Headroom)
		return
	}

	log.Debug("broker %s/%s %s in %dms | est=%d(%s) actual in=%d out=%d err=%.1f%%",
		r.Purpose, r.Provider, r.Method, r.Duration.Milliseconds(),
		r.Estimated.Tokens, r.Estimated.Confidence,
		r.Actual.InputTokens, r.Actual.OutputTokens, r.EstimateErrorPct)
}
