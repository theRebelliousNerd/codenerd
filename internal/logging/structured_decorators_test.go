package logging

import (
	"strings"
	"testing"
)

// ContextLogger and RequestLogger hardcoded text output, so a workspace running
// in json_format produced a file that was only mostly JSONL — and the lines
// that were dropped by a JSON consumer were exactly the ones carrying
// correlation IDs and context.

func TestContextLogger_WhenJSONFormat_ShouldWriteStructuredEntryWithFields(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "format": "json"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Get(CategoryKernel).WithContext(map[string]any{"shard": "coder-1"}).Info("context line")
	CloseAll()

	entry := findJSONEntry(t, readLog(t, ws, "kernel"), "context line")
	if entry["cat"] != string(CategoryKernel) {
		t.Errorf("category missing from entry: %v", entry)
	}
	if entry["lvl"] != "info" {
		t.Errorf("level missing from entry: %v", entry)
	}
	fields, ok := entry["fields"].(map[string]any)
	if !ok || fields["shard"] != "coder-1" {
		t.Errorf("context was not carried as structured fields: %v", entry)
	}
}

func TestRequestLogger_WhenJSONFormat_ShouldCarryRequestIDAsField(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "format": "json"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	WithRequestID(CategoryKernel, "req-42").WithField("op", "read").Info("request line")
	CloseAll()

	entry := findJSONEntry(t, readLog(t, ws, "kernel"), "request line")
	if entry["req"] != "req-42" {
		t.Errorf("request id not carried structurally: %v", entry)
	}
	fields, ok := entry["fields"].(map[string]any)
	if !ok || fields["op"] != "read" {
		t.Errorf("request fields not carried: %v", entry)
	}
}

func TestRequestLogger_WhenTextFormat_ShouldKeepLegacyLineShape(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	WithRequestID(CategoryKernel, "req-7").Info("plain request")
	CloseAll()

	if got := readLog(t, ws, "kernel"); !strings.Contains(got, "[INFO] [req:req-7] plain request") {
		t.Errorf("text output shape changed:\n%s", got)
	}
}

func TestRequestLogger_WhenErrorLogged_ShouldMirrorToProblemsLog(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	WithRequestID(CategoryKernel, "req-9").Error("request blew up")
	CloseAll()

	if got := readLog(t, ws, "problems"); !strings.Contains(got, "request blew up") {
		t.Errorf("request-scoped errors must reach the aggregated problems log:\n%s", got)
	}
}

func TestStructuredEntry_WhenJSONFormat_ShouldCarryCallerFileAndLine(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug", "format": "json"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Get(CategoryKernel).Info("caller line")
	// Through a convenience wrapper, which adds a stack frame: the caller must
	// still resolve to this test file rather than to logger_convenience.go.
	Kernel("wrapped caller line")
	CloseAll()

	content := readLog(t, ws, "kernel")
	for _, msg := range []string{"caller line", "wrapped caller line"} {
		entry := findJSONEntry(t, content, msg)
		file, _ := entry["file"].(string)
		if file != "structured_decorators_test.go" {
			t.Errorf("caller file for %q = %q, want structured_decorators_test.go", msg, file)
		}
		if line, _ := entry["line"].(float64); line <= 0 {
			t.Errorf("caller line for %q not populated: %v", msg, entry["line"])
		}
	}
}

func findJSONEntry(t *testing.T, content, message string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, message) {
			continue
		}
		if idx := strings.Index(line, "{"); idx >= 0 {
			entry := firstJSONLine(t, line[idx:])
			if entry["msg"] == message {
				return entry
			}
		}
	}
	t.Fatalf("no JSON entry with msg %q in:\n%s", message, content)
	return nil
}
