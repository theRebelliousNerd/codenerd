// Package context implements the Semantic Compression system for infinite context.
// This is the core implementation of §8.2 from the Cortex 1.5.0 specification.
//
// The system achieves "Infinite Context" by continuously discarding surface text
// and retaining only logical state.
package context

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"time"
)

// =============================================================================
// SECTION 1: Configuration Types
// =============================================================================

// CompressorConfig defines compression parameters and token budgets.
type CompressorConfig struct {
	// Token Budget Allocation (default: 128k tokens)
	TotalBudget    int // Total tokens available
	CoreReserve    int // Reserved for constitutional facts, schemas
	AtomReserve    int // For high-activation context atoms
	HistoryReserve int // For compressed history + recent turns
	WorkingReserve int // For current turn processing

	// Sliding Window Configuration
	RecentTurnWindow int // Number of recent turns to keep with full metadata

	// Compression Thresholds
	CompressionThreshold   float64 // Trigger compression at this % of budget
	TargetCompressionRatio float64 // Target ratio after compression
	ActivationThreshold    float64 // Minimum activation score to include

	// PredicatePriorities contains hardcoded predicate priorities.
	// DEPRECATED: Use predicate_corpus.db as the Single Source of Truth (SSOT).
	// The corpus is loaded via ActivationEngine.LoadPrioritiesFromCorpus().
	// This map is retained only as a fallback when corpus is unavailable.
	PredicatePriorities map[string]int
}

// DefaultConfig returns a configuration optimized for typical usage.
// Note: TotalBudget should be overridden from config.ContextWindow.MaxTokens.
func DefaultConfig() CompressorConfig {
	return CompressorConfig{
		// 200k context window budget (callers should override from config.ContextWindow.MaxTokens)
		TotalBudget:    200000,
		CoreReserve:    10000, // 5% - constitutional facts
		AtomReserve:    60000, // 30% - high-activation atoms
		HistoryReserve: 30000, // 15% - compressed history
		WorkingReserve: 100000, // 50% - working memory

		// Keep last 5 turns fully
		RecentTurnWindow: 5,

		// Compress at 60% usage, target 100:1 ratio
		CompressionThreshold:   0.60,
		TargetCompressionRatio: 100.0,
		// Activation threshold: facts need base (50) + recency (50) + relevance boost
		// to be included. This ensures only contextually relevant facts pass.
		// Base(50) + recency(50) = 100, threshold(105) requires additional relevance.
		// Facts with no query relevance are filtered out even if they're recent.
		ActivationThreshold: 105.0,

		// DEPRECATED: These hardcoded priorities are fallback only.
		// Use predicate_corpus.db via ActivationEngine.LoadPrioritiesFromCorpus().
		// Values here match policy.mg spreading activation for backwards compatibility.
		PredicatePriorities: map[string]int{
			// Core intent & focus
			"user_intent":           100,
			"focus_resolution":      100,
			"active_goal":           100,
			"new_fact":              100,
			"pending_clarification": 100,

			// Diagnostics & test state
			"diagnostic":     95,
			"test_state":     95,
			"block_commit":   95,
			"block_refactor": 90,

			// File & code context
			"file_topology":   80,
			"modified":        85,
			"impacted":        85,
			"symbol_graph":    75,
			"dependency_link": 70,

			// Campaign context
			"campaign":         90,
			"current_campaign": 95,
			"current_phase":    95,
			"campaign_task":    85,
			"campaign_phase":   80,

			// Issue context
			"issue_text":          90,
			"issue_keyword":       85,
			"file_mentioned":      85,
			"tiered_context_file": 80,

			// Shard delegation
			"delegate_task": 90,
			"shard_profile": 70,

			// Safety & permissions
			"permitted":          100,
			"dangerous_action":   100,
			"security_violation": 100,

			// Browser (usually lower priority)
			"dom_node":     20,
			"geometry":     20,
			"interactable": 30,

			// Memory shards (contextual)
			"vector_recall":  60,
			"knowledge_link": 60,
			"knowledge_atom": 65,

			// Activation tracking
			"activation":   50,
			"context_atom": 50,

			// Session state
			"session_state": 40,
			"turn_context":  40,
		},
	}
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
	cfg.CoreReserve = totalBudget * 5 / 100    // 5%
	cfg.AtomReserve = totalBudget * 30 / 100   // 30%
	cfg.HistoryReserve = totalBudget * 15 / 100 // 15%
	cfg.WorkingReserve = totalBudget * 50 / 100 // 50%
	return cfg
}

// =============================================================================
// SECTION 2: Compressed Context Types
// =============================================================================

// CompressedContext represents the minimal context for an LLM call.
// This replaces the raw conversation history with semantically compressed state.
type CompressedContext struct {
	// High-priority logical atoms serialized as Mangle notation
	ContextAtoms string

	// Core facts that are always included (constitutional, schemas)
	CoreFacts string

	// Compressed conversation history
	HistorySummary string

	// Recent turns with metadata only (no surface text)
	RecentTurns []CompressedTurn

	// Token accounting
	TokenUsage TokenUsage

	// Metadata
	GeneratedAt   time.Time
	TurnNumber    int
	CompressionID string
}

