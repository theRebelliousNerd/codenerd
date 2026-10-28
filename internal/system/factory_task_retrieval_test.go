package system

import (
	"testing"

	"codenerd/internal/config"
)

// Every task pass gets Jev's relevance judge when the typesafe section turns
// it on, with the thresholds its brief rule reads; otherwise the pass is
// keyword-only, as is a section that cannot resolve.
func TestTaskRetrieverConfig_CarriesJevRelevanceWhenConfigured(t *testing.T) {
	hasJevParam := func(cfg []config.Param) bool {
		for _, p := range cfg {
			if p.Key == "/jev_yes_percent" {
				return true
			}
		}
		return false
	}

	off := &config.UserConfig{}
	if cfg := taskRetrieverConfig(off, "/ws"); cfg.Relevance != nil || hasJevParam(cfg.Params) {
		t.Fatalf("no typesafe section, yet the pass judges relevance: %+v", cfg)
	}

	on := &config.UserConfig{TypeSafe: &config.TypeSafeConfig{Model: "jev-test", BaseURL: "https://jev.invalid", APIKey: "k", JudgmentNoPercent: 25}}
	cfg := taskRetrieverConfig(on, "/ws")
	if cfg.Relevance == nil || cfg.RelevanceDrop != 0.25 || cfg.RelevancePoolFactor == 0 || !hasJevParam(cfg.Params) {
		t.Fatalf("configured Jev did not reach the task pass: %+v", cfg)
	}

	disabled := false
	on.TypeSafe.RetrievalRelevance = &disabled
	if cfg := taskRetrieverConfig(on, "/ws"); cfg.Relevance != nil {
		t.Fatal("retrieval_relevance false still judges")
	}

	broken := &config.UserConfig{TypeSafe: &config.TypeSafeConfig{Model: "jev-test"}}
	if cfg := taskRetrieverConfig(broken, "/ws"); cfg.Relevance != nil {
		t.Fatal("an unresolvable section judges")
	}
}
