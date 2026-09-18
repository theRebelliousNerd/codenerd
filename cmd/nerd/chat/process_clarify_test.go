package chat

import (
	"context"
	"testing"
)

// TestProcessInputClarifyRouteRunsClarifierPath pins the /clarify lane end
// to end: the kernel derives route_decision(/clarify, /none) for a vague
// mutation, decideRoute maps it to RouteDecision{Kind: RouteClarify}, and
// processInput must run the clarification path instead of falling through
// to delegation. Before the fix, processInput never consumed RouteClarify,
// so this input produced a delegation/articulation message and never reached
// the clarifier.
func TestProcessInputClarifyRouteRunsClarifierPath(t *testing.T) {
	m := NewTestModel()
	if m.transducer == nil {
		t.Fatal("test model has no transducer wired")
	}

	const input = "fix it"

	intent, err := m.transducer.ParseIntentWithContext(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("parse intent for %q: %v", input, err)
	}
	route := m.decideRoute(input, intent, resolveShardTypeForIntent(intent))
	if route.Kind != RouteClarify {
		t.Fatalf("precondition: decideRoute(%q) = %v, want RouteClarify", input, route.Kind)
	}

	msg := m.processInput(input)()
	switch msg.(type) {
	case clarificationMsg, *clarificationMsg:
		// clarifier path taken; must not fall through to delegation.
	default:
		t.Fatalf("processInput(%q) with RouteClarify returned %T, want clarificationMsg", input, msg)
	}
}
