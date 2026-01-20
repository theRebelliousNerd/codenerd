package articulation

import (
	"strings"
	"testing"
)

func TestApplyConstitutionalOverride(t *testing.T) {
	envelope := &PiggybackEnvelope{
		Surface: "Original response",
		Control: ControlPacket{
			MangleUpdates: []string{"allowed_atom", "blocked_atom", "another_allowed"},
		},
	}

	blocked := []string{"blocked_atom"}
	reason := "Violation of safety rule X"

	override := ApplyConstitutionalOverride(envelope, blocked, reason)

	if override == nil {
		t.Fatal("Expected override, got nil")
	}

	// Check surface modification
	if !strings.Contains(override.ModifiedSurface, "SAFETY NOTICE") {
		t.Error("Expected safety notice in modified surface")
	}
	if !strings.Contains(override.ModifiedSurface, reason) {
		t.Error("Expected reason in modified surface")
	}

	// Check envelope mutation (ApplyConstitutionalOverride mutates the envelope ptr)
	if len(envelope.Control.MangleUpdates) != 2 {
		t.Errorf("Expected 2 atoms remaining, got %d", len(envelope.Control.MangleUpdates))
	}
	for _, atom := range envelope.Control.MangleUpdates {
		if atom == "blocked_atom" {
			t.Error("Blocked atom still present")
		}
	}
}

func TestProcessLLMResponse(t *testing.T) {
	// 1. Valid JSON
	raw := `{"control_packet":{}, "surface_response": "Hello"}`
	res := ProcessLLMResponse(raw)

	if res.Surface != "Hello" {
		t.Errorf("Expected surface 'Hello', got '%s'", res.Surface)
	}
	if res.ParseMethod != "json" {
		t.Errorf("Expected method 'json', got '%s'", res.ParseMethod)
	}
	if res.Control == nil {
		t.Error("Expected control packet")
	}

	// 2. Invalid JSON (fallback)
	rawInvalid := `Just plain text`
	resInvalid := ProcessLLMResponse(rawInvalid)

	if resInvalid.Surface != "Just plain text" {
		t.Errorf("Expected fallback surface, got '%s'", resInvalid.Surface)
	}
	if resInvalid.ParseMethod != "fallback" {
		t.Errorf("Expected method 'fallback', got '%s'", resInvalid.ParseMethod)
	}
	if resInvalid.Control != nil {
		t.Error("Expected nil control packet for fallback")
	}
}

func TestAppendReasoningDirective(t *testing.T) {
	base := "You are a bot."

	full := AppendReasoningDirective(base, true)
	if !strings.Contains(full, "REASONING TRACE (MANDATORY)") {
		t.Error("Expected full directive")
	}

	short := AppendReasoningDirective(base, false)
	if !strings.Contains(short, "REASONING OUTPUT") {
		t.Error("Expected short directive")
	}
}
