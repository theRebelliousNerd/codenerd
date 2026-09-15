package main

import (
	"reflect"
	"testing"

	"codenerd/internal/campaign"
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
