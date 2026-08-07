package config

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A nil block must behave exactly as before this config surface existed, so
// adding it cannot change any existing install's timing.
func TestLLMTimeoutsConfig_NilResolvesToDefaults(t *testing.T) {
	var c *LLMTimeoutsConfig
	got, err := c.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != DefaultLLMTimeouts() {
		t.Errorf("nil config must resolve to DefaultLLMTimeouts()")
	}
}

// The "fast" and "aggressive" profiles existed in the codebase but were
// reachable only from tests. Selecting them by name is the point of Profile.
func TestLLMTimeoutsConfig_ProfileSelection(t *testing.T) {
	for name, want := range map[string]LLMTimeouts{
		"":           DefaultLLMTimeouts(),
		"default":    DefaultLLMTimeouts(),
		"fast":       FastLLMTimeouts(),
		"aggressive": AggressiveLLMTimeouts(),
	} {
		got, err := (&LLMTimeoutsConfig{Profile: name}).Resolve()
		if err != nil {
			t.Fatalf("profile %q: %v", name, err)
		}
		if got != want {
			t.Errorf("profile %q resolved to the wrong baseline", name)
		}
	}
}

func TestLLMTimeoutsConfig_UnknownProfileIsAnError(t *testing.T) {
	if _, err := (&LLMTimeoutsConfig{Profile: "turbo"}).Resolve(); err == nil {
		t.Error("an unrecognised profile must fail loudly, not silently use defaults")
	}
}

// Per-field overrides layer on top of the chosen profile; untouched fields keep
// the profile value.
func TestLLMTimeoutsConfig_OverlaysOnProfile(t *testing.T) {
	got, err := (&LLMTimeoutsConfig{
		Profile:          "fast",
		OODALoopTimeout:  "90s",
		PerCallTimeout:   "1m30s",
		RetryBackoffBase: "250ms",
	}).Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.OODALoopTimeout != 90*time.Second {
		t.Errorf("OODALoopTimeout = %v, want 90s", got.OODALoopTimeout)
	}
	if got.PerCallTimeout != 90*time.Second {
		t.Errorf("PerCallTimeout = %v, want 90s", got.PerCallTimeout)
	}
	if got.RetryBackoffBase != 250*time.Millisecond {
		t.Errorf("RetryBackoffBase = %v, want 250ms", got.RetryBackoffBase)
	}
	// Not overridden -> still the fast profile's value.
	if got.ShardExecutionTimeout != FastLLMTimeouts().ShardExecutionTimeout {
		t.Errorf("ShardExecutionTimeout = %v, want the fast profile value %v",
			got.ShardExecutionTimeout, FastLLMTimeouts().ShardExecutionTimeout)
	}
}

// A typo'd duration must fail rather than silently leave a 30-minute default in
// place — that silence is the exact failure mode this config replaced.
func TestLLMTimeoutsConfig_MalformedDurationIsAnError(t *testing.T) {
	for _, bad := range []string{"30", "thirty minutes", "-5m", "0s"} {
		_, err := (&LLMTimeoutsConfig{OODALoopTimeout: bad}).Resolve()
		if err == nil {
			t.Errorf("OODALoopTimeout %q should be rejected", bad)
		} else if !strings.Contains(err.Error(), "ooda_loop_timeout") {
			t.Errorf("error for %q should name the field, got: %v", bad, err)
		}
	}
}

// MaxRetries is a pointer specifically so "never retry" is expressible.
func TestLLMTimeoutsConfig_MaxRetriesZeroIsHonoured(t *testing.T) {
	zero := 0
	got, err := (&LLMTimeoutsConfig{MaxRetries: &zero}).Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0 — an explicit zero must not read as absent", got.MaxRetries)
	}

	absent, err := (&LLMTimeoutsConfig{}).Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if absent.MaxRetries != DefaultLLMTimeouts().MaxRetries {
		t.Errorf("absent MaxRetries = %d, want the profile default %d",
			absent.MaxRetries, DefaultLLMTimeouts().MaxRetries)
	}
}

// LoadUserConfig decodes strictly, so the block must round-trip through the
// real UserConfig shape — a mismatched tag would break the whole config file,
// not just this field.
func TestUserConfig_LLMTimeoutsRoundTrip(t *testing.T) {
	raw := []byte(`{"llm_timeouts":{"profile":"fast","ooda_loop_timeout":"10m","max_retries":1}}`)
	var cfg UserConfig
	if err := decodeStrictJSON(raw, &cfg); err != nil {
		t.Fatalf("strict decode failed — llm_timeouts is not wired into UserConfig: %v", err)
	}
	if cfg.LLMTimeouts == nil {
		t.Fatal("llm_timeouts did not decode")
	}
	resolved, err := cfg.LLMTimeouts.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.OODALoopTimeout != 10*time.Minute || resolved.MaxRetries != 1 {
		t.Errorf("round-trip lost values: %+v", resolved)
	}

	// And it must marshal back to the same key names.
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"llm_timeouts"`) {
		t.Errorf("llm_timeouts missing from marshalled config: %s", out)
	}
}

// The per-slot completion ceiling must survive a strict decode too; without it
// worker and planner are pinned to the client's hardcoded default.
func TestUserConfig_SlotMaxOutputTokensRoundTrip(t *testing.T) {
	raw := []byte(`{"planner":{"provider":"dashscope","model":"qwen3.8-max","max_output_tokens":32768}}`)
	var cfg UserConfig
	if err := decodeStrictJSON(raw, &cfg); err != nil {
		t.Fatalf("strict decode failed — max_output_tokens is not on SecondaryLLMConfig: %v", err)
	}
	if cfg.Planner == nil || cfg.Planner.MaxOutputTokens != 32768 {
		t.Fatalf("planner.max_output_tokens did not decode: %+v", cfg.Planner)
	}
}
