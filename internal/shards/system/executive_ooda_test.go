package system

import (
	"testing"
	"time"

	"codenerd/internal/core"
)

func TestExecutiveOODATimeoutEmitsFact(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	cfg := DefaultExecutiveConfig()
	cfg.OODATimeout = 15 * time.Millisecond
	exec := NewExecutivePolicyShardWithConfig(cfg)
	exec.SetParentKernel(kernel)
	exec.DisableBootGuard()

	if err := kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []interface{}{"/current_intent", "/instruction", "/deploy", "", ""},
	}); err != nil {
		t.Fatalf("assert user_intent: %v", err)
	}

	intent := exec.latestUserIntent()
	exec.updateOODATimeout(intent, false)
	time.Sleep(25 * time.Millisecond)
	exec.updateOODATimeout(intent, false)

	facts, err := kernel.Query("ooda_timeout")
	if err != nil {
		t.Fatalf("Query(ooda_timeout) error = %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("ooda_timeout not asserted after timeout")
	}
}

func TestExecutiveOODATimeoutRespectsBootGuard(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	cfg := DefaultExecutiveConfig()
	cfg.OODATimeout = 10 * time.Millisecond
	exec := NewExecutivePolicyShardWithConfig(cfg)
	exec.SetParentKernel(kernel)

	if err := kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []interface{}{"/current_intent", "/instruction", "/deploy", "", ""},
	}); err != nil {
		t.Fatalf("assert user_intent: %v", err)
	}

	intent := exec.latestUserIntent()
	exec.updateOODATimeout(intent, false)
	time.Sleep(20 * time.Millisecond)
	exec.updateOODATimeout(intent, false)

	facts, err := kernel.Query("ooda_timeout")
	if err != nil {
		t.Fatalf("Query(ooda_timeout) error = %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("ooda_timeout should not be asserted while boot guard active")
	}

	// Sanity: once boot guard is disabled, timeout should trigger.
	exec.DisableBootGuard()
	exec.updateOODATimeout(intent, false)
	time.Sleep(20 * time.Millisecond)
	exec.updateOODATimeout(intent, false)

	facts, err = kernel.Query("ooda_timeout")
	if err != nil {
		t.Fatalf("Query(ooda_timeout) error = %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("ooda_timeout not asserted after boot guard disabled")
	}
}
