package browser

import (
	"strings"
	"testing"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/mangle"
)

func TestSessionManagerRedactsFactsBeforeSink(t *testing.T) {
	sink := &mockEngineSink{}
	cfg := DefaultConfig()
	cfg.ExtraSensitiveKeys = []string{"workspace-secret"}
	sm := NewSessionManagerWithSink(cfg, sink)
	facts := []mangle.Fact{
		{Predicate: "net_header", Args: []any{"session-1", "request-1", "req", "authorization", "Bearer top-secret"}},
		{Predicate: "input_event", Args: []any{"session-1", "password-field", "hunter2", int64(1)}},
		{Predicate: "react_prop", Args: []any{"component-1", "api_key", "key-value"}},
		{Predicate: "current_url", Args: []any{"session-1", "https://example.test/?workspace_secret=hidden"}},
	}
	if err := sm.addFacts(facts); err != nil {
		t.Fatalf("addFacts() error = %v", err)
	}

	got := sink.getFacts()
	if got[0].Args[4] != "Bearer "+browsersecurity.Redacted {
		t.Fatalf("authorization header was not redacted: %#v", got[0].Args[4])
	}
	if got[1].Args[2] != browsersecurity.Redacted || got[2].Args[2] != browsersecurity.Redacted {
		t.Fatalf("input/prop secrets were not redacted: %#v %#v", got[1], got[2])
	}
	if strings.Contains(got[3].Args[1].(string), "hidden") {
		t.Fatalf("URL secret was not redacted: %#v", got[3].Args[1])
	}
	if facts[0].Args[4] != "Bearer top-secret" {
		t.Fatal("fact redaction mutated the producer's input")
	}
}

func TestSessionManagerResolveOutputPathUsesWorkspacePolicy(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = workspace
	sm := NewSessionManagerWithSink(cfg, nil)
	valid, err := sm.ResolveOutputPath("shot.png", ".nerd/browser/screenshots", "default.png")
	if err != nil || !strings.HasPrefix(valid, workspace) {
		t.Fatalf("valid output path = %q, %v", valid, err)
	}
	if _, err := sm.ResolveOutputPath("../escape.png", ".nerd/browser/screenshots", "default.png"); err == nil {
		t.Fatal("manager output policy accepted traversal")
	}
}
