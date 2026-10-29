package tools

import (
	"context"
	"sync"
)

// Consumption is the material one tool call scanned to produce its result:
// bytes read and items (files, index entries, rows) visited. It is the
// numerator of a tool's condensation ratio, with the bytes the call returned
// as the denominator.
//
// A tool reports what it truly scanned or reports nothing. Nothing is a
// legitimate answer: the ledger records the call's consumption as unknown, and
// no rule reads an unknown as a small number.
type Consumption struct {
	Bytes int64
	Items int64
}

type consumptionKey struct{}

type consumptionLog struct {
	mu       sync.Mutex
	reported bool
	total    Consumption
}

// WithConsumptionLog returns a context in which ReportConsumed records, and a
// function that returns the sum of what was reported and whether anything was.
func WithConsumptionLog(ctx context.Context) (context.Context, func() (Consumption, bool)) {
	log := &consumptionLog{}
	return context.WithValue(ctx, consumptionKey{}, log), func() (Consumption, bool) {
		log.mu.Lock()
		defer log.mu.Unlock()
		return log.total, log.reported
	}
}

// ReportConsumed adds scanned material to the call's account. Reports are
// summed, so a tool that scans in several passes reports each pass. A report of
// zero is a measured zero (the tool scanned nothing); a tool that measured
// nothing does not call this. Without a log in the context it records nothing.
func ReportConsumed(ctx context.Context, bytes, items int64) {
	log, ok := ctx.Value(consumptionKey{}).(*consumptionLog)
	if !ok {
		return
	}
	log.mu.Lock()
	log.reported = true
	log.total.Bytes += max(bytes, 0)
	log.total.Items += max(items, 0)
	log.mu.Unlock()
}
