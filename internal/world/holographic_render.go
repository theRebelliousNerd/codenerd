package world

import (
	"context"

	"codenerd/internal/logging"
)

// Dimension atoms DecideRenderCount keys on. They are the atoms in
// internal/context/holographic_render.mg; the budget tests fail when a
// renderer passes a different string, because the policy then derives nothing
// and the block is withheld.
const (
	holoCallers          = "/callers"
	holoSignatures       = "/signatures"
	holoTypes            = "/types"
	holoImporters        = "/importers"
	holoOutline          = "/outline"
	holoOutlineSignature = "/outline_signature"
)

// PromptSectionWithBudget renders holographic context for prompt injection
// with every counted block sized by policy. The renderer measures each pool
// (how many lines, their mean rendered bytes) and asks decide, passing the
// dimension atom, how many to show. A decider failure withholds that block
// behind its honest remainder line rather than guessing a count. decide is
// spelled as a func literal on the signature, not a named type: the session's
// FileContextProvider cannot name a world type, and a named type on one side
// of the seam does not satisfy the literal on the other (the compiler reports
// "wrong type for method"). The session passes its working set's
// DecideRenderCount.
func (h *HolographicProvider) PromptSectionWithBudget(ctx context.Context, filePath string, budgetBytes int, decide func(ctx context.Context, dimension, target string, total, avgBytes, budgetBytes int) (int, error)) string {
	return h.promptSection(ctx, filePath, budgetBytes, decide)
}

// resolveRenderedLines applies one dimension's count decision to measured
// lines. No decider means no policy, so every line renders; a decider failure
// withholds every line behind the remainder the caller writes. The clamp is
// bounds safety for the slice, not a second decision.
func resolveRenderedLines(ctx context.Context, dimension, target string, lines []string, budgetBytes int, decide func(ctx context.Context, dimension, target string, total, avgBytes, budgetBytes int) (int, error)) []string {
	if len(lines) == 0 || decide == nil {
		return lines
	}
	sum := 0
	for _, line := range lines {
		sum += len(line)
	}
	avg := sum / len(lines)
	if avg < 1 {
		avg = 1
	}
	derived, err := decide(ctx, dimension, target, len(lines), avg, budgetBytes)
	if err != nil {
		logging.WorldDebug("holographic %s: policy failed (%v); withholding %d behind the remainder line", dimension, err, len(lines))
		return nil
	}
	if derived < 0 {
		derived = 0
	}
	if derived > len(lines) {
		derived = len(lines)
	}
	return lines[:derived]
}
