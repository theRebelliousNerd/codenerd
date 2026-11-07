package campaign

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// promotionStubKernel serves promote_to_long_term facts for applyLearnings.
type promotionStubKernel struct {
	core.Kernel
	facts []core.Fact
	err   error
}

func (s *promotionStubKernel) Query(predicate string) ([]core.Fact, error) {
	if s.err != nil {
		return nil, s.err
	}
	if predicate != "promote_to_long_term" {
		return nil, nil
	}
	return s.facts, nil
}

func TestApplyLearnings_RecordsPromotionFactValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		facts        []core.Fact
		taskDesc     string
		result       any
		queryErr     bool
		wantCount    int
		wantPattern  string
		wantFactFrag string
	}{
		{
			name: "happy records pattern and fact value",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/success_pattern"), "retry with backoff"}},
			},
			taskDesc:     "fix flaky test",
			result:       map[string]string{"out": "ok"},
			wantCount:    1,
			wantPattern:  "/success_pattern",
			wantFactFrag: "retry with backoff",
		},
		{
			name:         "edge empty facts records nothing",
			facts:        nil,
			taskDesc:     "nothing to learn",
			wantCount:    0,
			wantPattern:  "",
			wantFactFrag: "",
		},
		{
			name: "query error records nothing",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/success_pattern"), "not returned"}},
			},
			taskDesc:  "query failed",
			queryErr:  true,
			wantCount: 0,
		},
		{
			name: "nil result still records fact value",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/cache_key"), "cache fact value"}},
			},
			taskDesc:     "task with nil result",
			result:       nil,
			wantCount:    1,
			wantPattern:  "/cache_key",
			wantFactFrag: "cache fact value",
		},
		{
			name: "empty pattern with fact value",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom(""), "only value matters"}},
			},
			taskDesc:     "empty pattern",
			wantCount:    1,
			wantPattern:  "",
			wantFactFrag: "only value matters",
		},
		{
			name: "audited two-arg promotion keeps second arg",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/deploy_tip"), "blue-green deploy fact"}},
			},
			taskDesc:     "deploy task",
			result:       "done",
			wantCount:    1,
			wantPattern:  "/deploy_tip",
			wantFactFrag: "blue-green deploy fact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var qerr error
			if tt.queryErr {
				qerr = context.DeadlineExceeded
			}
			stub := &promotionStubKernel{facts: tt.facts, err: qerr}
			c := &Campaign{ID: "camp-learn"}
			o := &Orchestrator{kernel: stub, campaign: c}
			task := &Task{ID: "task-1", Description: tt.taskDesc}
			ctx := context.Background()
			origLen := len(o.campaign.Learnings)
			t.Cleanup(func() { _ = origLen })

			o.applyLearnings(ctx, task, tt.result)

			if len(o.campaign.Learnings) != tt.wantCount {
				t.Fatalf("learnings count = %d, want %d: %v", len(o.campaign.Learnings), tt.wantCount, o.campaign.Learnings)
			}
			if tt.wantCount == 0 {
				return
			}
			got := o.campaign.Learnings[0]
			if got.Pattern != tt.wantPattern {
				t.Errorf("Pattern = %q, want %q", got.Pattern, tt.wantPattern)
			}
			// The kernel fact value (Args[1]) must be preserved in the Learning's Fact.
			if tt.wantFactFrag != "" && !strings.Contains(got.Fact, tt.wantFactFrag) {
				t.Errorf("Learning.Fact does not carry fact value %q: %+v", tt.wantFactFrag, got)
			}
		})
	}
}

// TestApplyLearnings_ConcurrentCount exercises the learning count while other
// tasks append receipts. Run with -race to detect unsynchronized count reads.
func TestApplyLearnings_ConcurrentCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		facts []core.Fact
	}{
		{
			name:  "one promotion per call",
			facts: []core.Fact{{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/pattern"), "value"}}},
		},
		{
			name: "multiple promotions per call",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/first"), "first value"}},
				{Predicate: "promote_to_long_term", Args: []any{types.MangleAtom("/second"), "second value"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			const workers, callsPerWorker = 8, 32
			campaign := &Campaign{ID: "concurrent-learning"}
			orchestrator := &Orchestrator{kernel: &promotionStubKernel{facts: tt.facts}, campaign: campaign}
			task := &Task{ID: "task", Description: "concurrent promotion"}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			start := make(chan struct{})
			var workersDone sync.WaitGroup
			workersDone.Add(workers)
			for range workers {
				go func() {
					defer workersDone.Done()
					select {
					case <-start:
					case <-ctx.Done():
						return
					}
					for range callsPerWorker {
						orchestrator.applyLearnings(ctx, task, nil)
					}
				}()
			}
			close(start)
			workersDone.Wait()

			// A promotion is recorded once however many tasks see it
			// (campaign-learning-receipt-fidelity #2); concurrent calls must
			// neither race nor record it twice.
			if got, want := len(campaign.Learnings), len(tt.facts); got != want {
				t.Errorf("learning count = %d, want %d", got, want)
			}
		})
	}
}

type singleArgumentPromotionKernel struct {
	core.Kernel
}

func (k *singleArgumentPromotionKernel) Query(predicate string) ([]core.Fact, error) {
	if predicate != "promote_to_long_term" {
		return nil, nil
	}
	return []core.Fact{{Predicate: predicate, Args: []any{"pattern"}}}, nil
}

func TestApplyLearnings_PromotionWithoutValue(t *testing.T) {
	campaign := &Campaign{}
	o := &Orchestrator{kernel: &singleArgumentPromotionKernel{}, campaign: campaign}
	o.applyLearnings(context.Background(), &Task{ID: "task", Description: "task description"}, nil)
	if len(campaign.Learnings) != 1 {
		t.Fatalf("got %d learnings, want 1", len(campaign.Learnings))
	}
	got := campaign.Learnings[0]
	if got.Pattern != "pattern" || got.Fact != "task description" {
		t.Fatalf("got learning pattern %q and fact %q, want pattern and task description", got.Pattern, got.Fact)
	}
}
