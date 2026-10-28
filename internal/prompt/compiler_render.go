package prompt

import (
	"fmt"
	"time"

	"codenerd/internal/logging"
)

// fitAndAssemble runs steps 4-5 of a compile through one RenderSession: the
// fitter charges each atom at its rendered size, the assembler joins the same
// rendered fragments, and enforcement reassembles survivors from them without
// re-expanding a single placeholder.
func (c *JITPromptCompiler) fitAndAssemble(ordered []*OrderedAtom, cc *CompilationContext, budget int, stats *CompilationStats) ([]*OrderedAtom, string, error) {
	session := c.assembler.NewRenderSession(cc)

	fitStart := time.Now()
	// Defensive: a zero-value or partially-constructed compiler may have no
	// budget manager. Fall back to a default rather than nil-panicking.
	budgetMgr := c.budgetMgr
	if budgetMgr == nil {
		budgetMgr = NewTokenBudgetManager()
	}
	fitted, err := budgetMgr.Fit(ordered, budget, session.Tokens)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fit budget: %w", err)
	}
	stats.FitBudgetMs = time.Since(fitStart).Milliseconds()

	assembleStart := time.Now()
	prompt, err := session.Assemble(fitted)
	if err != nil {
		return nil, "", fmt.Errorf("failed to assemble prompt: %w", err)
	}

	// Enforce the budget on what was actually assembled. Fit now charges
	// rendered fragments, but section headers and separators are still only
	// visible in the joined text, so the final check stays.
	fitted, prompt, err = c.enforceAssembledBudget(fitted, cc, prompt, budget, budgetMgr, session)
	if err != nil {
		return nil, "", err
	}
	stats.AssembleMs = time.Since(assembleStart).Milliseconds()
	return fitted, prompt, nil
}

// enforceAssembledBudget makes the assembled prompt honour the token budget.
//
// One mechanism: shed whole optional atoms, lowest priority first (a missing
// exemplar is legible; half an exemplar is a lie). If the mandatory skeleton
// alone does not fit, the compile fails with an error naming the budget and
// the skeleton. It used to cut the prompt head+tail with a marker instead;
// that cut the identity and safety atoms — the ones that constrain the model
// — and shipped the result as a prompt. A turn that does not run is a
// configuration error the operator can see and fix; a turn that runs on a
// constitution with its ending missing is not.
func (c *JITPromptCompiler) enforceAssembledBudget(
	fitted []*OrderedAtom,
	cc *CompilationContext,
	prompt string,
	budget int,
	budgetMgr *TokenBudgetManager,
	session *RenderSession,
) ([]*OrderedAtom, string, error) {
	if budget <= 0 || prompt == "" || cc == nil {
		return fitted, prompt, nil
	}
	if EstimateTokens(prompt) <= budget {
		return fitted, prompt, nil
	}

	kept, shedPrompt, used := budgetMgr.ShedToFit(fitted, prompt, budget, session.Tokens, session.Assemble)
	if used <= budget {
		return kept, shedPrompt, nil
	}

	logging.Get(logging.CategoryJIT).Error(
		"JIT[%s] mandatory skeleton exceeds budget after shedding every optional atom: %d tokens vs budget %d — refusing to compile a cut prompt; raise context_window.max_tokens or jit.token_budget, or split the mandatory atoms",
		cc.ShardID, used, budget,
	)
	return kept, shedPrompt, fmt.Errorf(
		"prompt budget of %d tokens cannot hold the mandatory skeleton (%d tokens after shedding every optional atom) for shard %q: raise context_window.max_tokens or jit.token_budget, or split the mandatory atoms",
		budget, used, cc.ShardID)
}
