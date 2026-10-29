package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/mcp"
)

// The MCP transport fallback is a tunable owned by .nerd/config.json
// (integrations.default_timeout). These pin its default, the Get overlay,
// the Check errors and the install into the MCP package; the call-site
// test (Connect with an unusable timeout) lives in internal/mcp.
func TestDefaultIntegrationsConfig_DefaultTimeout(t *testing.T) {
	if got := DefaultIntegrationsConfig().DefaultTimeout; got != "30s" {
		t.Errorf("DefaultTimeout = %q, want 30s", got)
	}
}

func TestGetIntegrations_DefaultTimeout(t *testing.T) {
	var nilCfg *UserConfig
	if got := nilCfg.GetIntegrations(); got.DefaultTimeout != "30s" {
		t.Errorf("nil config DefaultTimeout = %q, want 30s", got.DefaultTimeout)
	}
	if got := (&UserConfig{}).GetIntegrations(); got.DefaultTimeout != "30s" {
		t.Errorf("absent section DefaultTimeout = %q, want 30s", got.DefaultTimeout)
	}
	cfg := &UserConfig{Integrations: &IntegrationsConfig{DefaultTimeout: "45s"}}
	if got := cfg.GetIntegrations(); got.DefaultTimeout != "45s" {
		t.Errorf("set DefaultTimeout = %q, want 45s", got.DefaultTimeout)
	}
}

func TestIntegrationsConfig_CheckDefaultTimeout(t *testing.T) {
	if p := DefaultIntegrationsConfig().Check("integrations"); len(p) != 0 {
		t.Errorf("defaults fail their own check: %+v", p)
	}
	if p := (IntegrationsConfig{}).Check("integrations"); len(p) != 0 {
		t.Errorf("an absent key fails its own check: %+v", p)
	}
	for _, bad := range []string{"bogus", "-1s", "0s"} {
		c := IntegrationsConfig{DefaultTimeout: bad}
		p := c.Check("integrations")
		if len(p) != 1 || p[0].Severity != SeverityError || p[0].Path != "integrations.default_timeout" {
			t.Errorf("timeout %q: problems = %+v, want one error at integrations.default_timeout", bad, p)
		}
	}
}

func TestIntegrationsConfig_ResolveDefaultTimeout(t *testing.T) {
	d, err := IntegrationsConfig{}.ResolveDefaultTimeout()
	if err != nil || d != 30*time.Second {
		t.Errorf("absent = %v, %v; want 30s, nil", d, err)
	}
	d, err = IntegrationsConfig{DefaultTimeout: "2m"}.ResolveDefaultTimeout()
	if err != nil || d != 2*time.Minute {
		t.Errorf("set = %v, %v; want 2m, nil", d, err)
	}
	if _, err := (IntegrationsConfig{DefaultTimeout: "bogus"}).ResolveDefaultTimeout(); err == nil {
		t.Error("an unparseable timeout resolved without error")
	}
}

// The MCP package's last-resort fallback and this default must stay the
// same value: the leaf cannot import config, so the pin lives here.
func TestIntegrationsDefaultTimeout_MatchesMCPFallback(t *testing.T) {
	t.Cleanup(func() { mcp.SetTransportTimeoutFallback(30 * time.Second) })
	mcp.SetTransportTimeoutFallback(0) // reset to the leaf last resort
	got := mcp.DefaultTransportTimeout()
	def, err := DefaultIntegrationsConfig().ResolveDefaultTimeout()
	if err != nil {
		t.Fatalf("ResolveDefaultTimeout: %v", err)
	}
	if got != def {
		t.Errorf("MCP last resort = %v, config default = %v", got, def)
	}
}

// A set default_timeout reaches the MCP fallback through a real load.
func TestLoadUserConfig_InstallsMCPFallback(t *testing.T) {
	t.Cleanup(func() { mcp.SetTransportTimeoutFallback(30 * time.Second) })
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"integrations":{"default_timeout":"45s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if got := mcp.DefaultTransportTimeout(); got != 45*time.Second {
		t.Errorf("installed MCP fallback = %v, want 45s", got)
	}
}

// A contradictory default_timeout refuses the file with the key named.
func TestLoadUserConfig_RefusesBadIntegrationsTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"integrations":{"default_timeout":"soon"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "integrations.default_timeout") {
		t.Errorf("LoadUserConfig error = %v, want integrations.default_timeout named", err)
	}
}
