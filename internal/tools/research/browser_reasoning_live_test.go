//go:build integration

package research_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/tools/research"
	"codenerd/internal/types"
)

type liveBrowserKernelSink struct{ kernel types.Kernel }

func (s liveBrowserKernelSink) AddFacts(facts []mangle.Fact) error {
	converted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
	}
	return s.kernel.AssertBatch(converted)
}

func TestBrowserReasoningToolsLiveCortexRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/fail":
			time.Sleep(120 * time.Millisecond)
			http.Error(w, "failed", http.StatusServiceUnavailable)
		case "/quiet":
			fmt.Fprint(w, `<html><body><p>Quiet session</p></body></html>`)
		default:
			fmt.Fprint(w, `<html><body><button id="trigger">Trigger failure</button><script>
document.getElementById('trigger').addEventListener('click', async () => {
  console.error('bpar3-console-marker');
  const toast = document.createElement('div');
  toast.setAttribute('role', 'alert');
  toast.className = 'toast error';
  toast.textContent = 'bpar3 save failed';
  document.body.appendChild(toast);
  await fetch('/fail');
});
</script></body></html>`)
		}
	}))
	defer server.Close()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	workspace := t.TempDir()
	cfg := browser.DefaultConfig()
	cfg.Headless = true
	cfg.EventThrottleMs = 10
	cfg.NavigationTimeoutMs = 10000
	cfg.WorkspaceRoot = workspace
	cfg.WritableRoots = []string{workspace}
	cfg.SessionStore = filepath.Join(workspace, "sessions.json")
	mgr := browser.NewSessionManagerWithSink(cfg, liveBrowserKernelSink{kernel: kernel})
	research.SetBrowserRuntime(mgr, kernel)
	defer research.ClearBrowserManager(mgr)
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session, err := mgr.CreateSession(ctx, server.URL)
	if err != nil {
		t.Skipf("Chrome unavailable: %v", err)
	}
	waitForBrowserHook(t, ctx, mgr, session.ID)

	observeRaw, err := research.BrowserObserveTool().Execute(ctx, map[string]any{
		"session_id": session.ID, "mode": "interactive", "view": "compact", "max_items": 10,
	})
	if err != nil {
		t.Fatalf("browser_observe: %v", err)
	}
	triggerRef := findInteractiveRef(t, observeRaw, "Trigger failure")

	actRaw, err := research.BrowserActTool().Execute(ctx, map[string]any{
		"session_id": session.ID,
		"operations": []any{map[string]any{"type": "interact", "ref": triggerRef, "action": "click"}},
		"view":       "full",
	})
	if err != nil {
		t.Fatalf("browser_act: %v", err)
	}
	var act map[string]any
	if err := json.Unmarshal([]byte(actRaw), &act); err != nil {
		t.Fatalf("decode browser_act: %v", err)
	}
	startedMS := int64(act["started_ms"].(float64))

	waitRaw, err := research.BrowserWaitTool().Execute(ctx, map[string]any{
		"session_id": session.ID, "mode": "conditions", "since_ms": startedMS,
		"conditions": []any{
			map[string]any{"predicate": "console_event", "match_args": []any{"error", "_", "_"}},
			map[string]any{"predicate": "failed_request_at", "match_args": []any{"_", "_", "_", "_"}},
			map[string]any{"predicate": "toast_notification", "match_args": []any{"_", "error", "_", "_"}},
		},
		"timeout_ms": 5000, "poll_interval_ms": 50,
	})
	if err != nil || !strings.Contains(waitRaw, `"status":"matched"`) {
		t.Fatalf("browser_wait conditions: %v, %s", err, waitRaw)
	}

	stableRaw, err := research.BrowserWaitTool().Execute(ctx, map[string]any{
		"session_id": session.ID, "mode": "stable", "since_ms": startedMS,
		"timeout_ms": 5000, "poll_interval_ms": 50, "network_idle_ms": 200, "dom_idle_ms": 100,
	})
	if err != nil || !strings.Contains(stableRaw, `"status":"stable"`) {
		t.Fatalf("browser_wait stable: %v, %s", err, stableRaw)
	}

	quiet, err := mgr.CreateTab(ctx, "", server.URL+"/quiet", false)
	if err != nil {
		t.Fatalf("create quiet session: %v", err)
	}
	waitForBrowserHook(t, ctx, mgr, quiet.ID)
	quietPage, _ := mgr.Page(quiet.ID)
	if _, err := quietPage.Context(ctx).Eval(`() => console.error('foreign-session-marker')`); err != nil {
		t.Fatalf("emit foreign console event: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	reasonRaw, err := research.BrowserReasonTool().Execute(ctx, map[string]any{
		"session_id": session.ID, "topic": "why_failed", "view": "full", "max_items": 20,
		"since_navigation": true,
	})
	if err != nil {
		t.Fatalf("browser_reason: %v", err)
	}
	if !strings.Contains(reasonRaw, `"status":"error"`) || !strings.Contains(reasonRaw, "bpar3-console-marker") ||
		!strings.Contains(reasonRaw, "bpar3 save failed") || !strings.Contains(reasonRaw, "/fail") ||
		strings.Contains(reasonRaw, "foreign-session-marker") {
		t.Fatalf("browser_reason missing scoped diagnosis: %s", reasonRaw)
	}

	mangleRaw, err := research.BrowserMangleTool().Execute(ctx, map[string]any{
		"operation": "query", "session_id": session.ID,
		"query": "failed_request_at(S, Request, URL, Status, Timestamp)", "view": "full", "max_items": 10,
	})
	if err != nil || !strings.Contains(mangleRaw, "/fail") || !strings.Contains(mangleRaw, "503") {
		t.Fatalf("browser_mangle live query: %v, %s", err, mangleRaw)
	}
}

func waitForBrowserHook(t *testing.T, ctx context.Context, mgr *browser.SessionManager, sessionID string) {
	t.Helper()
	page, ok := mgr.Page(sessionID)
	if !ok {
		t.Fatalf("missing page for %s", sessionID)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := page.Context(ctx).Eval(`() => Boolean(window.__browsernerdHooked)`)
		if err == nil && result.Value.Bool() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("browser event hook not installed for %s", sessionID)
}

func findInteractiveRef(t *testing.T, raw, label string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	data := payload["data"].(map[string]any)
	items := data["interactive"].([]any)
	for _, item := range items {
		row := item.(map[string]any)
		if row["label"] == label {
			return row["ref"].(string)
		}
	}
	t.Fatalf("interactive label %q not found in %s", label, raw)
	return ""
}
