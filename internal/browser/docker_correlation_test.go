package browser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser/security"
)

func TestCorrelateContainerLogs_WhenNoContainersConfigured_ShouldReturnEmptyAndNotCallFetcher(t *testing.T) {
	called := false
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		called = true
		return nil, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "oops", Timestamp: time.Now()}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: nil, Events: events, Window: time.Second, Redactor: nil})
	if len(result.Correlations) != 0 || len(result.Notes) != 0 || result.Truncated {
		t.Fatalf("expected empty result for no containers, got %+v", result)
	}
	if called {
		t.Fatal("fetcher should not be called when containers empty")
	}
	// Also test empty containers slice explicitly.
	called = false
	result = CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{}, Events: events, Window: time.Second, Redactor: nil})
	if called {
		t.Fatal("fetcher should not be called when containers empty slice")
	}
}

func TestCorrelateContainerLogs_WhenFetcherNil_ShouldNoteAndNotFail(t *testing.T) {
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: time.Now()}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: nil, Containers: []string{"app"}, Events: events, Window: time.Second, Redactor: nil})
	if len(result.Correlations) != 0 {
		t.Fatalf("expected no correlations when fetcher nil, got %d", len(result.Correlations))
	}
	if len(result.Notes) == 0 {
		t.Fatal("expected note when fetcher nil but containers requested")
	}
	found := false
	for _, n := range result.Notes {
		if strings.Contains(strings.ToLower(n), "no container log source") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected note about no container log source, got %v", result.Notes)
	}
}

func TestCorrelateContainerLogs_WhenLogWithinWindow_ShouldCorrelate(t *testing.T) {
	now := time.Now()
	eventTime := now
	logTime := eventTime.Add(-time.Second) // one second BEFORE, should be negative delta
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		return []ContainerLogLine{{Container: container, Timestamp: logTime, Message: "container error line"}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "browser failed", Timestamp: eventTime}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"app"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) != 1 {
		t.Fatalf("expected 1 correlation, got %d notes=%v", len(result.Correlations), result.Notes)
	}
	corr := result.Correlations[0]
	if corr.DeltaMs != -1000 {
		t.Fatalf("expected DeltaMs -1000, got %d", corr.DeltaMs)
	}
	if corr.Container != "app" || corr.EventKind != "console_error" {
		t.Fatalf("unexpected correlation: %+v", corr)
	}
}

func TestCorrelateContainerLogs_WhenLogOutsideWindow_ShouldNotCorrelate(t *testing.T) {
	now := time.Now()
	eventTime := now
	logTime := eventTime.Add(10 * time.Second) // outside 5s window
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		return []ContainerLogLine{{Container: container, Timestamp: logTime, Message: "far log"}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "failed_request", Detail: "detail", Timestamp: eventTime}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"app"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) != 0 {
		t.Fatalf("expected 0 correlations for outside window, got %d", len(result.Correlations))
	}
}

func TestCorrelateContainerLogs_WhenOneContainerFails_ShouldStillCorrelateTheOthers(t *testing.T) {
	now := time.Now()
	eventTime := now
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		if container == "bad" {
			return nil, context.DeadlineExceeded
		}
		return []ContainerLogLine{{Container: container, Timestamp: eventTime, Message: "good log"}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: eventTime}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"bad", "good"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) == 0 {
		t.Fatalf("expected correlation from good container, got none notes=%v", result.Notes)
	}
	foundNote := false
	for _, n := range result.Notes {
		if strings.Contains(n, "bad") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Fatalf("expected note naming failing container, got %v", result.Notes)
	}
}

func TestCorrelateContainerLogs_WhenSecretInLogLine_ShouldRedact(t *testing.T) {
	now := time.Now()
	secret := "Bearer supersecrettoken12345"
	// Verify our chosen secret is actually redacted by the real redactor; if not, pick one that is.
	r := security.NewRedactor(nil)
	if strings.Contains(r.SanitizeString("Authorization: "+secret), "supersecrettoken") {
		t.Fatalf("chosen secret %q is not redacted by security.NewRedactor, pick one that is", secret)
	}
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		return []ContainerLogLine{{Container: container, Timestamp: now, Message: "Authorization: " + secret}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail with password=hunter2", Timestamp: now}}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"app"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) == 0 {
		t.Fatal("expected correlation for redaction test")
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "supersecrettoken12345") {
		t.Fatalf("raw secret leaked in result: %s", text)
	}
	if strings.Contains(text, "hunter2") {
		t.Fatalf("raw password leaked in result: %s", text)
	}
	if !strings.Contains(text, security.Redacted) {
		t.Fatalf("expected redacted marker in result, got %s", text)
	}
}

