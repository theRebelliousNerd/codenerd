package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

func TestCredentialPools_AreWiredThroughAllBootSlots(t *testing.T) {
	t.Setenv("BOOT_POOL_KEY_A", "boot-pool-secret-a")
	t.Setenv("BOOT_POOL_KEY_B", "boot-pool-secret-b")
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/messages/count_tokens" {
			io.WriteString(w, `{"input_tokens":3}`)
			return
		}
		mu.Lock()
		seen = append(seen, r.Header.Get("x-api-key"))
		mu.Unlock()
		io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":1}}`)
	}))
	defer srv.Close()
	cfg := &config.UserConfig{Provider: "anthropic", Model: "claude-opus-5-5", BaseURL: srv.URL,
		APIKeyPools: map[string][]config.ProviderAPIKey{"anthropic": {{Name: "alpha", Env: "BOOT_POOL_KEY_A"}, {Name: "beta", Env: "BOOT_POOL_KEY_B"}}},
		Worker:      &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5"}, Planner: &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5"}}
	pc, err := perception.ProviderConfigFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	main, err := perception.NewClientFromConfig(pc)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := perception.NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	planner, err := perception.NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	boot := &bootContext{appCfg: cfg}
	classifier, ok := classificationClientFor(boot).(*core.ScheduledLLMCall)
	if !ok {
		t.Fatal("classification was not scheduled")
	}
	for _, client := range []perception.LLMClient{main, worker, planner, classifier.Client} {
		if _, err := client.Complete(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for i, key := range seen {
		expected := "boot-pool-secret-a"
		if i%2 == 1 {
			expected = "boot-pool-secret-b"
		}
		if key != expected {
			t.Fatalf("boot slots did not share pool selection: %v", seen)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("calls=%d", len(seen))
	}
	boot.classificationClient = classifier
	if shardRegistryContext(boot).ClassificationClient != perception.LLMClient(classifier) {
		t.Fatal("shard classification registration does not use the pooled classifier")
	}
}

func TestCredentialPoolEnvironmentRotationChangesCortexIdentity(t *testing.T) {
	t.Setenv("IDENTITY_POOL_KEY", "identity-old-secret")
	cfg := &config.UserConfig{Provider: "anthropic", Model: "claude-opus-5-5", APIKeyPools: map[string][]config.ProviderAPIKey{"anthropic": {{Name: "primary", Env: "IDENTITY_POOL_KEY"}}}}
	first, err := newCortexIdentity(t.TempDir(), cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("IDENTITY_POOL_KEY", "identity-new-secret")
	second, err := newCortexIdentity(first.Workspace, cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigDigest == second.ConfigDigest {
		t.Fatal("environment key rotation would reuse stale clients")
	}
	encoded, _ := json.Marshal(second)
	for _, secret := range []string{"identity-old-secret", "identity-new-secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("Cortex identity contains raw credentials")
		}
	}
}
