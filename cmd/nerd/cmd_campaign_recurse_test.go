package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/northstar"
)

func TestCampaignRecurseFlags_CoverEveryRecurseConfigField(t *testing.T) {
	covered := map[string]string{
		"MaxWaves":       "waves",
		"Angles":         "angles",
		"Subsystems":     "subsystem",
		"StallWaveLimit": "stall-waves",
		"ContextBudget":  "context-budget",
	}
	cfgType := reflect.TypeOf(campaign.RecurseConfig{})
	for i := range cfgType.NumField() {
		field := cfgType.Field(i)
		flagName, ok := covered[field.Name]
		if !ok {
			t.Errorf("RecurseConfig.%s has no `nerd campaign recurse` flag", field.Name)
			continue
		}
		if campaignRecurseCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("RecurseConfig.%s claims flag --%s, which is not registered", field.Name, flagName)
		}
	}
}

func TestResolveRecurseConfig_MapsFlags(t *testing.T) {
	cfg, err := resolveRecurseConfig(3, []string{"harden", "secure"}, []string{"session"}, 5, 1000)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.MaxWaves != 3 || cfg.StallWaveLimit != 5 || cfg.ContextBudget != 1000 {
		t.Fatalf("scalars wrong: %+v", cfg)
	}
	if len(cfg.Angles) != 2 || cfg.Angles[0] != campaign.RecurseAngleHarden || cfg.Angles[1] != campaign.RecurseAngleSecure {
		t.Fatalf("angles wrong: %+v", cfg.Angles)
	}
	if len(cfg.Subsystems) != 1 || cfg.Subsystems[0] != "session" {
		t.Fatalf("subsystems wrong: %+v", cfg.Subsystems)
	}
}

func TestResolveRecurseConfig_RejectsBadInput(t *testing.T) {
	if _, err := resolveRecurseConfig(1, []string{"vibes"}, nil, 0, 0); err == nil {
		t.Error("bad angle must fail")
	}
	if _, err := resolveRecurseConfig(1, []string{"harden", "wire", "review"}, nil, 0, 0); err == nil {
		t.Error("three angles must fail")
	}
	if _, err := resolveRecurseConfig(-1, nil, nil, 0, 0); err == nil {
		t.Error("negative waves must fail")
	}
}

func TestCheckRecurseYolo(t *testing.T) {
	unbounded := campaign.RecurseConfig{MaxWaves: 0}
	if err := checkRecurseYolo(unbounded, false); err == nil {
		t.Fatal("unbounded without yolo must be refused")
	}
	if err := checkRecurseYolo(unbounded, true); err != nil {
		t.Fatalf("unbounded with yolo must pass: %v", err)
	}
	bounded := campaign.RecurseConfig{MaxWaves: 2}
	if err := checkRecurseYolo(bounded, false); err != nil {
		t.Fatalf("bounded without yolo must pass: %v", err)
	}
}

func TestRecurseWaveConfig_FreshObserverPerLaterWave(t *testing.T) {
	sentinel := northstar.NewCampaignObserver(nil)
	base := campaign.OrchestratorConfig{Workspace: "w", NorthstarObserver: sentinel}
	calls := 0
	newObserver := func() *northstar.CampaignObserver {
		calls++
		return northstar.NewCampaignObserver(nil)
	}

	got0 := recurseWaveConfig(base, 0, newObserver)
	if got0.NorthstarObserver != sentinel {
		t.Fatalf("wave 0 must reuse base observer")
	}
	if calls != 0 {
		t.Fatalf("wave 0 must not create an observer: calls=%d", calls)
	}

	got1 := recurseWaveConfig(base, 1, newObserver)
	if got1.NorthstarObserver == nil {
		t.Fatal("wave 1 observer must be non-nil")
	}
	if got1.NorthstarObserver == sentinel {
		t.Fatal("wave 1 must not reuse the sentinel observer")
	}
	if got1.Workspace != "w" {
		t.Fatalf("wave 1 must keep base fields: Workspace=%q", got1.Workspace)
	}

	got2 := recurseWaveConfig(base, 2, newObserver)
	if got2.NorthstarObserver == nil {
		t.Fatal("wave 2 observer must be non-nil")
	}
	if got2.NorthstarObserver == sentinel {
		t.Fatal("wave 2 must not reuse the sentinel observer")
	}
	if got2.NorthstarObserver == got1.NorthstarObserver {
		t.Fatal("wave 1 and wave 2 observers must differ")
	}
	if calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
	if base.NorthstarObserver != sentinel {
		t.Fatal("base must not be mutated")
	}
}

func TestRecurseWaveConfig_ObserverRefsAreIndependent(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	a := northstar.BuildCampaignObserver(ws, nil, nil)
	b := northstar.BuildCampaignObserver(ws, nil, nil)
	if a == nil || b == nil {
		t.Skip("BuildCampaignObserver returned nil in this environment")
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 2 {
		t.Fatalf("refcount after two builds = %d, want 2", got)
	}
	a.Close()
	if got := northstar.GuardianRefCount(nerdDir); got != 1 {
		t.Fatalf("refcount after first Close = %d, want 1", got)
	}
	b.Close()
	if got := northstar.GuardianRefCount(nerdDir); got != 0 {
		t.Fatalf("refcount after second Close = %d, want 0", got)
	}
}
