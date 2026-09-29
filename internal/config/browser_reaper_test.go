package config

import "testing"

func TestBrowserReaperConfigDefaultsAndCheck(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config BrowserReaperConfig
		bad    bool
	}{
		{"defaults", BrowserReaperConfig{}, false},
		{"explicit", BrowserReaperConfig{TimeoutMs: 1000, PollIntervalMs: 10}, false},
		{"negative timeout", BrowserReaperConfig{TimeoutMs: -1}, true},
		{"negative poll", BrowserReaperConfig{PollIntervalMs: -1}, true},
		{"poll beyond timeout", BrowserReaperConfig{TimeoutMs: 10, PollIntervalMs: 20}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.config.Check("browser.reaper"); (len(got) > 0) != tc.bad {
				t.Fatalf("Check = %v", got)
			}
		})
	}
	cfg := (&UserConfig{}).GetBrowserConfig()
	if cfg.Reaper != DefaultBrowserReaperConfig() {
		t.Fatalf("defaults lost: %+v", cfg.Reaper)
	}
	cfg = (&UserConfig{Browser: &BrowserAutomationConfig{Reaper: BrowserReaperConfig{TimeoutMs: 900, PollIntervalMs: 9}}}).GetBrowserConfig()
	if cfg.Reaper.TimeoutMs != 900 || cfg.Reaper.PollIntervalMs != 9 {
		t.Fatalf("explicit settings lost: %+v", cfg.Reaper)
	}
	user := &UserConfig{Browser: &BrowserAutomationConfig{Reaper: BrowserReaperConfig{TimeoutMs: -1}}}
	found := false
	for _, problem := range user.Check(nil) {
		if problem.Path == "browser.reaper.timeout_ms" {
			found = true
		}
	}
	if !found {
		t.Fatal("UserConfig.Check did not check browser.reaper")
	}
}
