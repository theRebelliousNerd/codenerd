package context

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"strings"
	"unicode/utf8"
)

// =============================================================================
// Token Counting Utilities
// =============================================================================
// These utilities provide token estimation for context budget management.
// The heuristic is calibrated for Claude's tokenizer (~4 characters per token).

// TokenCounter provides token counting functionality.
type TokenCounter struct {
	// Calibration factor (characters per token)
	charsPerToken float64
}

// NewTokenCounter creates a new token counter with default calibration.
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{
		charsPerToken: 4.0, // Claude's approximate ratio
	}
}

// CountString estimates tokens in a string.
func (tc *TokenCounter) CountString(s string) int {
	if s == "" {
		return 0
	}
	// Use rune count for proper unicode handling
	runeCount := utf8.RuneCountInString(s)
	return int(float64(runeCount) / tc.charsPerToken)
}

// CountFact estimates tokens for a single fact.
func (tc *TokenCounter) CountFact(f core.Fact) int {
	// Predicate name + parentheses + dot = ~4 tokens minimum
	tokens := 4 + tc.CountString(f.Predicate)

	// Count each argument
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case string:
			if strings.HasPrefix(v, "/") {
				// Name constant - relatively short
				tokens += 1 + len(v)/4
			} else {
				// String value - full counting
				tokens += tc.CountString(v) + 2 // +2 for quotes
			}
		case int, int64, float64:
			tokens += 2 // Numbers are typically 1-2 tokens
		case bool:
			tokens += 1
		default:
			tokens += 3 // Unknown - estimate conservatively
		}
	}

	return tokens
}

// CountFacts estimates tokens for a slice of facts.
func (tc *TokenCounter) CountFacts(facts []core.Fact) int {
	total := 0
	for _, f := range facts {
		total += tc.CountFact(f)
	}
	return total
}

// CountScoredFacts estimates tokens for scored facts.
func (tc *TokenCounter) CountScoredFacts(facts []ScoredFact) int {
	total := 0
	for _, sf := range facts {
		total += tc.CountFact(sf.Fact)
	}
	return total
}

// CountTurn estimates tokens for a compressed turn.
func (tc *TokenCounter) CountTurn(turn CompressedTurn) int {
	tokens := 10 // Base overhead (role, timestamp, etc.)

	if turn.IntentAtom != nil {
		tokens += tc.CountFact(*turn.IntentAtom)
	}

	for _, f := range turn.FocusAtoms {
		tokens += tc.CountFact(f)
	}

	for _, f := range turn.ActionAtoms {
		tokens += tc.CountFact(f)
	}

	for _, f := range turn.ResultAtoms {
		tokens += tc.CountFact(f)
	}

	// Mangle updates
	for _, update := range turn.MangleUpdates {
		tokens += tc.CountString(update)
	}

	return tokens
}

// CountTurns estimates tokens for multiple turns.
func (tc *TokenCounter) CountTurns(turns []CompressedTurn) int {
	total := 0
	for _, t := range turns {
		total += tc.CountTurn(t)
	}
	return total
}

// CountCompressedContext estimates total tokens in a compressed context.
func (tc *TokenCounter) CountCompressedContext(ctx *CompressedContext) int {
	if ctx == nil {
		return 0
	}

	total := 0
	total += tc.CountString(ctx.ContextAtoms)
	total += tc.CountString(ctx.CoreFacts)
	total += tc.CountString(ctx.HistorySummary)
	total += tc.CountTurns(ctx.RecentTurns)

	return total
}

// =============================================================================
// Token Budget Management
// =============================================================================

// TokenBudget tracks token allocation and usage.
type TokenBudget struct {
	counter *TokenCounter
	config  CompressorConfig

	// Current usage
	used struct {
		core    int
		atoms   int
		history int
		recent  int
		working int
	}
}

// NewTokenBudget creates a new token budget with the given config.
func NewTokenBudget(config CompressorConfig) *TokenBudget {
	return &TokenBudget{
		counter: NewTokenCounter(),
		config:  config,
	}
}

