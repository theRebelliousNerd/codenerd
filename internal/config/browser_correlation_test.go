package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserAutomationConfig_WhenCorrelationContainersSet_ShouldSurviveGetBrowserConfig(t *testing.T) {
	containers := []string{"app", "db"}
	cfg := &UserConfig{
		Browser: &BrowserAutomationConfig{
			CorrelationContainers: append([]string(nil), containers...),
		},
	}
	got := cfg.GetBrowserConfig()
	if len(got.CorrelationContainers) != 2 {
		t.Fatalf("CorrelationContainers length=%d, want 2", len(got.CorrelationContainers))
	}
	if got.CorrelationContainers[0] != "app" || got.CorrelationContainers[1] != "db" {
		t.Fatalf("CorrelationContainers=%v, want [app db]", got.CorrelationContainers)
	}
}

func TestBrowserAutomationConfig_WhenAbsent_ShouldDefaultToDisabled(t *testing.T) {
	def := DefaultBrowserAutomationConfig()
	if len(def.CorrelationContainers) != 0 {
		t.Fatalf("DefaultBrowserAutomationConfig CorrelationContainers=%v, want empty (disabled)", def.CorrelationContainers)
	}
	empty := (&UserConfig{}).GetBrowserConfig()
	if len(empty.CorrelationContainers) != 0 {
		t.Fatalf("GetBrowserConfig with empty UserConfig CorrelationContainers=%v, want empty", empty.CorrelationContainers)
	}
	nilCfg := (*UserConfig)(nil).GetBrowserConfig()
	if len(nilCfg.CorrelationContainers) != 0 {
		t.Fatalf("nil UserConfig GetBrowserConfig CorrelationContainers=%v, want empty", nilCfg.CorrelationContainers)
	}
}

func TestUserConfig_ShouldAcceptCorrelationContainersKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"browser": {"correlation_containers": ["app"]}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig should accept correlation_containers key, got error: %v", err)
	}
	if cfg.Browser == nil {
		t.Fatal("Browser config is nil after decode")
	}
	if len(cfg.Browser.CorrelationContainers) != 1 || cfg.Browser.CorrelationContainers[0] != "app" {
		t.Fatalf("Browser.CorrelationContainers=%v, want [app]", cfg.Browser.CorrelationContainers)
	}
	// Also verify GetBrowserConfig carries it through (getter-drops-field guard)
	got := cfg.GetBrowserConfig()
	if len(got.CorrelationContainers) != 1 || got.CorrelationContainers[0] != "app" {
		t.Fatalf("GetBrowserConfig CorrelationContainers=%v, want [app]", got.CorrelationContainers)
	}
}
