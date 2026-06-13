package swebench

import (
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
