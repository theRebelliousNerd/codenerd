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

func TestExtractStringField(t *testing.T) {
	raw := `{"decision_summary":"thinking about it","other":"x"}`
	if got := extractStringField(raw, "decision_summary"); got != "thinking about it" {
		t.Errorf("extractStringField(decision_summary)=%q", got)
	}
	if got := extractStringField(raw, "missing"); got != "" {
		t.Errorf("missing field should yield empty, got %q", got)
	}
}

func TestTruncatedEnvelopeMessage(t *testing.T) {
	withSummary := truncatedEnvelopeMessage(`{"control_packet":{"decision_summary":"Fix the nil check in validateToken because the token can be absent.`)
	if !strings.Contains(withSummary, "It had decided:") ||
		!strings.Contains(withSummary, "Fix the nil check in validateToken because the token can be absent.") {
		t.Errorf("truncated message should surface the decision summary, got %q", withSummary)
	}
	// The retired field name is not read: a reply that still carries it gets
	// the generic fallback, not a recovered summary.
	if got := truncatedEnvelopeMessage(`{"reasoning_trace":"I was analyzing the bug"}`); strings.Contains(got, "I was analyzing the bug") {
		t.Errorf("retired reasoning_trace must not be recovered, got %q", got)
	}
	// No decision summary -> generic fallback (does not contain the "had decided" lead-in).
	if strings.Contains(truncatedEnvelopeMessage(`{}`), "had decided") {
		t.Error("no-summary message should be the generic fallback")
	}
}

// TestLooksLikePartialEnvelope_DecisionSummary pins the renamed marker: a
// truncated envelope that reached only the decision summary is still seen as
// an envelope, so its control JSON is never shown to the user as a reply.
func TestLooksLikePartialEnvelope_DecisionSummary(t *testing.T) {
	if !looksLikePartialEnvelope(`{"decision_summary":"Read the file first because`) {
		t.Error("a partial envelope opening with decision_summary must be recognised")
	}
}
