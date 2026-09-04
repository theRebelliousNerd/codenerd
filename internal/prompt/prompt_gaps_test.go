package prompt

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
)

// =============================================================================
// Config Factory Gap Tests
// =============================================================================

func TestConfigFactory_NilToolsAndPolicies(t *testing.T) {
	// GAP: Verify behavior when tools or policies in a returned ConfigAtom are nil.
	// Nil tools are fine (an empty allowlist fails closed at execution); nil
	// policies are rejected by Generate's validation because a turn with no
	// policy anchor would run unconstrained.
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/nil_tools": {
				Tools:    nil,
				Policies: []string{"policy/constitution.mg"},
			},
			"/nil_policies": {
				Tools:    []string{"t1"},
				Policies: nil,
			},
			"/both_nil": {
				Tools:    nil,
				Policies: nil,
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	cr := &CompilationResult{Prompt: "test"}

	tests := []struct {
		name    string
		intent  string
		wantErr bool
	}{
		{"nil_tools", "/nil_tools", false},
		{"nil_policies", "/nil_policies", true},
		{"both_nil", "/both_nil", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := factory.Generate(ctx, cr, tc.intent)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Generate(%q) succeeded with nil policies, want validation error", tc.intent)
				}
				return
			}
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}
			if cfg == nil {
				t.Fatal("Expected non-nil config")
			}
		})
	}
}

func TestConfigFactory_DeterministicOrdering(t *testing.T) {
	// GAP: Verify deterministic ordering of tools/policies when deduplicating from multiple intents.
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/a": {Tools: []string{"t3", "t1", "t2"}, Policies: []string{"policy/validation.mg", "policy/constitution.mg"}},
			"/b": {Tools: []string{"t2", "t4", "t1"}, Policies: []string{"policy/dreamer.mg", "policy/constitution.mg"}},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	cr := &CompilationResult{Prompt: "test"}

	// Run multiple times to verify determinism
	var firstTools []string
	for range 10 {
		cfg, err := factory.Generate(ctx, cr, "/a", "/b")
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if firstTools == nil {
			firstTools = cfg.AllowedTools
		} else {
			if len(cfg.AllowedTools) != len(firstTools) {
				t.Fatalf("Non-deterministic tool count: %d vs %d", len(cfg.AllowedTools), len(firstTools))
			}
			for j, tool := range cfg.AllowedTools {
				if tool != firstTools[j] {
					t.Fatalf("Non-deterministic tool ordering at index %d: %q vs %q", j, tool, firstTools[j])
				}
			}
		}
	}
}

func TestConfigFactory_SliceCopySafety(t *testing.T) {
	// GAP: Verify slice copy semantics to prevent aliasing cross-contamination.
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/coder": {
				Tools:    []string{"write_file", "read_file"},
				Policies: []string{"policy/coder_safety.mg"},
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	cr := &CompilationResult{Prompt: "test"}

	cfg1, err := factory.Generate(ctx, cr, "/coder")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Mutate cfg1's tools
	cfg1.AllowedTools[0] = "MUTATED"

	// Generate again — should NOT see the mutation
	cfg2, err := factory.Generate(ctx, cr, "/coder")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if cfg2.AllowedTools[0] == "MUTATED" {
		t.Error("Slice aliasing detected: mutation of cfg1 leaked into cfg2")
	}
}

func TestConfigFactory_RecursiveGeneration(t *testing.T) {
	// GAP: Verify stability under recursive ConfigFactory generations.
	provider := &MockConfigAtomProvider{
		atoms: map[string]ConfigAtom{
			"/coder": {
				Tools:    []string{"write_file", "read_file"},
				Policies: []string{"policy/coder_safety.mg"},
			},
		},
	}
	factory := NewConfigFactory(provider)
	ctx := context.Background()

	// Chain: generate config, use result as input for next
	result := &CompilationResult{Prompt: "seed"}
	for i := range 100 {
		cfg, err := factory.Generate(ctx, result, "/coder")
		if err != nil {
			t.Fatalf("Recursive generation failed at iteration %d: %v", i, err)
		}
		// Feed output back as input
		result = &CompilationResult{Prompt: cfg.IdentityPrompt}
	}
}

