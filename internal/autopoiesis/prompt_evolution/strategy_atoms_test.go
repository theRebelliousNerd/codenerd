package prompt_evolution

import (
	nerdconfig "codenerd/internal/config"
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

func TestStrategyIDFromAtomID(t *testing.T) {
	if got := StrategyIDFromAtomID("strategy/default-debugging-coder"); got != "default-debugging-coder" {
		t.Errorf("StrategyIDFromAtomID = %q, want the bare strategy id", got)
	}
	// A non-strategy atom must not be attributed to a strategy: crediting an
	// unrelated atom's outcome would corrupt the ranking SelectStrategies uses.
	for _, id := range []string{"methodology/debugging/core", "", "strategyfoo"} {
		if got := StrategyIDFromAtomID(id); got != "" {
			t.Errorf("StrategyIDFromAtomID(%q) = %q, want empty", id, got)
		}
	}
}

// TestStrategyAtom_IsUnscored: a learned strategy is never in the build-time
// importance ledger, so it carries no importance and no importance rule
// decides anything about it (prompt/importance.go). Its rank among strategies
// is SelectStrategies' measured success rate, not a hand weight.
func TestStrategyAtom_IsUnscored(t *testing.T) {
	atom := strategyAtom(&Strategy{ID: "s", ShardType: "/coder", Content: "reproduce first", SuccessRate: 1, SuccessCount: 100})
	if atom.Importance != prompt.ImportanceUnscored {
		t.Fatalf("strategy atom importance = %v, want unscored", atom.Importance)
	}
}

func TestStrategyAtom_IsScopedToItsShard(t *testing.T) {
	atom := strategyAtom(&Strategy{
		ID:        "default-debugging-coder",
		ShardType: "/coder",
		Content:   "reproduce first",
		Version:   3,
	})

	if atom.ID != "strategy/default-debugging-coder" {
		t.Errorf("atom ID = %q, want the strategy prefix", atom.ID)
	}
	if atom.Category != prompt.CategoryMethodology {
		t.Errorf("category = %q, want methodology", atom.Category)
	}
	// NormalizeSelectors strips the leading slash; the point is that exactly
	// one shard is selected, so a coder playbook never reaches a reviewer.
	if len(atom.ShardTypes) != 1 || atom.ShardTypes[0] != "coder" {
		t.Errorf("ShardTypes = %v, want exactly the owning shard", atom.ShardTypes)
	}
	if atom.Version != 3 {
		t.Errorf("version = %d, want the strategy's own version", atom.Version)
	}
}

func TestStrategyAtoms_NilStoreYieldsNothing(t *testing.T) {
	// Strategies can be disabled (EvolverConfig.EnableStrategies); that is a
	// configuration, not an error, and must not break compilation.
	p := NewStrategyAtomProvider(nil, nil)
	if got := p.StrategyAtoms(t.Context(), &prompt.CompilationContext{ShardType: "/coder"}); got != nil {
		t.Errorf("nil store returned %d atoms, want none", len(got))
	}
	var nilProvider *StrategyAtomProvider
	if got := nilProvider.StrategyAtoms(t.Context(), &prompt.CompilationContext{}); got != nil {
		t.Errorf("nil provider returned %d atoms", len(got))
	}
}

// TestStrategyAtoms_NoShardTypeSelectsNothing: strategies are stored per shard
// type, so without one there is no row to look up and guessing would serve a
// coder playbook to a reviewer.
func TestStrategyAtoms_NoShardTypeSelectsNothing(t *testing.T) {
	store := newTestStrategyStore(t)
	p := NewStrategyAtomProvider(store, NewProblemClassifier())

	if got := p.StrategyAtoms(t.Context(), &prompt.CompilationContext{SemanticQuery: "fix the crash"}); got != nil {
		t.Errorf("an unscoped context returned %d atoms, want none", len(got))
	}
}

// TestStrategyAtoms_ServesTheSeededPlaybook is the end of the gap this closes:
// GenerateDefaultStrategies has always seeded a per-problem-type playbook, and
// until now nothing ever put one in front of a model.
func TestStrategyAtoms_ServesTheSeededPlaybook(t *testing.T) {
	store := newTestStrategyStore(t)
	if err := store.GenerateDefaultStrategies(); err != nil {
		t.Fatalf("GenerateDefaultStrategies: %v", err)
	}

	p := NewStrategyAtomProvider(store, NewProblemClassifier())
	atoms := p.StrategyAtoms(t.Context(), &prompt.CompilationContext{
		ShardType:     "/coder",
		IntentVerb:    "/fix",
		SemanticQuery: "debug the nil pointer panic in the parser",
	})

	if len(atoms) == 0 {
		t.Fatal("no strategies served for a debugging task on /coder; the playbook is still write-only")
	}
	if len(atoms) > maxStrategyAtomsPerPrompt {
		t.Errorf("served %d strategies, capped at %d", len(atoms), maxStrategyAtomsPerPrompt)
	}
	for _, a := range atoms {
		if a.Content == "" {
			t.Error("served an empty strategy atom")
		}
	}
}

func TestStrategyQueryFor_UsesEveryAvailableSignal(t *testing.T) {
	// A delegated task often carries nothing but a verb, so the verb has to
	// reach the classifier or the whole task classifies as the default.
	got := strategyQueryFor(&prompt.CompilationContext{IntentVerb: "/test"})
	if got != "test" {
		t.Errorf("strategyQueryFor = %q, want the bare verb", got)
	}

	got = strategyQueryFor(&prompt.CompilationContext{
		SemanticQuery: "parser crash",
		IntentTarget:  "parser.go",
		IntentVerb:    "/fix",
	})
	for _, want := range []string{"parser crash", "parser.go", "fix"} {
		if !strings.Contains(got, want) {
			t.Errorf("strategyQueryFor = %q, missing %q", got, want)
		}
	}
}

func newTestStrategyStore(t *testing.T) *StrategyStore {
	t.Helper()
	store, err := NewStrategyStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStrategyStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestJudge_NilClientIsAnErrorNotAPanic: a boot with no LLM configured is a
// configuration state. Now that the cycle runs unattended on the maintenance
// schedule, a nil-deref there would take the process down mid-session.
func TestJudge_NilClientIsAnErrorNotAPanic(t *testing.T) {
	judge := NewTaskJudge(nil, "test-model", nil, nerdconfig.JITConfig{})

	verdict, err := judge.Evaluate(t.Context(), &ExecutionRecord{TaskID: "t1"})
	if err == nil {
		t.Fatal("a judge with no client must report an error, not evaluate")
	}
	if verdict != nil {
		t.Errorf("verdict = %+v, want nil", verdict)
	}

	// The batch path must survive it too: EvaluateBatch runs each record in
	// its own goroutine, where a panic is unrecoverable from the caller.
	verdicts, _ := judge.EvaluateBatch(t.Context(), []*ExecutionRecord{{TaskID: "t1"}, {TaskID: "t2"}})
	for i, v := range verdicts {
		if v != nil {
			t.Errorf("verdicts[%d] = %+v, want nil", i, v)
		}
	}
}
