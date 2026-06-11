package perception

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

type mockPerceptionPromptAssembler struct{}

func (m *mockPerceptionPromptAssembler) AssembleSystemPrompt(ctx context.Context, shardID, shardType string) (string, error) {
	return "mock prompt", nil
}

func (m *mockPerceptionPromptAssembler) JITReady() bool {
	return true
}

func TestUnderstandingTransducer_SettersAndGetters(t *testing.T) {
	mockClient := &baseMockLLMClient{}
	tr := NewUnderstandingTransducer(mockClient).(*UnderstandingTransducer)

	// SetPromptAssembler
	pa := &mockPerceptionPromptAssembler{}
	tr.SetPromptAssembler(pa)
	if tr.promptAssembler != pa {
		t.Errorf("SetPromptAssembler failed")
	}

	// SetStrategicContext
	tr.SetStrategicContext("my_context")
	if tr.strategicContext != "my_context" {
		t.Errorf("SetStrategicContext failed")
	}

	// SetKernel
	kernel := &core.RealKernel{}
	tr.SetKernel(kernel)
	if tr.kernel == nil {
		t.Errorf("SetKernel failed")
	}

	// GetLastUnderstanding
	u := &Understanding{ActionType: "test_action"}
	tr.lastUnderstanding = u
	ret := tr.GetLastUnderstanding()
	if ret != u {
		t.Errorf("GetLastUnderstanding failed")
	}
}

// TestUnderstandingTransducer_IsQuestionMapping guards the IsQuestion carrier:
// the routing arbitration policy depends on this signal to send questions to a
// direct answer instead of shard delegation.
func TestUnderstandingTransducer_IsQuestionMapping(t *testing.T) {
	tr := &UnderstandingTransducer{}

	// Explicit signal wins.
	u := &Understanding{
		ActionType:   "investigate",
		SemanticType: "state",
		Signals:      Signals{IsQuestion: true},
	}
	if intent := tr.understandingToIntent(u); !intent.IsQuestion {
		t.Error("IsQuestion signal not carried into Intent")
	}

	// Interrogative semantic type is a backup signal.
	u = &Understanding{
		ActionType:   "explain",
		SemanticType: "definition",
		Signals:      Signals{IsQuestion: false},
	}
	if intent := tr.understandingToIntent(u); !intent.IsQuestion {
		t.Error("interrogative semantic_type (definition) should set IsQuestion")
	}

	// Action requests stay non-question.
	u = &Understanding{
		ActionType:   "modify",
		SemanticType: "state",
		Signals:      Signals{IsQuestion: false},
	}
	if intent := tr.understandingToIntent(u); intent.IsQuestion {
		t.Error("action request wrongly marked as question")
	}
}

func TestUnderstandingTransducer_ResolveFocus(t *testing.T) {
	mockClient := &baseMockLLMClient{}
	tr := NewUnderstandingTransducer(mockClient).(*UnderstandingTransducer)

	ctx := context.Background()

	// With candidates
	res, err := tr.ResolveFocus(ctx, "ref", []string{"cand1", "cand2"})
	if err != nil {
		t.Errorf("ResolveFocus error: %v", err)
	}
	if res.ResolvedPath != "cand1" {
		t.Errorf("expected cand1, got %s", res.ResolvedPath)
	}

	// Without candidates
	res, err = tr.ResolveFocus(ctx, "ref", nil)
	if err != nil {
		t.Errorf("ResolveFocus error: %v", err)
	}
	if res.ResolvedPath != "ref" {
		t.Errorf("expected ref, got %s", res.ResolvedPath)
	}
}

func TestUnderstandingTransducer_ParseIntentWithGCD(t *testing.T) {
	// Not easily testable without mocking LLM responses which is complex here,
	// but we can check empty input fallback.
	mockClient := &baseMockLLMClient{}
	tr := NewUnderstandingTransducer(mockClient).(*UnderstandingTransducer)
	ctx := context.Background()

	intent, updates, err := tr.ParseIntentWithGCD(ctx, "   ", nil, 3)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if intent.Verb != "/explain" {
		t.Errorf("expected /explain, got %s", intent.Verb)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0 updates")
	}
}
