package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The usage section loads from config.json, and a price that cannot be
// applied refuses the file instead of metering at a nonsense rate.
func TestUsageSection_ShouldLoadAndRefuseNegativePrices(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"usage":{"prices":{"acme-1":{"input_per_mtok":1.5,"output_per_mtok":6}},"event_log":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(good)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	uc := cfg.GetUsageConfig()
	if !uc.EventLog || uc.Prices["acme-1"] != (UsagePrice{InputPerMTok: 1.5, OutputPerMTok: 6}) {
		t.Fatalf("usage section = %+v", uc)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"usage":{"prices":{"acme-1":{"input_per_mtok":-1,"output_per_mtok":6}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadUserConfig(bad)
	var cerr *ConfigError
	if !errors.As(err, &cerr) || !strings.Contains(err.Error(), "usage.prices.acme-1") {
		t.Fatalf("a negative price was not refused by path: %v", err)
	}

	if p := (UsageConfig{Prices: map[string]UsagePrice{" ": {}}}).Check("usage"); len(p) != 1 {
		t.Errorf("an empty model prefix was not refused: %+v", p)
	}
	var none *UserConfig
	if uc := none.GetUsageConfig(); uc.EventLog || len(uc.Prices) != 0 {
		t.Errorf("absent section: %+v, want built-in prices and no event log", uc)
	}
}
