package main

import (
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

// Cobra/chat assault parity, enforced rather than documented.
//
// Assault was chat-only, so the one campaign type meant to run unattended for
// hours could not be scripted or scheduled. Adding the command is half the fix;
// the other half is making sure it does not fall behind AssaultConfig the next
// time a knob is added. Every field of AssaultConfig must be reachable from a
// flag, so a new field fails this test until an operator can set it.
func TestCampaignAssaultFlags_CoverEveryAssaultConfigField(t *testing.T) {
	// field name -> flag that sets it
	covered := map[string]string{
		"Scope":                 "scope",
		"Include":               "include",
		"Exclude":               "exclude",
		"BatchSize":             "batch-size",
		"Cycles":                "cycles",
		"DefaultTimeoutSeconds": "timeout",
		"Stages":                "stages",
		"LogMaxBytes":           "log-max-bytes",
		"EnableNemesis":         "no-nemesis",
		"MaxRemediationTasks":   "max-remediation",
		"ContextBudget":         "context-budget",
	}

	cfgType := reflect.TypeOf(campaign.AssaultConfig{})
	for i := range cfgType.NumField() {
		field := cfgType.Field(i)
		flagName, ok := covered[field.Name]
		if !ok {
			t.Errorf("AssaultConfig.%s has no `nerd campaign assault` flag. "+
				"Chat can set it and the CLI cannot, which is the asymmetry this command exists to remove.",
				field.Name)
			continue
		}
		if campaignAssaultCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("AssaultConfig.%s claims flag --%s, which is not registered", field.Name, flagName)
		}
	}

	for name := range covered {
		if _, found := cfgType.FieldByName(name); !found {
			t.Errorf("flag map names AssaultConfig.%s, which no longer exists", name)
		}
	}
}

func resetAssaultFlags() {
	assaultScope = ""
	assaultInclude = nil
	assaultExclude = nil
	assaultBatchSize = 0
	assaultCycles = 0
	assaultTimeoutSecs = 0
	assaultStages = nil
	assaultCommand = ""
	assaultNoNemesis = false
	assaultMaxRemedy = 0
	assaultContextBudget = 0
	assaultLogMaxBytes = 0
	assaultDryRun = false
}

func TestAssaultConfigFromFlags_ShouldApplyEveryKnob(t *testing.T) {
	resetAssaultFlags()
	t.Cleanup(resetAssaultFlags)

	assaultScope = "package"
	assaultInclude = []string{"internal/core"}
	assaultExclude = []string{"internal/core/defaults"}
	assaultBatchSize = 4
	assaultCycles = 3
	assaultTimeoutSecs = 120
	assaultStages = []string{"go_test", "go_vet"}
	assaultMaxRemedy = 7
	assaultContextBudget = 50000
	assaultLogMaxBytes = 1024

	cfg, err := assaultConfigFromFlags(nil)
	if err != nil {
		t.Fatalf("assaultConfigFromFlags: %v", err)
	}

	if cfg.Scope != campaign.AssaultScopePackage {
		t.Errorf("Scope = %s, want %s", cfg.Scope, campaign.AssaultScopePackage)
	}
	if cfg.BatchSize != 4 || cfg.Cycles != 3 || cfg.DefaultTimeoutSeconds != 120 {
		t.Errorf("batch/cycles/timeout = %d/%d/%d, want 4/3/120", cfg.BatchSize, cfg.Cycles, cfg.DefaultTimeoutSeconds)
	}
	if cfg.MaxRemediationTasks != 7 || cfg.ContextBudget != 50000 || cfg.LogMaxBytes != 1024 {
		t.Errorf("remediation/context/log = %d/%d/%d", cfg.MaxRemediationTasks, cfg.ContextBudget, cfg.LogMaxBytes)
	}
	if len(cfg.Stages) != 2 {
		t.Fatalf("stages = %+v, want 2", cfg.Stages)
	}
	if cfg.Stages[0].Kind != campaign.AssaultStageGoTest || cfg.Stages[1].Kind != campaign.AssaultStageGoVet {
		t.Errorf("stage kinds = %s,%s", cfg.Stages[0].Kind, cfg.Stages[1].Kind)
	}
	// Normalize must have propagated the per-stage timeout.
	for _, s := range cfg.Stages {
		if s.TimeoutSeconds != 120 {
			t.Errorf("stage %s timeout = %d, want 120", s.Kind, s.TimeoutSeconds)
		}
	}
	if len(cfg.Include) != 1 || len(cfg.Exclude) != 1 {
		t.Errorf("include/exclude not carried: %v / %v", cfg.Include, cfg.Exclude)
	}
}

func TestAssaultConfigFromFlags_PositionalScope_ShouldWork(t *testing.T) {
	resetAssaultFlags()
	t.Cleanup(resetAssaultFlags)

	cfg, err := assaultConfigFromFlags([]string{"repo"})
	if err != nil {
		t.Fatalf("assaultConfigFromFlags: %v", err)
	}
	if cfg.Scope != campaign.AssaultScopeRepo {
		t.Fatalf("positional scope ignored: got %s", cfg.Scope)
	}
}

func TestAssaultConfigFromFlags_WhenScopeUnknown_ShouldError(t *testing.T) {
	resetAssaultFlags()
	t.Cleanup(resetAssaultFlags)

	assaultScope = "galaxy"
	if _, err := assaultConfigFromFlags(nil); err == nil {
		t.Fatal("an unknown scope must be rejected, not silently defaulted")
	}
}

// The `command` stage is useless without a template, and defaulting it would
// run an empty command against every target in the repository.
func TestAssaultConfigFromFlags_WhenCommandStageHasNoTemplate_ShouldError(t *testing.T) {
	resetAssaultFlags()
	t.Cleanup(resetAssaultFlags)

	assaultStages = []string{"command"}
	_, err := assaultConfigFromFlags(nil)
	if err == nil {
		t.Fatal("the command stage must require --command")
	}
	if !strings.Contains(err.Error(), "--command") {
		t.Errorf("the error should name the missing flag; got %v", err)
	}
}

func TestAssaultConfigFromFlags_WhenNoNemesisRemovesEveryStage_ShouldError(t *testing.T) {
	resetAssaultFlags()
	t.Cleanup(resetAssaultFlags)

	assaultStages = []string{"nemesis_review"}
	assaultNoNemesis = true
	if _, err := assaultConfigFromFlags(nil); err == nil {
		t.Fatal("a configuration with zero stages must be rejected rather than running a no-op assault")
	}
}

func TestCampaignStartOverride_ShouldBeConsumedOnce(t *testing.T) {
	camp := campaign.NewAdversarialAssaultCampaign(t.TempDir(), campaign.DefaultAssaultConfig())
	setCampaignStartOverride(camp)
	t.Cleanup(func() { setCampaignStartOverride(nil) })

	if got := takeCampaignStartOverride(); got != camp {
		t.Fatal("override was not returned")
	}
	if got := takeCampaignStartOverride(); got != nil {
		t.Fatal("override survived a take; a later unrelated `campaign start` would silently run the assault plan")
	}
}
