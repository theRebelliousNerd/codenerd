package research

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/types"
)

func TestWhenCorrelationDisabled_ShouldOmitContainerKeys(t *testing.T) {
	// Manager with no CorrelationContainers disables container correlation entirely.
	// CorrelateContainerErrors should return an entirely empty result, and the
	// diagnosis publishing logic must omit both container keys rather than
	// emitting empty arrays.
	cfg := browser.DefaultConfig()
	// Ensure no containers (default is nil/empty)
	mgr := browser.NewSessionManager(cfg, nil)
	events := []browser.RuntimeErrorEvent{
		{Kind: "failed_request", Detail: "/api status=500", Timestamp: time.Now()},
		{Kind: "console_error", Detail: "console: boom", Timestamp: time.Now()},
	}
	result := mgr.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result.Correlations) != 0 || len(result.Notes) != 0 {
		t.Fatalf("expected entirely empty result when correlation disabled, got correlations=%d notes=%v", len(result.Correlations), result.Notes)
	}
	// Simulate the publishing decision in executeBrowserReason: omit both keys when result is empty.
	data := map[string]any{
		"correlations": []map[string]any{{"dummy": 1}},
	}
	hasContainerData := len(result.Correlations) > 0 || len(result.Notes) > 0
	if hasContainerData {
		data["container_correlations"] = result.Correlations
		if len(result.Notes) > 0 {
			data["container_correlation_notes"] = result.Notes
		}
	}
	if _, ok := data["container_correlations"]; ok {
		t.Fatal("container_correlations should be omitted when correlation disabled (empty result)")
	}
	if _, ok := data["container_correlation_notes"]; ok {
		t.Fatal("container_correlation_notes should be omitted when correlation disabled")
	}
	// Also verify counts would be omitted (no noise when off)
	counts := map[string]int{"correlations": 1}
	if hasContainerData {
		counts["container_correlations"] = len(result.Correlations)
	}
	if _, ok := counts["container_correlations"]; ok {
		t.Fatal("container_correlations count should be omitted when correlation disabled")
	}
	// Verify evidence handle would be omitted
	evidenceHandles := []string{"reason:sess:correlations"}
	if hasContainerData {
		evidenceHandles = append(evidenceHandles, "reason:sess:container_correlations")
	}
	for _, h := range evidenceHandles {
		if strings.Contains(h, "container_correlations") {
			t.Fatal("container_correlations evidence handle should be omitted when disabled")
		}
	}
}

func TestWhenFactHasNoTimestamp_ShouldBeSkipped(t *testing.T) {
	now := time.Now().UnixMilli()
	// Fact with determinable timestamp (valid int64)
	validFailed := types.Fact{
		Predicate: "failed_request_at",
		Args:      []any{"session-a", "req-1", "/api", int64(500), now},
	}
	// Fact with undeterminable timestamp: timestamp index exists but value is string, factTimestamp returns 0
	noTimeFailed := types.Fact{
		Predicate: "failed_request_at",
		Args:      []any{"session-a", "req-2", "/api/missing", int64(500), "not-a-number"},
	}
	// Also test missing arg case: timestamp index out of range -> 0
	missingArg := types.Fact{
		Predicate: "failed_request_at",
		Args:      []any{"session-a", "req-3", "/api"},
	}
	validVisible := types.Fact{
		Predicate: "user_visible_error",
		Args:      []any{"session-a", "console", "boom", now},
	}
	noTimeVisible := types.Fact{
		Predicate: "user_visible_error",
		Args:      []any{"session-a", "console", "no time", "bad-timestamp"},
	}

	events := adaptRuntimeErrorEvents(
		[]types.Fact{validFailed, noTimeFailed, missingArg},
		[]types.Fact{validVisible, noTimeVisible},
	)
	// Should only contain the two valid-time facts, not the ones with zero time
	if len(events) != 2 {
		t.Fatalf("expected 2 events (valid only), got %d: %+v", len(events), events)
	}
	for _, ev := range events {
		if ev.Timestamp.IsZero() {
			t.Fatalf("adapted event has zero time, should have been skipped: %+v", ev)
		}
		if ev.Timestamp.UnixMilli() == 0 {
			t.Fatalf("adapted event has epoch time, should have been skipped: %+v", ev)
		}
	}
	// Ensure detail is populated and kind correct
	foundFailed := false
	foundConsole := false
	for _, ev := range events {
		if ev.Kind == "failed_request" {
			foundFailed = true
			if !strings.Contains(ev.Detail, "/api") {
				t.Fatalf("failed_request detail should contain URL, got %q", ev.Detail)
			}
		}
		if ev.Kind == "console_error" {
			foundConsole = true
			if !strings.Contains(ev.Detail, "boom") {
				t.Fatalf("console_error detail should contain message, got %q", ev.Detail)
			}
		}
	}
	if !foundFailed || !foundConsole {
		t.Fatalf("expected both kinds present, got %+v", events)
	}
}

