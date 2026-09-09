package prompt

import (
	"context"
	"strings"
	"testing"
)

// mkAtom builds an OrderedAtom with a content body of roughly the requested
// token count (EstimateTokens is len/4).
func mkAtom(id string, cat AtomCategory, tokens int, mandatory bool, score float64) *OrderedAtom {
	a := NewPromptAtom(id, cat, strings.Repeat("x", tokens*4))
	a.IsMandatory = mandatory
	return &OrderedAtom{Atom: a, Score: score, RenderMode: "standard"}
}

// Before this pass, Fit charged per-atom token counts and the assembler then
// expanded {{...}} placeholders and joined sections — so the emitted prompt
// could exceed the budget Fit had just certified. The compiler DETECTED that
// ("budget breach" in logCompilationStats) and shipped the prompt anyway.
// ShedToFit is the enforcement; these cases are the proof.
func TestShedToFit(t *testing.T) {
	mgr := NewTokenBudgetManager()

	tests := []struct {
		name       string
		atoms      []*OrderedAtom
		budget     int
		promptSize int // tokens the "assembled" prompt reports before shedding
		wantKept   []string
		wantDrop   []string
	}{
		{
			name: "in budget: nothing is shed",
			atoms: []*OrderedAtom{
				mkAtom("safety", CategorySafety, 100, true, 90),
				mkAtom("exemplar", CategoryExemplar, 100, false, 10),
			},
			budget:     10000,
			promptSize: 200,
			wantKept:   []string{"safety", "exemplar"},
		},
		{
			name: "over budget: lowest priority category sheds first",
			atoms: []*OrderedAtom{
				mkAtom("safety", CategorySafety, 500, true, 90),
				mkAtom("protocol", CategoryProtocol, 500, false, 80),
				mkAtom("exemplar", CategoryExemplar, 5000, false, 70),
			},
			budget:     1500,
			promptSize: 6000,
			wantKept:   []string{"safety", "protocol"},
			wantDrop:   []string{"exemplar"},
		},
		{
			name: "over budget: lowest score sheds first within a priority",
			atoms: []*OrderedAtom{
				mkAtom("safety", CategorySafety, 100, true, 90),
				mkAtom("keep", CategoryExemplar, 2000, false, 99),
				mkAtom("drop", CategoryExemplar, 2000, false, 1),
			},
			budget:     2500,
			promptSize: 4100,
			wantKept:   []string{"safety", "keep"},
			wantDrop:   []string{"drop"},
		},
		{
			name: "mandatory atoms are never shed, even when they alone overflow",
			atoms: []*OrderedAtom{
				mkAtom("safety", CategorySafety, 5000, true, 90),
				mkAtom("identity", CategoryIdentity, 5000, true, 90),
			},
			budget:     1000,
			promptSize: 10000,
			wantKept:   []string{"safety", "identity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The fake assembler models the real one: concatenate the atom
			// bodies, then charge a fixed expansion surcharge that Fit could
			// not have seen. On the first call it reports promptSize.
			calls := 0
			assemble := func(atoms []*OrderedAtom) (string, error) {
				calls++
				var b strings.Builder
				for _, oa := range atoms {
					b.WriteString(oa.Atom.Content)
				}
				return b.String(), nil
			}

			initial := strings.Repeat("x", tt.promptSize*4)
			kept, got, used := mgr.ShedToFit(tt.atoms, initial, tt.budget, assemble)

			keptIDs := map[string]bool{}
			for _, oa := range kept {
				keptIDs[oa.Atom.ID] = true
			}
			for _, id := range tt.wantKept {
				if !keptIDs[id] {
					t.Errorf("atom %q was shed but must be kept", id)
				}
			}
			for _, id := range tt.wantDrop {
				if keptIDs[id] {
					t.Errorf("atom %q survived but must be shed", id)
				}
			}
			if used != EstimateTokens(got) {
				t.Errorf("reported usage %d disagrees with the returned prompt (%d)", used, EstimateTokens(got))
			}
			if len(tt.wantDrop) > 0 && used > tt.budget {
				t.Errorf("still over budget after shedding: %d > %d", used, tt.budget)
			}
			if calls > maxShedPasses {
				t.Errorf("re-assembled %d times, want <= %d", calls, maxShedPasses)
			}
		})
	}
}