// =============================================================================
// Budget Manager Gap Tests
// =============================================================================

func TestTokenBudgetManager_NegativeTokenCounts(t *testing.T) {
	// GAP: Test negative TokenCount values to ensure they don't cause infinite loops or underflow.
	mgr := NewTokenBudgetManager()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategorySafety, TokenCount: -100, IsMandatory: true, Content: "safety"}, Score: 1.0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryIdentity, TokenCount: -1, Content: "identity"}, Score: 0.9},
		{Atom: &PromptAtom{ID: "c", Category: CategoryContext, TokenCount: 50, Content: "normal"}, Score: 0.5},
	}

	result, err := mgr.Fit(atoms, 10000)
	if err != nil {
		t.Fatalf("Fit failed: %v", err)
	}

	// Negative token counts should be clamped to 0 in Fit
	for _, oa := range result {
		if oa.Atom.TokenCount < 0 {
			t.Errorf("Atom %q still has negative TokenCount: %d", oa.Atom.ID, oa.Atom.TokenCount)
		}
	}
}

func TestTokenBudgetManager_MassiveTokenOverflow(t *testing.T) {
	// GAP: Test massive token counts (math.MaxInt) that could cause integer overflow.
	mgr := NewTokenBudgetManager()

	atoms := []*OrderedAtom{
		// Non-mandatory huge atoms — should be rejected by budget
		{Atom: &PromptAtom{ID: "huge1", Category: CategorySafety, TokenCount: math.MaxInt32, Content: "x"}, Score: 1.0},
		{Atom: &PromptAtom{ID: "huge2", Category: CategoryIdentity, TokenCount: math.MaxInt32, Content: "y"}, Score: 0.9},
		{Atom: &PromptAtom{ID: "normal", Category: CategoryContext, TokenCount: 100, Content: "z"}, Score: 0.5},
	}

	// Should not panic or overflow — budget is small so huge atoms get rejected
	result, err := mgr.Fit(atoms, 10000)
	if err != nil {
		t.Fatalf("Fit failed: %v", err)
	}

	// The huge non-mandatory atoms should NOT fit (they exceed allocation)
	totalTokens := 0
	for _, oa := range result {
		totalTokens += oa.Atom.TokenCount
	}
	if totalTokens > 10000 {
		t.Errorf("Total tokens %d exceeds budget 10000 — overflow or budget bypass", totalTokens)
	}
}

func TestTokenBudgetManager_ConcurrentAccess(t *testing.T) {
	// GAP: Test concurrent Fit alongside SetCategoryBudget/SetStrategy/SetReservedHeadroom.
	mgr := NewTokenBudgetManager()

	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "a", Category: CategorySafety, TokenCount: 100, IsMandatory: true, Content: "x"}, Score: 1.0},
		{Atom: &PromptAtom{ID: "b", Category: CategoryContext, TokenCount: 50, Content: "y"}, Score: 0.5},
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// Reader: call Fit repeatedly
	go func() {
		defer wg.Done()
		for range 100 {
			_, _ = mgr.Fit(atoms, 10000)
		}
	}()

	// Writer 1: mutate budgets
	go func() {
		defer wg.Done()
		for range 100 {
			mgr.SetCategoryBudget(CategoryBudget{
				Category:    CategoryContext,
				BasePercent: 0.2,
				MinTokens:   100,
				MaxTokens:   5000,
				Priority:    PriorityMedium,
			})
		}
	}()

	// Writer 2: mutate strategy/headroom
	go func() {
		defer wg.Done()
		for range 100 {
			mgr.SetStrategy(StrategyBalanced)
			mgr.SetReservedHeadroom(200)
		}
	}()

	// Must not panic (race detector will catch data races)
	wg.Wait()
}

// =============================================================================
// Resolver Gap Tests
// =============================================================================

func TestResolver_EmptyDependsOnStrings(t *testing.T) {
	// GAP A2: Verify Resolve with empty strings in DependsOn.
	resolver := NewDependencyResolver()

	atoms := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "a", DependsOn: []string{"", "b"}}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
	}

	ordered, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 2 {
		t.Errorf("Expected 2 atoms, got %d", len(ordered))
	}
}

