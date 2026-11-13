package config

import "testing"

// Self-repair's knobs reach the kernel as config_param rows, with their
// defaults when unset and 0/1 for the switch.
func TestLoggingParams_SelfRepairKnobs(t *testing.T) {
	byKey := func(ps []Param) map[string]int64 {
		out := map[string]int64{}
		for _, p := range ps {
			out[p.Key] = p.Value
		}
		return out
	}
	def := byKey((*LoggingConfig)(nil).Params())
	for key, want := range map[string]int64{
		"/logging_self_problem_warn_repeats":      DefaultSelfProblemWarnRepeats,
		"/logging_self_repair_enabled":            1,
		"/logging_self_repair_cooldown_sec":       DefaultSelfRepairCooldownSec,
		"/logging_self_repair_max_failures":       DefaultSelfRepairMaxFailures,
		"/logging_self_repair_max_jobs":           DefaultSelfRepairMaxJobs,
		"/logging_self_repair_verify_window_sec":  DefaultSelfRepairVerifyWindowSec,
		"/logging_self_optimize_min_count":        DefaultSelfOptimizeMinCount,
		"/logging_self_repair_no_defect_regrowth": DefaultSelfRepairNoDefectRegrowth,
		"/logging_self_repair_fix_budget_sec":     DefaultSelfRepairFixBudgetSec,
	} {
		if got, ok := def[key]; !ok || got != want {
			t.Errorf("default %s = %d (present %v), want %d", key, got, ok, want)
		}
	}
	set := byKey((&LoggingConfig{SelfRepairDisabled: true, SelfRepairCooldownSec: 5, SelfRepairMaxJobs: 3}).Params())
	if set["/logging_self_repair_enabled"] != 0 || set["/logging_self_repair_cooldown_sec"] != 5 || set["/logging_self_repair_max_jobs"] != 3 {
		t.Fatalf("configured params = %v", set)
	}
	if got := (&LoggingConfig{SelfRepairPollSec: 7}).SelfRepairPoll(); got.Seconds() != 7 {
		t.Fatalf("poll = %v", got)
	}
	if got := (*LoggingConfig)(nil).SelfRepairCheckTimeout(); int(got.Seconds()) != DefaultSelfRepairCheckTimeoutSec {
		t.Fatalf("check timeout default = %v", got)
	}
}

func TestCheck_RejectsNegativeSelfRepairKnobs(t *testing.T) {
	cfg := DefaultUserConfig()
	cfg.Logging = &LoggingConfig{SelfRepairMaxFailures: -1}
	found := false
	for _, p := range cfg.Check(nil) {
		if p.Path == "logging.self_repair_max_failures" && p.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("a negative self_repair_max_failures passed the config check")
	}
}
