package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// The audit's witness (ollama-shard-model-override), through the boot's own
// pieces: an Ollama worker slot configured for model A, behind the scheduler
// with the router the boot installs, and a shard profile that names model B
// and no provider. The request body must name B; before the model-only route
// it named A, because Ollama's request builders read the client's own model.
func TestModelOnlyProfile_ReachesTheOllamaWire(t *testing.T) {
	var mu sync.Mutex
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		models = append(models, body.Model)
		mu.Unlock()
		io.WriteString(w, `{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
	}))
	defer srv.Close()

	serving := &config.SecondaryLLMConfig{Provider: "ollama", Model: "model-a", Endpoint: srv.URL}
	cfg := &config.UserConfig{Provider: "ollama", Model: "model-a", Worker: serving}
	worker, err := perception.NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	scheduled := core.NewScheduledLLMCall("model-override-witness", worker)
	scheduled.Router = profileRouter(cfg, serving, nil)

	ctxB := types.WithModelName(context.Background(), "model-b")
	if _, err := scheduled.CompleteWithSystem(ctxB, "sys", "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduled.CompleteWithSystem(context.Background(), "sys", "hi"); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "model-b" || models[1] != "model-a" {
		t.Fatalf("wire models = %v, want [model-b model-a]", models)
	}

	// Preparation names the model the wire does.
	resolved, routed, err := scheduled.ClientFor(ctxB)
	if err != nil || !routed {
		t.Fatalf("ClientFor: routed %v err %v", routed, err)
	}
	var id types.ModelIdentifier
	broker.Walk(resolved, func(layer types.LLMClient) bool {
		if found, ok := layer.(types.ModelIdentifier); ok {
			id = found
			return false
		}
		return true
	})
	if id == nil {
		t.Fatalf("routed client %T reports no identity anywhere in its chain", resolved)
	}
	if provider, model := id.ModelIdentity(); provider != "ollama" || model != "model-b" {
		t.Fatalf("routed identity = (%q, %q), want (ollama, model-b)", provider, model)
	}
}
