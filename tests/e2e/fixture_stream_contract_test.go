//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

func TestE2E_FixtureStreamContract_WholeCompletionMocks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client types.LLMClient
		want   string
	}{
		{"session-virtualstore-kernel", &mockLLMClient{}, "mock streaming response"},
		{"async-lifecycle", &talMockLLMClient{}, "mock streaming response"},
		{"cross-boundary", &cbMockLLMClient{}, "mock streaming response"},
		{"campaign-session", &campaignMockLLMClient{}, "mock streaming response"},
		{"orchestrator", &oeMockLLMClient{}, "mock streaming response"},
		{"perception-contract", &pceMockLLMClient{}, "mock streaming response"},
		{"orchestrator-race", &oerMockLLMClient{}, "mock response"},
		{"clean-loop", &sclMockLLMClient{}, "mock streaming response"},
		{"context-isolation", &sciMockLLMClient{}, "mock streaming response"},
		{"piggyback-boundary", &pbMockLLMClient{}, "mock streaming response"},
		{"executor-kernel", &sekMockLLMClient{}, "mock streaming response"},
		{"tool-fallback", &tsfMockLLMClient{}, "mock streaming response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, done := tc.client.CompleteWithStreaming(context.Background(), "offline system", "offline request", false)
			text, result := types.CollectStream(context.Background(), content, done, nil)
			if text != tc.want || result.Err != nil || len(result.GroundingSources) != 0 {
				t.Fatalf("fixture text/terminal/grounding changed: text=%q result=%+v", text, result)
			}
		})
	}
}

func TestE2E_FixtureStreamContract_DelegatedSuccessFailureAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client *mockRealLLM
		cancel bool
		want   string
		err    error
	}{
		{"success", &mockRealLLM{responses: []string{"original completion"}}, false, "original completion", nil},
		{"error", &mockRealLLM{errQueue: []error{errors.New("scripted completion failure")}}, false, "", errors.New("scripted completion failure")},
		{"canceled", &mockRealLLM{blockCh: make(chan struct{})}, true, "", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			content, done := tc.client.CompleteWithStreaming(ctx, "offline system", "offline request", false)
			text, result := types.CollectStream(context.Background(), content, done, nil)
			if text != tc.want || len(result.GroundingSources) != 0 {
				t.Fatalf("delegating fixture changed output: text=%q result=%+v", text, result)
			}
			if (tc.err == nil) != (result.Err == nil) || (tc.err != nil && result.Err.Error() != tc.err.Error()) {
				t.Fatalf("delegating fixture changed error: got=%v want=%v", result.Err, tc.err)
			}
		})
	}
	model := &pipelineModel{}
	want, err := model.CompleteWithSystem(context.Background(), "offline system", "offline request")
	if err != nil {
		t.Fatal(err)
	}
	content, done := model.CompleteWithStreaming(context.Background(), "offline system", "offline request", false)
	text, result := types.CollectStream(context.Background(), content, done, nil)
	if text != want || result.Err != nil || len(result.GroundingSources) != 0 {
		t.Fatalf("pipeline fixture did not delegate its original completion: text=%q result=%+v", text, result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	content, done = model.CompleteWithStreaming(ctx, "offline system", "offline request", false)
	text, result = types.CollectStream(context.Background(), content, done, nil)
	if text != "" || !errors.Is(result.Err, context.Canceled) || len(result.GroundingSources) != 0 {
		t.Fatalf("pipeline fixture lost completion cancellation: text=%q result=%+v", text, result)
	}
}

func TestE2E_FixtureBudgetCounter_DeclaredRenderedAndMandatory(t *testing.T) {
	count := pcbRenderCounter()
	atom := &prompt.PromptAtom{Content: "one", TokenCount: 100, ContentConcise: "two", ContentMin: "three"}
	if count(atom, "standard") != 100 || count(atom, "concise") != prompt.EstimateTokens("two") || count(atom, "min") != prompt.EstimateTokens("three") {
		t.Fatal("counter lost declared floors or selected render-mode text")
	}
	manager := prompt.NewTokenBudgetManager()
	manager.SetReservedHeadroom(0)
	_, err := manager.Fit([]*prompt.OrderedAtom{{Atom: &prompt.PromptAtom{ID: "required", Category: prompt.CategoryCapability, Content: "required obligations", IsMandatory: true, TokenCount: 100}}}, 1, count)
	if err == nil {
		t.Fatal("current mandatory fixture silently dropped an obligation")
	}
}