func TestShouldCapAdaptedEvents(t *testing.T) {
	now := time.Now().UnixMilli()
	// Create more than cap (32) facts
	var failed []types.Fact
	var visible []types.Fact
	for i := 0; i < 25; i++ {
		failed = append(failed, types.Fact{
			Predicate: "failed_request_at",
			Args:      []any{"session-a", "req", "/api/" + string(rune('a'+i)), int64(500), now + int64(i*100)},
		})
	}
	for i := 0; i < 25; i++ {
		visible = append(visible, types.Fact{
			Predicate: "user_visible_error",
			Args:      []any{"session-a", "console", "msg", now + int64((25+i)*100)},
		})
	}
	// Total 50 facts, cap is 32, most recent 32 should be kept
	events := adaptRuntimeErrorEvents(failed, visible)
	if len(events) > maxAdaptedContainerEvents {
		t.Fatalf("expected at most %d events, got %d", maxAdaptedContainerEvents, len(events))
	}
	if len(events) != maxAdaptedContainerEvents {
		t.Fatalf("expected exactly %d events when over cap, got %d", maxAdaptedContainerEvents, len(events))
	}
	// Verify most recent are kept: the oldest timestamps should have been dropped.
	// Our visible facts have later timestamps than failed, so they should dominate.
	// Check that timestamps are sorted descending (most recent first) as per helper.
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.After(events[i-1].Timestamp) {
			t.Fatalf("events not sorted most-recent first at index %d: %v after %v", i, events[i].Timestamp, events[i-1].Timestamp)
		}
	}
	// Ensure no zero times
	for _, ev := range events {
		if ev.Timestamp.IsZero() {
			t.Fatalf("zero timestamp should not be present after cap: %+v", ev)
		}
	}
}

func TestAdaptRuntimeErrorEvents_ShouldUseSameTimestampExtractionAsCorrelate(t *testing.T) {
	// Ensure that timestamp extraction for container correlation matches correlateBrowserFailures.
	// Both use factTimestamp with browserPredicateSpecs, so a fact with string timestamp
	// should be skipped in both.
	now := time.Now().UnixMilli()
	valid := types.Fact{Predicate: "failed_request_at", Args: []any{"s", "r", "/api", int64(500), now}}
	invalid := types.Fact{Predicate: "failed_request_at", Args: []any{"s", "r", "/api", int64(500), "bad"}}

	events := adaptRuntimeErrorEvents([]types.Fact{valid, invalid}, nil)
	if len(events) != 1 {
		t.Fatalf("expected 1 event after skipping invalid timestamp, got %d", len(events))
	}
	// Also ensure correlateBrowserFailures would skip the invalid for delta calc by virtue of timestamp 0
	// We simulate: factTimestamp for invalid should be 0
	if ts := factTimestamp(invalid, browserPredicateSpecs[invalid.Predicate]); ts != 0 {
		t.Fatalf("factTimestamp for invalid should be 0, got %d", ts)
	}
}
