package system

import (
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// The worker-tier classification client must carry the worker's own model.
// The classification factory prefers ClassificationModel over Model, so the
// old code — which copied the main-tier classification_model into the worker
// config — sent Meta's model name to OpenRouter and 403'd every
// classification (observed live in dogfood).
func TestClassificationClientFor_WorkerTierUsesWorkerModel(t *testing.T) {
	appCfg := &config.UserConfig{
		Provider:            "meta",
		Model:               "muse-spark-1.3-contributor",
		ClassificationModel: "muse-spark-1.3-contributor",
		MetaAPIKey:          "test-meta-key",
		OpenRouterAPIKey:    "test-or-key",
		Worker:              &config.SecondaryLLMConfig{Provider: "openrouter", Model: "stealth/union-alpha"},
	}
	bctx := &bootContext{
		appCfg: appCfg,
		providerCfgForClassification: &perception.ProviderConfig{
			Provider: perception.ProviderMeta,
			Model:    "muse-spark-1.3-contributor",
			APIKey:   "test-meta-key",
		},
	}
	got := classificationClientFor(bctx)
	sched, ok := got.(*core.ScheduledLLMCall)
	if !ok {
		t.Fatalf("classification client is %T, want *core.ScheduledLLMCall", got)
	}
	getter, ok := sched.Client.(interface{ GetModel() string })
	if !ok {
		t.Fatalf("scheduled inner client is %T, has no GetModel", sched.Client)
	}
	if model := getter.GetModel(); model != "stealth/union-alpha" {
		t.Errorf("worker-tier classification model = %q, want stealth/union-alpha (main-tier leak)", model)
	}
}

// Appendix N3 (Docs/audits/stream-completion-contract.md): the registered
// perception firewall rebuilt a worker-only classifier, so a worker failing at
// call time degraded the firewall's intent and the main-tier secondary armed
// for the direct transducer was never asked. Both now classify through the one
// resolved failover pair.
func TestShardRegistry_FirewallClassifiesThroughTheResolvedFailoverPair(t *testing.T) {
	bctx := &bootContext{
		appCfg: &config.UserConfig{
			Provider:         "meta",
			Model:            "muse-spark-1.3-contributor",
			MetaAPIKey:       "test-meta-key",
			OpenRouterAPIKey: "test-or-key",
			Worker:           &config.SecondaryLLMConfig{Provider: "openrouter", Model: "stealth/union-alpha"},
		},
		providerCfgForClassification: &perception.ProviderConfig{
			Provider: perception.ProviderMeta,
			Model:    "muse-spark-1.3-contributor",
			APIKey:   "test-meta-key",
		},
	}
	bctx.classificationClient = classificationClientFor(bctx)
	got := shardRegistryContext(bctx).ClassificationClient
	if got != bctx.classificationClient {
		t.Fatalf("firewall classifier = %T, want the transducer's resolved client", got)
	}
	sched, ok := got.(*core.ScheduledLLMCall)
	if !ok {
		t.Fatalf("firewall classifier is %T, want one scheduled call", got)
	}
	if _, ok := sched.Client.(*perception.FallbackClient); !ok {
		t.Fatalf("firewall classifier wraps %T, want the worker->main failover pair", sched.Client)
	}
}
