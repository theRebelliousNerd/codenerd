package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
)

// TestProcessInputClarifyRouteRunsClarifierPath pins the /clarify lane of
// processInput (cmd/nerd/chat/process.go): when kernel routing arbitration
// derives route_decision(/clarify, /none) (policy/routing_arbitration.mg),
// processInput must take the clarifier path (Branch 1, gated by
// routeWantsClarify) and must not delegate.
//
// The intent is tuned so that NO OTHER clarify trigger fires, which is what
// makes this a pin on routeWantsClarify rather than on the heuristics:
//   - confidence 0.47 sits below the 50-point delegation gate (so the kernel
//     derives /clarify) but above the 0.45 fallback-clarification gate;
//   - Target and Constraint are both set, so the auto-clarifier's
//     needs-details check and the fallback missing-target check stay off;
//   - the kernel holds no clarification_question for the intent, so the
//     kernel-question branch (1.4.1) cannot fire either.
//
// The test asserts each of those preconditions explicitly: if a retune ever
// makes one fire, the test fails loudly instead of passing for the wrong
// reason. Removing `routeWantsClarify` from the Branch 1 condition then
// leaves no clarify trigger, and the final assertion fails.
type clarifyPinTransducer struct {
	mu     sync.Mutex
	intent perception.Intent
	calls  int
}

func (t *clarifyPinTransducer) ParseIntent(_ context.Context, _ string) (perception.Intent, error) {
	return t.ParseIntentWithContext(context.Background(), "", nil)
}

func (t *clarifyPinTransducer) ParseIntentWithContext(_ context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	return t.intent, nil
}

func (t *clarifyPinTransducer) ParseIntentWithGCD(_ context.Context, _ string, _ []perception.ConversationTurn, _ int) (perception.Intent, []string, error) {
	i, err := t.ParseIntentWithContext(context.Background(), "", nil)
	return i, nil, err
}

func (t *clarifyPinTransducer) ResolveFocus(_ context.Context, _ string, _ []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (t *clarifyPinTransducer) SetPromptAssembler(_ perception.PromptAssembler) {}

func (t *clarifyPinTransducer) SetStrategicContext(_ string) {}

// clarifyPinExecutor is a stub session.TaskExecutor that answers the
// requirements_interrogator call and records every execution so the test can
// prove the delegate shard was never invoked.
type clarifyPinExecutor struct {
	mu     sync.Mutex
	calls  []string
	result string
}

func (e *clarifyPinExecutor) Execute(_ context.Context, req session.TaskRequest) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, fmt.Sprintf("%v", req))
	return e.result, nil
}

func (e *clarifyPinExecutor) executed() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func TestProcessInputClarifyRouteRunsClarifierPath(t *testing.T) {
	const input = "fix it"
	intent := perception.Intent{
		Category:   "/mutation",
		Verb:       "/fix",
		Target:     "main.go",
		Constraint: "startup crash",
		Confidence: 0.47,
		Response:   "parsed",
	}

	transducer := &clarifyPinTransducer{intent: intent}
	executor := &clarifyPinExecutor{result: "Which startup crash: main.go init or config load?"}

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	m := NewTestModel(WithTransducer(transducer))
	m.kernel = k
	m.taskExecutor = executor

	// Precondition 1: the kernel must route this intent to the clarify lane.
	route := m.decideRoute(input, intent, resolveShardTypeForIntent(intent))
	if route.Kind != RouteClarify {
		t.Fatalf("precondition: decideRoute(%q) = %v, want RouteClarify", input, route.Kind)
	}

	// Preconditions 2-3: the heuristic clarify triggers must stay off, so the
	// only clarify source is the route-gated Branch 1.
	if m.shouldAutoClarify(&intent, input) {
		t.Fatal("pin invalid: shouldAutoClarify fires for this intent; Branch 1 is no longer the only clarify source")
	}
	if m.shouldClarifyIntent(&intent, input) {
		t.Fatal("pin invalid: shouldClarifyIntent fires for this intent; Branch 1 is no longer the only clarify source")
	}

	// Precondition 4: the kernel must hold no clarification question for this
	// intent once processInput seeds it, so branch 1.4.1 cannot fire either.
	m.clearStaleKernelFacts()
	_ = m.kernel.RetractFact(core.Fact{Predicate: "user_intent", Args: []any{"/current_intent"}})
	if err := m.kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []any{"/current_intent", intent.Category, intent.Verb, intent.Target, intent.Constraint},
	}); err != nil {
		t.Fatalf("precondition: assert user_intent: %v", err)
	}
	if question, _, ok := m.shouldClarifyFromKernel(&intent, input); ok {
		t.Fatalf("pin invalid: shouldClarifyFromKernel fires (%q); Branch 1 is no longer the only clarify source", question)
	}
	m.clearStaleKernelFacts()

	// Drive the lane.
	msg := m.processInput(input)()

	// The clarifier path (Branch 1) returns an assistantMsg carrying a pending
	// ClarifyUpdate and no ShardResult. Delegation would carry a ShardResult;
	// the fallback branches return clarificationMsg.
	am, ok := msg.(assistantMsg)
	if !ok {
		if pam, ok := msg.(*assistantMsg); ok {
			am = *pam
		} else {
			t.Fatalf("processInput(%q) with RouteClarify returned %T, want assistantMsg with ClarifyUpdate", input, msg)
		}
	}
	if am.ClarifyUpdate == nil || !am.ClarifyUpdate.LaunchClarifyPending {
		t.Fatalf("processInput(%q) with RouteClarify returned assistantMsg without pending ClarifyUpdate (ShardResult set: %v), want the clarifier path",
			input, am.ShardResult != nil)
	}
	if am.ShardResult != nil {
		t.Fatalf("processInput(%q) with RouteClarify delegated to %q, want the clarifier path", input, am.ShardResult.ShardType)
	}

	// The only execution must be the clarifier shard; the delegate shard
	// (coder) must never run.
	calls := executor.executed()
	if len(calls) != 1 {
		t.Fatalf("executor calls = %d, want exactly 1 (the clarifier shard)", len(calls))
	}
	if !strings.Contains(calls[0], "requirements_interrogator") {
		t.Fatalf("executor call = %q, want the requirements_interrogator shard", calls[0])
	}
}
