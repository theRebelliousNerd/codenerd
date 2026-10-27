package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

func TestClassificationClientFor_AnthropicCarriesWorkspaceAndEndpointToHTTP(t *testing.T) {
	for _, override := range []bool{false, true} {
		name := "shared_settings"
		workspace := "wrkspc_shared"
		if override {
			name = "slot_override"
			workspace = "wrkspc_worker"
		}
		t.Run(name, func(t *testing.T) {
			var counted, inferred atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("anthropic-workspace-id") != workspace || r.Header.Get("anthropic-version") != config.AnthropicAPIVersion {
					t.Error("boot classification lost Anthropic workspace/version")
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/messages/count_tokens":
					counted.Add(1)
					io.WriteString(w, `{"input_tokens":3}`)
				case "/messages":
					inferred.Add(1)
					var req map[string]any
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					if req["model"] != "claude-haiku-5-5" || req["output_config"].(map[string]any)["effort"] != "low" {
						t.Error("classification lost worker model/low effort")
					}
					io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":1}}`)
				default:
					t.Errorf("unexpected route %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer srv.Close()
			app := &config.UserConfig{Provider: "anthropic", AnthropicAPIKey: "test-key", BaseURL: srv.URL, Anthropic: &config.AnthropicProviderConfig{WorkspaceID: "wrkspc_shared"}, Worker: &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5"}}
			if override {
				app.BaseURL = "http://127.0.0.1:1"
				app.Worker.Endpoint = srv.URL
				app.Worker.Anthropic = &config.AnthropicProviderConfig{WorkspaceID: "wrkspc_worker"}
			}
			boot := &bootContext{appCfg: app}
			scheduled, ok := classificationClientFor(boot).(*core.ScheduledLLMCall)
			if !ok {
				t.Fatal("classification client not scheduled")
			}
			// Exercise the resolved boot client, including broker counting;
			// scheduler behavior is independently covered by core tests.
			got, err := scheduled.Client.Complete(context.Background(), "say hello")
			if err != nil || got != "hello" || inferred.Load() != 1 || counted.Load() != 1 {
				t.Fatalf("response=%q inference=%d count=%d err=%v", got, inferred.Load(), counted.Load(), err)
			}
			resolved := boot.providerCfgForClassification
			if resolved.Anthropic == nil || resolved.Anthropic.WorkspaceID != workspace || resolved.BaseURL != srv.URL {
				t.Fatal("shard classification registration would lose resolved settings")
			}
		})
	}
}