func TestResolver_NilDependsOnSlice(t *testing.T) {
	// GAP A3: Verify Resolve with nil and empty DependsOn slices.
	resolver := NewDependencyResolver()

	atoms := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "a", DependsOn: nil}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "b", DependsOn: []string{}}, Combined: 0.6},
	}

	ordered, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 2 {
		t.Errorf("Expected 2 atoms, got %d", len(ordered))
	}
}

func TestResolver_SelfDependency(t *testing.T) {
	// GAP B1: Verify self-dependency handling (case-sensitive).
	resolver := NewDependencyResolver()

	atoms := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "foo", DependsOn: []string{"foo"}}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "Foo", DependsOn: []string{"foo"}}, Combined: 0.6},
	}

	// Should not panic or infinite loop
	ordered, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 2 {
		t.Errorf("Expected 2 atoms, got %d", len(ordered))
	}
}

func TestResolver_DuplicateDependencyEdges(t *testing.T) {
	// GAP B2: Verify topological sort handles duplicate edges (A depends on [B, B]).
	resolver := NewDependencyResolver()

	atoms := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "a", DependsOn: []string{"b", "b"}}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "b"}, Combined: 0.6},
	}

	ordered, err := resolver.Resolve(atoms)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(ordered) != 2 {
		t.Errorf("Expected 2 atoms, got %d", len(ordered))
	}

	// B must come before A
	positions := make(map[string]int)
	for _, oa := range ordered {
		positions[oa.Atom.ID] = oa.Order
	}
	if positions["b"] >= positions["a"] {
		t.Errorf("Duplicate edges caused incorrect ordering: b=%d, a=%d", positions["b"], positions["a"])
	}
}

func TestResolver_DeepChainNoStackOverflow(t *testing.T) {
	// GAP C1: Verify DetectCycles with deep chains (1000+ atoms).
	resolver := NewDependencyResolver()

	// Build chain: atom_0 <- atom_1 <- atom_2 <- ... <- atom_999
	atoms := make([]*PromptAtom, 1000)
	for i := range 1000 {
		id := fmt.Sprintf("atom_%d", i)
		var deps []string
		if i > 0 {
			deps = []string{fmt.Sprintf("atom_%d", i-1)}
		}
		atoms[i] = &PromptAtom{ID: id, DependsOn: deps}
	}

	// Should not stack overflow
	cycle := resolver.DetectCycles(atoms)
	if cycle != nil {
		t.Errorf("False cycle detected in linear chain: %v", cycle)
	}
}

func TestResolver_CyclePathContent(t *testing.T) {
	// GAP C2: Verify cycle path contains specific atom IDs.
	resolver := NewDependencyResolver()

	atoms := []*PromptAtom{
		{ID: "x", DependsOn: []string{"y"}},
		{ID: "y", DependsOn: []string{"z"}},
		{ID: "z", DependsOn: []string{"x"}},
	}

	cycle := resolver.DetectCycles(atoms)
	if cycle == nil {
		t.Fatal("Expected cycle to be detected")
	}

	// Verify cycle path contains all 3 nodes
	found := map[string]bool{}
	for _, id := range cycle {
		found[id] = true
	}
	for _, expected := range []string{"x", "y", "z"} {
		if !found[expected] {
			t.Errorf("Cycle path missing node %q: %v", expected, cycle)
		}
	}
}

func TestResolver_TieBreakerDeterminism(t *testing.T) {
	// GAP D2: Verify tie-breaker determinism with identical scores.
	resolver := NewDependencyResolver()

	atoms := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "alpha"}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "beta"}, Combined: 0.5},
		{Atom: &PromptAtom{ID: "gamma"}, Combined: 0.5},
	}

	var firstOrder []string
	for i := range 20 {
		ordered, err := resolver.Resolve(atoms)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
		ids := make([]string, len(ordered))
		for j, oa := range ordered {
			ids[j] = oa.Atom.ID
		}
		if firstOrder == nil {
			firstOrder = ids
		} else {
			for j, id := range ids {
				if id != firstOrder[j] {
					t.Fatalf("Non-deterministic ordering at run %d: got %v, want %v", i, ids, firstOrder)
				}
			}
		}
	}
}
