package verification

// Fail-closed verification tests.
//
// A verification that could not run (LLM failure, timeout, malformed
// judgment, missing client) must never be recorded as a success. These tests
// pin that contract: the verifier returns an error wrapping
// ErrVerificationUnavailable, persists the attempt with success=false, runs
// the shard exactly once (retrying cannot fix a broken verifier), and still
// returns the shard's output so the caller can show it with a warning.

import (
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"errors"
	"strings"
	"testing"
)

// stubLLMClient is a minimal perception.LLMClient fake. Only
// CompleteWithSystem is behaviour-driven; the rest report unimplemented.
type stubLLMClient struct {
	completeWithSystem func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

var _ perception.LLMClient = (*stubLLMClient)(nil)

func (s *stubLLMClient) Complete(_ context.Context, _ string) (string, error) {
	return "", errors.New("stubLLMClient: Complete not implemented")
}

func (s *stubLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if s.completeWithSystem != nil {
		return s.completeWithSystem(ctx, systemPrompt, userPrompt)
	}
	return "", errors.New("stubLLMClient: CompleteWithSystem not implemented")
}

func (s *stubLLMClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	textCh := make(chan string)
	errCh := make(chan error, 1)
	errCh <- errors.New("stubLLMClient: CompleteWithStreaming not implemented")
	close(textCh)
	close(errCh)
	return textCh, errCh
}

func (s *stubLLMClient) CompleteWithTools(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, errors.New("stubLLMClient: CompleteWithTools not implemented")
}

// stubTaskExecutor is a session.TaskExecutor fake that counts executions.
type stubTaskExecutor struct {
	calls  int
	result string
	err    error
}

var _ session.TaskExecutor = (*stubTaskExecutor)(nil)

func (s *stubTaskExecutor) Execute(_ context.Context, _ session.TaskRequest) (string, error) {
	s.calls++
	return s.result, s.err
}

func (s *stubTaskExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return s.Execute(ctx, req)
}

func (s *stubTaskExecutor) ExecuteAsync(_ context.Context, _ session.TaskRequest) (string, error) {
	return "", errors.New("stubTaskExecutor: ExecuteAsync not implemented")
}

func (s *stubTaskExecutor) GetResult(_ string) (string, bool, error) {
	return "", false, errors.New("stubTaskExecutor: GetResult not implemented")
}

func (s *stubTaskExecutor) WaitForResult(_ context.Context, _ string) (string, error) {
	return "", errors.New("stubTaskExecutor: WaitForResult not implemented")
}

func newFailClosedTestStore(t *testing.T) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Close error: %v", err)
		}
	})
	return db
}

func storedVerifications(t *testing.T, db *store.LocalStore, sessionID string) []store.VerificationRecord {
	t.Helper()
	recs, err := db.GetVerificationHistory(sessionID, 10)
	if err != nil {
		t.Fatalf("GetVerificationHistory error: %v", err)
	}
	return recs
}

func assertFailClosed(t *testing.T, result string, verification *VerificationResult, err error, exec *stubTaskExecutor) {
	t.Helper()
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("VerifyWithRetry error = %v, want it to wrap ErrVerificationUnavailable", err)
	}
	// The shard's actual output is still returned so the caller can show it.
	if result != "shard output here" {
		t.Fatalf("VerifyWithRetry result = %q, want shard output preserved", result)
	}
	if verification == nil {
		t.Fatal("verification should not be nil")
	}
	if verification.Success {
		t.Error("verification.Success = true, want false (fail closed)")
	}
	if verification.Confidence != 0 {
		t.Errorf("verification.Confidence = %v, want 0", verification.Confidence)
	}
	if !strings.Contains(verification.Reason, "verification unavailable") {
		t.Errorf("verification.Reason = %q, want it to report unavailability", verification.Reason)
	}
	// Re-running the shard cannot fix a broken verifier: exactly one run.
	if exec.calls != 1 {
		t.Errorf("shard executed %d times, want exactly 1 (no retry on verifier outage)", exec.calls)
	}
}

