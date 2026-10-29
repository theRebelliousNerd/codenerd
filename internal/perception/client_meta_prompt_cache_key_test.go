package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// sentCacheKeys runs tool-loop requests through the Responses path and returns
// the prompt_cache_key each carried on the wire.
func sentCacheKeys(t *testing.T, requests []struct {
	system string
	tools  []ToolDefinition
}) []string {
	t.Helper()
	var mu sync.Mutex
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			PromptCacheKey string `json:"prompt_cache_key"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		keys = append(keys, req.PromptCacheKey)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metaResponsesOKReply))
	}))
	defer srv.Close()

	client := newTestCompatClient(t, ProviderMeta, srv.URL)
	for _, r := range requests {
		if _, err := client.CompleteWithToolResults(context.Background(), r.system, metaResponsesHistory(), r.tools); err != nil {
			t.Fatalf("CompleteWithToolResults: %v", err)
		}
	}
	return keys
}

// The prompt cache key follows the tool catalog, the first bytes on the wire:
// two turns of one persona (same catalog, system prompts that differ in their
// per-turn tail) share a route, so a new turn's first round can reuse the
// skeleton the last one cached; a different catalog -- another persona, slot
// or shard -- gets its own route instead of competing for one. It used to be
// one key for every request (codenerd:<model>:<maxOutput>).
func TestMetaPromptCacheKeyFollowsTheToolCatalog(t *testing.T) {
	coder := []ToolDefinition{
		{Name: "read_file", Description: "read a file", InputSchema: map[string]any{"type": "object"}},
		{Name: "edit_lines", Description: "edit lines", InputSchema: map[string]any{"type": "object"}},
	}
	reviewer := []ToolDefinition{
		{Name: "read_file", Description: "read a file", InputSchema: map[string]any{"type": "object"}},
		{Name: "git_diff", Description: "show the diff", InputSchema: map[string]any{"type": "object"}},
	}
	keys := sentCacheKeys(t, []struct {
		system string
		tools  []ToolDefinition
	}{
		{"SKELETON\n\nturn one: retrieved knowledge A", coder},
		{"SKELETON\n\nturn two: retrieved knowledge B", coder},
		{"SKELETON\n\nturn one: retrieved knowledge A", reviewer},
	})
	if len(keys) != 3 {
		t.Fatalf("server saw %d requests, want 3", len(keys))
	}
	for i, k := range keys {
		if k == "" {
			t.Fatalf("request %d carried no prompt_cache_key", i)
		}
	}
	if keys[0] != keys[1] {
		t.Errorf("two turns on one catalog got different routes: %q vs %q", keys[0], keys[1])
	}
	if keys[0] == keys[2] {
		t.Errorf("two catalogs share one route %q", keys[0])
	}
}
