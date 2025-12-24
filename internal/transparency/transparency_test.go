package transparency

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/config"
)

func TestTransparencyManagerEnableToggle(t *testing.T) {
	tm := NewTransparencyManager(nil)
	if tm.IsEnabled() {
		t.Fatalf("expected disabled by default")
	}
	tm.Enable()
	if !tm.IsEnabled() {
		t.Fatalf("expected enabled after Enable")
	}
	if tm.Toggle() {
		t.Fatalf("expected toggle to disable")
	}
	if tm.IsEnabled() {
		t.Fatalf("expected disabled after toggle")
	}
}

func TestTransparencyManagerStatusAndFormatError(t *testing.T) {
	cfg := &config.TransparencyConfig{
		Enabled:            true,
		ShardPhases:        true,
		SafetyExplanations: true,
		VerboseErrors:      true,
	}
	tm := NewTransparencyManager(cfg)

	tm.StartShard("shard-1", "coder", "task")
	status := tm.GetStatus()
	if !strings.Contains(status, "Transparency Status") {
		t.Fatalf("expected status output")
	}
	if !strings.Contains(status, "Active Operations") {
		t.Fatalf("expected active operations section")
	}

	formatted := tm.FormatError(errors.New("permission denied"))
	if !strings.Contains(formatted, "Suggested fixes") {
		t.Fatalf("expected verbose error formatting")
	}

	tm.Disable()
	formatted = tm.FormatError(errors.New("permission denied"))
	if strings.Contains(formatted, "Suggested fixes") {
		t.Fatalf("expected concise error formatting when disabled")
	}
}
