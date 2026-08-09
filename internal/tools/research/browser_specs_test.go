package research

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/browser"
)

func TestBrowserSpecsListAndGetBoundedWorkspaceDocs(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, ".nerd", "browser", "specs")
	if err := os.MkdirAll(specRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	doc := `---
name: Login flow
source: web/Login.tsx
binding:
  - { kind: route, target: /login }
tags: [authentication]
invariants:
  - name: no-visible-error
    query: "user_visible_error(S, _, _, _)"
    expect: absent
---
# Login flow
Successful authentication reaches the dashboard at https://example.test/callback?token=secret-value.
`
	if err := os.WriteFile(filepath.Join(specRoot, "login.md"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := browser.DefaultConfig()
	cfg.WorkspaceRoot = root
	manager := browser.NewSessionManagerWithSink(cfg, nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)

	listed, err := BrowserSpecsTool().Execute(context.Background(), map[string]any{"operation": "list"})
	if err != nil || !strings.Contains(listed, "Login flow") {
		t.Fatalf("browser_specs list: %v, %s", err, listed)
	}
	got, err := BrowserSpecsTool().Execute(context.Background(), map[string]any{
		"operation": "get", "route": "/login/reset", "terms": []any{"authentication"}, "view": "full",
	})
	if err != nil {
		t.Fatalf("browser_specs get: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatal(err)
	}
	if result["count"] != float64(1) || !strings.Contains(got, "Successful authentication") || !strings.Contains(got, "no-visible-error") || strings.Contains(got, "secret-value") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("browser_specs get result: %s", got)
	}
}

func TestBrowserSpecsRejectsUnboundedInputs(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.WorkspaceRoot = t.TempDir()
	manager := browser.NewSessionManagerWithSink(cfg, nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)

	terms := make([]any, 21)
	for index := range terms {
		terms[index] = "term"
	}
	if _, err := BrowserSpecsTool().Execute(context.Background(), map[string]any{
		"operation": "get", "terms": terms,
	}); err == nil || !strings.Contains(err.Error(), "terms exceeds") {
		t.Fatalf("expected term bound, got %v", err)
	}
	if _, err := BrowserSpecsTool().Execute(context.Background(), map[string]any{
		"operation": "get", "file": "x.go", "from": 10,
	}); err == nil || !strings.Contains(err.Error(), "provided together") {
		t.Fatalf("expected range validation, got %v", err)
	}
}
