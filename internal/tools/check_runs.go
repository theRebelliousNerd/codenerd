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
	// OKExitCodes belongs to this invocation, never a later declaration.
	OKExitCodes []int
	// ExecutionFailed disqualifies an exit returned with an execution error
	// or cancellation. ExitCode remains the raw observation.
	ExecutionFailed bool
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
		out := make([]AcceptanceRun, len(log.runs))
		for i, run := range log.runs {
			out[i] = cloneAcceptanceRun(run)
		}
		return out
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
	log.runs = append(log.runs, cloneAcceptanceRun(run))
	log.mu.Unlock()
}

func cloneAcceptanceRun(run AcceptanceRun) AcceptanceRun {
	run.Argv = append([]string(nil), run.Argv...)
	run.OKExitCodes = append([]int(nil), run.OKExitCodes...)
	return run
}
