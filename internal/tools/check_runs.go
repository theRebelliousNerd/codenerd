package tools

import (
	"context"
	"sync"
)

// An AcceptanceRun is one campaign acceptance command the tool layer actually
// started: the argv it ran and the exit code it ended with. It is a separate
// receipt from TestRun on purpose: a doc-checker pass must never satisfy the
// /test_run gate, which reads only TestRun.
type AcceptanceRun struct {
	Argv     []string
	ExitCode int
}

type acceptanceRunLogKey struct{}

type acceptanceRunLog struct {
	mu   sync.Mutex
	runs []AcceptanceRun
}

// WithAcceptanceRunLog returns a context in which RecordAcceptanceRun
// records, and a function that returns what was recorded.
func WithAcceptanceRunLog(ctx context.Context) (context.Context, func() []AcceptanceRun) {
	log := &acceptanceRunLog{}
	return context.WithValue(ctx, acceptanceRunLogKey{}, log), func() []AcceptanceRun {
		log.mu.Lock()
		defer log.mu.Unlock()
		return append([]AcceptanceRun(nil), log.runs...)
	}
}

// RecordAcceptanceRun records that an acceptance command was started, where
// the process is started -- never where a check is only planned. Without a
// log in the context it records nothing.
func RecordAcceptanceRun(ctx context.Context, run AcceptanceRun) {
	log, ok := ctx.Value(acceptanceRunLogKey{}).(*acceptanceRunLog)
	if !ok {
		return
	}
	log.mu.Lock()
	log.runs = append(log.runs, run)
	log.mu.Unlock()
}
