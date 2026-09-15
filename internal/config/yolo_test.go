package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestYoloMode_DefaultOffNilSafe(t *testing.T) {
	var nilCfg *UserConfig
	if nilCfg.YoloMode() {
		t.Fatal("nil config must not be yolo")
	}
	if DefaultUserConfig().YoloMode() {
		t.Fatal("default config must not be yolo")
	}
	cfg := DefaultUserConfig()
	cfg.Yolo = true
	if !cfg.YoloMode() {
		t.Fatal("Yolo=true must report yolo mode")
	}
}

func TestYoloMode_JSONKey(t *testing.T) {
	cfg := DefaultUserConfig()
	cfg.Yolo = true
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"yolo":true`) {
		t.Fatalf("yolo must serialize as \"yolo\": %s", raw[:200])
	}
	var back UserConfig
	if err := json.Unmarshal([]byte(`{"yolo":true}`), &back); err != nil {
		t.Fatal(err)
	}
	if !back.YoloMode() {
		t.Fatal("yolo:true must parse")
	}
}
