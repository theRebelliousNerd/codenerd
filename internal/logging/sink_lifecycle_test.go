package logging

import (
	"strings"
	"testing"
	"time"
)

// CloseAll closed only the category loggers, so a caller doing the obvious
// thing at shutdown leaked the audit log and the LLM I/O trace.

func TestCloseAll_WhenCalled_ShouldCloseAuditAndLLMIOToo(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Get(CategoryKernel).Info("category sink open")
	Audit().SessionStart("session-1")
	LogLLMResponse("callsite", "response body", time.Millisecond, 1)

	if auditFile == nil {
		t.Fatal("expected the audit sink to be open before CloseAll")
	}
	if llmIO == nil || !llmIO.enabled {
		t.Fatal("expected the LLM I/O sink to be open before CloseAll")
	}

	CloseAll()

	if auditFile != nil {
		t.Error("CloseAll left the audit log open")
	}
	if llmIO != nil && llmIO.enabled {
		t.Error("CloseAll left the LLM I/O trace open")
	}

	// Closing must flush, not discard.
	if audit := readLog(t, ws, "audit"); !strings.Contains(audit, "session-1") {
		t.Error("audit content was lost on close")
	}
	if trace := readLog(t, ws, "llm_io"); !strings.Contains(trace, "response body") {
		t.Error("LLM I/O content was lost on close")
	}
}

func TestCloseAll_WhenCalledTwice_ShouldNotPanic(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "trace_llm_io": true`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Get(CategoryKernel).Info("line")
	CloseAll()
	CloseAll()
	CloseAudit()
	CloseLLMIOLogger()
}

func TestCloseAll_WhenLoggingAfterClose_ShouldNotPanic(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	stale := Get(CategoryKernel)
	CloseAll()

	// A handle captured before shutdown must degrade quietly: logging is not
	// allowed to take down the process it observes.
	stale.Info("after close")
	stale.Error("after close")
	Audit().SessionEnd("s", 1, 1)
}
