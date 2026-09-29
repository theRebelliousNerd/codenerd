package world

import (
	"context"

	"codenerd/internal/logging"
)

// The caller-limit decider contract: how many of a holographic callers pool
// to render, from totalCallers (how many the pool holds), avgBytesPerCaller
// (their mean rendered line in bytes) and budgetBytes (the session's render
// budget in bytes). Spelled as a func literal on every signature instead of a
// named type because the session's FileContextProvider cannot name a world
// type without importing it: a named type on one side of the seam does not
// satisfy the literal on the other (the compiler reports "wrong type for
// method"). The session passes its working set's DecideCallerLimit as the
// value, so nothing here imports the engine's package and nothing there
// imports the provider. The counts it returns slice the existing ranking;
// the remainder line still states the true rest.

// PromptSectionWithCallerBudget renders holographic context for prompt
// injection with the callers block sized by policy. The renderer measures its
// pool (how many callers, their mean rendered bytes) and asks decide how many
// to show; a decider failure withholds the block behind its honest remainder
// line ("and N more callers; callers_of ...") rather than guessing a count.
// Every other dimension renders as PromptSection does.
func (h *HolographicProvider) PromptSectionWithCallerBudget(ctx context.Context, filePath string, budgetBytes int, decide func(ctx context.Context, target string, totalCallers, avgBytesPerCaller, budgetBytes int) (int, error)) string {
	return h.promptSection(ctx, filePath, budgetBytes, decide)
}

// resolveCallerLines applies the callers decision to measured lines. No
// decider means no policy, so every line renders; a decider failure withholds
// every line behind the remainder the caller writes. The clamp is bounds
// safety for the slice, not a second decision.
func resolveCallerLines(ctx context.Context, target string, lines []string, budgetBytes int, decide func(ctx context.Context, target string, totalCallers, avgBytesPerCaller, budgetBytes int) (int, error)) []string {
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
	derived, err := decide(ctx, target, len(lines), avg, budgetBytes)
	if err != nil {
		logging.WorldDebug("holographic callers: policy failed (%v); withholding %d callers behind the remainder line", err, len(lines))
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
