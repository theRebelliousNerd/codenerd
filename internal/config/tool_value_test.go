package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolValueConfig_AbsentSectionIsTheDefaults(t *testing.T) {
	var c *UserConfig
	if got, want := c.GetToolValueConfig(), DefaultToolValueConfig(); got != want {
		t.Errorf("nil config = %+v, want %+v", got, want)
	}
	partial := &UserConfig{ToolValue: &ToolValueConfig{MinCalls: 5}}
	got := partial.GetToolValueConfig()
	if got.MinCalls != 5 || got.MinSessions != DefaultToolValueConfig().MinSessions {
		t.Errorf("partial section = %+v: an explicit key wins and an absent one defaults", got)
	}
}

func TestToolValueConfig_ParamsAreEveryKeyThePolicyRequires(t *testing.T) {
	want := []string{
		"/tool_value_min_sessions", "/tool_value_min_calls", "/tool_value_min_condensation_percent",
		"/tool_value_min_leverage_percent", "/tool_value_dead_leverage_percent", "/tool_value_bytes_per_token",
	}
	got := map[string]int64{}
	for _, p := range DefaultToolValueConfig().Params() {
		got[p.Key] = p.Value
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("no param for %s; policy/tool_value.mg requires it", key)
		}
	}
	if got["/tool_value_bytes_per_token"] != BytesPerToken {
		t.Errorf("bytes per token %d: the policy and the estimator must divide by one number", got["/tool_value_bytes_per_token"])
	}
}

func TestToolValueConfig_CheckRefusesContradictions(t *testing.T) {
	for name, c := range map[string]ToolValueConfig{
		"negative sessions":   {MinSessions: -1},
		"dead bar over earns": {MinLeveragePercent: 200, DeadLeveragePercent: 300},
		"dead bar equals":     {MinLeveragePercent: 500, DeadLeveragePercent: 500},
		"hit rate over 100":   {MinHitPercent: 101},
		"negative judged":     {MinJudgedCalls: -1},
	} {
		if len(c.Check("tool_value")) == 0 {
			t.Errorf("%s accepted", name)
		}
	}
	if problems := DefaultToolValueConfig().Check("tool_value"); len(problems) != 0 {
		t.Errorf("the defaults are refused: %v", problems)
	}
}

func TestToolValueConfig_LoadsFromTheFileAndFailsTheCheckThere(t *testing.T) {
	var c UserConfig
	if err := json.Unmarshal([]byte(`{"tool_value":{"min_sessions":3,"dead_leverage_percent":9000}}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.GetToolValueConfig().MinSessions != 3 {
		t.Errorf("min_sessions not read from the file: %+v", c.ToolValue)
	}
	var found bool
	for _, p := range c.Check([]byte(`{"tool_value":{}}`)) {
		found = found || strings.HasPrefix(p.Path, "tool_value.")
	}
	if !found {
		t.Error("UserConfig.Check does not check the tool_value section")
	}
}
