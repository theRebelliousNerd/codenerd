package atomjudge

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	"codenerd/internal/typesafe"
)

// The actual compiler and shipped Mangle rules, with deterministic vector and
// Jev answers. No provider is contacted; input estimates are not billed usage.
type budgetKernel struct{ k *core.RealKernel }

func (q *budgetKernel) ContentFingerprint() (string, error) { return q.k.ContentFingerprint() }

func (q *budgetKernel) Query(p string) ([]prompt.Fact, error) {
	rows, err := q.k.Query(p)
	out := make([]prompt.Fact, len(rows))
	for i, f := range rows {
		out[i] = prompt.Fact{Predicate: f.Predicate, Args: f.Args}
	}
	return out, err
}

func (q *budgetKernel) AssertBatch(rows []any) error {
	facts := make([]core.Fact, 0, len(rows))
	for _, raw := range rows {
		switch f := raw.(type) {
		case string:
			parsed, err := core.ParseFactString(strings.TrimSuffix(strings.TrimSpace(f), "."))
			if err != nil {
				return err
			}
			facts = append(facts, parsed)
		case prompt.Fact:
			facts = append(facts, core.Fact{Predicate: f.Predicate, Args: f.Args})
		default:
			return fmt.Errorf("unsupported fact %T", raw)
		}
	}
	return q.k.AssertBatch(facts)
}

func (q *budgetKernel) NewCompilationScope() (prompt.KernelCompilationScope, error) {
	return &budgetKernel{k: q.k.Clone()}, nil
}
func (q *budgetKernel) Close() error { return nil }

type budgetSearcher func(string) []prompt.SearchResult

func (f budgetSearcher) Search(_ context.Context, query string, _ int) ([]prompt.SearchResult, error) {
	return f(query), nil
}
func (budgetSearcher) EmbedQuery(context.Context, string) (store.QueryEmbedding, error) {
	return store.QueryEmbedding{}, nil
}

