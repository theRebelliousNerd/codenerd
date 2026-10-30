package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

// Feedback names the review that produced the finding: a completed
// multi-shard review's own ID, not an alias computed from the turn. With no
// review to apply it to, feedback is refused instead of being filed under
// "unknown", and a location whose line is not a number is refused.
func TestReviewFeedback_UsesTheCompletedReviewsOwnID(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)

	if _, ok := m.currentReviewID(); ok {
		t.Fatal("a review ID was produced with no review")
	}
	next, _ := m.handleAcceptFindingCommand("/accept-finding billing.go:12", []string{"/accept-finding", "billing.go:12"})
	m = next.(Model)
	if last := m.history[len(m.history)-1].Content; !strings.Contains(last, "run `/review` first") {
		t.Fatalf("feedback with no review was accepted: %q", last)
	}

	m.lastShardResult = &ShardResult{ShardType: "reviewer", Timestamp: time.Unix(1_800_000_000, 0), TurnNumber: 3,
		ExtraData: map[string]any{"review_id": "review-1793300000000000000"}}
	if id, ok := m.currentReviewID(); !ok || id != "review-1793300000000000000" {
		t.Fatalf("currentReviewID = %q, %v; want the aggregated review's own ID", id, ok)
	}
	next, _ = m.handleAcceptFindingCommand("/accept-finding billing.go:x", []string{"/accept-finding", "billing.go:x"})
	m = next.(Model)
	if last := m.history[len(m.history)-1].Content; !strings.Contains(last, "not a line number") {
		t.Fatalf("a non-numeric line was accepted: %q", last)
	}

	// A single reviewer run without an ID keeps its alias.
	m.lastShardResult = &ShardResult{ShardType: "reviewer", Timestamp: time.Unix(1_800_000_000, 0), TurnNumber: 3}
	if id, ok := m.currentReviewID(); !ok || id != "review-3-1800000000" {
		t.Fatalf("single-run alias = %q, %v", id, ok)
	}
}
