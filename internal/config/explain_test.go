package config

import "testing"

func TestExplainConfig_DefaultsAndCheck(t *testing.T) {
	var nilCfg *UserConfig
	if got := nilCfg.GetExplainConfig(); got != DefaultExplainConfig() {
		t.Fatalf("nil config = %+v, want defaults", got)
	}
	partial := ExplainConfig{MaxNodes: 10}.WithDefaults()
	if partial.MaxNodes != 10 || partial.MaxProofs != DefaultExplainConfig().MaxProofs {
		t.Fatalf("WithDefaults = %+v", partial)
	}
	if probs := DefaultExplainConfig().Check("explain"); len(probs) != 0 {
		t.Fatalf("defaults fail their own check: %v", probs)
	}
	if probs := (ExplainConfig{MaxEdges: -1}).Check("explain"); len(probs) != 1 || probs[0].Path != "explain.max_edges" {
		t.Fatalf("negative bound not reported: %v", probs)
	}
	if probs := (ExplainConfig{MaxProofs: 9, MaxNodes: 4}).Check("explain"); len(probs) != 1 {
		t.Fatalf("proofs exceeding nodes not reported: %v", probs)
	}
	cfg := DefaultUserConfig()
	cfg.Explain = &ExplainConfig{MaxBytes: -5}
	found := false
	for _, p := range cfg.Check(nil) {
		if p.Path == "explain.max_bytes" {
			found = true
		}
	}
	if !found {
		t.Fatal("UserConfig.Check does not validate the explain section")
	}
}
