package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

func TestYolo_TogglesAndPersists(t *testing.T) {
	m := NewTestModel()
	m.workspace = t.TempDir()
	if err := os.MkdirAll(filepath.Join(m.workspace, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Config = config.DefaultUserConfig()

	updated, _ := m.handleCmdYolo("/yolo", []string{"/yolo"})
	result := updated.(Model)
	if !result.yoloEnabled() {
		t.Fatal("bare /yolo must toggle on from off")
	}
	raw, err := os.ReadFile(filepath.Join(m.workspace, ".nerd", "config.json"))
	if err != nil {
		t.Fatalf("yolo must persist: %v", err)
	}
	if !strings.Contains(string(raw), `"yolo": true`) {
		t.Fatalf("persisted config missing yolo:true: %s", raw[:200])
	}

	updated, _ = result.handleCmdYolo("/yolo off", []string{"/yolo", "off"})
	if updated.(Model).yoloEnabled() {
		t.Fatal("/yolo off must toggle off")
	}
}

func TestYolo_StatusAndUsage(t *testing.T) {
	m := NewTestModel()
	m.Config = config.DefaultUserConfig()

	updated, _ := m.handleCmdYolo("/yolo status", []string{"/yolo", "status"})
	last := updated.(Model).history[len(updated.(Model).history)-1]
	if !strings.Contains(last.Content, "OFF") {
		t.Fatalf("status must report OFF: %q", last.Content)
	}
	updated, _ = m.handleCmdYolo("/yolo frobnicate", []string{"/yolo", "frobnicate"})
	last = updated.(Model).history[len(updated.(Model).history)-1]
	if !strings.Contains(last.Content, "Usage") {
		t.Fatalf("bad arg must show usage: %q", last.Content)
	}
}

func TestYolo_NilConfigIsOff(t *testing.T) {
	m := NewTestModel()
	m.Config = nil
	if m.yoloEnabled() {
		t.Fatal("nil config must not be yolo")
	}
	// Toggling with no config must not panic; it just cannot persist.
	updated, _ := m.handleCmdYolo("/yolo", []string{"/yolo"})
	_ = updated.(Model)
}
