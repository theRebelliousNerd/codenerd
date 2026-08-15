package core

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/transparency"
)

// TestRouteAction_WhenKernelDenies_ShouldAutoReportSafetyViolation proves the
// operator surface exists for a refusal.
//
// Before this wiring a constitutional denial reached the audit log and the
// error string and nothing else: SafetyReporter had no automatic producer, so
// `/transparency`'s "Recent Safety Blocks" section could never render even
// though every deny path was firing.
func TestRouteAction_WhenKernelDenies_ShouldAutoReportSafetyViolation(t *testing.T) {
	tm := transparency.NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		SafetyExplanations: true,
	})
	prev := transparency.SetProcessManager(tm)
	t.Cleanup(func() { transparency.SetProcessManager(prev) })

	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.DisableBootGuard() // routing is blocked until the first user interaction
	vs.SetKernel(&stubKernel{
		permitted: []Fact{{Predicate: "permitted", Args: []any{"/list_files"}}},
	})

	// read_file is not destructive, so this reaches the kernel permission gate
	// rather than the dreamer gate.
	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"act_deny_1", "/read_file", "main.go"},
	})
	if err == nil {
		t.Fatal("expected the action to be denied")
	}

	violations := tm.SafetyReporter().GetRecentViolations(0)
	if len(violations) != 1 {
		t.Fatalf("expected exactly one auto-reported violation, got %d", len(violations))
	}
	v := violations[0]
	// ActionType is stored without the Mangle name-constant slash.
	if v.Action != "read_file" || v.Target != "main.go" {
		t.Errorf("violation lost action/target context: %+v", v)
	}
	if v.Rule == "" {
		t.Error("violation must name the rule that refused")
	}

	// And the returned error must carry its category rather than relying on
	// substring matching downstream.
	var boundary *transparency.BoundaryError
	if !errors.As(err, &boundary) {
		t.Fatalf("expected a typed BoundaryError from the deny path, got %T", err)
	}
	if boundary.TransparencyCategory() != transparency.ErrorCategorySafety {
		t.Errorf("expected safety category, got %s", boundary.TransparencyCategory())
	}
	if transparency.ClassifyError(err).Category != transparency.ErrorCategorySafety {
		t.Error("ClassifyError should honor the declared category")
	}
}

// TestRouteAction_WhenDestructiveAndDreamerMissing_ShouldReportDenial covers
// the fail-closed dreamer gate, which is a refusal an operator must also see.
func TestRouteAction_WhenDestructiveAndDreamerMissing_ShouldReportDenial(t *testing.T) {
	tm := transparency.NewTransparencyManager(&config.TransparencyConfig{
		Enabled:            true,
		SafetyExplanations: true,
	})
	prev := transparency.SetProcessManager(tm)
	t.Cleanup(func() { transparency.SetProcessManager(prev) })

	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.DisableBootGuard()
	vs.SetKernel(&stubKernel{permitted: []Fact{{Predicate: "permitted", Args: []any{"/exec_cmd"}}}})

	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"act_deny_2", "/exec_cmd", "echo hi"},
	})
	if err == nil {
		t.Fatal("expected the destructive action to fail closed without a dreamer")
	}
	if len(tm.SafetyReporter().GetRecentViolations(0)) == 0 {
		t.Fatal("expected the fail-closed refusal to reach the safety reporter")
	}
}
