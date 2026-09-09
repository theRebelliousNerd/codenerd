package shards_test

import (
	"strings"
	"testing"

	"codenerd/internal/core"
	shards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// TestReviewFeedback_ReachesTheKernel is the wiring test for a learning loop
// that was complete on the logic side and starved on the Go side.
//
// reviewer.mg derives review_suspect, review_rejection_count and
// reviewer_needs_validation from user_rejected_finding/5 and
// user_accepted_finding/4 (schemas_tools.mg:339,343). Nothing produced either
// fact. AcceptReviewFinding and RejectReviewFinding checked a
// ReviewerFeedbackProvider that no type in the repo implements and that
// nothing calls SetReviewerFeedbackProvider for, so in every production
// process the user's judgement on a review finding went into a nil check and
// stopped there.
//
// The chat commands at cmd/nerd/chat/commands_handlers_misc.go:205 and :255 are
// where a person actually says "this finding was wrong".
func TestReviewFeedback_ReachesTheKernel(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)

	sm.RejectReviewFinding("rev-1", "a.go", 10, "false positive")
	sm.AcceptReviewFinding("rev-1", "b.go", 20)

	rejected, err := kernel.Query("user_rejected_finding")
	if err != nil {
		t.Fatalf("query rejections: %v", err)
	}
	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejection fact, got %d", len(rejected))
	}
	if got := types.ExtractString(rejected[0].Args[0]); got != "rev-1" {
		t.Errorf("rejection lost its review id: %q", got)
	}
	if got := types.ExtractString(rejected[0].Args[3]); got != "false positive" {
		t.Errorf("rejection lost its reason: %q", got)
	}

	accepted, err := kernel.Query("user_accepted_finding")
	if err != nil {
		t.Fatalf("query acceptances: %v", err)
	}
	if len(accepted) != 1 {
		t.Fatalf("expected 1 acceptance fact, got %d", len(accepted))
	}
}

// TestReviewFeedback_DrivesSuspicion proves the loop closes: two rejections in
// one review must make the kernel derive review_suspect and, from it,
// reviewer_needs_validation — which CheckReviewNeedsValidation already reads
// back through its kernel fallback.
func TestReviewFeedback_DrivesSuspicion(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)

	if sm.CheckReviewNeedsValidation("rev-2") {
		t.Fatal("a review with no rejections must not need validation")
	}

	sm.RejectReviewFinding("rev-2", "a.go", 10, "false positive")
	sm.RejectReviewFinding("rev-2", "b.go", 20, "false positive")

	reasons := sm.GetReviewSuspectReasons("rev-2")
	if len(reasons) == 0 {
		t.Fatal("two rejections in one review must make it suspect; reviewer.mg's multiple_rejections rule did not fire")
	}
	if !sm.CheckReviewNeedsValidation("rev-2") {
		t.Fatal("a suspect review must need validation")
	}
}

// TestReviewFeedback_AccuracyReport replaces a function that returned the fixed
// string "Review feedback provider not available" in every production process.
func TestReviewFeedback_AccuracyReport(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)

	if got := sm.GetReviewAccuracyReport("rev-3"); !strings.Contains(got, "no findings") {
		t.Errorf("empty review should say so plainly, got %q", got)
	}

	sm.AcceptReviewFinding("rev-3", "a.go", 1)
	sm.AcceptReviewFinding("rev-3", "b.go", 2)
	sm.AcceptReviewFinding("rev-3", "c.go", 3)
	sm.RejectReviewFinding("rev-3", "d.go", 4, "noise")

	report := sm.GetReviewAccuracyReport("rev-3")
	if strings.Contains(report, "not available") {
		t.Fatalf("report still reports an absent provider: %q", report)
	}
	for _, want := range []string{"rev-3", "4 finding", "3 accepted", "1 rejected", "75%"} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}
}

// TestReviewFeedback_AccuracyIsRecomputedNotAppended pins the reason the
// refresh retracts first. Mangle facts are a set: asserting a second
// review_accuracy row for the same review leaves both visible, and every rule
// reading it — reviewer.mg's high_rejection_rate suspicion joins on it — turns
// non-deterministic.
func TestReviewFeedback_AccuracyIsRecomputedNotAppended(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)

	for i := 0; i < 5; i++ {
		sm.AcceptReviewFinding("rev-4", "a.go", i)
	}

	facts, err := kernel.Query("review_accuracy")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected exactly 1 review_accuracy row, got %d: %+v", len(facts), facts)
	}
	// Total must reflect all five, not the first one.
	if got := facts[0].Args[1]; got != int64(5) {
		t.Errorf("review_accuracy total = %v, want 5", got)
	}
}

// TestReviewFeedback_DegradesWithoutKernel keeps the chat commands safe on a
// manager that was never given a kernel.
func TestReviewFeedback_DegradesWithoutKernel(t *testing.T) {
	sm := shards.NewShardManager()
	sm.AcceptReviewFinding("rev-5", "a.go", 1)
	sm.RejectReviewFinding("rev-5", "a.go", 2, "why")

	if got := sm.GetReviewAccuracyReport("rev-5"); !strings.Contains(got, "no kernel") {
		t.Errorf("expected an honest no-kernel message, got %q", got)
	}
}

// TestReviewFeedback_ProviderStillWins keeps the interface path intact for
// whenever a ReviewerFeedbackProvider is written.
func TestReviewFeedback_ProviderStillWins(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	sm := shards.NewShardManager()
	sm.SetParentKernel(kernel)
	fake := &fakeReviewerFeedback{}
	sm.SetReviewerFeedbackProvider(fake)

	sm.AcceptReviewFinding("rev-6", "a.go", 1)
	sm.RejectReviewFinding("rev-6", "a.go", 2, "why")

	if fake.accepted != 1 || fake.rejected != 1 {
		t.Fatalf("provider not consulted: accepted=%d rejected=%d", fake.accepted, fake.rejected)
	}
	// With a provider installed the kernel fallback must not also fire, or the
	// same judgement would be recorded twice.
	facts, err := kernel.Query("user_accepted_finding")
	if err == nil && len(facts) != 0 {
		t.Errorf("kernel fallback ran alongside the provider: %+v", facts)
	}
	if got := sm.GetReviewAccuracyReport("rev-6"); got != "provider-report" {
		t.Errorf("provider report not used, got %q", got)
	}
}

type fakeReviewerFeedback struct {
	accepted int
	rejected int
}

func (f *fakeReviewerFeedback) NeedsValidation(string) bool       { return false }
func (f *fakeReviewerFeedback) GetSuspectReasons(string) []string { return nil }
func (f *fakeReviewerFeedback) AcceptFinding(string, string, int) { f.accepted++ }
func (f *fakeReviewerFeedback) RejectFinding(string, string, int, string) {
	f.rejected++
}
func (f *fakeReviewerFeedback) GetAccuracyReport(string) string { return "provider-report" }