func TestCorrelateContainerLogs_WhenTooManyContainers_ShouldTruncateAndNote(t *testing.T) {
	now := time.Now()
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		return []ContainerLogLine{{Container: container, Timestamp: now, Message: "log"}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: now}}
	containers := make([]string, 12)
	for i := range containers {
		containers[i] = "c" + string(rune('a'+i))
	}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: containers, Events: events, Window: 5 * time.Second, Redactor: nil})
	if !result.Truncated {
		t.Fatal("expected truncated when too many containers")
	}
	if len(result.Notes) == 0 {
		t.Fatal("expected note when truncating containers")
	}
	found := false
	for _, n := range result.Notes {
		if strings.Contains(strings.ToLower(n), "skipped") && strings.Contains(n, "4") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected note naming how many skipped (4), got %v", result.Notes)
	}
	// Fetcher should have been called at most maxCorrelationContainers times (8).
	// We verify indirectly by checking that result has at most 8 correlations (one per container within window).
	if len(result.Correlations) > 8 {
		t.Fatalf("expected at most 8 correlations, got %d", len(result.Correlations))
	}
}

func TestCorrelateContainerLogs_WhenContextCancelled_ShouldReturnPartialResult(t *testing.T) {
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		calls++
		if container == "a" {
			// Cancel after first fetch so next iteration sees cancellation.
			cancel()
			return []ContainerLogLine{{Container: container, Timestamp: now, Message: "log a"}}, nil
		}
		return []ContainerLogLine{{Container: container, Timestamp: now, Message: "log b"}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: now}}
	result := CorrelateContainerLogs(ctx, ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"a", "b"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) == 0 {
		t.Fatalf("expected partial result with at least one correlation, got none")
	}
	if calls != 1 {
		t.Fatalf("expected fetcher called once due to cancellation, got %d", calls)
	}
	found := false
	for _, n := range result.Notes {
		if strings.Contains(strings.ToLower(n), "cancel") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected note about context cancellation, got %v", result.Notes)
	}
}

func TestCorrelateContainerLogs_ShouldSortDeterministically(t *testing.T) {
	now := time.Now()
	// Create events and logs with varying deltas to test sorting.
	eventTime := now
	fetcher := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		// Return logs with different deltas: 100ms, 500ms, 10ms etc.
		return []ContainerLogLine{
			{Container: container, Timestamp: eventTime.Add(500 * time.Millisecond), Message: "b message"},
			{Container: container, Timestamp: eventTime.Add(10 * time.Millisecond), Message: "a message"},
			{Container: container, Timestamp: eventTime.Add(100 * time.Millisecond), Message: "c message"},
		}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: eventTime}}
	result1 := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"app"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	result2 := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher, Containers: []string{"app"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result1.Correlations) != 3 || len(result2.Correlations) != 3 {
		t.Fatalf("expected 3 correlations, got %d and %d", len(result1.Correlations), len(result2.Correlations))
	}
	// Check deterministic: same input twice identical output.
	for i := range result1.Correlations {
		if result1.Correlations[i] != result2.Correlations[i] {
			t.Fatalf("non-deterministic output at index %d: %+v vs %+v", i, result1.Correlations[i], result2.Correlations[i])
		}
	}
	// Check closest-in-time first: deltas should be 10, 100, 500 ms in absolute order.
	expected := []int64{10, 100, 500}
	for i, exp := range expected {
		abs := result1.Correlations[i].DeltaMs
		if abs < 0 {
			abs = -abs
		}
		if abs != exp {
			t.Fatalf("expected sorted delta %d at index %d, got %d", exp, i, abs)
		}
	}
	// Also test tie-breaker: same delta, different container/message should be ordered by container then message.
	fetcher2 := func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		// Two containers, same timestamp -> same delta, should be ordered by container.
		if container == "a" {
			return []ContainerLogLine{{Container: "a", Timestamp: eventTime, Message: "zebra"}}, nil
		}
		return []ContainerLogLine{{Container: "b", Timestamp: eventTime, Message: "apple"}}, nil
	}
	result := CorrelateContainerLogs(context.Background(), ContainerCorrelationRequest{Fetcher: fetcher2, Containers: []string{"a", "b"}, Events: events, Window: 5 * time.Second, Redactor: nil})
	if len(result.Correlations) != 2 {
		t.Fatalf("expected 2 tie-breaker correlations, got %d", len(result.Correlations))
	}
	if result.Correlations[0].Container != "a" || result.Correlations[1].Container != "b" {
		t.Fatalf("expected container tie-breaker order a then b, got %+v", result.Correlations)
	}
}
