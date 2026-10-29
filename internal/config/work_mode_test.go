package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkMode_DefaultProposeNilSafe(t *testing.T) {
	var nilCfg *UserConfig
	if nilCfg.YoloMode() || nilCfg.EffectiveWorkMode() != WorkModePropose {
		t.Fatal("nil config must be propose, not yolo")
	}
	if got := DefaultUserConfig().EffectiveWorkMode(); got != WorkModePropose {
		t.Fatalf("default work mode = %q, want propose", got)
	}
	if (&UserConfig{}).EffectiveWorkMode() != WorkModePropose {
		t.Fatal("an absent work_mode must read as propose")
	}
	cfg := DefaultUserConfig()
	cfg.WorkMode = WorkModeYolo
	if !cfg.YoloMode() {
		t.Fatal("work_mode yolo must report yolo mode")
	}
}

func TestWorkMode_CycleVisitsEveryModeOnce(t *testing.T) {
	seen := map[WorkMode]bool{}
	m := WorkModeYolo
	for range WorkModes {
		seen[m] = true
		m = m.Next()
	}
	if m != WorkModeYolo || len(seen) != 3 {
		t.Fatalf("cycle did not return to yolo through all three modes: %v", seen)
	}
}

func TestWorkMode_JSONKeyAndValidation(t *testing.T) {
	cfg := DefaultUserConfig()
	cfg.WorkMode = WorkModeExplicit
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"work_mode":"explicit"`) {
		t.Fatalf("work mode must serialize as work_mode: %s", raw[:200])
	}
	var back UserConfig
	if err := decodeStrictJSON([]byte(`{"work_mode":"yolo"}`), &back); err != nil {
		t.Fatal(err)
	}
	if !back.YoloMode() {
		t.Fatal(`work_mode "yolo" must parse as yolo`)
	}
	for _, retired := range []string{`{"yolo":true}`, `{"continuation_mode":0}`} {
		var c UserConfig
		if err := decodeStrictJSON([]byte(retired), &c); err == nil {
			t.Errorf("%s must be refused: work_mode replaced it", retired)
		}
	}
	bad := DefaultUserConfig()
	bad.WorkMode = "auto"
	found := false
	for _, p := range Errors(bad.Check(nil)) {
		if p.Path == "work_mode" {
			found = true
		}
	}
	if !found {
		t.Fatal(`work_mode "auto" must be a config error`)
	}
}