func budgetCompiler(t *testing.T, atoms []*prompt.PromptAtom, search budgetSearcher) *prompt.JITPromptCompiler {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	c, err := prompt.NewJITPromptCompiler(prompt.WithKernel(&budgetKernel{k: k}), prompt.WithEmbeddedCorpus(prompt.NewEmbeddedCorpus(atoms)), prompt.WithVectorSearcher(search))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func budgetContext(request string) *prompt.CompilationContext {
	cc := prompt.NewCompilationContext().WithSemanticQuery(request, 10).WithTokenBudget(20000, 1000)
	cc.ShardType, cc.IntentVerb, cc.IntentTarget, cc.Language = "/coder", "/fix", "broker.go", "/go"
	return cc
}

func includedIDs(r *prompt.CompilationResult) []string {
	var ids []string
	for _, a := range r.IncludedAtoms {
		ids = append(ids, a.ID)
	}
	slices.Sort(ids)
	return ids
}

func observedWork(f *fakeAsker) (atoms, batches, estimatedInput int) {
	for i, qs := range f.bodies {
		atoms += len(qs)
		body, _ := json.Marshal(struct {
			State     any                          `json:"state"`
			Questions map[string]typesafe.Question `json:"questions"`
		}{f.states[i], qs})
		estimatedInput += prompt.EstimateTokens(string(body))
	}
	return atoms, len(f.bodies), estimatedInput
}

func TestRelevanceBudget_ControlledSelectionComparison(t *testing.T) {
	atoms := []*prompt.PromptAtom{prompt.NewPromptAtom("identity/core", prompt.CategoryIdentity, "Preserve mandatory obligations.")}
	atoms[0].IsMandatory = true
	dep := prompt.NewPromptAtom("required/dependency", prompt.CategoryExemplar, "Required by the mandatory identity.")
	atoms[0].DependsOn = []string{dep.ID}
	atoms = append(atoms, dep)
	for i := range 12 {
		atoms = append(atoms, atom(fmt.Sprintf("flesh/%02d", i), "guidance"))
	}
	search := budgetSearcher(func(string) []prompt.SearchResult {
		var out []prompt.SearchResult
		for i := range 12 {
			score := .70 - float64(i)*.025
			if i < 2 {
				score = .99 - float64(i)*.01
			}
			out = append(out, prompt.SearchResult{AtomID: fmt.Sprintf("flesh/%02d", i), Score: score})
		}
		return out
	})
	var baseline, judged []string
	for _, tc := range []struct {
		name               string
		off                bool
		maxAtoms, maxInput int
		wantCalls          int
	}{
		{"vector-only", true, 0, 0, 0},
		{"generous-limits-control", false, 100000, 100000000, 1},
		{"bounded-within-budget", false, 6, 10000, 1},
		{"atom-budget-fallback", false, 5, 10000, 0},
		{"input-budget-fallback", false, 6, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAsker{answer: func(_ string, q typesafe.Question) string {
				g := q.(typesafe.Noul).Instructions.(map[string]any)["guidance"].(guidance)
				if g["id"] == "identity/core" {
					t.Error("mandatory atom was judged")
				}
				if g["id"] == "flesh/00" || g["id"] == "flesh/02" {
					return `{"type":"noul","noul":0.95}`
				}
				return `{"type":"noul","noul":0.05}`
			}}
			c := budgetCompiler(t, atoms, search)
			if !tc.off {
				c.SetAtomRelevance(RelevanceFor(f, &config.TypeSafeConfig{AtomRelevancePool: 6, AtomRelevanceMaxAtoms: tc.maxAtoms, AtomRelevanceMaxInputTokens: tc.maxInput, AtomRelevanceMaxConcurrentBatches: 1}))
			}
			result, err := c.Compile(context.Background(), budgetContext("fix broker panic"))
			if err != nil {
				t.Fatal(err)
			}
			ids := includedIDs(result)
			if !slices.Contains(ids, "identity/core") || !slices.Contains(ids, dep.ID) {
				t.Fatalf("mandatory/dependency lost: %v", ids)
			}
			if result.TotalTokens > 19000 {
				t.Fatalf("prompt exceeds final budget: %d", result.TotalTokens)
			}
			n, batches, estimate := observedWork(f)
			if batches != tc.wantCalls {
				t.Fatalf("calls %d, want %d", batches, tc.wantCalls)
			}
			switch tc.name {
			case "vector-only":
				baseline = ids
				if !slices.Contains(ids, "flesh/01") || slices.Contains(ids, "flesh/02") {
					t.Fatalf("fixture needs slot noise and pool rescue: %v", ids)
				}
			case "generous-limits-control":
				judged = ids
				if slices.Contains(ids, "flesh/01") || !slices.Contains(ids, "flesh/02") {
					t.Fatalf("Jev admission incorrect: %v", ids)
				}
			case "bounded-within-budget":
				if !slices.Equal(ids, judged) {
					t.Fatalf("bounded %v != unconstrained %v", ids, judged)
				}
			default:
				if !slices.Equal(ids, baseline) {
					t.Fatalf("fallback %v != vector-only %v", ids, baseline)
				}
			}
			t.Logf("arm=%s observed_asked_atoms=%d observed_Ask_calls=%d estimated_input_tokens=%d final_prompt_estimated_tokens=%d selected=%v", tc.name, n, batches, estimate, result.TotalTokens, ids)
		})
	}
}

func TestRelevanceBudget_CarryAndSlotsShareBudgetWithoutPublishingFallback(t *testing.T) {
	identity := prompt.NewPromptAtom("identity/core", prompt.CategoryIdentity, "Keep the obligations.")
	identity.IsMandatory = true
	atoms := []*prompt.PromptAtom{identity, atom("a", "guidance a"), atom("b", "guidance b"), atom("c", "guidance c")}
	search := budgetSearcher(func(query string) []prompt.SearchResult { return []prompt.SearchResult{{AtomID: query, Score: .9}} })
	c := budgetCompiler(t, atoms, search)
	f := yesAsker()
	sec := &config.TypeSafeConfig{AtomRelevancePool: 1, AtomRelevanceMaxAtoms: 1}
	c.SetAtomRelevance(RelevanceFor(f, sec))
	compile := func(request string) []string {
		t.Helper()
		r, err := c.Compile(context.Background(), budgetContext(request))
		if err != nil {
			t.Fatal(err)
		}
		return includedIDs(r)
	}
	if ids := compile("a"); !slices.Contains(ids, "a") {
		t.Fatalf("first prompt %v", ids)
	}
	if ids := compile("b"); !slices.Contains(ids, "b") || slices.Contains(ids, "a") {
		t.Fatalf("fallback must be vector-only: %v", ids)
	}
	if len(f.bodies) != 1 {
		t.Fatalf("carry plus slot escaped combined atom budget: %d calls", len(f.bodies))
	}
	// No judgment must not publish b as carry. An unsettled third judgment
	// therefore holds the last successfully judged/fitted a, not b.
	f.answer = func(string, typesafe.Question) string { return `{"type":"noul","noul":0.5}` }
	sec.AtomRelevanceMaxAtoms = 2
	c.SetAtomRelevance(RelevanceFor(f, sec))
	if ids := compile("c"); !slices.Contains(ids, "a") || !slices.Contains(ids, "c") || slices.Contains(ids, "b") {
		t.Fatalf("carry advanced on unavailable judgment: %v", ids)
	}
}

