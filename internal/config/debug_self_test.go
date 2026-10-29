package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDebugSelfMode_DefaultOffNilSafe(t *testing.T) {
	var nilCfg *UserConfig
	if nilCfg.DebugSelfMode() {
		t.Fatal("nil config must not be debug-self")
	}
	if DefaultUserConfig().DebugSelfMode() {
		t.Fatal("default config must not be debug-self")
	}
	var nilLogging *LoggingConfig
	if nilLogging.EffectiveDebugMode() {
		t.Fatal("nil logging config must not be effectively debug")
	}
	if got := nilLogging.ToLoggingConfig(); got.DebugMode || got.DebugSelf {
		t.Fatal("nil logging config must convert to zero logging config")
	}
}

func TestDebugSelfMode_JSONKeyAndImpliesDebugMode(t *testing.T) {
	var cfg UserConfig
	if err := json.Unmarshal([]byte(`{"logging":{"debug_self":true}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.DebugSelfMode() {
		t.Fatal(`logging.debug_self:true must parse`)
	}
	// debug_self implies debug mode wherever the effective config is computed.
	if !cfg.GetLogging().DebugMode {
		t.Fatal("GetLogging must report DebugMode=true when debug_self is on")
	}
	eff := cfg.GetLogging()
	lc := eff.ToLoggingConfig()
	if !lc.DebugMode || !lc.DebugSelf {
		t.Fatalf("ToLoggingConfig = %+v, want DebugMode and DebugSelf true", lc)
	}
	if !cfg.Logging.EffectiveDebugMode() || !cfg.Logging.IsCategoryEnabled("kernel") {
		t.Fatal("debug_self must enable categories via effective debug mode")
	}

	cfg.Logging.DebugSelf = true
	cfg.Logging.DebugMode = false
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"debug_self":true`) {
		t.Fatalf("debug_self must serialize under logging: %s", raw[:300])
	}
	// Enabling debug_self must not persist debug_mode:true or work_mode yolo.
	if strings.Contains(string(raw), `"debug_mode":true`) || strings.Contains(string(raw), `"work_mode":"yolo"`) {
		t.Fatalf("debug_self must not flip persisted debug_mode/yolo: %s", raw[:300])
	}
}
