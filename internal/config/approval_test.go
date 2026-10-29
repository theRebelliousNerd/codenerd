package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApprovalSection_LoadsDefaultsAndResolves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"approval":{"request_ttl":"2m"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	p, err := cfg.GetApprovalConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if p.RequestTTL != 2*time.Minute || p.PreviewMaxBytes != DefaultApprovalConfig().PreviewMaxBytes || p.JournalRetention != 168*time.Hour {
		t.Fatalf("resolved %+v", p)
	}
	var absent *UserConfig
	if got := absent.GetApprovalConfig(); got != DefaultApprovalConfig() {
		t.Fatalf("absent section = %+v", got)
	}
	if hasError(DefaultUserConfig().Check(nil)) {
		t.Fatal("the default config fails its own check")
	}
}

func TestApprovalSection_CheckRefusesContradictions(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  ApprovalConfig
		path string
	}{
		"ttl not a duration":     {ApprovalConfig{RequestTTL: "soon"}, "request_ttl"},
		"ttl not positive":       {ApprovalConfig{RequestTTL: "-1m"}, "request_ttl"},
		"tiny preview":           {ApprovalConfig{PreviewMaxBytes: 10}, "preview_max_bytes"},
		"retention below ttl":    {ApprovalConfig{RequestTTL: "2h", JournalRetention: "1h"}, "journal_retention"},
		"retention not duration": {ApprovalConfig{JournalRetention: "forever"}, "journal_retention"},
	} {
		problems := tc.cfg.Check("approval")
		if len(problems) != 1 || !strings.HasSuffix(problems[0].Path, tc.path) {
			t.Errorf("%s: problems = %+v", name, problems)
		}
		if _, err := tc.cfg.Resolve(); err == nil {
			t.Errorf("%s: Resolve accepted it", name)
		}
	}
}
