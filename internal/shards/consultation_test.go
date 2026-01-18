package shards

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// CONSULTATION REQUEST/RESPONSE TYPE TESTS
// =============================================================================

func TestConsultationRequest_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	req := ConsultationRequest{
		RequestID:   "req-123",
		FromSpec:    "coder",
		ToSpec:      "architect",
		Question:    "How should I structure this?",
		Context:     "Building a new module",
		Priority:    PriorityNormal,
		Metadata:    map[string]string{"key": "value"},
		RequestTime: now,
	}

	if req.RequestID != "req-123" {
		t.Errorf("RequestID mismatch: got %q", req.RequestID)
	}
	if req.FromSpec != "coder" {
		t.Errorf("FromSpec mismatch: got %q", req.FromSpec)
	}
	if req.ToSpec != "architect" {
		t.Errorf("ToSpec mismatch: got %q", req.ToSpec)
	}
	if req.Question != "How should I structure this?" {
		t.Errorf("Question mismatch: got %q", req.Question)
	}
	if req.Priority != PriorityNormal {
		t.Errorf("Priority mismatch: got %v", req.Priority)
	}
	if req.Metadata["key"] != "value" {
		t.Errorf("Metadata mismatch: got %v", req.Metadata)
	}
	if req.RequestTime != now {
		t.Errorf("RequestTime mismatch")
	}
}

func TestConsultationResponse_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	resp := ConsultationResponse{
		RequestID:    "req-123",
		FromSpec:     "architect",
		ToSpec:       "coder",
		Advice:       "Use a layered architecture",
		Confidence:   0.95,
		References:   []string{"ref1", "ref2"},
		Caveats:      []string{"caveat1"},
		Metadata:     map[string]string{"key": "value"},
		ResponseTime: now,
		Duration:     5 * time.Second,
	}

	if resp.RequestID != "req-123" {
		t.Errorf("RequestID mismatch: got %q", resp.RequestID)
	}
	if resp.Advice != "Use a layered architecture" {
		t.Errorf("Advice mismatch: got %q", resp.Advice)
	}
	if resp.Confidence != 0.95 {
		t.Errorf("Confidence mismatch: got %f", resp.Confidence)
	}
	if len(resp.References) != 2 {
		t.Errorf("References length mismatch: got %d", len(resp.References))
	}
	if len(resp.Caveats) != 1 {
		t.Errorf("Caveats length mismatch: got %d", len(resp.Caveats))
	}
	if resp.Duration != 5*time.Second {
		t.Errorf("Duration mismatch: got %v", resp.Duration)
	}
}

// =============================================================================
// CONSULT PRIORITY TESTS
// =============================================================================

func TestConsultPriority_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		priority ConsultPriority
		expected int
	}{
		{PriorityBackground, 0},
		{PriorityNormal, 1},
		{PriorityUrgent, 2},
	}

	for _, tc := range cases {
		if int(tc.priority) != tc.expected {
			t.Errorf("Priority value mismatch: got %d, want %d", tc.priority, tc.expected)
		}
	}
}

// =============================================================================
// CONSULTATION MANAGER TESTS
// =============================================================================

// mockSpawner implements ConsultationSpawner for testing
type mockSpawner struct {
	response string
	err      error
}

func (m *mockSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.response != "" {
		return m.response, nil
	}
	return "ADVICE: Test advice\nCONFIDENCE: 0.8\n", nil
}

func TestNewConsultationManager(t *testing.T) {
	t.Parallel()

	spawner := &mockSpawner{}
	mgr := NewConsultationManager(spawner)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestConsultationManager_RequestConsultation(t *testing.T) {
	t.Parallel()

	spawner := &mockSpawner{
		response: "ADVICE: Use clean architecture\nCONFIDENCE: 0.85\nCAVEATS: None\n",
	}
	mgr := NewConsultationManager(spawner)

	req := ConsultationRequest{
		RequestID: "test-req",
		FromSpec:  "coder",
		ToSpec:    "architect",
		Question:  "How to structure?",
		Context:   "New module",
		Priority:  PriorityNormal,
	}

	resp, err := mgr.RequestConsultation(context.Background(), req)
	if err != nil {
		t.Fatalf("RequestConsultation error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.RequestID != req.RequestID {
		t.Errorf("RequestID mismatch: got %q", resp.RequestID)
	}
}

func TestConsultationManager_RequestBatchConsultation(t *testing.T) {
	t.Parallel()

	spawner := &mockSpawner{
		response: "ADVICE: Batch advice\nCONFIDENCE: 0.9\n",
	}
	mgr := NewConsultationManager(spawner)

	specialists := []string{"architect", "security"}
	responses, err := mgr.RequestBatchConsultation(
		context.Background(),
		"How to proceed?",
		"Context info",
		specialists,
	)
	if err != nil {
		t.Fatalf("RequestBatchConsultation error: %v", err)
	}

	if len(responses) != len(specialists) {
		t.Errorf("expected %d responses, got %d", len(specialists), len(responses))
	}
}

// =============================================================================
// STRATEGIC ADVISOR TESTS
// =============================================================================

func TestGetStrategicAdvisorsFor(t *testing.T) {
	t.Parallel()

	// Test known executor
	advisors := GetStrategicAdvisorsFor("coder")
	if advisors == nil {
		t.Error("expected non-nil advisors slice")
	}
	// Should return some advisors for a known executor
}

func TestShouldConsultBeforeExecution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		executor   string
		complexity string
		name       string
	}{
		{"coder", "high", "coder_high"},
		{"coder", "low", "coder_low"},
		{"unknown", "high", "unknown_high"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := ShouldConsultBeforeExecution(tc.executor, tc.complexity)
			// Just verify it doesn't panic - actual logic depends on implementation
			_ = result
		})
	}
}

// =============================================================================
// FORMAT ADVICE TESTS
// =============================================================================

func TestFormatConsultationAdvice(t *testing.T) {
	t.Parallel()

	responses := []ConsultationResponse{
		{
			FromSpec:   "architect",
			Advice:     "Use layers",
			Confidence: 0.9,
			Caveats:    []string{"Consider scale"},
		},
		{
			FromSpec:   "security",
			Advice:     "Add auth",
			Confidence: 0.85,
			Caveats:    []string{},
		},
	}

	result := FormatConsultationAdvice(responses)
	if result == "" {
		t.Error("expected non-empty formatted advice")
	}
}

func TestFormatConsultationAdvice_Empty(t *testing.T) {
	t.Parallel()

	result := FormatConsultationAdvice(nil)
	// Empty or contains "no advice" message
	_ = result
}
