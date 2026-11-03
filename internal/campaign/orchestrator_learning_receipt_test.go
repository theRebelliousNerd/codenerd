package campaign

import (
	"context"
	"testing"

	"codenerd/internal/core"
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
				{Predicate: "promote_to_long_term", Args: []any{"/success_pattern", "retry with backoff"}},
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
			name: "nil result still records fact value",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{"cache-key", "cache fact value"}},
			},
			taskDesc:     "task with nil result",
			result:       nil,
			wantCount:    1,
			wantPattern:  "cache-key",
			wantFactFrag: "cache fact value",
		},
		{
			name: "empty pattern with fact value",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{"", "only value matters"}},
			},
			taskDesc:     "empty pattern",
			wantCount:    1,
			wantPattern:  "",
			wantFactFrag: "only value matters",
		},
		{
			name: "audited two-arg promotion keeps second arg",
			facts: []core.Fact{
				{Predicate: "promote_to_long_term", Args: []any{"deploy-tip", "blue-green deploy fact"}},
			},
			taskDesc:     "deploy task",
			result:       "done",
			wantCount:    1,
			wantPattern:  "deploy-tip",
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
			// The kernel fact value (Args[1]) must be preserved in the Learning.
			// Current code drops Args[1] and stores task text instead.
			if tt.wantFactFrag != "" {
				hay := got.Fact + "|" + got.Type + "|" + got.Pattern
				found := false
				for i := 0; i+len(tt.wantFactFrag) <= len(hay); i++ {
					if hay[i:i+len(tt.wantFactFrag)] == tt.wantFactFrag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Learning does not carry fact value %q: %+v", tt.wantFactFrag, got)
				}
			}
		})
	}
}
