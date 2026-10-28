package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A config that sets only some core limits must survive a save and a load.
// The unset ones loaded as zero, Save wrote the zeros out as explicit keys,
// and the next load refused them (max_total_memory_mb must be >= 512): one
// Shift+Tab or /yolo, which save the config, and codeNERD would not start.
func TestCoreLimits_PartialSectionSurvivesSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"core_limits": {"max_concurrent_shards": 4, "max_concurrent_api_calls": 12}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		data, _ := os.ReadFile(path)
		t.Fatalf("a saved config no longer loads: %v\n%s", err, data)
	}
}
