package context

import (
	"context"
	"fmt"
	"strconv"

	"codenerd/internal/mangle"
)

// Dimension atoms the holographic render policy keys on. The world renderer
// passes the same strings; holographic_render.mg maps each one to
// /working_holographic_<dim>_share_percent. HolographicDimOutlineSignature is
// the per-entry character allowance, not a count of lines and not its own share.
const (
	HolographicDimCallers          = "/callers"
	HolographicDimSignatures       = "/signatures"
	HolographicDimTypes            = "/types"
	HolographicDimImporters        = "/importers"
	HolographicDimOutline          = "/outline"
	HolographicDimOutlineSignature = "/outline_signature"
)

// DecideRenderCount derives how many of one holographic pool the renderer
// shows, or, for HolographicDimOutlineSignature, how many runes of each
// outline signature to keep. total is the pool size (for the signature
// allowance, the entries the outline count decision kept), avgBytes their
// mean rendered line in bytes, budgetBytes the session's render budget in
// bytes. The policy spends that dimension's configured share of the budget.
// Go measures; holographic_render.mg decides.
//
// The per-render facts are replaced, not accumulated, like the ledger's: one
// call derives exactly one N, and a call never sees another's pool. An engine
// failure is an error, not a number: the renderer withholds the block behind
// its honest remainder line rather than guessing a count.
func (w *WorkingSet) DecideRenderCount(ctx context.Context, dimension, target string, total, avgBytes, budgetBytes int) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !validHolographicDimension(dimension) {
		return 0, fmt.Errorf("holographic dimension %q is not a name constant", dimension)
	}
	if total < 0 {
		total = 0
	}
	// The count rule divides the allowance by the mean line. A mean below one
	// byte is not a measurement (every rendered line holds at least a name),
	// and zero would abort the fixpoint in fn:div. The signature allowance
	// does not use the mean; the same floor keeps the fact well typed.
	if avgBytes < 1 {
		avgBytes = 1
	}
	if budgetBytes < 0 {
		budgetBytes = 0
	}
	// The signature rule divides the outline allowance by the entries shown.
	// Zero entries has no per-entry allowance, and fn:div would abort.
	if dimension == HolographicDimOutlineSignature && total < 1 {
		return 0, fmt.Errorf("holographic outline signature allowance needs the entries being rendered, got %d", total)
	}
	facts := []mangle.Fact{
		{Predicate: "holographic_render_pool", Args: []any{dimension, target, int64(total)}},
		{Predicate: "holographic_render_bytes", Args: []any{dimension, target, int64(avgBytes)}},
		{Predicate: "holographic_render_budget", Args: []any{target, int64(budgetBytes)}},
	}
	if err := w.engine.ReplaceControlFacts(facts,
		"holographic_render_pool", "holographic_render_bytes", "holographic_render_budget"); err != nil {
		return 0, err
	}
	rows, err := w.engine.Query(ctx, fmt.Sprintf("holographic_to_render(%s, Target, N)", dimension))
	if err != nil {
		return 0, err
	}
	if len(rows.Bindings) != 1 {
		return 0, fmt.Errorf("holographic policy derived %d %s counts for %q (pool %d, mean %d bytes, budget %d bytes), want one", len(rows.Bindings), dimension, target, total, avgBytes, budgetBytes)
	}
	n, err := strconv.Atoi(fmt.Sprint(rows.Bindings[0]["N"]))
	if err != nil {
		return 0, fmt.Errorf("holographic %s count is not a number: %v", dimension, rows.Bindings[0]["N"])
	}
	// Bounds safety, not a decision. A count slices a ranked prefix, so it
	// has to be a valid prefix. The outline signature allowance is a rune
	// budget, which is larger than the entry count whenever the block has
	// room; clamping it to the entry count would cut every signature to a
	// handful of glyphs.
	if n < 0 {
		n = 0
	}
	if dimension != HolographicDimOutlineSignature && n > total {
		n = total
	}
	return n, nil
}

// validHolographicDimension is query hygiene. The dimension is spliced into
// a Mangle query, so it has to be a name constant and nothing else. Which
// dimensions derive a count is the policy's decision: an unknown atom derives
// nothing and DecideRenderCount reports that.
func validHolographicDimension(dimension string) bool {
	if len(dimension) < 2 || dimension[0] != '/' {
		return false
	}
	for i := 1; i < len(dimension); i++ {
		c := dimension[i]
		if c != '_' && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
