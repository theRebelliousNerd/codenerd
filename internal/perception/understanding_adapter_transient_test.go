package perception

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"codenerd/internal/types"
)

// transientErrLLMClient is an LLMClient whose classification call (CompleteWithSystem,
// which LLMTransducer.Understand drives) returns a configurable error. It lets us
// prove that the ErrLLMUnavailable sentinel survives the wrapping chain
// (CompleteWithSystem -> Understand "LLM classification failed: %w" ->
// ParseIntentWithContext errors.Is check) and is recorded on the degraded Intent.
type transientErrLLMClient struct {
	err error
}

func (m *transientErrLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", m.err
}

func (m *transientErrLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", m.err
}

func (m *transientErrLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, m.err
}

func (m *transientErrLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string)
	errorChan := make(chan error, 1)
	errorChan <- m.err
	close(contentChan)
	close(errorChan)
	return contentChan, errorChan
}

// TestParseIntentWithContext_TransientFailureMarksIntent proves Layer 2a: when the
// model is transiently unavailable (the 503/5xx case wrapped as ErrLLMUnavailable
// at the Gemini retry sites and re-wrapped by Understand), ParseIntentWithContext
// must (1) still return a nil error to preserve its degraded-/explain contract,
// (2) set Intent.TransientFailure so the firewall can distinguish this from user
// ambiguity, and (3) give an honest "couldn't reach the model" response rather
// than "I had trouble understanding that".
func TestParseIntentWithContext_TransientFailureMarksIntent(t *testing.T) {
	// Mirror the real chain: max-retries wraps the sentinel, then Understand
	// wraps that again. errors.Is must see through both %w layers.
	wrapped := fmt.Errorf("max retries exceeded: %w", ErrLLMUnavailable)
	client := &transientErrLLMClient{err: wrapped}
	tr := NewUnderstandingTransducer(client).(*UnderstandingTransducer)

	intent, err := tr.ParseIntentWithContext(context.Background(), "what is the jit system?", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext returned non-nil error %v; the degraded-intent contract requires nil", err)
	}
	if !intent.TransientFailure {
		t.Fatalf("intent.TransientFailure = false; a transient ErrLLMUnavailable must be marked so the firewall can report /llm_unavailable")
	}
	if intent.Verb != "/explain" {
		t.Fatalf("intent.Verb = %q, want /explain (degraded fallback)", intent.Verb)
	}
	// The user-facing response must be the honest "model unreachable" text, not a
	// "you were unclear" framing.
	if intent.Response == "" {
		t.Fatalf("intent.Response is empty; expected an honest model-unavailable message")
	}
	if got := intent.Response; got == fmt.Sprintf("I had trouble understanding that: %v", wrapped) {
		t.Fatalf("intent.Response still uses the misleading user-ambiguity message: %q", got)
	}
}

// TestParseIntentWithContext_NonTransientErrorNotMarked is the negative control:
// a generic (non-sentinel) LLM failure must NOT be flagged transient, so it falls
// through to the existing /llm_failed (or /heuristic_low) handling rather than
// claiming the model was overloaded. This is the partition that keeps the new
// /llm_unavailable reason honest.
func TestParseIntentWithContext_NonTransientErrorNotMarked(t *testing.T) {
	client := &transientErrLLMClient{err: errors.New("malformed JSON from model")}
	tr := NewUnderstandingTransducer(client).(*UnderstandingTransducer)

	intent, err := tr.ParseIntentWithContext(context.Background(), "refactor the parser", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext returned non-nil error %v; degraded path must return nil", err)
	}
	if intent.TransientFailure {
		t.Fatalf("intent.TransientFailure = true for a non-transient error; only ErrLLMUnavailable should set it")
	}
	if intent.Verb != "/explain" {
		t.Fatalf("intent.Verb = %q, want /explain (degraded fallback)", intent.Verb)
	}
}
