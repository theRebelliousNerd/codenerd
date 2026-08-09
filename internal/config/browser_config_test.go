package config

import "testing"

func TestGetBrowserConfigDefaultsAndExplicitIsolation(t *testing.T) {
	defaults := (&UserConfig{}).GetBrowserConfig()
	if defaults.MultiTabDefault == nil || !*defaults.MultiTabDefault {
		t.Fatal("browser tabs should share a profile by default")
	}
	if defaults.MaxTabs != 32 || defaults.MaxBrowsers != 4 {
		t.Fatalf("unexpected browser limits: tabs=%d browsers=%d", defaults.MaxTabs, defaults.MaxBrowsers)
	}
	if defaults.EvidenceEnabled == nil || !*defaults.EvidenceEnabled || defaults.MaxEvidenceFiles != 16 || defaults.MaxEvidenceFileBytes != 4<<20 {
		t.Fatalf("unexpected browser evidence defaults: %+v", defaults)
	}

	shared := false
	cfg := (&UserConfig{Browser: &BrowserAutomationConfig{
		MultiTabDefault:      &shared,
		MaxTabs:              7,
		MaxBrowsers:          2,
		IdleTabTimeoutMs:     5000,
		MaxEvidenceFiles:     3,
		MaxEvidenceFileBytes: 1024,
	}}).GetBrowserConfig()
	if cfg.MultiTabDefault == nil || *cfg.MultiTabDefault {
		t.Fatal("explicit multi_tab_default=false was not preserved")
	}
	if cfg.MaxTabs != 7 || cfg.MaxBrowsers != 2 || cfg.IdleTabTimeoutMs != 5000 {
		t.Fatalf("explicit browser limits were not preserved: %+v", cfg)
	}
	if cfg.ViewportWidth != 1920 || cfg.NavigationTimeoutMs != 30000 {
		t.Fatalf("browser zero values were not defaulted: %+v", cfg)
	}
	if cfg.MaxEvidenceFiles != 3 || cfg.MaxEvidenceFileBytes != 1024 {
		t.Fatalf("explicit evidence limits were not preserved: %+v", cfg)
	}
}

func TestGetBrowserConfigNormalizesInvalidLimits(t *testing.T) {
	cfg := (&UserConfig{Browser: &BrowserAutomationConfig{
		ViewportWidth:        -1,
		ViewportHeight:       -1,
		NavigationTimeoutMs:  -1,
		MaxTabs:              -1,
		MaxBrowsers:          -1,
		IdleTabTimeoutMs:     -1,
		MaxEvidenceFiles:     -1,
		MaxEvidenceFileBytes: -1,
	}}).GetBrowserConfig()
	if cfg.ViewportWidth != 1920 || cfg.ViewportHeight != 1080 || cfg.NavigationTimeoutMs != 30000 {
		t.Fatalf("invalid browser dimensions/timeouts were not normalized: %+v", cfg)
	}
	if cfg.MaxTabs != 32 || cfg.MaxBrowsers != 4 || cfg.IdleTabTimeoutMs != 0 {
		t.Fatalf("invalid browser limits were not normalized: %+v", cfg)
	}
	if cfg.MaxEvidenceFiles != 16 || cfg.MaxEvidenceFileBytes != 4<<20 {
		t.Fatalf("invalid evidence limits were not normalized: %+v", cfg)
	}

	capped := (&UserConfig{Browser: &BrowserAutomationConfig{
		MaxEvidenceFiles: 1000, MaxEvidenceFileBytes: 1 << 30,
	}}).GetBrowserConfig()
	if capped.MaxEvidenceFiles != 256 || capped.MaxEvidenceFileBytes != 64<<20 {
		t.Fatalf("evidence hard caps were not enforced: %+v", capped)
	}
}
