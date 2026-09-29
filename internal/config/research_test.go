package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The research bounds are tunables owned by .nerd/config.json. These pin
// their defaults, the Get overlay, the duration parsing and the Check
// errors; the call-site tests live with the tools.
func TestDefaultResearchConfig(t *testing.T) {
	got := DefaultResearchConfig()
	if got.WebFetchTimeout != "60s" || got.Context7Timeout != "30s" ||
		got.WebSearchTimeout != "30s" || got.BrowserExtractTimeout != "10s" {
		t.Errorf("default timeouts = %+v", got)
	}
	if got.BrowserExtractMaxChars != 8000 || got.BrowserExtractMaxCharsCap != 32000 ||
		got.BrowserReasonItems != 20 || got.BrowserReasonCompactItems != 10 {
		t.Errorf("default windows = %+v", got)
	}
}

func TestGetResearchConfig_Overlay(t *testing.T) {
	var nilCfg *UserConfig
	if got := nilCfg.GetResearchConfig(); got != DefaultResearchConfig() {
		t.Errorf("nil config = %+v, want defaults", got)
	}
	if got := (&UserConfig{}).GetResearchConfig(); got != DefaultResearchConfig() {
		t.Errorf("absent section = %+v, want defaults", got)
	}
	cfg := &UserConfig{Research: &ResearchConfig{
		WebFetchTimeout:           "61s",
		Context7Timeout:           "31s",
		WebSearchTimeout:          "32s",
		BrowserExtractTimeout:     "11s",
		BrowserExtractMaxChars:    111,
		BrowserExtractMaxCharsCap: 222,
		BrowserReasonItems:        33,
		BrowserReasonCompactItems: 44,
	}}
	got := cfg.GetResearchConfig()
	if got.WebFetchTimeout != "61s" || got.Context7Timeout != "31s" ||
		got.WebSearchTimeout != "32s" || got.BrowserExtractTimeout != "11s" ||
		got.BrowserExtractMaxChars != 111 || got.BrowserExtractMaxCharsCap != 222 ||
		got.BrowserReasonItems != 33 || got.BrowserReasonCompactItems != 44 {
		t.Errorf("set section = %+v, want the set values", got)
	}
}

func TestResearchConfig_Resolve(t *testing.T) {
	p, err := ResearchConfig{WebFetchTimeout: "61s"}.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.WebFetchTimeout != 61*time.Second {
		t.Errorf("WebFetchTimeout = %v, want 61s", p.WebFetchTimeout)
	}
	if p.Context7Timeout != 30*time.Second || p.BrowserExtractTimeout != 10*time.Second {
		t.Errorf("absent durations did not take defaults: %+v", p)
	}
	if p.BrowserExtractMaxChars != 8000 || p.BrowserReasonCompactItems != 10 {
		t.Errorf("absent windows did not take defaults: %+v", p)
	}
	if _, err := (ResearchConfig{WebFetchTimeout: "bogus"}).Resolve(); err == nil {
		t.Error("Resolve accepted an unparseable duration")
	}
}

func TestResearchConfig_Check(t *testing.T) {
	if p := DefaultResearchConfig().Check("research"); len(p) != 0 {
		t.Errorf("defaults fail their own check: %+v", p)
	}
	cases := []struct {
		name string
		set  func(*ResearchConfig)
		path string
	}{
		{"bad fetch duration", func(c *ResearchConfig) { c.WebFetchTimeout = "bogus" }, "research.web_fetch_timeout"},
		{"negative fetch", func(c *ResearchConfig) { c.WebFetchTimeout = "-1s" }, "research.web_fetch_timeout"},
		{"zero search", func(c *ResearchConfig) { c.WebSearchTimeout = "0s" }, "research.web_search_timeout"},
		{"bad context7", func(c *ResearchConfig) { c.Context7Timeout = "a minute" }, "research.context7_timeout"},
		{"bad extract timeout", func(c *ResearchConfig) { c.BrowserExtractTimeout = "soon" }, "research.browser_extract_timeout"},
		{"negative window", func(c *ResearchConfig) { c.BrowserExtractMaxChars = -5 }, "research.browser_extract_max_chars"},
		{"negative cap", func(c *ResearchConfig) { c.BrowserExtractMaxCharsCap = -5 }, "research.browser_extract_max_chars_cap"},
		{"negative reason", func(c *ResearchConfig) { c.BrowserReasonItems = -5 }, "research.browser_reason_items"},
		{"negative compact", func(c *ResearchConfig) { c.BrowserReasonCompactItems = -5 }, "research.browser_reason_compact_items"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultResearchConfig()
			tc.set(&c)
			problems := c.Check("research")
			if len(problems) != 1 || problems[0].Severity != SeverityError || problems[0].Path != tc.path {
				t.Errorf("problems = %+v, want one error at %s", problems, tc.path)
			}
		})
	}
	cap := DefaultResearchConfig()
	cap.BrowserExtractMaxChars, cap.BrowserExtractMaxCharsCap = 9000, 8000
	if p := cap.Check("research"); len(p) != 1 || p[0].Path != "research.browser_extract_max_chars_cap" {
		t.Errorf("a cap below its default was not refused: %+v", p)
	}
}

// A set research section reaches the installed policy through a real load;
// a missing file installs the defaults.
func TestLoadUserConfig_InstallsResearchPolicy(t *testing.T) {
	t.Cleanup(func() { SetResearchPolicy(mustResolveResearchDefaults()) })
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{"research":{"web_fetch_timeout":"61s","browser_extract_max_chars":111,` +
		`"browser_extract_max_chars_cap":222,"browser_reason_items":33,"browser_reason_compact_items":44}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := ResolvedResearchPolicy()
	if got.WebFetchTimeout != 61*time.Second || got.BrowserExtractMaxChars != 111 ||
		got.BrowserExtractMaxCharsCap != 222 || got.BrowserReasonItems != 33 ||
		got.BrowserReasonCompactItems != 44 {
		t.Errorf("installed policy = %+v, want the set values", got)
	}
	if _, err := LoadUserConfig(filepath.Join(dir, "missing.json")); err != nil {
		t.Fatalf("LoadUserConfig (missing): %v", err)
	}
	if got := ResolvedResearchPolicy(); got != mustResolveResearchDefaults() {
		t.Errorf("missing file installed %+v, want defaults", got)
	}
}

// A contradictory research section refuses the file with the key named.
func TestLoadUserConfig_RefusesBadResearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"research":{"web_fetch_timeout":"bogus"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "research.web_fetch_timeout") {
		t.Errorf("LoadUserConfig error = %v, want research.web_fetch_timeout named", err)
	}
}
