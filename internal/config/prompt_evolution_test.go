package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptEvolutionSection_LoadsDefaultsAndChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"prompt_evolution":{"max_cycle_atoms":5,"trial_ttl_hours":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := cfg.GetPromptEvolutionConfig()
	if got.MaxCycleAtoms != 5 || got.TrialTTLHours != 1 {
		t.Errorf("explicit values lost: %+v", got)
	}
	if got.RecordQueueCapacity != DefaultPromptEvolutionConfig().RecordQueueCapacity {
		t.Errorf("absent field not defaulted: %+v", got)
	}

	var empty *UserConfig
	if empty.GetPromptEvolutionConfig() != DefaultPromptEvolutionConfig() {
		t.Error("nil config is not the defaults")
	}

	bad := PromptEvolutionConfig{MaxCycleJudgeCalls: -1, ClaimLeaseSeconds: -5}
	problems := bad.Check("prompt_evolution")
	if len(problems) != 2 || !strings.Contains(problems[0].Path, "max_cycle_judge_calls") {
		t.Errorf("negative bounds not refused: %+v", problems)
	}
	if p := DefaultPromptEvolutionConfig().Check("prompt_evolution"); len(p) != 0 {
		t.Errorf("defaults fail their own check: %+v", p)
	}

	user := DefaultUserConfig()
	user.PromptEvolution = &bad
	if !hasError(user.Check(nil)) {
		t.Error("the config check does not include the prompt_evolution section")
	}
}
