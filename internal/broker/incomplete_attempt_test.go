package broker

import (
	"context"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

func TestSettleKeepsIncompleteAttemptBeforeOrAfterReportedUsage(t *testing.T) {
	for _, before := range []bool{true, false} {
		name := "after_report"
		if before {
			name = "before_report"
		}
		t.Run(name, func(t *testing.T) {
			counter, sink := &recordingCounter{}, &captureSink{}
			ledger := NewLedger(LedgerConfig{})
			c := &core{cfg: Config{Counter: counter, Ledger: ledger, Sink: sink}}
			obs := &callObserver{}
			ctx := usage.WithObserver(context.Background(), obs)
			if before {
				usage.NoteUnknownUsage(ctx)
			}
			usage.RecordFromContext(ctx, usage.Report{Model: "fixture", Usage: types.UsageMetadata{InputTokens: 1000, OutputTokens: 10, Known: types.UsageTokens}})
			if !before {
				usage.NoteUnknownUsage(ctx)
			}
			c.settle(Receipt{Started: time.Now(), Estimated: Count{Tokens: 1000}}, &Request{Model: "fixture", User: "request"}, obs, nil, nil)
			receipt, ok := sink.last()
			if !ok || receipt.UsageStatus != UsagePartial || receipt.Actual.InputTokens != 1000 || receipt.Actual.OutputTokens != 10 || receipt.Attempts != 1 {
				t.Fatalf("incomplete attempt lost or billed twice: %+v", receipt)
			}
			if got := ledger.Total(); got.InputTokens != 1000 || got.OutputTokens != 10 || got.Calls != 1 {
				t.Fatalf("known spend changed: %+v", got)
			}
			if len(counter.observed) != 0 {
				t.Fatalf("lower-bound usage trained calibration: %+v", counter.observed)
			}
		})
	}
}

func TestCallObserverConcurrentIncompleteAttemptsAndReports(t *testing.T) {
	const reports = 32
	obs := &callObserver{}
	ctx := usage.WithObserver(context.Background(), obs)
	var workers sync.WaitGroup
	for range reports {
		workers.Go(func() { usage.NoteUnknownUsage(ctx) })
		workers.Go(func() {
			usage.RecordFromContext(ctx, usage.Report{Usage: types.UsageMetadata{InputTokens: 2, OutputTokens: 1, Known: types.UsageTokens}})
			obs.result()
		})
	}
	workers.Wait()
	got, count := obs.result()
	if count != reports || got.InputTokens != 2*reports || got.OutputTokens != reports || !got.Partial {
		t.Fatalf("concurrent uncertainty or counts lost: usage=%+v reports=%d", got, count)
	}
}
