package policy

import (
	"os"
	"testing"

	"codenerd/internal/mangle"
)

func TestBrowserReasoningRulesAreSessionScopedAndTyped(t *testing.T) {
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer engine.Close()
	for _, path := range []string{"../schemas_browser.mg", "browser.mg"} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if loadErr := engine.LoadSchemaString(string(content)); loadErr != nil {
			t.Fatalf("load %s: %v", path, loadErr)
		}
	}

	if err := engine.AddFacts([]mangle.Fact{
		{Predicate: "net_request", Args: []any{"session-a", "req-a", "GET", "/api/fail", "fetch", int64(1000)}},
		{Predicate: "net_response", Args: []any{"session-a", "req-a", int64(503), int64(10), int64(1400)}},
		{Predicate: "net_request", Args: []any{"session-b", "req-b", "GET", "/api/ok", "fetch", int64(1000)}},
		{Predicate: "net_response", Args: []any{"session-b", "req-b", int64(200), int64(5), int64(10)}},
		{Predicate: "console_event", Args: []any{"session-a", "error", "boom", int64(1100)}},
		{Predicate: "toast_notification", Args: []any{"session-a", "save failed", "error", "dom", int64(1200)}},
		{Predicate: "browser_page_state", Args: []any{"session-a", "/form", "/false", "/true", int64(1300)}},
	}); err != nil {
		t.Fatalf("AddFacts: %v", err)
	}

	failed, err := engine.GetFacts("failed_request_at")
	if err != nil || len(failed) != 1 || failed[0].Args[0] != "session-a" || failed[0].Args[3] != int64(503) {
		t.Fatalf("failed_request_at = %+v, %v", failed, err)
	}
	slow, err := engine.GetFacts("slow_api_at")
	if err != nil || len(slow) != 1 || slow[0].Args[0] != "session-a" {
		t.Fatalf("slow_api_at = %+v, %v", slow, err)
	}
	visible, err := engine.GetFacts("user_visible_error")
	if err != nil || len(visible) != 2 {
		t.Fatalf("user_visible_error = %+v, %v", visible, err)
	}
	blocked, err := engine.GetFacts("interaction_blocked_at")
	if err != nil || len(blocked) != 1 || blocked[0].Args[0] != "session-a" {
		t.Fatalf("interaction_blocked_at = %+v, %v", blocked, err)
	}
}
