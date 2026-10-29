package session

import (
	"context"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// TestProcess_ReachesTheLearningSinks is the test the unit tests cannot be: it
// drives a whole turn through Process and asserts the record comes out the
// other end.
//
// The unit tests above prove each link — recordTurn delivers to a sink, the
// boot installs one, CloneForTask inherits it. None of them would catch the
// failure that actually matters, which is persistTurn never calling recordTurn
// at all. Every link intact and the chain broken is precisely the shape this
// whole PR is fixing, so it would be a poor joke to reproduce it here.
func TestProcess_ReachesTheLearningSinks(t *testing.T) {
	turnSink := &recordingSink{}
	ctxSink := &contextSink{}
	e := newLearningTestExecutor(t, "I could not find that file.")
	e.SetTurnRecorder(turnSink)
	e.SetContextFeedbackRecorder(ctxSink)

	if _, err := e.Process(context.Background(), "explain the parser"); err != nil {
		t.Fatalf("Process: %v", err)
	}

	records := turnSink.all()
	if len(records) != 1 {
		t.Fatalf("a completed turn produced %d records, want 1 — persistTurn is not reporting turns", len(records))
	}

	rec := records[0]
	if rec.IntentVerb != "/explain" {
		t.Errorf("IntentVerb = %q, want the turn's verb — it is the key failures group under", rec.IntentVerb)
	}
	if rec.Task != "explain the parser" {
		t.Errorf("Task = %q, want the request the turn was given", rec.Task)
	}
	if rec.Response != "I could not find that file." {
		t.Errorf("Response = %q, want the model's answer — the judge grades from it", rec.Response)
	}
	if rec.Outcome == "" {
		t.Error("Outcome is empty; without the kernel verdict the record cannot be graded")
	}
	if len(rec.AtomIDs) != 1 || rec.AtomIDs[0] != "methodology/explain" {
		t.Errorf("AtomIDs = %v, want the compiled atoms — they are the credit-assignment handle", rec.AtomIDs)
	}
}

// TestProcess_ContextRatingReachesItsSink covers the other sink over the same
// path: the model's rating rides in on the Piggyback envelope mid-turn and has
// to survive until persistTurn, which is where the turn number and manifest
// live.
func TestProcess_ContextRatingReachesItsSink(t *testing.T) {
	ctxSink := &contextSink{}
	// The real envelope keys, not the Go field names: the surface is
	// "surface_response" and the packet is "control_packet".
	e := newLearningTestExecutor(t, `{"control_packet":{"context_feedback":{"overall_usefulness":0.9,"helpful_facts":["file_topology"],"noise_facts":["dom_node"],"missing_context":"needed the dependency graph"}},"surface_response":"Here is the parser."}`)
	e.SetContextFeedbackRecorder(ctxSink)

	res, err := e.Process(context.Background(), "explain the parser")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	// The control packet must be stripped before the text reaches the user.
	if res.Response != "Here is the parser." {
		t.Errorf("Response = %q, want the surface text only", res.Response)
	}

	ratings := ctxSink.all()
	if len(ratings) != 1 {
		t.Fatalf("the model rated its context and %d ratings were kept, want 1", len(ratings))
	}
	r := ratings[0]
	if r.OverallUsefulness != 0.9 {
		t.Errorf("OverallUsefulness = %v, want 0.9", r.OverallUsefulness)
	}
	if len(r.HelpfulPredicates) != 1 || r.HelpfulPredicates[0] != "file_topology" {
		t.Errorf("HelpfulPredicates = %v", r.HelpfulPredicates)
	}
	if len(r.NoisePredicates) != 1 || r.NoisePredicates[0] != "dom_node" {
		t.Errorf("NoisePredicates = %v", r.NoisePredicates)
	}
	if r.MissingContext != "needed the dependency graph" {
		t.Errorf("MissingContext = %q", r.MissingContext)
	}
	if r.IntentVerb != "/explain" {
		t.Errorf("IntentVerb = %q; a rating is scoped to the verb it was given under", r.IntentVerb)
	}
}

// TestProcess_UnratedTurnRecordsNoContextFeedback: most turns carry no rating,
// and inventing one would be worse than having none.
func TestProcess_UnratedTurnRecordsNoContextFeedback(t *testing.T) {
	ctxSink := &contextSink{}
	e := newLearningTestExecutor(t, "plain prose, no control packet")
	e.SetContextFeedbackRecorder(ctxSink)

	if _, err := e.Process(context.Background(), "explain the parser"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := ctxSink.all(); len(got) != 0 {
		t.Fatalf("an unrated turn produced %d ratings: %+v", len(got), got)
	}
}

// newLearningTestExecutor builds an executor whose LLM answers with the given
// text and whose compiler reports one known atom, so a turn can be driven end
// to end without a network.
func newLearningTestExecutor(t *testing.T, response string) *Executor {
	t.Helper()

	// Both completion paths answer identically. generateResponse picks
	// CompleteWithSystem when no tools are configured and CompleteWithTools
	// when some are, and a harness that only stubs one silently returns an
	// empty response through the other.
	client := &MockLLMClient{
		CompleteWithSystemFunc: func(_ context.Context, _, _ string) (string, error) {
			return response, nil
		},
		CompleteWithToolsFunc: func(_ context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: response}, nil
		},
	}
	transducer := &MockTransducer{
		ParseIntentWithContextFunc: func(_ context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/explain", Category: "/query", Target: "parser"}, nil
		},
	}
	compiler := &MockJITCompiler{
		CompileFunc: func(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{
				Prompt:        "you are a test agent",
				IncludedAtoms: []*prompt.PromptAtom{{ID: "methodology/explain"}},
				Manifest:      &prompt.PromptManifest{ContextHash: "hash-under-test"},
			}, nil
		},
	}
	configFactory := &MockConfigFactory{
		GenerateFunc: func(_ context.Context, _ *prompt.CompilationResult, _ ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{}, nil
		},
	}

	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, client, compiler, configFactory, transducer)
	e.SetSessionID("learning-e2e")
	return e
}
