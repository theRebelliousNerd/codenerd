package browser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser/security"
)

func TestSessionManager_WhenNoContainersConfigured_ShouldReturnEmpty(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewSessionManager(cfg, nil)
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "oops", Timestamp: time.Now()}}
	result := mgr.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result.Correlations) != 0 {
		t.Fatalf("expected no correlations when no containers configured, got %d", len(result.Correlations))
	}
	if len(result.Notes) != 0 {
		t.Fatalf("expected no notes when correlation off by default, got %v", result.Notes)
	}
	if result.Truncated {
		t.Fatalf("expected not truncated when off, got %+v", result)
	}
	// Also test with explicit empty slice
	cfg2 := DefaultConfig()
	cfg2.CorrelationContainers = []string{}
	mgr2 := NewSessionManagerWithSink(cfg2, nil)
	result2 := mgr2.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result2.Correlations) != 0 || len(result2.Notes) != 0 {
		t.Fatalf("expected empty for empty slice, got %+v", result2)
	}
	// Nil manager should also be safe
	var nilMgr *SessionManager
	result3 := nilMgr.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result3.Correlations) != 0 || len(result3.Notes) != 0 {
		t.Fatalf("expected empty for nil manager, got %+v", result3)
	}
}

func TestSessionManager_WhenContainersConfiguredButNoDockerPath_ShouldNoteNoSource(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CorrelationContainers = []string{"app"}
	cfg.DockerPath = ""
	mgr := NewSessionManager(cfg, nil)
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: time.Now()}}
	result := mgr.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result.Correlations) != 0 {
		t.Fatalf("expected no correlations with nil fetcher, got %d", len(result.Correlations))
	}
	if len(result.Notes) == 0 {
		t.Fatal("expected note when fetcher nil but containers requested")
	}
	found := false
	for _, n := range result.Notes {
		if strings.Contains(strings.ToLower(n), "no container log source is configured") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected note about no container log source is configured, got %v", result.Notes)
	}
}

func TestSessionManager_ShouldRedactUsingManagerRedactor(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CorrelationContainers = []string{"app"}
	cfg.DockerPath = ""
	mgr := NewSessionManagerWithSink(cfg, nil)

	secret := "Bearer supersecrettoken12345"
	// Verify chosen secret is actually redacted by real redactor.
	r := security.NewRedactor(nil)
	if strings.Contains(r.SanitizeString("Authorization: "+secret), "supersecrettoken12345") {
		t.Fatalf("chosen secret %q is not redacted by security.NewRedactor, pick one that is", secret)
	}

	now := time.Now()
	mgr.containerFetcher = func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		return []ContainerLogLine{{Container: container, Timestamp: now, Message: "Authorization: " + secret}}, nil
	}
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail with password=hunter2", Timestamp: now}}
	result := mgr.CorrelateContainerErrors(context.Background(), events, 5*time.Second)
	if len(result.Correlations) == 0 {
		t.Fatal("expected at least one correlation for redaction test")
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
		t.Fatalf("expected redacted marker %q in result, got %s", security.Redacted, text)
	}
}

func TestSessionManager_WhenBothConstructorsUsed_ShouldWireFetcherIdentically(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CorrelationContainers = []string{"app", "db"}
	cfg.DockerPath = ""
	events := []RuntimeErrorEvent{{Kind: "console_error", Detail: "detail", Timestamp: time.Now()}}

	mgr1 := NewSessionManager(cfg, nil)
	result1 := mgr1.CorrelateContainerErrors(context.Background(), events, 5*time.Second)

	mgr2 := NewSessionManagerWithSink(cfg, nil)
	result2 := mgr2.CorrelateContainerErrors(context.Background(), events, 5*time.Second)

	// Both should have same behaviour: no correlations, note about no source, not truncated panicking.
	if len(result1.Correlations) != len(result2.Correlations) {
		t.Fatalf("constructors diverged: correlations %d vs %d", len(result1.Correlations), len(result2.Correlations))
	}
	if len(result1.Notes) != len(result2.Notes) {
		t.Fatalf("constructors diverged: notes %v vs %v", result1.Notes, result2.Notes)
	}
	found1 := false
	for _, n := range result1.Notes {
		if strings.Contains(strings.ToLower(n), "no container log source is configured") {
			found1 = true
			break
		}
	}
	found2 := false
	for _, n := range result2.Notes {
		if strings.Contains(strings.ToLower(n), "no container log source is configured") {
			found2 = true
			break
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected both constructors to note no source, got %v vs %v", result1.Notes, result2.Notes)
	}
	if result1.Truncated != result2.Truncated {
		t.Fatalf("truncated mismatch: %v vs %v", result1.Truncated, result2.Truncated)
	}
}
