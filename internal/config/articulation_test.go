package config

import (
	"strings"
	"testing"
)

func TestArticulationConfig_DefaultShareIsAQuarterOfThePromptBudget(t *testing.T) {
	def := DefaultArticulationConfig()
	if problems := def.Check("articulation"); len(problems) != 0 {
		t.Fatalf("the default does not check clean: %v", problems)
	}
	if def.SessionContextSharePercent != 25 {
		t.Fatalf("session_context_share_percent = %d, want 25", def.SessionContextSharePercent)
	}
	var nilCfg *UserConfig
	if got := nilCfg.GetArticulationConfig().SessionContextSharePercent; got != 25 {
		t.Fatalf("a nil config's share = %d, want 25", got)
	}
	if got := (ArticulationConfig{}).WithDefaults().SessionContextSharePercent; got != 25 {
		t.Fatalf("an absent share = %d, want the default 25", got)
	}
	// 25% of the default 200000-token prompt budget, at 4 bytes a token.
	budget := DefaultJITConfig().TokenBudget
	chars := budget * def.SessionContextSharePercent * BytesPerToken / 100
	if budget != 200000 || chars != 200000 {
		t.Fatalf("default ceiling would be %d chars from budget %d; want 200000 chars from 200000 tokens", chars, budget)
	}
	if chars <= 32*1024 {
		t.Fatalf("default ceiling %d is not above the old 32 KiB block cap", chars)
	}
}

func TestArticulationConfig_LoadAndCheck(t *testing.T) {
	prev := ResolvedArticulationConfig()
	t.Cleanup(func() { SetArticulationConfig(prev) })

	cfg, err := LoadUserConfig(writeCampaignConfig(t, `{"articulation":{"session_context_share_percent":40}}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.GetArticulationConfig().SessionContextSharePercent; got != 40 {
		t.Fatalf("loaded share = %d, want 40", got)
	}
	if got := ResolvedArticulationConfig().SessionContextSharePercent; got != 40 {
		t.Fatalf("installed share = %d, want 40", got)
	}

	absent, err := LoadUserConfig(writeCampaignConfig(t, `{"articulation":{"session_context_share_percent":0}}`))
	if err != nil {
		t.Fatalf("an explicit 0 was refused: %v", err)
	}
	if got := absent.GetArticulationConfig().SessionContextSharePercent; got != 25 {
		t.Fatalf("explicit 0 took %d, want the default 25", got)
	}

	for _, body := range []string{
		`{"articulation":{"session_context_share_percent":101}}`,
		`{"articulation":{"session_context_share_percent":-1}}`,
	} {
		_, err := LoadUserConfig(writeCampaignConfig(t, body))
		if err == nil || !strings.Contains(err.Error(), "articulation.session_context_share_percent") {
			t.Fatalf("load %s error = %v, want articulation.session_context_share_percent", body, err)
		}
	}

	wired := (&UserConfig{Articulation: &ArticulationConfig{SessionContextSharePercent: 101}}).Check(nil)
	found := false
	for _, p := range wired {
		if p.Severity == SeverityError && p.Path == "articulation.session_context_share_percent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("UserConfig.Check did not surface the share: %+v", wired)
	}
	if p := DefaultUserConfig().Check(nil); hasError(p) {
		t.Fatalf("the default config fails its own check: %+v", p)
	}
}
