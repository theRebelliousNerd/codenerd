package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func writeCampaignConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The defaults are a policy a campaign can run on.
func TestCampaignConfig_TheDefaultsCheckClean(t *testing.T) {
	if problems := DefaultCampaignConfig().Check("campaign"); len(problems) != 0 {
		t.Fatalf("the defaults do not check clean: %v", problems)
	}
	if _, err := DefaultCampaignConfig().Resolve(); err != nil {
		t.Fatalf("the defaults do not resolve: %v", err)
	}
}

// A key the user writes reaches the policy; every key they leave out is the
// default, not a zero.
func TestCampaignConfig_AKeyInTheFileReachesThePolicy(t *testing.T) {
	path := writeCampaignConfig(t, `{"campaign": {"max_task_attempts": 1, "retry_backoff_base": "2s", "replan_on_checkpoint_failure": false, "checkpoint_min_confidence": 0}}`)
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	policy, err := cfg.GetCampaignConfig().Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if policy.MaxTaskAttempts != 1 {
		t.Errorf("max_task_attempts = %d, want the file's 1", policy.MaxTaskAttempts)
	}
	if policy.RetryBackoffBase != 2*time.Second {
		t.Errorf("retry_backoff_base = %s, want the file's 2s", policy.RetryBackoffBase)
	}
	if policy.ReplanOnCheckpointFailure {
		t.Error("replan_on_checkpoint_failure: the file's false was read as the default true")
	}
	if policy.CheckpointMinConfidence != 0 {
		t.Errorf("checkpoint_min_confidence = %d, want the file's 0", policy.CheckpointMinConfidence)
	}
	def := DefaultCampaignConfig()
	if policy.MaxCheckpointAttempts != def.MaxCheckpointAttempts || policy.AcceptanceRounds != def.AcceptanceRounds {
		t.Errorf("absent keys did not take the defaults: %+v", policy)
	}
}

// Strict decoding reaches into the section: a key that is not a field is a
// load error, not a silently ignored typo.
func TestCampaignConfig_AnUnknownKeyIsRefused(t *testing.T) {
	path := writeCampaignConfig(t, `{"campaign": {"max_task_attempt": 1}}`)
	if _, err := LoadUserConfig(path); err == nil {
		t.Fatal("campaign.max_task_attempt (a typo) loaded; the strict decoder must refuse it")
	}
}

func TestCampaignConfig_CheckNamesEachContradiction(t *testing.T) {
	bad := CampaignConfig{
		MaxTaskAttempts:         -1,
		RetryBackoffBase:        "1m",
		RetryBackoffMax:         "10s",
		VerifyBuildTimeout:      "soon",
		CheckpointMinConfidence: func() *int { v := 101; return &v }(),
	}
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
		"campaign.max_task_attempts",
		"campaign.retry_backoff_max",
		"campaign.verify_build_timeout",
		"campaign.checkpoint_min_confidence",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Check did not name %s; it found %v", want, problems)
		}
	}
	if _, err := bad.Resolve(); err == nil {
		t.Fatal("a section with contradictions resolved; the orchestrator would run on it")
	}
}

// Every knob a rule reads reaches the kernel under the key the rule names.
func TestCampaignPolicy_ParamsCarryEveryPolicyKnob(t *testing.T) {
	policy, err := DefaultCampaignConfig().Resolve()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, f := range ParamFacts(policy.Params()) {
		if f.Predicate != ConfigParamPredicate || len(f.Args) != 2 {
			t.Fatalf("malformed param fact %v", f)
		}
		got[string(f.Args[0].(types.MangleAtom))] = f.Args[1].(int64)
	}
	for key, want := range map[string]int64{
		"/campaign_max_task_attempts":            4,
		"/campaign_repro_after_failures":         2,
		"/campaign_replan_at_attempt_cap":        1,
		"/campaign_max_checkpoint_attempts":      3,
		"/campaign_replan_on_checkpoint_failure": 1,
		"/campaign_checkpoint_min_confidence":    50,
		"/campaign_acceptance_rounds":            3,
		"/campaign_upstream_inline_max_bytes":    48 * 1024,
	} {
		if got[key] != want {
			t.Errorf("config_param(%s) = %d, want %d", key, got[key], want)
		}
	}
}
