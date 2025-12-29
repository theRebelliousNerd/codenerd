package context_harness

import (
	"context"

	"codenerd/internal/core"
	internalcontext "codenerd/internal/context"
)

// ContextEngine defines the interface for context compression and retrieval.
// Both MockContextEngine (fast, for CI) and RealIntegrationEngine (real components)
// implement this interface, enabling dual-mode testing.
type ContextEngine interface {
	// CompressTurn compresses a single turn into semantic facts.
	// Returns the generated facts, compressed token count, and any error.
	CompressTurn(ctx context.Context, turn *Turn) ([]core.Fact, int, error)

	// RetrieveContext retrieves relevant facts for a query within a token budget.
	// Uses spreading activation scoring (mock or real depending on engine).
	RetrieveContext(ctx context.Context, query string, tokenBudget int) ([]core.Fact, error)

	// GetCompressionStats returns original and compressed token counts.
	GetCompressionStats() (originalTokens, compressedTokens int)

	// GetActivationBreakdown returns the 7-component activation scoring breakdown
	// for a specific fact ID. Returns nil if not available (mock mode).
	GetActivationBreakdown(factID string) *ActivationBreakdown

	// SetCampaignContext sets the campaign context for campaign-aware activation.
	// Used by real engine to enable phase-based boosting.
	SetCampaignContext(ctx *internalcontext.CampaignActivationContext)

	// SetIssueContext sets the issue context for SWE-bench style activation.
	// Used by real engine for tiered file boosting.
	SetIssueContext(ctx *internalcontext.IssueActivationContext)

	// Reset clears all state for a fresh test run.
	Reset() error

	// GetMode returns whether this is a mock or real engine.
	GetMode() EngineMode
}

// ActivationBreakdown shows the 7-component scoring for a fact.
// This enables validation that the real system is correctly applying boosts.
type ActivationBreakdown struct {
	FactID string

	// The 7 scoring components from ActivationEngine
	BaseScore       float64 // From predicate corpus priority (0-100)
	RecencyBoost    float64 // From timestamp tracking (0-50)
	RelevanceBoost  float64 // From intent matching (0-50)
	DependencyBoost float64 // From symbol graph spreading (0-50)
	CampaignBoost   float64 // From campaign context (0-50)
	SessionBoost    float64 // From session-specific facts (0-30)
	IssueBoost      float64 // From issue context tiers (0-50)

	// Combined final score
	TotalScore float64
}

// CompressionStats provides detailed compression metrics.
type CompressionStats struct {
	OriginalTokens   int
	CompressedTokens int
	Ratio            float64 // Original / Compressed

	// LLM summary details (only in real mode)
	SummaryGenerated bool
	SummaryTokens    int
	KeyInsights      []string
}

// ActivationValidation defines expected activation scoring for checkpoint validation.
// Used to verify that specific facts receive the correct boost components.
type ActivationValidation struct {
	FactPattern string // Regex to match fact predicate

	// Minimum expected values for each component
	MinBaseScore       float64
	MinRecencyBoost    float64
	MinRelevanceBoost  float64
	MinDependencyBoost float64
	MinCampaignBoost   float64
	MinSessionBoost    float64
	MinIssueBoost      float64
	MinTotalScore      float64
}

// ValidateActivation checks if a fact's activation breakdown meets expectations.
func (av *ActivationValidation) ValidateActivation(breakdown *ActivationBreakdown) error {
	if breakdown == nil {
		return nil // No breakdown available (mock mode) - skip validation
	}

	if breakdown.BaseScore < av.MinBaseScore {
		return &ActivationValidationError{
			Component: "BaseScore",
			Expected:  av.MinBaseScore,
			Actual:    breakdown.BaseScore,
		}
	}
	if breakdown.RecencyBoost < av.MinRecencyBoost {
		return &ActivationValidationError{
			Component: "RecencyBoost",
			Expected:  av.MinRecencyBoost,
			Actual:    breakdown.RecencyBoost,
		}
	}
	if breakdown.RelevanceBoost < av.MinRelevanceBoost {
		return &ActivationValidationError{
			Component: "RelevanceBoost",
			Expected:  av.MinRelevanceBoost,
			Actual:    breakdown.RelevanceBoost,
		}
	}
	if breakdown.DependencyBoost < av.MinDependencyBoost {
		return &ActivationValidationError{
			Component: "DependencyBoost",
			Expected:  av.MinDependencyBoost,
			Actual:    breakdown.DependencyBoost,
		}
	}
	if breakdown.CampaignBoost < av.MinCampaignBoost {
		return &ActivationValidationError{
			Component: "CampaignBoost",
			Expected:  av.MinCampaignBoost,
			Actual:    breakdown.CampaignBoost,
		}
	}
	if breakdown.SessionBoost < av.MinSessionBoost {
		return &ActivationValidationError{
			Component: "SessionBoost",
			Expected:  av.MinSessionBoost,
			Actual:    breakdown.SessionBoost,
		}
	}
	if breakdown.IssueBoost < av.MinIssueBoost {
		return &ActivationValidationError{
			Component: "IssueBoost",
			Expected:  av.MinIssueBoost,
			Actual:    breakdown.IssueBoost,
		}
	}
	if breakdown.TotalScore < av.MinTotalScore {
		return &ActivationValidationError{
			Component: "TotalScore",
			Expected:  av.MinTotalScore,
			Actual:    breakdown.TotalScore,
		}
	}

	return nil
}

// ActivationValidationError indicates a component didn't meet expectations.
type ActivationValidationError struct {
	Component string
	Expected  float64
	Actual    float64
}

func (e *ActivationValidationError) Error() string {
	return e.Component + " validation failed: expected >= " +
		formatFloat(e.Expected) + ", got " + formatFloat(e.Actual)
}

func formatFloat(f float64) string {
	return string(rune(int(f*100)/100 + '0'))
}
