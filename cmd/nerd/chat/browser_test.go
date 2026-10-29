package chat

import (
	"strings"
	"testing"
)

// TestBrowserCommand_NilManager verifies the /browser handler copes with a nil
// manager — the common case before the first browser use. It must not panic and
// must add exactly one assistant message mentioning that browser automation is
// not running.
func TestBrowserCommand_NilManager(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	// NewTestModel leaves browserMgr nil; be explicit.
	m.browserMgr = nil
	initial := len(m.history)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleCmdBrowser panicked with nil manager: %v", r)
		}
	}()

	newModel, cmd := m.handleCmdBrowser("/browser", []string{"/browser"})
	if cmd != nil {
		t.Errorf("expected nil Cmd for status report, got %v", cmd)
	}
	result, ok := newModel.(Model)
	if !ok {
		t.Fatalf("handleCmdBrowser returned wrong model type %T", newModel)
	}
	if len(result.history) != initial+1 {
		t.Fatalf("expected %d messages, got %d (history=%v)", initial+1, len(result.history), result.history)
	}
	last := result.history[len(result.history)-1]
	if last.Role != "assistant" {
		t.Errorf("expected assistant role, got %q", last.Role)
	}
	if !strings.Contains(strings.ToLower(last.Content), "browser automation is not running") {
		t.Errorf("expected message to mention 'browser automation is not running', got %q", last.Content)
	}
	// Status report must not switch to ListView (it is not a picker).
	if result.viewMode == ListView {
		t.Errorf("expected viewMode != ListView for status report, got ListView")
	}
}

// TestBrowserCommand_ViaHandleCommand_Nil ensures the dispatcher routes
// /browser to the handler even when the manager is nil.
func TestBrowserCommand_ViaHandleCommand_Nil(t *testing.T) {
	t.Parallel()
	m := NewTestModel()
	m.browserMgr = nil
	initial := len(m.history)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleCommand /browser panicked: %v", r)
		}
	}()

	newModel, _ := m.handleCommand("/browser")
	result, ok := newModel.(Model)
	if !ok {
		t.Fatalf("handleCommand returned wrong type %T", newModel)
	}
	if len(result.history) != initial+1 {
		t.Fatalf("expected %d messages via handleCommand, got %d", initial+1, len(result.history))
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(strings.ToLower(last.Content), "browser automation is not running") {
		t.Errorf("handleCommand /browser should mention 'browser automation is not running', got %q", last.Content)
	}
}
