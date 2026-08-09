//go:build integration

package research_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/browser"
	browsersecurity "codenerd/internal/browser/security"
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
	specRoot := filepath.Join(workspace, ".nerd", "browser", "specs")
	if err := os.MkdirAll(specRoot, 0o700); err != nil {
		t.Fatalf("create live spec root: %v", err)
	}
	liveSpec := `---
name: BPAR live failure contract
binding:
  - { kind: route, target: / }
invariants:
  - name: visible-error-correlated
    query: "user_visible_error(S, Kind, Message, Timestamp)"
    expect: present
  - name: no-fatal-console
    query: "console_event(S, \"fatal\", Message, Timestamp)"
    expect: absent
---
# BPAR live failure contract
The test route must correlate its expected failure without a fatal console event.
`
	if err := os.WriteFile(filepath.Join(specRoot, "live-failure.md"), []byte(liveSpec), 0o600); err != nil {
		t.Fatalf("write live spec: %v", err)
	}
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
		"include_specs": true, "spec_terms": []any{"failure"},
	})
	if err != nil {
		t.Fatalf("browser_observe: %v", err)
	}
	if !strings.Contains(observeRaw, `"spec_context"`) || !strings.Contains(observeRaw, "BPAR live failure contract") {
		t.Fatalf("browser_observe missing spec context: %s", observeRaw)
	}
	triggerRef := findInteractiveRef(t, observeRaw, "Trigger failure")

	actRaw, err := research.BrowserActTool().Execute(ctx, map[string]any{
		"session_id": session.ID,
		"operations": []any{map[string]any{"type": "interact", "ref": triggerRef, "action": "click"}},
		"view":       "full", "include_specs": true,
	})
	if err != nil {
		t.Fatalf("browser_act: %v", err)
	}
	if !strings.Contains(actRaw, `"spec_context"`) {
		t.Fatalf("browser_act missing spec context: %s", actRaw)
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

	specGetRaw, err := research.BrowserSpecsTool().Execute(ctx, map[string]any{
		"operation": "get", "session_id": session.ID, "terms": []any{"failure"}, "view": "full",
	})
	if err != nil || !strings.Contains(specGetRaw, "BPAR live failure contract") {
		t.Fatalf("browser_specs live get: %v, %s", err, specGetRaw)
	}
	specCheckRaw, err := research.BrowserSpecsTool().Execute(ctx, map[string]any{
		"operation": "check", "session_id": session.ID, "view": "full", "diagnose_on_failure": false,
	})
	if err != nil || !strings.Contains(specCheckRaw, `"status":"passed"`) || !strings.Contains(specCheckRaw, `"checked":2`) {
		t.Fatalf("browser_specs live check: %v, %s", err, specCheckRaw)
	}

	evidenceRaw, err := research.BrowserEvidenceTool().Execute(ctx, map[string]any{
		"operation": "read", "session_id": session.ID, "max_items": 100,
	})
	if err != nil || !strings.Contains(evidenceRaw, `"type":"act"`) ||
		!strings.Contains(evidenceRaw, `"type":"reason"`) || strings.Contains(evidenceRaw, "foreign-session-marker") {
		t.Fatalf("browser_evidence live read: %v, %s", err, evidenceRaw)
	}
	exportRaw, err := research.BrowserEvidenceTool().Execute(ctx, map[string]any{
		"operation": "export", "session_id": session.ID, "max_items": 100,
	})
	if err != nil {
		t.Fatalf("browser_evidence live export: %v", err)
	}
	var exported map[string]any
	if err := json.Unmarshal([]byte(exportRaw), &exported); err != nil {
		t.Fatalf("decode browser_evidence export: %v", err)
	}
	exportPath := exported["path"].(string)
	private, err := browsersecurity.IsPrivatePath(exportPath, false)
	if err != nil || !private {
		t.Fatalf("browser evidence export is not current-user-only: private=%v err=%v path=%s", private, err, exportPath)
	}
	exportedBytes, err := os.ReadFile(exportPath)
	if err != nil || strings.Contains(string(exportedBytes), "foreign-session-marker") {
		t.Fatalf("browser evidence export scope: %v, %s", err, exportedBytes)
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
