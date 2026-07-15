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
