package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func typeSafeErrors(c *UserConfig) []string {
	var out []string
	for _, p := range c.Check(nil) {
		if p.Severity == SeverityError && strings.HasPrefix(p.Path, "typesafe") {
			out = append(out, p.Path)
		}
	}
	return out
}

func TestTypeSafe_ValidSectionResolves(t *testing.T) {
	t.Setenv("TS_TEST_KEY", "k-123")
	raw := []byte(`{"typesafe":{"model":"jev-1.14.0","base_url":"https://api.example.test/","api_key_env":"TS_TEST_KEY","request_timeout":"5s","long_context":true}}`)
	cfg, err := DecodeUserConfig(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := typeSafeErrors(cfg); len(got) != 0 {
		t.Fatalf("valid section refused: %v", got)
	}
	r, err := cfg.TypeSafe.Resolve()
	if err != nil || r == nil {
		t.Fatalf("Resolve = %v, %v", r, err)
	}
	if r.Model != "jev-1.14.0" || r.BaseURL != "https://api.example.test" || r.APIKey != "k-123" || r.Timeout != 5*time.Second || !r.LongContext {
		t.Errorf("resolved = %+v", r)
	}
}

func TestTypeSafe_InlineKeyAndTimeoutDefault(t *testing.T) {
	c := &TypeSafeConfig{Model: "m", BaseURL: "http://localhost:1", APIKey: "inline"}
	r, err := c.Resolve()
	if err != nil || r.APIKey != "inline" || r.Timeout != 30*time.Second {
		t.Fatalf("Resolve = %+v, %v", r, err)
	}
}

func TestTypeSafe_AbsentIsDisabled(t *testing.T) {
	cfg := &UserConfig{}
	if got := typeSafeErrors(cfg); len(got) != 0 {
		t.Errorf("absent section produced errors: %v", got)
	}
	r, err := cfg.TypeSafe.Resolve()
	if r != nil || err != nil {
		t.Errorf("absent Resolve = %v, %v; want nil, nil", r, err)
	}
	if got := typeSafeErrors(DefaultUserConfig()); len(got) != 0 {
		t.Errorf("default config carries typesafe errors: %v", got)
	}
}

func TestTypeSafe_PresentButIncompleteIsAnError(t *testing.T) {
	good := TypeSafeConfig{Model: "m", BaseURL: "https://x.test", APIKey: "k"}
	cases := map[string]func(*TypeSafeConfig){
		"typesafe.model":           func(c *TypeSafeConfig) { c.Model = " " },
		"typesafe.base_url":        func(c *TypeSafeConfig) { c.BaseURL = "" },
		"typesafe.api_key":         func(c *TypeSafeConfig) { c.APIKey = ""; c.APIKeyEnv = "" },
		"typesafe.request_timeout": func(c *TypeSafeConfig) { c.RequestTimeout = "soon" },
	}
	for path, mutate := range cases {
		c := good
		mutate(&c)
		found := false
		for _, p := range (&UserConfig{TypeSafe: &c}).Check(nil) {
			found = found || (p.Severity == SeverityError && p.Path == path)
		}
		if !found {
			t.Errorf("%s: no error for %+v", path, c)
		}
		if _, err := c.Resolve(); err == nil {
			t.Errorf("%s: Resolve accepted %+v", path, c)
		}
	}
}

func TestTypeSafe_BothKeyAndKeyEnvIsAnError(t *testing.T) {
	c := TypeSafeConfig{Model: "m", BaseURL: "https://x.test", APIKey: "k", APIKeyEnv: "TS_X"}
	if got := typeSafeErrors(&UserConfig{TypeSafe: &c}); len(got) != 1 || got[0] != "typesafe.api_key" {
		t.Errorf("errors = %v", got)
	}
}

func TestTypeSafe_BaseURLShapes(t *testing.T) {
	for _, bad := range []string{"api.example.test", "ftp://x.test", "https://x.test/?k=1", "https://u:p@x.test"} {
		c := TypeSafeConfig{Model: "m", BaseURL: bad, APIKey: "k"}
		if _, err := c.Resolve(); err == nil {
			t.Errorf("base_url %q accepted", bad)
		}
	}
}

func TestTypeSafe_UnsetKeyEnvFailsAtResolveNotCheck(t *testing.T) {
	c := TypeSafeConfig{Model: "m", BaseURL: "https://x.test", APIKeyEnv: "TS_DEFINITELY_UNSET_VAR"}
	if got := typeSafeErrors(&UserConfig{TypeSafe: &c}); len(got) != 0 {
		t.Errorf("Check read the environment: %v", got)
	}
	if _, err := c.Resolve(); err == nil || !strings.Contains(err.Error(), "TS_DEFINITELY_UNSET_VAR") {
		t.Errorf("Resolve err = %v", err)
	}
}

func TestTypeSafe_UnknownKeyIsRefused(t *testing.T) {
	if _, err := DecodeUserConfig([]byte(`{"typesafe":{"model":"m","modle":"x"}}`)); err == nil {
		t.Error("an unknown typesafe key was accepted")
	}
}

// The full document of a file with no section keeps Jev off: a zeroed section
// would be present and refused.
func TestTypeSafe_FullJSONKeepsAbsentOffAndPresentWhole(t *testing.T) {
	full, err := FullJSON([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(full), `"typesafe": null`) {
		t.Errorf("absent section not written as null")
	}
	if got := ImplicitFields(full); len(got) != 0 {
		t.Errorf("implicit fields: %v", got)
	}
	cfg, err := DecodeUserConfig(full)
	if err != nil || cfg.TypeSafe != nil {
		t.Fatalf("decode: %v, section %v", err, cfg.TypeSafe)
	}

	present, err := FullJSON([]byte(`{"typesafe":{"model":"m","base_url":"https://x.test","api_key_env":"K"}}`))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = DecodeUserConfig(present)
	if err != nil || cfg.TypeSafe == nil || cfg.TypeSafe.Check("typesafe") != nil {
		t.Fatalf("present section did not round-trip: %v %+v", err, cfg.TypeSafe)
	}
}

func TestTypeSafeUses_DefaultsSwitchesAndTuning(t *testing.T) {
	var none *TypeSafeConfig
	if u := none.Uses(); u.TurnJudgments || u.RetrievalRelevance {
		t.Fatal("no section must mean every use off")
	}
	off := false
	c := &TypeSafeConfig{RetrievalRelevance: &off, BatchSize: 7}
	u := c.Uses()
	if !u.TurnJudgments || u.RetrievalRelevance || !u.ReviewChecks || !u.DiagnosisTriage {
		t.Fatalf("switches = %+v, want every use on but retrieval relevance", u)
	}
	if u.BatchSize != 7 || u.RecentTurns != DefaultTypeSafeUses().RecentTurns {
		t.Fatalf("tuning = %+v, want batch 7 and the default window", u)
	}
	if !u.CorrectionJudgments || !u.TaskVerdicts || !u.ProblemTypes || !u.ToolChecks || !u.AutopoiesisChecks || !u.AtomChecks {
		t.Fatalf("learning uses = %+v, want each on when its key is absent", u)
	}
	learnOff := &TypeSafeConfig{CorrectionJudgments: &off, TaskVerdicts: &off, ProblemTypes: &off, ToolChecks: &off, AutopoiesisChecks: &off, AtomChecks: &off}
	if lu := learnOff.Uses(); lu.CorrectionJudgments || lu.TaskVerdicts || lu.ProblemTypes || lu.ToolChecks || lu.AutopoiesisChecks || lu.AtomChecks || !lu.TurnJudgments {
		t.Fatalf("learning switches off = %+v, want those off and the turn judgments on", lu)
	}
	bad := &TypeSafeConfig{Model: "m", BaseURL: "https://x", APIKey: "k", RecentTurns: -1}
	found := false
	for _, p := range bad.Check("typesafe") {
		if p.Path == "typesafe.recent_turns" {
			found = true
		}
	}
	if !found {
		t.Fatal("a negative recent_turns must be refused")
	}
}

// The turn judgments have their own bound, shorter than a request's: routing
// waits for them. It defaults, takes an override, and refuses nonsense.
func TestTypeSafeUses_TurnJudgmentTimeout(t *testing.T) {
	c := &TypeSafeConfig{Model: "m", BaseURL: "https://x", APIKey: "k"}
	if got := c.Uses().TurnJudgmentTimeout; got != defaultTurnJudgmentTimeout {
		t.Fatalf("default = %s, want %s", got, defaultTurnJudgmentTimeout)
	}
	c.TurnJudgmentTimeout = "750ms"
	if got := c.Uses().TurnJudgmentTimeout; got != 750*time.Millisecond {
		t.Fatalf("override = %s, want 750ms", got)
	}
	for _, bad := range []string{"soon", "-1s", "0s"} {
		c.TurnJudgmentTimeout = bad
		found := false
		for _, p := range c.Check("typesafe") {
			if p.Path == "typesafe.turn_judgment_timeout" {
				found = true
			}
		}
		if !found {
			t.Errorf("turn_judgment_timeout %q was accepted", bad)
		}
	}
}

func TestTypeSafe_NoProbabilityFollowsTheThreshold(t *testing.T) {
	var none *TypeSafeConfig
	if got := none.NoProbability(); got != float64(DefaultJudgmentNoPercent)/100 {
		t.Fatalf("default = %v", got)
	}
	if got := (&TypeSafeConfig{JudgmentNoPercent: 20}).NoProbability(); got != 0.2 {
		t.Fatalf("configured = %v, want 0.2", got)
	}
}

// TestTypeSafeUses_AtomRelevance: the per-turn prompt-atom judgment is on with
// its pool and timeout defaults when the section exists, off by its switch,
// tunable, and a bad timeout is refused.
func TestTypeSafeUses_AtomRelevance(t *testing.T) {
	u := (&TypeSafeConfig{}).Uses()
	if !u.AtomRelevance || u.AtomRelevancePool != 40 || u.AtomRelevanceTimeout != 4*time.Second {
		t.Fatalf("defaults = on %v pool %d timeout %s", u.AtomRelevance, u.AtomRelevancePool, u.AtomRelevanceTimeout)
	}
	off := false
	if (&TypeSafeConfig{AtomRelevance: &off}).Uses().AtomRelevance {
		t.Fatal("atom_relevance: false left the use on")
	}
	tuned := (&TypeSafeConfig{AtomRelevancePool: 12, AtomRelevanceTimeout: "1500ms"}).Uses()
	if tuned.AtomRelevancePool != 12 || tuned.AtomRelevanceTimeout != 1500*time.Millisecond {
		t.Fatalf("tuning ignored: %+v", tuned)
	}
	if (*TypeSafeConfig)(nil).Uses().AtomRelevance {
		t.Fatal("an absent section turned atom relevance on")
	}
	bad := TypeSafeConfig{Model: "m", BaseURL: "https://x", APIKey: "k", AtomRelevanceTimeout: "soon", AtomRelevancePool: -1}
	var fields []string
	for _, p := range bad.Check("typesafe") {
		fields = append(fields, p.Path)
	}
	if !slices.Contains(fields, "typesafe.atom_relevance_timeout") || !slices.Contains(fields, "typesafe.atom_relevance_pool") {
		t.Fatalf("bad atom relevance tuning not refused: %v", fields)
	}
	if yes, no := (&TypeSafeConfig{JudgmentYesPercent: 80, JudgmentNoPercent: 20}).Thresholds(); yes != 80 || no != 20 {
		t.Fatalf("Thresholds = %d/%d", yes, no)
	}
}

func TestTypeSafeUses_AtomRelevanceWorkLimits(t *testing.T) {
	got := (&TypeSafeConfig{}).Uses()
	if got.AtomRelevanceMaxAtoms != 80 || got.AtomRelevanceMaxInputTokens != 80000 || got.AtomRelevanceMaxConcurrentBatches != 2 {
		t.Fatalf("default work limits: %+v", got)
	}
	c := TypeSafeConfig{Model: "m", BaseURL: "https://x", APIKey: "k",
		AtomRelevanceMaxAtoms: 12, AtomRelevanceMaxInputTokens: 5000, AtomRelevanceMaxConcurrentBatches: 1}
	got = c.Uses()
	if got.AtomRelevanceMaxAtoms != 12 || got.AtomRelevanceMaxInputTokens != 5000 || got.AtomRelevanceMaxConcurrentBatches != 1 {
		t.Fatalf("configured limits: %+v", got)
	}
	if problems := c.Check("typesafe"); len(problems) != 0 {
		t.Fatalf("valid work limits refused: %v", problems)
	}
	c.AtomRelevanceMaxAtoms, c.AtomRelevanceMaxInputTokens, c.AtomRelevanceMaxConcurrentBatches = -1, -1, -1
	var fields []string
	for _, p := range c.Check("typesafe") {
		fields = append(fields, p.Path)
	}
	for _, name := range []string{"atom_relevance_max_atoms", "atom_relevance_max_input_tokens", "atom_relevance_max_concurrent_batches"} {
		if !slices.Contains(fields, "typesafe."+name) {
			t.Errorf("negative %s accepted: %v", name, fields)
		}
	}
}
