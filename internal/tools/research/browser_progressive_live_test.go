//go:build integration

package research

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/mangle"
	"codenerd/internal/tools"

	"github.com/go-rod/rod/lib/launcher"
)

type progressiveLiveSink struct {
	mu    sync.Mutex
	facts []mangle.Fact
}

func (s *progressiveLiveSink) AddFacts(facts []mangle.Fact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facts = append(s.facts, facts...)
	return nil
}

func (s *progressiveLiveSink) containsString(value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, fact := range s.facts {
		for _, arg := range fact.Args {
			if text, ok := arg.(string); ok && strings.Contains(text, value) {
				return true
			}
		}
	}
	return false
}

func TestBrowserProgressiveTools_Live(t *testing.T) {
	path, found := launcher.LookPath()
	if !found || path == "" {
		t.Skip("Chrome not found")
	}
	controlURL, err := launcher.New().Headless(true).Launch()
	if err != nil {
		t.Skipf("launch Chrome: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/next" {
			fmt.Fprintln(w, `<html><body><h1>Next</h1></body></html>`)
			return
		}
		fmt.Fprintln(w, `<html><body><button id="save">Save</button><input id="name"><input id="password" name="password" type="password"><a href="/next">Next</a><details><summary>More</summary><p>Hidden</p></details><table><tr><th>Name</th></tr><tr data-row-id="one"><td>One</td></tr></table></body></html>`)
	}))
	defer server.Close()

	cfg := browser.DefaultConfig()
	cfg.DebuggerURL = controlURL
	cfg.Headless = true
	cfg.WorkspaceRoot = t.TempDir()
	sink := &progressiveLiveSink{}
	mgr := browser.NewSessionManagerWithSink(cfg, sink)
	SetBrowserManager(mgr)
	defer ClearBrowserManager(mgr)
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("register tools: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	created := executeRegisteredJSON(t, ctx, registry, "browser_act", map[string]any{
		"view":       "full",
		"operations": []any{map[string]any{"type": "session_create", "url": server.URL}},
	})
	sessionID, _ := created["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("browser_act did not return a session: %#v", created)
	}

	observed := executeRegisteredJSON(t, ctx, registry, "browser_observe", map[string]any{
		"session_id": sessionID, "mode": "composite", "view": "full", "visible_only": true,
	})
	data, _ := observed["data"].(map[string]any)
	counts, _ := data["counts"].(map[string]any)
	if counts["navigation"].(float64) == 0 || counts["grids"].(float64) == 0 || counts["hidden"].(float64) == 0 {
		t.Fatalf("bounded discovery slices missing: %#v", counts)
	}
	elements, _ := data["interactive"].([]any)
	refs := make(map[string]string)
	for _, raw := range elements {
		element, _ := raw.(map[string]any)
		fingerprint, _ := element["fingerprint"].(map[string]any)
		id, _ := fingerprint["id"].(string)
		refs[id], _ = element["ref"].(string)
	}
	if refs["save"] == "" || refs["name"] == "" || refs["password"] == "" {
		t.Fatalf("missing live refs: %#v", refs)
	}

	const secret = "progressive-live-secret-8391"
	acted := executeRegisteredJSON(t, ctx, registry, "browser_act", map[string]any{
		"session_id": sessionID, "view": "full",
		"operations": []any{
			map[string]any{"type": "fill", "fields": []any{
				map[string]any{"ref": refs["name"], "value": "Ada"},
				map[string]any{"ref": refs["password"], "value": secret},
			}},
			map[string]any{"type": "interact", "ref": refs["save"], "action": "click"},
		},
	})
	actedJSON, _ := json.Marshal(acted)
	if success, _ := acted["success"].(bool); !success || strings.Contains(string(actedJSON), secret) || sink.containsString(secret) {
		t.Fatalf("live action failed or leaked secret: %s", actedJSON)
	}

	history := executeRegisteredJSON(t, ctx, registry, "browser_act", map[string]any{
		"session_id": sessionID, "view": "full",
		"operations": []any{
			map[string]any{"type": "key", "key": "End"},
			map[string]any{"type": "navigate", "url": server.URL + "/next"},
			map[string]any{"type": "history", "action": "back"},
		},
	})
	if success, _ := history["success"].(bool); !success {
		t.Fatalf("key/history action route failed: %#v", history)
	}

	refreshed := executeRegisteredJSON(t, ctx, registry, "browser_observe", map[string]any{
		"session_id": sessionID, "mode": "interactive", "view": "full", "visible_only": true,
	})
	refreshedData, _ := refreshed["data"].(map[string]any)
	refreshedElements, _ := refreshedData["interactive"].([]any)
	for _, raw := range refreshedElements {
		element, _ := raw.(map[string]any)
		fingerprint, _ := element["fingerprint"].(map[string]any)
		if fingerprint["id"] == "save" {
			refs["save"], _ = element["ref"].(string)
		}
	}

	stale := executeRegisteredJSON(t, ctx, registry, "browser_act", map[string]any{
		"session_id": sessionID, "view": "full",
		"operations": []any{
			map[string]any{"type": "navigate", "url": server.URL + "/next"},
			map[string]any{"type": "interact", "ref": refs["save"], "action": "click"},
		},
	})
	if success, _ := stale["success"].(bool); success {
		t.Fatalf("stale ref did not fail closed: %#v", stale)
	}

	if err := mgr.Navigate(ctx, sessionID, server.URL); err != nil {
		t.Fatalf("restore fixture page: %v", err)
	}
	screenshot := executeRegisteredJSON(t, ctx, registry, "browser_observe", map[string]any{
		"session_id": sessionID, "mode": "screenshot", "view": "compact",
	})
	screenshotData, _ := screenshot["data"].(map[string]any)
	screenshotMeta, _ := screenshotData["screenshot"].(map[string]any)
	screenshotPath, _ := screenshotMeta["path"].(string)
	if screenshotPath == "" || !strings.HasPrefix(filepath.Clean(screenshotPath), filepath.Clean(cfg.WorkspaceRoot)) {
		t.Fatalf("screenshot escaped workspace policy: %#v", screenshotMeta)
	}
	if _, err := os.Stat(screenshotPath); err != nil {
		t.Fatalf("screenshot evidence missing: %v", err)
	}
	executeRegisteredJSON(t, ctx, registry, "browser_observe", map[string]any{"session_id": sessionID, "mode": "dom_snapshot", "view": "summary"})
	executeRegisteredJSON(t, ctx, registry, "browser_observe", map[string]any{"session_id": sessionID, "mode": "react", "view": "summary"})
}

func executeRegisteredJSON(t *testing.T, ctx context.Context, registry *tools.Registry, name string, args map[string]any) map[string]any {
	t.Helper()
	result, err := registry.Execute(ctx, name, args)
	if err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Result), &decoded); err != nil {
		t.Fatalf("decode %s result %q: %v", name, result.Result, err)
	}
	return decoded
}
