package logging

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The trace had only disabled-path coverage: every existing test asserted that
// nothing is written when trace_llm_io is off, so the format markers the log is
// read by (and the redaction that makes it safe to keep) were unverified.

func TestLLMIOTrace_WhenTracingEnabled_ShouldWriteExpectedMarkers(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !IsLLMIOTracingEnabled() {
		t.Fatal("expected LLM I/O tracing to be enabled")
	}

	LogLLMRequest("perception-transducer",
		"you are a transducer",
		"convert this",
		[]LLMMessage{{Role: "user", Content: "earlier turn"}},
		"claude-opus-4", 0.2)
	LogLLMResponse("perception-transducer", "user_intent(/edit).", 1500*time.Millisecond, 7)
	LogLLMError("articulation-emitter", errors.New("429 rate limited"), 250*time.Millisecond)

	CloseLLMIOLogger()
	trace := readLog(t, ws, "llm_io")

	for _, marker := range []string{
		"═══ LLM REQUEST [perception-transducer]",
		"MODEL: claude-opus-4",
		"TEMPERATURE: 0.20",
		"─── BEGIN SYSTEM PROMPT ───",
		"you are a transducer",
		"─── END SYSTEM PROMPT ───",
		"CONVERSATION HISTORY (1 turns):",
		"[1][USER] earlier turn",
		"─── BEGIN USER PROMPT ───",
		"convert this",
		"TOTAL ESTIMATED TOKENS:",
		"═══ END REQUEST [perception-transducer] ═══",
		"═══ LLM RESPONSE [perception-transducer]",
		"user_intent(/edit).",
		"─── END RESPONSE ───",
		"═══ LLM ERROR [articulation-emitter]",
		"429 rate limited",
	} {
		if !strings.Contains(trace, marker) {
			t.Errorf("trace missing marker %q\n---\n%s", marker, trace)
		}
	}
}

func TestLLMIOTrace_WhenPromptCarriesSecrets_ShouldRedactByDefault(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	const key = "sk-live0123456789abcdefXYZ"
	LogLLMRequest("coder-shard",
		"env: ANTHROPIC_API_KEY="+key,
		"run the tool with Authorization: Bearer tok-0123456789abcdef",
		[]LLMMessage{{Role: "assistant", Content: "previous key was ghp_0123456789abcdefghijABCDEF"}},
		"model", 0)
	LogLLMResponse("coder-shard", "echoing "+key, time.Second, 3)
	LogLLMError("coder-shard", errors.New("401 for key "+key), time.Second)

	CloseLLMIOLogger()
	trace := readLog(t, ws, "llm_io")

	for _, secret := range []string{key, "tok-0123456789abcdef", "ghp_0123456789abcdefghijABCDEF"} {
		if strings.Contains(trace, secret) {
			t.Errorf("secret %q reached the LLM I/O trace on disk", secret)
		}
	}
	if !strings.Contains(trace, RedactionPlaceholder) {
		t.Error("expected redaction placeholder in trace")
	}
	// Redaction must not cost the reader the surrounding context.
	if !strings.Contains(trace, "ANTHROPIC_API_KEY") {
		t.Error("expected the key NAME to survive so the leak is identifiable")
	}
}

func TestLLMIOTrace_WhenRawTraceOptIn_ShouldNotRedact(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true, "trace_llm_io_raw": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	const key = "sk-live0123456789abcdefXYZ"
	LogLLMResponse("coder-shard", "echoing "+key, time.Second, 3)
	CloseLLMIOLogger()

	trace := readLog(t, ws, "llm_io")
	if !strings.Contains(trace, key) {
		t.Error("trace_llm_io_raw must disable redaction; the value was masked anyway")
	}
}

func TestLLMIOTrace_WhenTracingDisabled_ShouldWriteNoFile(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if IsLLMIOTracingEnabled() {
		t.Fatal("tracing must stay off unless trace_llm_io is set")
	}
	LogLLMRequest("x", "system", "user", nil, "model", 0)

	if matches := globLogs(t, ws, "llm_io"); len(matches) != 0 {
		t.Errorf("expected no llm_io log, found %v", matches)
	}
}

func TestLLMIOTrace_WhenPromptLengthReported_ShouldCountOriginalChars(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	system := "key=sk-live0123456789abcdefXYZ"
	LogLLMRequest("x", system, "", nil, "model", 0)
	CloseLLMIOLogger()

	// Context accounting must describe what the model received, not what
	// survived redaction, or the trace misreports token usage.
	trace := readLog(t, ws, "llm_io")
	want := "SYSTEM PROMPT (" + itoa(len(system)) + " chars"
	if !strings.Contains(trace, want) {
		t.Errorf("expected %q in trace\n---\n%s", want, trace)
	}
}