// A shed pass that fails to re-assemble must not degrade into an outage: a
// budget overshoot is a degradation, a failed compile is a broken turn.
func TestShedToFit_AssemblyFailureKeepsOriginal(t *testing.T) {
	mgr := NewTokenBudgetManager()
	atoms := []*OrderedAtom{
		mkAtom("safety", CategorySafety, 100, true, 90),
		mkAtom("exemplar", CategoryExemplar, 5000, false, 10),
	}
	original := strings.Repeat("x", 20000)
	kept, got, _ := mgr.ShedToFit(atoms, original, 100, func([]*OrderedAtom) (string, error) {
		return "", context.Canceled
	})
	if got != original {
		t.Error("assembly failure must return the original prompt unchanged")
	}
	if len(kept) != len(atoms) {
		t.Errorf("assembly failure must keep every atom, got %d of %d", len(kept), len(atoms))
	}
}

// The end-to-end contract: an adversarial atom set compiled at a realistic
// shard budget must produce a prompt inside that budget, and must say so when
// it had to cut.
func TestEnforceAssembledBudget_AdversarialInput(t *testing.T) {
	c, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	defer func() { _ = c.Close() }()

	const budget = 4096 // the budget subagents really compile at

	tests := []struct {
		name      string
		atoms     []*OrderedAtom
		wantUnder bool
		wantMark  bool
	}{
		{
			name: "optional bloat is shed back under budget",
			atoms: []*OrderedAtom{
				mkAtom("id", CategoryIdentity, 200, true, 90),
				mkAtom("bloat1", CategoryExemplar, 20000, false, 10),
				mkAtom("bloat2", CategoryKnowledge, 20000, false, 20),
			},
			wantUnder: true,
		},
		{
			name: "a mandatory skeleton that alone overflows is cut visibly",
			atoms: []*OrderedAtom{
				mkAtom("id", CategoryIdentity, 40000, true, 90),
			},
			wantUnder: true,
			wantMark:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := NewCompilationContextWithBudget(budget)
			assembled, err := c.assembler.Assemble(tt.atoms, cc)
			if err != nil {
				t.Fatalf("Assemble: %v", err)
			}
			if EstimateTokens(assembled) <= budget {
				t.Fatalf("test setup is not adversarial: %d tokens already fits %d", EstimateTokens(assembled), budget)
			}

			_, got := c.enforceAssembledBudget(tt.atoms, cc, assembled, budget)
			used := EstimateTokens(got)

			if tt.wantUnder && used > budget {
				t.Errorf("prompt is still over budget: %d > %d", used, budget)
			}
			if tt.wantMark && !strings.Contains(got, "truncated") {
				t.Error("a truncated prompt must carry a visible marker; silent truncation makes the model reason on a lie")
			}
		})
	}
}

// calculateAllocations clamps each category to its MinTokens floor without
// consulting what is left, so on a small budget the per-category allocations
// sum to more than the budget. Pass 2 always checked the global budget; pass 1
// did not, and subagents compile at exactly this size.
func TestFit_SmallBudgetDoesNotOverAllocate(t *testing.T) {
	mgr := NewTokenBudgetManager()

	budgets := []int{1024, 2048, 4096, 8192}
	for _, budget := range budgets {
		t.Run(strings.Join([]string{"budget", string(rune('0' + budget/1024))}, "_"), func(t *testing.T) {
			// One sizeable atom in every category with a MinTokens floor, so
			// every floor is claimed.
			var atoms []*OrderedAtom
			for _, cat := range []AtomCategory{
				CategorySafety, CategoryIdentity, CategoryProtocol, CategoryMethodology,
				CategoryCapability, CategoryHallucination, CategoryLanguage, CategoryFramework,
				CategoryDomain, CategoryContext, CategoryKnowledge, CategoryExemplar,
			} {
				atoms = append(atoms, mkAtom("a/"+string(cat), cat, 2000, false, 50))
			}

			fitted, err := mgr.Fit(atoms, budget)
			if err != nil {
				t.Fatalf("Fit(%d): %v", budget, err)
			}
			total := 0
			for _, oa := range fitted {
				total += tokenCountForMode(oa.Atom, oa.RenderMode)
			}
			if total > budget {
				t.Errorf("Fit emitted %d tokens against a %d budget", total, budget)
			}
		})
	}
}

