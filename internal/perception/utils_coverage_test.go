package perception

import (
	"errors"
	"testing"
)

// --- requiresJSONOutput ---

func TestRequiresJSONOutput_WhenMangleSynthMarker_ShouldReturnTrue(t *testing.T) {
	got := requiresJSONOutput("system prompt with mangle_synth_v1", "")
	if !got {
		t.Error("expected true for mangle_synth_v1 in system prompt")
	}
}

func TestRequiresJSONOutput_WhenJSONMimeType_ShouldReturnTrue(t *testing.T) {
	got := requiresJSONOutput("", "please return application/json format")
	if !got {
		t.Error("expected true for application/json in user prompt")
	}
}

func TestRequiresJSONOutput_WhenNoMarker_ShouldReturnFalse(t *testing.T) {
	got := requiresJSONOutput("normal system prompt", "normal user prompt")
	if got {
		t.Error("expected false when no JSON markers present")
	}
}

func TestRequiresJSONOutput_WhenBothEmpty_ShouldReturnFalse(t *testing.T) {
	got := requiresJSONOutput("", "")
	if got {
		t.Error("expected false for empty prompts")
	}
}

// --- RecordLLMCall and GetLLMMetrics ---

func TestRecordLLMCall_WhenSuccess_ShouldUpdateMetrics(t *testing.T) {
	// Record a successful call
	RecordLLMCall("test_cat", "test_type", 100, 500, nil)

	snapshot := GetLLMMetrics()
	m, ok := snapshot["test_cat:test_type"]
	if !ok {
		t.Fatal("expected metrics for test_cat:test_type")
	}
	if m.Calls == 0 {
		t.Error("expected Calls > 0")
	}
	if m.TokensUsed == 0 {
		t.Error("expected TokensUsed > 0")
	}
}

func TestRecordLLMCall_WhenError_ShouldIncrementErrors(t *testing.T) {
	RecordLLMCall("err_cat", "err_type", 50, 200, errors.New("test error"))

	snapshot := GetLLMMetrics()
	m, ok := snapshot["err_cat:err_type"]
	if !ok {
		t.Fatal("expected metrics for err_cat:err_type")
	}
	if m.Errors == 0 {
		t.Error("expected Errors > 0")
	}
}

func TestGetLLMMetrics_ShouldReturnSnapshot(t *testing.T) {
	// Just verify it doesn't panic and returns a map
	snapshot := GetLLMMetrics()
	if snapshot == nil {
		t.Error("expected non-nil snapshot")
	}
}

// --- NewConsolidationWorker ---

func TestNewConsolidationWorker_WhenNilEngine_ShouldNotPanic(t *testing.T) {
	cw := NewConsolidationWorker(nil)
	if cw == nil {
		t.Fatal("expected non-nil worker")
	}
	if cw.queue == nil {
		t.Error("expected queue to be initialized")
	}
	if cw.quit == nil {
		t.Error("expected quit to be initialized")
	}
}
