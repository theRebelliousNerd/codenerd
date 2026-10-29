package context

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"fmt"
	"strings"
)

// Reference implementations and fixtures for tests. Production summarizes
// through generateObservationMaskedSummary and builds its compressor with
// NewCompressorWithParams.

// generateSimpleSummary creates a basic summary without LLM.
func (c *Compressor) generateSimpleSummary(turns []CompressedTurn) string {
	if len(turns) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Compressed History (Turns %d-%d)\n", turns[0].TurnNumber, turns[len(turns)-1].TurnNumber))

	for _, turn := range turns {
		if turn.IntentAtom != nil {
			sb.WriteString(turn.IntentAtom.String())
			sb.WriteString("\n")
		}
		writeCappedResultAtoms(&sb, turn)
	}

	return sb.String()
}

// NewConfigWithBudget creates a CompressorConfig with a specific total budget.
// Use this to create a config from config.ContextWindow.MaxTokens.
// The reserves are automatically calculated as percentages of the total budget.
func NewConfigWithBudget(totalBudget int) CompressorConfig {
	if totalBudget <= 0 {
		totalBudget = 200000 // Default 200k tokens
	}

	cfg := DefaultConfig()
	cfg.TotalBudget = totalBudget
	cfg.CoreReserve = totalBudget * 5 / 100     // 5%
	cfg.AtomReserve = totalBudget * 30 / 100    // 30%
	cfg.HistoryReserve = totalBudget * 15 / 100 // 15%
	cfg.WorkingReserve = totalBudget * 50 / 100 // 50%
	return cfg
}

// NewCompressorWithConfig creates a compressor with custom configuration.
func NewCompressorWithConfig(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient, cfg config.ContextWindowConfig) *Compressor {
	// Convert config.ContextWindowConfig to context.CompressorConfig
	compCfg := CompressorConfig{
		TotalBudget:            cfg.MaxTokens,
		CoreReserve:            cfg.MaxTokens * cfg.CoreReservePercent / 100,
		AtomReserve:            cfg.MaxTokens * cfg.AtomReservePercent / 100,
		HistoryReserve:         cfg.MaxTokens * cfg.HistoryReservePercent / 100,
		WorkingReserve:         cfg.MaxTokens * cfg.WorkingReservePercent / 100,
		RecentTurnWindow:       cfg.RecentTurnWindow,
		CompressionThreshold:   cfg.CompressionThreshold,
		TargetCompressionRatio: cfg.TargetCompressionRatio,
		ActivationThreshold:    cfg.ActivationThreshold,
		PredicatePriorities:    DefaultConfig().PredicatePriorities,
	}

	logging.Context("Compressor initialized with custom config: budget=%d tokens, threshold=%.0f%%, window=%d turns",
		compCfg.TotalBudget, compCfg.CompressionThreshold*100, compCfg.RecentTurnWindow)
	logging.ContextDebug("Token allocation: core=%d, atoms=%d, history=%d, working=%d",
		compCfg.CoreReserve, compCfg.AtomReserve, compCfg.HistoryReserve, compCfg.WorkingReserve)

	return newCompressorWithCompressorConfig(kernel, localStorage, llmClient, compCfg)
}
