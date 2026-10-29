package core

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// TestRouteAction_CLINextActionShape_PassesParsing ties the CLI's
// nextActionFact shape (cmd/nerd/cmd_instruction.go) to parseActionFact's
// (ActionID, Type, Target) contract — the F-ROUTE-1 regression pair.
//
// The 3-arg shape the CLI now builds must get PAST parsing (any later
// failure, e.g. a permission gate, is a different error). The legacy 2-arg
// shape must still be rejected at parse with the "requires at least 3
// arguments" error, so this test fails loudly if either side of the
// contract drifts.
func TestRouteAction_CLINextActionShape_PassesParsing(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	cliShaped := Fact{
		Predicate: "next_action",
		Args:      []any{"cli-1", types.MangleAtom("/analyze_code"), "internal/prompt"},
	}
	if _, err := vs.RouteAction(ctx, cliShaped); err != nil &&
		strings.Contains(err.Error(), "requires at least 3 arguments") {
		t.Fatalf("CLI-shaped next_action fact failed parsing: %v", err)
	}

	legacyTwoArg := Fact{
		Predicate: "next_action",
		Args:      []any{types.MangleAtom("/analyze_code"), "explain the OODA loop"},
	}
	_, err := vs.RouteAction(ctx, legacyTwoArg)
	if err == nil || !strings.Contains(err.Error(), "requires at least 3 arguments") {
		t.Fatalf("legacy 2-arg fact should be rejected at parse, got: %v", err)
	}
}

// TestCheckKernelPermitted_PendingActionOpensGate is the second half of the
// F-ROUTE-1 regression. constitution.mg only derives permitted/3 from
// safe_action(Action) + a matching pending_action/5, so a caller that routes
// without filing pending_action is default-denied ("action analyze_code not
// permitted by kernel policy" — observed live). The CLI now files the request
// (cmd/nerd assertPendingAction) with Target and the canonical empty-payload
// "{}" exactly as CheckKernelPermitted recomputes them from a bare 3-arg fact.
func TestCheckKernelPermitted_PendingActionOpensGate(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.SetKernel(kernel)

	target := "explain what the OODA loop does in this codebase"

	// Red half: no pending_action filed → default deny.
	if vs.CheckKernelPermitted("analyze_code", target, map[string]any{}) {
		t.Fatal("gate should default-deny before pending_action is filed")
	}

	// Green half: file the request exactly as the CLI does.
	if err := kernel.Assert(Fact{
		Predicate: "pending_action",
		Args:      []any{"cli-1", types.MangleAtom("/analyze_code"), target, "{}", int64(1)},
	}); err != nil {
		t.Fatalf("Assert pending_action: %v", err)
	}

	if !vs.CheckKernelPermitted("analyze_code", target, map[string]any{}) {
		t.Fatal("gate should open: safe_action(/analyze_code) + matching pending_action filed")
	}
}
