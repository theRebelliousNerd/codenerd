package articulation

import (
	"strings"
	"testing"
)

func TestEmitterCreateAndMarshalEnvelope(t *testing.T) {
	e := NewEmitter()
	intent := IntentClassification{Category: "/instruction", Verb: "/fix", Confidence: 0.9}
	memOps := []MemoryOperation{{Op: "note", Key: "k", Value: "v"}}

	env := e.CreateEnvelope("hello", intent, []string{"foo(/bar)."}, memOps)
	if env.Surface != "hello" || env.Control.IntentClassification.Verb != "/fix" {
		t.Errorf("envelope not assembled correctly: %+v", env)
	}
	if len(env.Control.MangleUpdates) != 1 || len(env.Control.MemoryOperations) != 1 {
		t.Error("envelope updates/memory ops missing")
	}

	data, err := e.MarshalEnvelope(env)
	if err != nil || len(data) == 0 {
		t.Fatalf("MarshalEnvelope: err=%v len=%d", err, len(data))
	}
	if !strings.Contains(string(data), "surface_response") {
		t.Error("marshaled envelope missing surface_response key")
	}
}

func TestEnvelopeStateHelpers(t *testing.T) {
	env := PiggybackEnvelope{
		Control: ControlPacket{
			SelfCorrection:   &SelfCorrection{Triggered: true, Hypothesis: "h"},
			MemoryOperations: []MemoryOperation{{Op: "note"}, {Op: "forget"}, {Op: "note"}},
		},
	}
	if !HasSelfCorrection(env) {
		t.Error("HasSelfCorrection should be true")
	}
	if !HasMemoryOperations(env) {
		t.Error("HasMemoryOperations should be true")
	}
	if notes := GetMemoryOperationsByType(env, "note"); len(notes) != 2 {
		t.Errorf("GetMemoryOperationsByType(note)=%d, want 2", len(notes))
	}
	if forgets := GetMemoryOperationsByType(env, "forget"); len(forgets) != 1 {
		t.Errorf("GetMemoryOperationsByType(forget)=%d, want 1", len(forgets))
	}

	empty := PiggybackEnvelope{}
	if HasSelfCorrection(empty) || HasMemoryOperations(empty) {
		t.Error("empty envelope should report no self-correction/memory ops")
	}
}

func TestExtractSurfaceOnly(t *testing.T) {
	raw := `{"control_packet":{"intent_classification":{"category":"/x","verb":"/y","confidence":1.0}},"surface_response":"the answer"}`
	if got := ExtractSurfaceOnly(raw); got != "the answer" {
		t.Errorf("ExtractSurfaceOnly(valid)=%q, want 'the answer'", got)
	}
	// Non-envelope input falls back to the trimmed raw text.
	if got := ExtractSurfaceOnly("  just text  "); got != "just text" {
		t.Errorf("ExtractSurfaceOnly(plain)=%q, want 'just text'", got)
	}
}

func TestExtractStringField(t *testing.T) {
	raw := `{"reasoning_trace":"thinking about it","other":"x"}`
	if got := extractStringField(raw, "reasoning_trace"); got != "thinking about it" {
		t.Errorf("extractStringField(reasoning_trace)=%q", got)
	}
	if got := extractStringField(raw, "missing"); got != "" {
		t.Errorf("missing field should yield empty, got %q", got)
	}
}

func TestTruncatedEnvelopeMessage(t *testing.T) {
	withReasoning := truncatedEnvelopeMessage(`{"reasoning_trace":"I was analyzing the bug"}`)
	if !strings.Contains(withReasoning, "I was analyzing the bug") {
		t.Errorf("truncated message should surface the reasoning trace, got %q", withReasoning)
	}
	// No reasoning trace -> generic fallback (does not contain the "working on" lead-in).
	if strings.Contains(truncatedEnvelopeMessage(`{}`), "working on") {
		t.Error("no-reasoning message should be the generic fallback")
	}
}