// CompressedTurn represents a single conversation turn with surface text removed.
// Only the logical essence is retained.
type CompressedTurn struct {
	TurnNumber int
	Role       string // "user" or "assistant"
	Timestamp  time.Time

	// Metrics
	OriginalTokens int // Estimated tokens (user input + surface response) before compression

	// The semantic essence of this turn
	IntentAtom  *core.Fact  // user_intent atom (if user turn)
	FocusAtoms  []core.Fact // focus_resolution atoms
	ActionAtoms []core.Fact // Actions taken (if assistant turn)
	ResultAtoms []core.Fact // Results/outcomes

	// Metadata
	MangleUpdates    []string // Raw mangle_updates from control packet
	MemoryOperations []perception.MemoryOperation

	// IMPORTANT: No surface text stored - this is the key to compression
}

// TokenUsage tracks token allocation across context components.
type TokenUsage struct {
	Total     int // Total tokens used
	Core      int // Tokens used for core facts
	Atoms     int // Tokens used for context atoms
	History   int // Tokens used for compressed history
	Recent    int // Tokens used for recent turns
	Available int // Tokens still available

	// Compression metrics
	CompressionRatio float64 // Ratio of original to compressed
	OriginalTokens   int     // Tokens before compression
}

// =============================================================================
// SECTION 3: Activation Types
// =============================================================================

// ScoredFact represents a fact with its activation score.
type ScoredFact struct {
	Fact  core.Fact
	Score float64

	// Scoring components (8 total)
	BaseScore       float64 // From predicate priority
	RecencyScore    float64 // Based on when it was added
	RelevanceScore  float64 // Based on relation to current intent
	DependencyScore float64 // Based on dependency chain distance
	CampaignScore   float64 // Campaign-specific boost
	SessionScore    float64 // Session-specific boost
	IssueScore      float64 // Issue/SWE-bench specific boost
	FeedbackScore   float64 // Learned predicate usefulness from LLM feedback
}

// ActivationState tracks the current activation state of the system.
type ActivationState struct {
	// Current intent being processed
	ActiveIntent *core.Fact

	// Currently focused files/symbols
	FocusedPaths   []string
	FocusedSymbols []string

	// High-activation facts
	HotFacts []ScoredFact

	// Recently added facts (for recency scoring)
	RecentFacts []core.Fact

	// Timestamp of last activation update
	LastUpdate time.Time
}

// =============================================================================
// SECTION 4: Turn Processing Types
// =============================================================================

// Turn represents a single conversation turn before compression.
type Turn struct {
	Number    int
	Role      string
	Timestamp time.Time

	// Raw inputs
	UserInput string

	// Parsed outputs (from Piggyback Protocol)
	SurfaceResponse string
	ControlPacket   *perception.ControlPacket

	// Extracted atoms
	ExtractedAtoms []core.Fact
}

// TurnResult represents the result of processing a turn.
type TurnResult struct {
	// Atoms extracted and committed to kernel
	CommittedAtoms []core.Fact

	// Memory operations performed
	MemoryOps []perception.MemoryOperation

	// Whether compression was triggered
	CompressionTriggered bool

	// Updated token usage
	TokenUsage TokenUsage
}

// =============================================================================
// SECTION 5: History Compression Types
// =============================================================================

// HistorySegment represents a compressed segment of conversation history.
type HistorySegment struct {
	ID        string
	StartTurn int
	EndTurn   int

	// Compressed summary
	Summary string

	// Key atoms from this segment
	KeyAtoms []core.Fact

	// Metrics
	OriginalTokens   int
	CompressedTokens int
	CompressionRatio float64

	// Timestamp
	CompressedAt time.Time
}

// RollingSummary maintains a continuously updated summary of conversation history.
type RollingSummary struct {
	// Current summary text
	Text string

	// Segments that have been compressed into this summary
	Segments []HistorySegment

	// Total turns compressed
	TotalTurns int

	// Compression metrics
	TotalOriginalTokens   int
	TotalCompressedTokens int
	OverallRatio          float64

	// Last update
	LastUpdate time.Time
}

// =============================================================================
// SECTION 6: Persistence Types
// =============================================================================

// CompressedState represents the full compressed state for persistence.
type CompressedState struct {
	// Session identification
	SessionID string
	Version   string // Schema version for forward compatibility

	// Current state
	TurnNumber int
	Timestamp  time.Time

	// Compressed data
	RollingSummary RollingSummary
	RecentTurns    []CompressedTurn

	// Activation state
	HotFacts []ScoredFact

	// Metrics
	TotalCompressedTurns int
	CompressionRatio     float64
}