func TestRelevanceBudget_CachedAnswersDoNotSpendWorkBudget(t *testing.T) {
	atoms := []*prompt.PromptAtom{prompt.NewPromptAtom("identity/core", prompt.CategoryIdentity, "Keep obligations."), atom("a", "guidance a"), atom("b", "guidance b"), atom("c", "guidance c")}
	atoms[0].IsMandatory = true
	includeC := false
	search := budgetSearcher(func(string) []prompt.SearchResult {
		out := []prompt.SearchResult{{AtomID: "a", Score: .9}, {AtomID: "b", Score: .89}}
		if includeC {
			out = append(out, prompt.SearchResult{AtomID: "c", Score: .88})
		}
		return out
	})
	f := &fakeAsker{answer: func(_ string, q typesafe.Question) string {
		g := q.(typesafe.Noul).Instructions.(map[string]any)["guidance"].(guidance)
		if g["id"] == "a" {
			return `{"type":"noul","noul":0.05}`
		}
		return `{"type":"noul","noul":0.95}`
	}}
	c := budgetCompiler(t, atoms, search)
	sec := &config.TypeSafeConfig{AtomRelevancePool: 2, AtomRelevanceMaxAtoms: 2}
	c.SetAtomRelevance(RelevanceFor(f, sec))
	first, err := c.Compile(context.Background(), budgetContext("fix"))
	if err != nil {
		t.Fatal(err)
	}
	sec.AtomRelevanceMaxAtoms, sec.AtomRelevanceMaxInputTokens = 1, 1
	c.SetAtomRelevance(RelevanceFor(f, sec)) // Invalidates compiled prompt cache.
	second, err := c.Compile(context.Background(), budgetContext("fix"))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.bodies) != 1 || !slices.Equal(includedIDs(first), includedIDs(second)) {
		t.Fatalf("cached work re-asked or selection changed: calls %d, first %v second %v", len(f.bodies), includedIDs(first), includedIDs(second))
	}
	// A new uncached candidate exceeds the remaining input budget. Previously
	// cached refusals must also be withheld: partial evidence would drop a.
	includeC = true
	sec.AtomRelevancePool = 3
	c.SetAtomRelevance(RelevanceFor(f, sec))
	third, err := c.Compile(context.Background(), budgetContext("fix"))
	if err != nil {
		t.Fatal(err)
	}
	ids := includedIDs(third)
	if len(f.bodies) != 1 || !slices.Contains(ids, "a") || !slices.Contains(ids, "b") || !slices.Contains(ids, "c") {
		t.Fatalf("budget failure used partial cached evidence: calls %d, selected %v", len(f.bodies), ids)
	}
}

// Deadline includes time spent queued behind a worker.
func TestRelevanceBudget_DeadlineStopsQueuedWork(t *testing.T) {
	f := &gatedRelevanceAsker{started: make(chan struct{}, 3), release: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	got, err := (RelevanceJudge{Asker: f, MaxAtoms: 100, MaxConcurrentBatches: 1}).JudgeAtomRelevance(ctx, prompt.AtomRelevanceRequest{Request: "r"}, relevanceAtoms(81))
	if got != nil || err != context.DeadlineExceeded || f.calls.Load() != 1 {
		t.Fatalf("deadline returned %v, %v, calls %d", got, err, f.calls.Load())
	}
}
