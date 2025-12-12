package prompt

import (
	"fmt"
	"sort"
	"sync"

	"codenerd/internal/logging"
)

// =============================================================================
// EMBEDDED BASELINE PROMPT ASSEMBLY
// =============================================================================
// When JIT is unavailable or disabled, we still want a single source of truth
// for system prompts. The legacy Articulation PromptAssembler uses this helper
// to assemble a deterministic baseline prompt from embedded mandatory atoms.
//
// This keeps Piggyback Protocol, shard identities, and safety/methodology rules
// in YAML atoms rather than hard-coded Go strings.

var (
	embeddedCorpusOnce   sync.Once
	embeddedCorpusCached *EmbeddedCorpus
	embeddedCorpusErr    error
)

func getEmbeddedCorpusCached() (*EmbeddedCorpus, error) {
	embeddedCorpusOnce.Do(func() {
		embeddedCorpusCached, embeddedCorpusErr = LoadEmbeddedCorpus()
	})
	return embeddedCorpusCached, embeddedCorpusErr
}

// AssembleEmbeddedBaselinePrompt assembles a baseline system prompt consisting of
// embedded mandatory atoms that match the given context. It performs dependency
// ordering and budget fitting, but does not use Mangle or vector search.
func AssembleEmbeddedBaselinePrompt(cc *CompilationContext) (string, error) {
	corpus, err := getEmbeddedCorpusCached()
	if err != nil {
		return "", err
	}

	candidates := corpus.All()
	if len(candidates) == 0 {
		return "", nil
	}

	// Select mandatory atoms matching context.
	var mandatory []*PromptAtom
	for _, atom := range candidates {
		if !atom.IsMandatory {
			continue
		}
		if atom.MatchesContext(cc) {
			mandatory = append(mandatory, atom)
		}
	}

	if len(mandatory) == 0 {
		return "", nil
	}

	// Convert to scored atoms for dependency ordering/budget fitting.
	scored := make([]*ScoredAtom, 0, len(mandatory))
	for _, atom := range mandatory {
		scored = append(scored, &ScoredAtom{
			Atom:            atom,
			LogicScore:      1.0,
			VectorScore:     0.0,
			Combined:        float64(atom.Priority) / 100.0,
			SelectionReason: "baseline:mandatory_embedded",
			Source:          "skeleton",
		})
	}

	// Stable sort by priority for deterministic ordering before dependency resolve.
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Atom.Priority > scored[j].Atom.Priority
	})

	resolver := NewDependencyResolver()
	ordered, err := resolver.Resolve(scored)
	if err != nil {
		return "", fmt.Errorf("baseline dependency resolve failed: %w", err)
	}

	budgetMgr := NewTokenBudgetManager()
	budget := 0
	if cc != nil {
		budget = cc.AvailableTokens()
	}
	if budget <= 0 {
		budget = DefaultCompilerConfig().DefaultTokenBudget
	}

	fitted, err := budgetMgr.Fit(ordered, budget)
	if err != nil {
		// Baseline should still return something even if budgeting fails.
		logging.Get(logging.CategoryJIT).Warn("Baseline budget fit failed: %v (returning ordered atoms)", err)
		fitted = ordered
	}

	assembler := NewFinalAssembler()
	promptText, err := assembler.Assemble(fitted, cc)
	if err != nil {
		return "", fmt.Errorf("baseline assembly failed: %w", err)
	}

	return promptText, nil
}

