package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ui.northstar_list_limit is the one bound on the north star auto interview's
// lists: it loads from config.json, defaults when absent, and is refused below 1.
func TestUI_NorthstarListLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"ui":{"northstar_list_limit":3}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got := cfg.GetUIConfig().NorthstarListLimit; got != 3 {
		t.Errorf("ui.northstar_list_limit = %d, want 3", got)
	}
	var empty *UserConfig
	if got := empty.GetUIConfig().NorthstarListLimit; got != DefaultUIConfig().NorthstarListLimit {
		t.Errorf("absent ui section: limit %d, want the default", got)
	}
	if got := (UIConfig{SplitPaneRatio: 0.5}).WithDefaults().NorthstarListLimit; got != DefaultUIConfig().NorthstarListLimit {
		t.Errorf("absent key: limit %d, want the default", got)
	}
	if p := (UIConfig{NorthstarListLimit: -2}).Check("ui"); len(p) != 1 || !strings.Contains(p[0].Path, "northstar_list_limit") {
		t.Errorf("a negative limit was not refused: %+v", p)
	}
}
