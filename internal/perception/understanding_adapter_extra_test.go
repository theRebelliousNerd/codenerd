package perception

import (
	"context"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
)

func TestUnderstandingTransducer_SettersAndGetters(t *testing.T) {
	mockClient := &baseMockLLMClient{}
	tr := NewUnderstandingTransducer(mockClient).(*UnderstandingTransducer)

	// SetPromptAssembler
	pa := &articulation.PromptAssembler{}
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
	if tr.rawKernel != kernel {
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

func TestUnderstandingTransducer_likelyTopicChange(t *testing.T) {
	// Signal 1: Question marks
	if !likelyTopicChange("what is this?", nil) {
		t.Errorf("expected true for question mark")
	}

	// Signal 2: Keywords
	if !likelyTopicChange("explain this code", nil) {
		t.Errorf("expected true for 'explain'")
	}
	if likelyTopicChange("fix the tests", nil) {
		t.Errorf("expected false for 'fix'")
	}

	// Signal 3: Length spike
	history := []int{10, 10, 10}
	if !likelyTopicChange("this is a very long message that should trigger a spike because it is more than 30 characters", history) {
		t.Errorf("expected true for length spike")
	}
}

func TestUnderstandingTransducer_computeStabilityScore(t *testing.T) {
	if score := computeStabilityScore([]string{"fix", "fix", "fix"}); score != 100 {
		t.Errorf("expected 100, got %d", score)
	}
	if score := computeStabilityScore([]string{"fix", "test", "fix"}); score != 0 {
		t.Errorf("expected 0, got %d", score)
	}
	if score := computeStabilityScore([]string{"fix", "fix", "test", "test"}); score != 66 {
		// 3 pairs: (fix,fix)=1, (fix,test)=0, (test,test)=1 -> 2/3 = 66
		t.Errorf("expected 66, got %d", score)
	}
	if score := computeStabilityScore([]string{}); score != 0 {
		t.Errorf("expected 0 for empty")
	}
}

func TestUnderstandingTransducer_assertStabilityFacts(t *testing.T) {
	mockClient := &baseMockLLMClient{}
	tr := NewUnderstandingTransducer(mockClient).(*UnderstandingTransducer)

	// Without kernel
	if tr.assertStabilityFacts("test", nil, nil, nil) {
		t.Errorf("expected false without kernel")
	}

	// We can't easily test the true kernel tx behavior here without a full Mangle engine setup.
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
