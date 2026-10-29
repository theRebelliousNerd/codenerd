package chat

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// clarifyStubTransducer is a deterministic perception stub for the clarify-lane
// pin. It returns one canned intent regardless of input, the same way the
// integration harness's chatLoopTransducer does, but it lives in the default
// build so this test runs under plain `go test ./...`.
type clarifyStubTransducer struct {
	intent perception.Intent
	calls  int
}

func (s *clarifyStubTransducer) ParseIntentWithContext(_ context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
	s.calls++
	return s.intent, nil
}

// ParseIntent satisfies the perception.Transducer interface. The clarify lane
// under test goes through ParseIntentWithContext; this entry point returns
// the same canned intent so the stub is assignable anywhere a Transducer is
// required.
func (s *clarifyStubTransducer) ParseIntent(_ context.Context, _ string) (perception.Intent, error) {
	s.calls++
	return s.intent, nil
}

// ParseIntentWithGCD satisfies the perception.Transducer interface. The
// clarify lane under test never calls it; the stub returns the canned intent
// with no suggestions.
func (s *clarifyStubTransducer) ParseIntentWithGCD(_ context.Context, _ string, _ []perception.ConversationTurn, _ int) (perception.Intent, []string, error) {
	return s.intent, nil, nil
}

// ResolveFocus satisfies the perception.Transducer interface. Focus
// resolution is unused by the clarify lane; the stub returns the zero value.
func (s *clarifyStubTransducer) ResolveFocus(_ context.Context, _ string, _ []string) (perception.FocusResolution, error) {
	var zero perception.FocusResolution
	return zero, nil
}

// SetPromptAssembler satisfies the perception.Transducer interface. The stub
// has no prompt assembly to configure.
func (s *clarifyStubTransducer) SetPromptAssembler(_ perception.PromptAssembler) {}

// SetStrategicContext satisfies the perception.Transducer interface. The stub
// carries no strategic context.
func (s *clarifyStubTransducer) SetStrategicContext(_ string) {}

// TestProcessInputClarifyRouteRunsClarifierPath pins the /clarify lane end to
// end: a low-confidence, target-less mutation must derive RouteClarify from
// the kernel (route_decision(/clarify, /none)) and processInput must consume
// that decision via the clarification path (routeWantsClarify) instead of
// delegating to the coder shard.
func TestProcessInputClarifyRouteRunsClarifierPath(t *testing.T) {
	workspace := SetupLiveWorkspace(t)

	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("kernel creation failed: %v", err)
	}

	mockClient := NewMockLLMClient()
	mockClient.SetDefaultResponse("What would you like me to fix?")

	intent := perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "none",
		Constraint: "",
		Confidence: 0.4,
		Response:   "What would you like me to fix?",
	}
	tr := &clarifyStubTransducer{intent: intent}

	m := NewTestModel(WithSize(100, 50))
	m.kernel = kernel
	m.workspace = workspace
	m.client = mockClient
	m.transducer = tr
	m.virtualStore = core.NewVirtualStore(nil)

	const input = "fix it"

	// DECIDE half: seed user_intent exactly like production does, then prove
	// the kernel routes this turn to the clarify lane.
	assertRouteIntent(t, m, intent)
	route := m.decideRoute(input, intent, resolveShardTypeForIntent(intent))
	if route.Kind != RouteClarify {
		t.Fatalf("precondition: decideRoute(%q) = %v, want RouteClarify", input, route.Kind)
	}

	// ACT half: drive the full processInput pipeline and prove the turn takes
	// the clarifier path rather than the delegation path.
	msg := m.processInput(input)()
	if tr.calls != 1 {
		t.Errorf("transducer call count: got %d, want 1", tr.calls)
	}
	switch msg := msg.(type) {
	case assistantMsg:
		if msg.ShardResult != nil && msg.ShardResult.ShardType == "coder" {
			t.Fatalf("coder shard delegated for ambiguous %q (RouteClarify must not delegate)", input)
		}
		if msg.ClarifyUpdate == nil || !msg.ClarifyUpdate.LaunchClarifyPending {
			t.Fatalf("processInput(%q) with RouteClarify returned assistantMsg without pending clarification (ShardResult=%+v)", input, msg.ShardResult)
		}
	case clarificationMsg:
		if msg.PendingIntent == nil {
			t.Fatalf("processInput(%q) with RouteClarify returned clarificationMsg with no pending intent", input)
		}
	default:
		t.Fatalf("processInput(%q) with RouteClarify returned %T, want clarifier-lane message", input, msg)
	}
}
