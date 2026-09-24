package prompt

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// queryVectorSearcher answers a vector search with the hits registered for a
// word the query contains: two turns asking different things get different
// flesh, as a live embedding index gives them.
type queryVectorSearcher struct{ hits map[string][]SearchResult }

func (v *queryVectorSearcher) Search(_ context.Context, query string, _ int) ([]SearchResult, error) {
	for word, hits := range v.hits {
		if strings.Contains(query, word) {
			return hits, nil
		}
	}
	return nil, nil
}

func (v *queryVectorSearcher) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// turnCompile is one coder turn on a Go file: the kernel carries this turn's
// specialist knowledge, the query selects this turn's vector hits.
func turnCompile(t *testing.T, c *JITPromptCompiler, k *core.RealKernel, verb, query, knowledge string) *CompilationResult {
	t.Helper()
	fact := core.Fact{Predicate: "specialist_knowledge", Args: []any{"coder", core.MangleAtom("/turn_topic"), knowledge}}
	if err := k.Assert(fact); err != nil {
		t.Fatalf("assert specialist_knowledge: %v", err)
	}
	t.Cleanup(func() { _ = k.RetractFact(fact) })
	cc := coderTurnContext(t, verb, "internal/context/working_set.go", "/go", kernelTargetNeeds(t, k, "/go"))
	cc.SemanticQuery = query
	result, err := c.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := k.RetractFact(fact); err != nil {
		t.Fatalf("retract specialist_knowledge: %v", err)
	}
	return result
}

// assertSkeletonIsSharedPrefix checks that every atom the tier function calls
// stable renders before the first per-turn atom, and that the two prompts are
// byte-identical up to that point.
func assertSkeletonIsSharedPrefix(t *testing.T, a, b *CompilationResult, stable func(*PromptAtom) bool) {
	t.Helper()
	volatileStart := len(a.Prompt)
	lastStableEnd := 0
	var volatileIDs []string
	for _, atom := range a.IncludedAtoms {
		body := strings.TrimSpace(contentForMode(atom, "standard"))
		if body == "" || strings.Contains(body, "{{") {
			continue
		}
		at := strings.Index(a.Prompt, body)
		if at < 0 {
			continue // rendered in another mode; located by its neighbours
		}
		if stable(atom) {
			lastStableEnd = max(lastStableEnd, at+len(body))
		} else {
			volatileIDs = append(volatileIDs, atom.ID)
			volatileStart = min(volatileStart, at)
		}
	}
	if len(volatileIDs) == 0 {
		t.Fatal("the turn compiled no per-turn atom; the test proves nothing")
	}
	if lastStableEnd > volatileStart {
		t.Errorf("a stable atom ends at byte %d, after the first per-turn atom starts at %d (per-turn: %v)",
			lastStableEnd, volatileStart, volatileIDs)
	}
	shared := commonPrefixLen(a.Prompt, b.Prompt)
	t.Logf("prompt %d bytes: stable atoms end at %d, first per-turn atom at %d %v, shared prefix %d (%.0f%%)",
		len(a.Prompt), lastStableEnd, volatileStart, volatileIDs, shared, 100*float64(shared)/float64(len(a.Prompt)))
	if shared < lastStableEnd {
		t.Errorf("two turns share %d prefix bytes; the stable atoms run to byte %d", shared, lastStableEnd)
	}
}

// Two turns of one persona compile prompts whose first per-turn byte comes
// after the whole skeleton, so a provider's prompt cache serves the skeleton on
// a new turn's first round. Measured 2026-09-22: two turns 94.5% identical
// shared only their first 53%, because a turn's vector hits and retrieved
// knowledge sat among the skeleton's categories.
func TestCompiledPromptKeepsTheSkeletonAsASharedPrefix(t *testing.T) {
	c, k := newEmbeddedCorpusCompiler(t)
	vs := &queryVectorSearcher{hits: map[string][]SearchResult{
		"race": {
			{AtomID: "identity/coder/quality_principles", Score: 0.95},
			{AtomID: "methodology/debugging/binary_search", Score: 0.94},
		},
		"leak": {
			{AtomID: "hallucination/coder/confident_comment", Score: 0.95},
			{AtomID: "methodology/debugging/rubber_duck", Score: 0.94},
		},
	}}
	if err := WithVectorSearcher(vs)(c); err != nil {
		t.Fatalf("WithVectorSearcher: %v", err)
	}

	sameVerbA := turnCompile(t, c, k, "/fix", "fix the race in the working set", "turn A: the working set is read under its lock")
	sameVerbB := turnCompile(t, c, k, "/fix", "fix the leak in the working set", "turn B: every observation carries a revision")
	for _, r := range []*CompilationResult{sameVerbA, sameVerbB} {
		if !strings.Contains(r.Prompt, "turn A:") && !strings.Contains(r.Prompt, "turn B:") {
			t.Fatal("the turn's kernel-injected knowledge is not in its prompt")
		}
	}

	// The skeleton: mandatory atoms the corpus authored, not this compile's
	// kernel facts or lookups.
	skeleton := func(a *PromptAtom) bool {
		return a.IsMandatory && !a.KernelInjected && !a.RetrievedContext
	}
	t.Run("same verb: the whole skeleton", func(t *testing.T) {
		assertSkeletonIsSharedPrefix(t, sameVerbA, sameVerbB, skeleton)
	})
	t.Run("another verb: the persona skeleton", func(t *testing.T) {
		otherVerb := turnCompile(t, c, k, "/refactor", "fix the leak in the working set", "turn B: every observation carries a revision")
		assertSkeletonIsSharedPrefix(t, sameVerbA, otherVerb, func(a *PromptAtom) bool {
			return skeleton(a) && len(a.IntentVerbs) == 0 && len(a.Languages) == 0 && len(a.Frameworks) == 0 &&
				len(a.WorldStates) == 0 && len(a.RequiresTools) == 0 && len(a.CampaignPhases) == 0 &&
				len(a.BuildLayers) == 0 && len(a.InitPhases) == 0 && len(a.NorthstarPhases) == 0 &&
				len(a.OuroborosStages) == 0
		})
	})
}

// The tier order itself, on synthetic atoms of one category: the persona
// skeleton, then the turn's skeleton, then vector hits, then kernel-injected
// context, then retrieved knowledge -- whatever the resolver's order.
func TestAssemblyTiersOrderStableToVolatile(t *testing.T) {
	mk := func(id string, order int, set func(*PromptAtom)) *OrderedAtom {
		a := &PromptAtom{ID: id, Category: CategoryMethodology, Content: "body-" + id}
		set(a)
		return &OrderedAtom{Atom: a, Order: order, RenderMode: "standard"}
	}
	atoms := []*OrderedAtom{
		mk("retrieved", 0, func(a *PromptAtom) { a.RetrievedContext = true }),
		mk("injected", 1, func(a *PromptAtom) { a.IsMandatory = true; a.KernelInjected = true }),
		mk("flesh", 2, func(a *PromptAtom) {}),
		mk("turn", 3, func(a *PromptAtom) { a.IsMandatory = true; a.IntentVerbs = []string{"/fix"} }),
		mk("persona", 4, func(a *PromptAtom) { a.IsMandatory = true; a.ShardTypes = []string{"/coder"} }),
	}
	out, err := NewFinalAssembler().Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	prev := -1
	for _, id := range []string{"persona", "turn", "flesh", "injected", "retrieved"} {
		at := strings.Index(out, "body-"+id)
		if at <= prev {
			t.Fatalf("%s at %d, not after the previous tier (%d):\n%s", id, at, prev, out)
		}
		prev = at
	}
}
