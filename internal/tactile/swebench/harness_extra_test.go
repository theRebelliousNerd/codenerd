package swebench

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/tactile/python"
)

func TestNewHarnessAndGetters(t *testing.T) {
	inst := &Instance{
		InstanceID: "django__django-11001",
		Repo:       "django/django",
		BaseCommit: "deadbeef",
	}
	// A nil executor is fine: NewHarness only wires configuration and does not
	// touch Docker until Setup/Initialize is called.
	h := NewHarness(inst, python.EnvironmentConfig{}, nil)
	if h == nil {
		t.Fatal("NewHarness returned nil")
	}
	if h.Instance() != inst {
		t.Error("Instance() should return the wired instance")
	}
	if h.Environment() == nil {
		t.Error("Environment() should be non-nil after construction")
	}
	// State is readable without an initialized container.
	if h.State() == "" {
		t.Error("State() should report a non-empty environment state")
	}
}

func TestNewHarness_NilInstance_ShouldTolerateNil(t *testing.T) {
	h := NewHarness(nil, python.EnvironmentConfig{}, nil)
	if h == nil {
		t.Fatal("NewHarness(nil) returned nil")
	}
	if h.Instance() == nil {
		t.Error("Instance() should be non-nil after NewHarness(nil)")
	}
	if h.Environment() == nil {
		t.Error("Environment() should be non-nil after NewHarness(nil)")
	}
	if h.State() == "" {
		t.Error("State() should report a non-empty environment state")
	}
}

func TestNewHarness_WorkspaceDir_ShouldDefaultToTestbed(t *testing.T) {
	cases := map[string]python.EnvironmentConfig{
		"empty defaults":     {},
		"workspace defaults": {WorkspaceDir: "/workspace"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			inst := &Instance{InstanceID: "x", Repo: "o/r", BaseCommit: "abc"}
			h := NewHarness(inst, cfg, nil)
			if h == nil || h.Environment() == nil {
				t.Fatal("harness or environment is nil")
			}
			// NewHarness normalizes "" and "/workspace" to SWE-bench "/testbed".
			// RepoPath is WorkspaceDir + "/" + RepoName, so it pins the default.
			if got := h.Environment().RepoPath(); got != "/testbed/r" {
				t.Errorf("RepoPath should be /testbed/r after workspace default, got %q", got)
			}
		})
	}
	t.Run("custom preserved", func(t *testing.T) {
		inst := &Instance{InstanceID: "x", Repo: "o/r", BaseCommit: "abc"}
		h := NewHarness(inst, python.EnvironmentConfig{WorkspaceDir: "/custom"}, nil)
		if h == nil || h.Environment() == nil {
			t.Fatal("harness or environment is nil")
		}
		if got := h.Environment().RepoPath(); got != "/custom/r" {
			t.Errorf("custom WorkspaceDir should be preserved, got %q", got)
		}
	})
}

func TestHarness_Evaluate_NilPrediction_ShouldFailClosed(t *testing.T) {
	inst := &Instance{InstanceID: "django__django-11001", Repo: "django/django", BaseCommit: "deadbeef"}
	h := NewHarness(inst, python.EnvironmentConfig{}, nil)
	result, err := h.Evaluate(context.Background(), nil)
	if err == nil {
		t.Fatal("Evaluate(nil prediction) should return an error")
	}
	if !strings.Contains(err.Error(), "nil prediction") {
		t.Errorf("error should mention nil prediction, got %q", err.Error())
	}
	if result == nil {
		t.Fatal("Evaluate(nil prediction) should return a non-nil result")
	}
	if result.PatchApplied {
		t.Error("PatchApplied should be false for nil prediction")
	}
	if result.ErrorPhase != "patch" {
		t.Errorf("ErrorPhase should be patch, got %q", result.ErrorPhase)
	}
	if result.Error == "" {
		t.Error("Error should be recorded for nil prediction")
	}
	if result.InstanceID != inst.InstanceID {
		t.Errorf("InstanceID should be %q, got %q", inst.InstanceID, result.InstanceID)
	}
	if result.Resolved {
		t.Error("Resolved should be false for nil prediction")
	}
	// Fail-closed error paths must still record completion timing.
	if result.StartedAt.IsZero() {
		t.Error("StartedAt should be recorded even on nil-prediction error")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should be recorded even on nil-prediction error")
	}
	if result.CompletedAt.Before(result.StartedAt) {
		t.Error("CompletedAt should not precede StartedAt on nil-prediction error")
	}
	if result.Duration < 0 {
		t.Errorf("Duration should be non-negative, got %v", result.Duration)
	}
	if result.TestDuration != result.Duration {
		t.Errorf("TestDuration should mirror Duration on early error, got %v vs %v", result.TestDuration, result.Duration)
	}
}

func TestHarness_Evaluate_NilHarness_ShouldError(t *testing.T) {
	var h *Harness
	if _, err := h.Evaluate(context.Background(), &Prediction{}); err == nil {
		t.Error("nil harness Evaluate should return an error")
	}
	if _, err := h.EvaluateWithReset(context.Background(), &Prediction{}); err == nil {
		t.Error("nil harness EvaluateWithReset should return an error")
	}
}

func TestHarness_EvaluateWithReset_NilPrediction_ShouldPreserveResult(t *testing.T) {
	inst := &Instance{InstanceID: "django__django-11001", Repo: "django/django", BaseCommit: "deadbeef"}
	h := NewHarness(inst, python.EnvironmentConfig{}, nil)
	result, err := h.EvaluateWithReset(context.Background(), nil)
	if err == nil {
		t.Fatal("EvaluateWithReset(nil prediction) should return an error")
	}
	if result == nil {
		t.Fatal("EvaluateWithReset(nil prediction) should return a non-nil result")
	}
	if result.ErrorPhase != "patch" {
		t.Errorf("ErrorPhase should be patch, got %q", result.ErrorPhase)
	}
	if result.InstanceID != inst.InstanceID {
		t.Errorf("InstanceID should be %q, got %q", inst.InstanceID, result.InstanceID)
	}
	if result.PatchApplied {
		t.Error("PatchApplied should be false for nil prediction")
	}
	if result.StartedAt.IsZero() {
		t.Error("StartedAt should be recorded even on nil-prediction error")
	}
	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should be recorded even on nil-prediction error")
	}
	if result.TestDuration != result.Duration {
		t.Errorf("TestDuration should mirror Duration on early error, got %v vs %v", result.TestDuration, result.Duration)
	}
}

func TestHarness_Reset_NilHarness_ShouldError(t *testing.T) {
	var h *Harness
	if err := h.Reset(context.Background()); err == nil {
		t.Error("nil harness Reset should return an error")
	}
}

func TestHarness_Reset_NilEnvironment_ShouldError(t *testing.T) {
	h := &Harness{}
	if err := h.Reset(context.Background()); err == nil {
		t.Error("Reset with nil environment should return an error")
	}
}

func TestHarness_Evaluate_NilEnvironment_ShouldError(t *testing.T) {
	inst := &Instance{InstanceID: "django__django-11001", Repo: "django/django", BaseCommit: "deadbeef"}
	h := &Harness{}
	// Same-package construction lets the test pin the nil-environment guard
	// without going through NewHarness, which always wires an environment.
	// Accessing unexported fields is intentional: it isolates the guard.
	h.instance = inst
	if _, err := h.Evaluate(context.Background(), &Prediction{}); err == nil {
		t.Error("Evaluate with nil environment should return an error")
	}
	if _, err := h.EvaluateWithReset(context.Background(), &Prediction{}); err == nil {
		t.Error("EvaluateWithReset with nil environment should return an error")
	}
}
