package shards_test

import (
	"strings"
	"testing"

	"codenerd/internal/core"
	shards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// The report counted feedback records as "finding(s) judged": one finding
// accepted twice and then rejected read as three findings judged, 67%
// accepted. It now counts distinct findings by their latest verdict and
// shows the events beside them.
func TestReviewFeedback_CountsFindingsByTheirLatestVerdict(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)
	assert := func(f types.Fact) {
		t.Helper()
		if err := kernel.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	// One finding: accepted at t=100 and t=200, rejected at t=300.
	assert(types.Fact{Predicate: "user_accepted_finding", Args: []any{"rev-a", "billing.go", int64(12), int64(100)}})
	assert(types.Fact{Predicate: "user_accepted_finding", Args: []any{"rev-a", "billing.go", int64(12), int64(200)}})
	assert(types.Fact{Predicate: "user_rejected_finding", Args: []any{"rev-a", "billing.go", int64(12), "noise", int64(300)}})
	report := sm.GetReviewAccuracyReport("rev-a")
	for _, want := range []string{"1 finding(s) judged", "0 accepted", "1 rejected", "3 feedback event(s)"} {
		if !strings.Contains(report, want) {
			t.Errorf("one finding judged three times: report lacks %q:\n%s", want, report)
		}
	}

	// Two findings, both rejected, then line 12 accepted.
	assert(types.Fact{Predicate: "user_rejected_finding", Args: []any{"rev-b", "billing.go", int64(12), "noise", int64(100)}})
	assert(types.Fact{Predicate: "user_rejected_finding", Args: []any{"rev-b", "billing.go", int64(20), "noise", int64(110)}})
	assert(types.Fact{Predicate: "user_accepted_finding", Args: []any{"rev-b", "billing.go", int64(12), int64(120)}})
	report = sm.GetReviewAccuracyReport("rev-b")
	for _, want := range []string{"2 finding(s) judged", "1 accepted", "1 rejected", "50%", "3 feedback event(s)"} {
		if !strings.Contains(report, want) {
			t.Errorf("a reversed verdict: report lacks %q:\n%s", want, report)
		}
	}
}