func TestVerifyWithRetry_WhenVerificationErrors_ShouldFailClosed(t *testing.T) {
	ctx := context.Background()
	db := newFailClosedTestStore(t)
	client := &stubLLMClient{
		completeWithSystem: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("model overloaded")
		},
	}
	exec := &stubTaskExecutor{result: "shard output here"}
	v := NewTaskVerifier(client, db, nil, nil)
	v.SetTaskExecutor(exec)
	v.SetSessionContext("failclosed-verify-error", 1)

	result, verification, err := v.VerifyWithRetry(ctx, "implement feature X", "/fix", 3)
	assertFailClosed(t, result, verification, err, exec)

	// The outage must be persisted as a failure, never as a success.
	recs := storedVerifications(t, db, "failclosed-verify-error")
	if len(recs) != 1 {
		t.Fatalf("stored %d verification records, want exactly 1", len(recs))
	}
	if recs[0].Success {
		t.Error("stored record Success = true, want false")
	}
}

func TestVerifyWithRetry_WhenNilClient_ShouldFailClosed(t *testing.T) {
	ctx := context.Background()
	db := newFailClosedTestStore(t)
	exec := &stubTaskExecutor{result: "shard output here"}
	v := NewTaskVerifier(nil, db, nil, nil)
	v.SetTaskExecutor(exec)
	v.SetSessionContext("failclosed-nil-client", 1)

	result, verification, err := v.VerifyWithRetry(ctx, "implement feature X", "/fix", 3)
	assertFailClosed(t, result, verification, err, exec)

	recs := storedVerifications(t, db, "failclosed-nil-client")
	if len(recs) != 1 {
		t.Fatalf("stored %d verification records, want exactly 1", len(recs))
	}
	if recs[0].Success {
		t.Error("stored record Success = true, want false")
	}
}

func TestVerifyWithRetry_WhenVerificationSucceeds_ShouldStoreSuccess(t *testing.T) {
	ctx := context.Background()
	db := newFailClosedTestStore(t)
	client := &stubLLMClient{
		completeWithSystem: func(_ context.Context, _, _ string) (string, error) {
			return `{"success":true,"confidence":0.9,"reason":"clean implementation"}`, nil
		},
	}
	exec := &stubTaskExecutor{result: "func Add(a, b int) int { return a + b }"}
	v := NewTaskVerifier(client, db, nil, nil)
	v.SetTaskExecutor(exec)
	v.SetSessionContext("failclosed-success", 1)

	result, verification, err := v.VerifyWithRetry(ctx, "implement feature X", "/fix", 3)
	if err != nil {
		t.Fatalf("VerifyWithRetry error: %v", err)
	}
	if result != "func Add(a, b int) int { return a + b }" {
		t.Fatalf("VerifyWithRetry result = %q, want shard output", result)
	}
	if verification == nil || !verification.Success {
		t.Fatalf("verification = %#v, want success", verification)
	}
	if exec.calls != 1 {
		t.Errorf("shard executed %d times, want exactly 1", exec.calls)
	}

	recs := storedVerifications(t, db, "failclosed-success")
	if len(recs) != 1 {
		t.Fatalf("stored %d verification records, want exactly 1", len(recs))
	}
	if !recs[0].Success {
		t.Error("stored record Success = false, want true for a genuinely verified result")
	}
}

func TestVerifyWithRetry_WhenVerificationKeepsFailing_ShouldExhaustRetries(t *testing.T) {
	const maxRetries = 3
	ctx := context.Background()
	db := newFailClosedTestStore(t)
	client := &stubLLMClient{
		completeWithSystem: func(_ context.Context, _, _ string) (string, error) {
			return `{"success":false,"confidence":0.2,"reason":"mock code found",` +
				`"quality_violations":["mock_code"],"evidence":["line 1: Mock"],` +
				`"suggestions":["implement for real"]}`, nil
		},
	}
	exec := &stubTaskExecutor{result: "func MockThing() {}"}
	v := NewTaskVerifier(client, db, nil, nil)
	v.SetTaskExecutor(exec)
	v.SetSessionContext("failclosed-max-retries", 1)

	_, verification, err := v.VerifyWithRetry(ctx, "implement feature X", "/fix", maxRetries)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("VerifyWithRetry error = %v, want ErrMaxRetriesExceeded", err)
	}
	// cmd/nerd/chat/process.go compares this string; it must stay unchanged.
	if err.Error() != "max retries exceeded - escalating to user" {
		t.Errorf("error message = %q, must stay unchanged", err.Error())
	}
	if verification == nil || verification.Success {
		t.Fatalf("verification = %#v, want a failed verification", verification)
	}
	if exec.calls != maxRetries {
		t.Errorf("shard executed %d times, want %d retries for a genuine verification failure", exec.calls, maxRetries)
	}
}