// End-to-end through Compile, not through the enforcement helper directly.
//
// This is the contract the whole pass exists to establish: whatever the corpus
// and the kernel hand the compiler, the string it returns fits the budget it
// was given. Before this pass the compiler measured the overshoot, logged
// "budget breach", and returned the over-budget prompt anyway.
func TestCompile_ResultAlwaysFitsBudget(t *testing.T) {
	tests := []struct {
		name   string
		budget int
		atoms  []*PromptAtom
	}{
		{
			name:   "mandatory atoms far larger than a subagent budget",
			budget: 4096,
			atoms: []*PromptAtom{
				bigAtom("id/huge", CategoryIdentity, 30000, true),
				bigAtom("safety/huge", CategorySafety, 30000, true),
			},
		},
		{
			name:   "a corpus of oversized optional atoms",
			budget: 8192,
			atoms: []*PromptAtom{
				bigAtom("id/small", CategoryIdentity, 100, true),
				bigAtom("lang/huge", CategoryLanguage, 40000, false),
				bigAtom("exemplar/huge", CategoryExemplar, 40000, false),
				bigAtom("knowledge/huge", CategoryKnowledge, 40000, false),
			},
		},
		{
			name:   "a tiny budget with a mixed corpus",
			budget: 1024,
			atoms: []*PromptAtom{
				bigAtom("id/small", CategoryIdentity, 200, true),
				bigAtom("proto/mid", CategoryProtocol, 5000, false),
				bigAtom("exemplar/mid", CategoryExemplar, 5000, false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, a := range tt.atoms {
				a.TokenCount = EstimateTokens(a.Content)
				a.ContentHash = HashContent(a.Content)
			}

			compiler, err := NewJITPromptCompiler(
				WithEmbeddedCorpus(NewEmbeddedCorpus(tt.atoms)),
				WithKernel(&mockKernel{facts: atomsToFacts(tt.atoms)}),
			)
			if err != nil {
				t.Fatalf("NewJITPromptCompiler: %v", err)
			}
			defer func() { _ = compiler.Close() }()

			// Reserve an eighth for the completion, as the runtime does. The
			// budget the compiler enforces is AvailableTokens, not TokenBudget.
			cc := NewCompilationContext().WithTokenBudget(tt.budget, tt.budget/8)
			effective := cc.AvailableTokens()

			result, err := compiler.Compile(context.Background(), cc)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}

			used := EstimateTokens(result.Prompt)
			if used > effective {
				t.Fatalf("Compile returned %d tokens against a %d-token budget; "+
					"the compiler must enforce the budget it reports, not merely measure it",
					used, effective)
			}
			// A prompt that had to be cut must say so — silent truncation makes
			// the model reason on a lie.
			if used > 0 && result.Prompt != "" {
				total := 0
				for _, a := range tt.atoms {
					total += a.TokenCount
				}
				if total > effective && !IsClamped(result.Prompt) &&
					!strings.Contains(result.Prompt, "truncated") &&
					result.AtomsIncluded == len(tt.atoms) {
					t.Error("every atom survived a budget it cannot fit, with no truncation marker")
				}
			}
		})
	}
}

func bigAtom(id string, cat AtomCategory, tokens int, mandatory bool) *PromptAtom {
	a := NewPromptAtom(id, cat, strings.Repeat("w", tokens*4))
	a.IsMandatory = mandatory
	return a
}

// The budget contract promises whole low-priority sections are dropped before
// a high-priority one is shredded. If the per-atom estimate is bad enough that
// four shed passes do not converge, the loop must still have drained every
// optional atom before the caller falls through to whole-prompt truncation —
// otherwise truncation would cut an identity atom to keep an exemplar.
func TestShedToFit_FinalPassDrainsAllOptionalAtoms(t *testing.T) {
	mgr := NewTokenBudgetManager()

	atoms := []*OrderedAtom{
		mkAtom("safety", CategorySafety, 100, true, 90),
	}
	for i := 0; i < 40; i++ {
		atoms = append(atoms, mkAtom("ex"+string(rune('a'+i%26))+string(rune('0'+i/26)),
			CategoryExemplar, 50, false, float64(i)))
	}

	// An assembler whose output never shrinks: the shed loop can never
	// converge, so it must exhaust its passes and drain everything optional.
	stuck := func([]*OrderedAtom) (string, error) {
		return strings.Repeat("x", 40000), nil
	}

	kept, _, _ := mgr.ShedToFit(atoms, strings.Repeat("x", 40000), 1000, stuck)

	for _, oa := range kept {
		if !oa.Atom.IsMandatory {
			t.Fatalf("optional atom %q survived a non-converging shed; "+
				"whole-atom eviction must be exhausted before truncation", oa.Atom.ID)
		}
	}
	if len(kept) != 1 {
		t.Errorf("kept %d atoms, want only the mandatory one", len(kept))
	}
}
