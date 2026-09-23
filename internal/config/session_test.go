package config

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The defaults are a policy a session can run on.
func TestSessionConfig_TheDefaultsCheckClean(t *testing.T) {
	if problems := DefaultSessionConfig().Check("session"); len(problems) != 0 {
		t.Fatalf("the defaults do not check clean: %v", problems)
	}
	if _, err := DefaultSessionConfig().Resolve(); err != nil {
		t.Fatalf("the defaults do not resolve: %v", err)
	}
}

// A key the user writes reaches the executor; every key they leave out is
// the default, not a zero. history_turn_window 0 is a value (no prior
// turns), not "absent".
func TestSessionConfig_AKeyInTheFileReachesThePolicy(t *testing.T) {
	path := writeCampaignConfig(t, `{"session": {"step_plan_min_sites": 3, "step_plan_timeout": "45s", "history_turn_window": 0}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	policy, err := cfg.GetSessionConfig().Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.StepPlanMinSites != 3 {
		t.Errorf("step_plan_min_sites = %d, want the file's 3", policy.StepPlanMinSites)
	}
	if policy.StepPlanTimeout != 45*time.Second {
		t.Errorf("step_plan_timeout = %s, want the file's 45s", policy.StepPlanTimeout)
	}
	if policy.HistoryTurnWindow != 0 {
		t.Errorf("history_turn_window = %d, want the file's 0", policy.HistoryTurnWindow)
	}
	def := DefaultSessionConfig()
	if policy.StepPlanMaxSteps != def.StepPlanMaxSteps || policy.RepairMaxAttempts != def.RepairMaxAttempts || policy.HistoryCharBudget != def.HistoryCharBudget {
		t.Errorf("absent keys did not take the defaults: %+v", policy)
	}
}

func TestSessionConfig_AnUnknownKeyIsRefused(t *testing.T) {
	path := writeCampaignConfig(t, `{"session": {"step_plan_min_site": 3}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("session.step_plan_min_site (a typo) loaded; the strict decoder must refuse it")
	}
}

func TestSessionConfig_CheckNamesEachContradiction(t *testing.T) {
	window := -1
	bad := SessionConfig{
		StepPlanMinSites:  5,
		StepPlanMaxSteps:  4,
		RepairMaxAttempts: -2,
		ToolTimeout:       "soon",
		HistoryTurnWindow: &window,
	}
	problems := bad.Check("session")
	var paths []string
	for _, p := range problems {
		if p.Severity != SeverityError {
			t.Errorf("%s is %s, want an error", p.Path, p.Severity)
		}
		paths = append(paths, p.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{
		"session.step_plan_max_steps",
		"session.repair_max_attempts",
		"session.tool_timeout",
		"session.history_turn_window",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, problems)
		}
	}
	if _, err := bad.Resolve(); err == nil {
		t.Fatal("a section with contradictions resolved; the executor would run on it")
	}
	// The loader refuses the same file.
	path := writeCampaignConfig(t, `{"session": {"repair_max_attempts": -2}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("a session section with a negative attempt count loaded")
	}
}

// Every knob a rule reads reaches the kernel under the key the rule names
// (turn_steps.mg reads /session_step_plan_min_sites).
func TestSessionPolicy_ParamsCarryEveryPolicyKnob(t *testing.T) {
	policy, err := DefaultSessionConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, f := range ParamFacts(policy.Params()) {
		got[string(f.Args[0].(types.MangleAtom))] = f.Args[1].(int64)
	}
	if got["/session_step_plan_min_sites"] != int64(policy.StepPlanMinSites) {
		t.Fatalf("params = %v, want /session_step_plan_min_sites = %d", got, policy.StepPlanMinSites)
	}
}