// Allocate attempts to allocate tokens for a category.
// Returns true if allocation succeeded, false if over budget.
func (tb *TokenBudget) Allocate(category string, tokens int) bool {
	switch category {
	case "core":
		if tb.used.core+tokens > tb.config.CoreReserve {
			logging.ContextDebug("Token allocation REJECTED: %s +%d would exceed budget (%d > %d)",
				category, tokens, tb.used.core+tokens, tb.config.CoreReserve)
			return false
		}
		tb.used.core += tokens
	case "atoms":
		if tb.used.atoms+tokens > tb.config.AtomReserve {
			logging.ContextDebug("Token allocation REJECTED: %s +%d would exceed budget (%d > %d)",
				category, tokens, tb.used.atoms+tokens, tb.config.AtomReserve)
			return false
		}
		tb.used.atoms += tokens
	case "history":
		if tb.used.history+tokens > tb.config.HistoryReserve {
			logging.ContextDebug("Token allocation REJECTED: %s +%d would exceed budget (%d > %d)",
				category, tokens, tb.used.history+tokens, tb.config.HistoryReserve)
			return false
		}
		tb.used.history += tokens
	case "recent":
		// Recent is part of history reserve
		if tb.used.recent+tokens > tb.config.HistoryReserve-tb.used.history {
			logging.ContextDebug("Token allocation REJECTED: %s +%d would exceed budget (%d > %d)",
				category, tokens, tb.used.recent+tokens, tb.config.HistoryReserve-tb.used.history)
			return false
		}
		tb.used.recent += tokens
	case "working":
		if tb.used.working+tokens > tb.config.WorkingReserve {
			logging.ContextDebug("Token allocation REJECTED: %s +%d would exceed budget (%d > %d)",
				category, tokens, tb.used.working+tokens, tb.config.WorkingReserve)
			return false
		}
		tb.used.working += tokens
	default:
		logging.Get(logging.CategoryContext).Warn("Unknown token category: %s", category)
		return false
	}
	logging.ContextDebug("Token allocation: %s +%d (new total: %d)", category, tokens, tb.TotalUsed())
	return true
}

// Release releases tokens from a category.
func (tb *TokenBudget) Release(category string, tokens int) {
	switch category {
	case "core":
		tb.used.core = max(0, tb.used.core-tokens)
	case "atoms":
		tb.used.atoms = max(0, tb.used.atoms-tokens)
	case "history":
		tb.used.history = max(0, tb.used.history-tokens)
	case "recent":
		tb.used.recent = max(0, tb.used.recent-tokens)
	case "working":
		tb.used.working = max(0, tb.used.working-tokens)
	}
}

// TotalUsed returns total tokens currently used.
func (tb *TokenBudget) TotalUsed() int {
	return tb.used.core + tb.used.atoms + tb.used.history + tb.used.recent + tb.used.working
}

// Available returns tokens still available.
func (tb *TokenBudget) Available() int {
	return tb.config.TotalBudget - tb.TotalUsed()
}

// Utilization returns the current utilization as a percentage.
func (tb *TokenBudget) Utilization() float64 {
	return float64(tb.TotalUsed()) / float64(tb.config.TotalBudget)
}

// ShouldCompress returns true if compression should be triggered.
func (tb *TokenBudget) ShouldCompress() bool {
	utilization := tb.Utilization()
	shouldCompress := utilization >= tb.config.CompressionThreshold
	if shouldCompress {
		logging.ContextDebug("ShouldCompress: YES (%.1f%% >= %.1f%% threshold)",
			utilization*100, tb.config.CompressionThreshold*100)
	}
	return shouldCompress
}

// GetUsage returns detailed token usage.
func (tb *TokenBudget) GetUsage() TokenUsage {
	return TokenUsage{
		Total:     tb.TotalUsed(),
		Core:      tb.used.core,
		Atoms:     tb.used.atoms,
		History:   tb.used.history,
		Recent:    tb.used.recent,
		Available: tb.Available(),
	}
}

// Reset resets all usage counters.
func (tb *TokenBudget) Reset() {
	tb.used.core = 0
	tb.used.atoms = 0
	tb.used.history = 0
	tb.used.recent = 0
	tb.used.working = 0
}

// =============================================================================
// Helper Functions
// =============================================================================

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// EstimateCompressionRatio estimates the compression ratio achievable.
func EstimateCompressionRatio(originalTokens, factsCount int) float64 {
	if factsCount == 0 {
		return 1.0
	}
	// Estimate: each fact compresses to ~10 tokens average
	compressedTokens := factsCount * 10
	if compressedTokens >= originalTokens {
		return 1.0
	}
	return float64(originalTokens) / float64(compressedTokens)
}
