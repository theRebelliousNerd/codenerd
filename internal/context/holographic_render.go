package context

import (
	"context"
	"fmt"
	"strconv"

	"codenerd/internal/mangle"
)

// DecideCallerLimit derives how many of a holographic callers pool the
// renderer shows. total is how many callers the pool holds, avgBytes their
// mean rendered line in bytes, budgetBytes the session's render budget in
// bytes; the policy spends the configured share of that budget
// (/working_holographic_caller_share_percent) on this block. Go measures all
// three; the rule (holographic_render.mg) decides.
//
// The per-render facts are replaced, not accumulated, like the ledger's: one
// render derives exactly one N, and a render never sees another's pool. An
// engine failure is an error, not a number: the renderer withholds the block
// behind its honest remainder line rather than guessing a count.
func (w *WorkingSet) DecideCallerLimit(ctx context.Context, target string, total, avgBytes, budgetBytes int) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if total < 0 {
		total = 0
	}
	// The rule divides the allowance by the mean line. A mean below one byte
	// is not a measurement (every rendered line holds at least a name), and
	// zero would abort the fixpoint in fn:div.
	if avgBytes < 1 {
		avgBytes = 1
	}
	if budgetBytes < 0 {
		budgetBytes = 0
	}
	facts := []mangle.Fact{
		{Predicate: "holographic_caller_pool", Args: []any{target, int64(total)}},
		{Predicate: "holographic_caller_bytes", Args: []any{target, int64(avgBytes)}},
		{Predicate: "holographic_render_budget", Args: []any{target, int64(budgetBytes)}},
	}
	if err := w.engine.ReplaceControlFacts(facts,
		"holographic_caller_pool", "holographic_caller_bytes", "holographic_render_budget"); err != nil {
		return 0, err
	}
	rows, err := w.engine.Query(ctx, "holographic_callers_to_render(Target, N)")
	if err != nil {
		return 0, err
	}
	if len(rows.Bindings) == 0 {
		return 0, fmt.Errorf("holographic policy derived no caller count for %q (pool %d, mean %d bytes, budget %d bytes)", target, total, avgBytes, budgetBytes)
	}
	n, err := strconv.Atoi(fmt.Sprint(rows.Bindings[0]["N"]))
	if err != nil {
		return 0, fmt.Errorf("holographic caller count is not a number: %v", rows.Bindings[0]["N"])
	}
	// Bounds safety, not a decision: the renderer slices a ranked prefix to
	// this number, so it must be a valid prefix even if the policy surprises.
	if n < 0 {
		n = 0
	}
	if n > total {
		n = total
	}
	return n, nil
}
