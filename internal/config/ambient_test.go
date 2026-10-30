package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAmbientConfig_DefaultsAndCheck(t *testing.T) {
	var nilCfg *UserConfig
	if got := nilCfg.GetAmbientConfig(); got != DefaultAmbientConfig() || got.MaxAge() <= 0 {
		t.Fatalf("nil config = %+v, want the positive default", got)
	}
	var cfg UserConfig
	if err := json.Unmarshal([]byte(`{"ambient":{"max_age_seconds":60}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetAmbientConfig().MaxAge(); got != time.Minute {
		t.Fatalf("configured max age = %v, want 1m", got)
	}
	cfg.Ambient = &AmbientConfig{MaxAgeSeconds: -1}
	found := false
	for _, p := range cfg.Check(nil) {
		if p.Path == "ambient.max_age_seconds" {
			found = true
		}
	}
	if !found {
		t.Fatal("a negative max age is not refused by the config check")
	}
}
