package config

import (
	"strings"
	"testing"
	"time"
)

func TestCampaignIntelligence_CheckNamesANegativeCountAndANonPositiveDuration(t *testing.T) {
	bad := CampaignConfig{Intelligence: &CampaignIntelligenceConfig{
		MaxChurnHotspots: -3,
		PerSystemTimeout: "0s",
		ConsultTimeout:   "-1s",
	}}
	problems := bad.Check("campaign")
	var paths []string
	for _, p := range problems {
		if p.Severity != SeverityError {
			t.Errorf("%s is %s, want an error", p.Path, p.Severity)
		}
		paths = append(paths, p.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{
		"campaign.intelligence.max_churn_hotspots",
		"campaign.intelligence.per_system_timeout",
		"campaign.intelligence.consult_timeout",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, problems)
		}
	}
	if _, err := bad.Resolve(); err == nil {
		t.Fatal("a section with a negative count and a non-positive duration resolved")
	}

	path := writeCampaignConfig(t, `{"campaign":{"intelligence":{"max_churn_hotspots":-3,"per_system_timeout":"0s","consult_timeout":"-1s"}}}`)
	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("a file with campaign.intelligence.max_churn_hotspots = -3 loaded")
	}
	for _, want := range []string{
		"campaign.intelligence.max_churn_hotspots",
		"campaign.intelligence.per_system_timeout",
		"campaign.intelligence.consult_timeout",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("load error did not name %s: %v", want, err)
		}
	}
}

func TestCampaignIntelligence_AbsentBlockIsTheDefaults(t *testing.T) {
	def, err := DefaultCampaignConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{}`,
		`{"campaign":{}}`,
		`{"campaign":{"intelligence":{}}}`,
		`{"campaign":{"max_task_attempts":4}}`,
	} {
		path := writeCampaignConfig(t, body)
		cfg, err := LoadUserConfig(path)
		if err != nil {
			t.Fatalf("load %s: %v", body, err)
		}
		policy, err := cfg.GetCampaignConfig().Resolve()
		if err != nil {
			t.Fatalf("resolve %s: %v", body, err)
		}
		if policy.Intelligence != def.Intelligence {
			t.Fatalf("absent intelligence block (%s) = %+v, want the defaults %+v", body, policy.Intelligence, def.Intelligence)
		}
	}
}

func TestCampaignIntelligence_AFalseFlagStaysFalse(t *testing.T) {
	path := writeCampaignConfig(t, `{"campaign":{"intelligence":{"max_churn_hotspots":17,"per_system_timeout":"45s","enable_world_model":false}}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := cfg.GetCampaignConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	got := policy.Intelligence
	if got.MaxChurnHotspots != 17 {
		t.Errorf("max_churn_hotspots = %d, want 17", got.MaxChurnHotspots)
	}
	if got.PerSystemTimeout != 45*time.Second {
		t.Errorf("per_system_timeout = %s, want 45s", got.PerSystemTimeout)
	}
	if got.EnableWorldModel {
		t.Error("enable_world_model: the file's false was read as the default true")
	}
	if got.MaxLearnings != 100 || got.ConsultTimeout != 2*time.Minute || !got.EnableGitHistory {
		t.Errorf("absent intelligence keys did not take the defaults: %+v", got)
	}
}

// A count of 0 is absent. Check requires >= 1 only after WithDefaults, so an
// explicit 0 becomes the default and a negative stays and is named. The same
// rule as the rest of the campaign section.
func TestCampaignIntelligence_ExplicitZeroCountIsTheDefault(t *testing.T) {
	path := writeCampaignConfig(t, `{"campaign":{"intelligence":{"max_churn_hotspots":0,"git_history_depth":0}}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := cfg.GetCampaignConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if policy.Intelligence.MaxChurnHotspots != 50 || policy.Intelligence.GitHistoryDepth != 100 {
		t.Fatalf("explicit 0 did not take the defaults: %+v", policy.Intelligence)
	}
}

// Each switch is its own bool. One shared allocation would make setting any
// flag false turn the rest off.
func TestCampaignIntelligence_FlagsDoNotAlias(t *testing.T) {
	d := DefaultCampaignIntelligenceConfig()
	if d.EnableWorldModel == d.EnableGitHistory || d.EnableWorldModel == d.EnableShardConsult {
		t.Fatal("default enable flags share one bool")
	}
	filled := (CampaignIntelligenceConfig{}).WithDefaults()
	*filled.EnableWorldModel = false
	if !*filled.EnableGitHistory || !*filled.EnableShardConsult {
		t.Fatalf("clearing enable_world_model cleared another flag: git=%v consult=%v", *filled.EnableGitHistory, *filled.EnableShardConsult)
	}
}

// nerd config check lists these through ImplicitFields, which walks JSON tags.
// A tag that is not a real field never appears.
func TestCampaignIntelligence_CheckListsEveryKey(t *testing.T) {
	implicit := ImplicitFields([]byte(`{}`))
	joined := "\n" + strings.Join(implicit, "\n") + "\n"
	for _, key := range []string{
		"per_system_timeout",
		"consult_timeout",
		"max_churn_hotspots",
		"max_learnings",
		"max_mcp_tools",
		"max_previous_campaigns",
		"git_history_depth",
		"enable_world_model",
		"enable_git_history",
		"enable_learning_store",
		"enable_knowledge_graph",
		"enable_cold_storage",
		"enable_safety_check",
		"enable_autopoiesis",
		"enable_mcp_tools",
		"enable_previous_campaigns",
		"enable_shard_consult",
		"enable_test_coverage",
		"enable_code_patterns",
	} {
		needle := "\ncampaign.intelligence." + key + "\n"
		if !strings.Contains(joined, needle) {
			t.Errorf("implicit fields do not list campaign.intelligence.%s", key)
		}
	}
	if strings.Contains(joined, "gather_timeout") {
		t.Error("gather_timeout is listed; that phase clock has no key")
	}
}

func TestCampaignIntelligence_UnknownKeyIsRefused(t *testing.T) {
	path := writeCampaignConfig(t, `{"campaign":{"intelligence":{"max_churn_hotspot":1}}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("campaign.intelligence.max_churn_hotspot (a typo) loaded; the strict decoder must refuse it")
	}
}
