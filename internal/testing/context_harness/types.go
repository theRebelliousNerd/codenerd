package context_harness

import (
	"time"
)

// EngineMode specifies whether to use mock or real components.
type EngineMode string

const (
	// MockMode uses fast mock implementations for CI.
	// Facts persist within scenario but use simplified scoring.
	MockMode EngineMode = "mock"

	// RealMode uses real ActivationEngine, Compressor, and Kernel.
	// Required for integration scenarios testing actual system behavior.
	RealMode EngineMode = "real"
)

// ScenarioCategory groups scenarios by what they test.
type ScenarioCategory string

const (
	CategoryMock        ScenarioCategory = "mock"        // Fast mock scenarios (original 8)
	CategoryIntegration ScenarioCategory = "integration" // Real component scenarios (new 6)
)

// Scenario represents a complete test scenario for the context system.
type Scenario struct {
	ScenarioID  string // kebab-case identifier (e.g., "debugging-marathon")
	Name        string // Human-readable name (e.g., "Debugging Marathon")
	Description string
	Turns       []Turn
	Checkpoints []Checkpoint
	ExpectedMetrics Metrics

	// Engine configuration (new for dual-mode support)
	Mode     EngineMode       // Required engine mode (mock/real)
	Category ScenarioCategory // Scenario category for filtering

	// Initial facts to seed the kernel (for integration scenarios)
	InitialFacts []string // Mangle fact strings to assert at start
}

// Turn represents a single interaction in a coding session.
type Turn struct {
	TurnID   int
	Speaker  string // "user" or "assistant"
	Message  string
	Intent   string // "debug", "implement", "test", "refactor", etc.
	Metadata TurnMetadata

	// Campaign context (for integration scenarios)
	CampaignPhase   string // e.g., "planning", "implementation", "testing"
	PhaseTransition bool   // True if this turn triggers ResetPhaseContext + ActivatePhase
}

// TurnMetadata contains rich context about the turn.
type TurnMetadata struct {
	FilesReferenced []string
	SymbolsReferenced []string
	ErrorMessages   []string
	Topics          []string
	IsQuestionReferringBack bool // "What was the original error?"
	ReferencesBackToTurn    *int  // nil if not referencing back
}

// Checkpoint defines validation points in the scenario.
type Checkpoint struct {
	AfterTurn    int
	Query        string   // Simulated query for retrieval
	MustRetrieve []string // Fact IDs that MUST be in retrieval results
	ShouldAvoid  []string // Facts that should NOT be retrieved (noise)
	MinRecall    float64  // Minimum acceptable recall
	MinPrecision float64  // Minimum acceptable precision
	Description  string

	// Advanced validation for integration scenarios (optional)
	ValidateActivation   *ActivationValidation   // Verify 8-component scoring (including feedback)
	ValidateCompression  *CompressionCheckpoint  // Verify compression behavior
	ValidateFeedback     *FeedbackValidation     // Verify feedback learning behavior
}

// FeedbackValidation validates context feedback learning at a checkpoint.
type FeedbackValidation struct {
	// Expected minimum feedback samples collected by this turn
	MinFeedbackSamples int
	// Expected predicates that should have positive usefulness scores
	ExpectedHelpful []string
	// Expected predicates that should have negative usefulness scores
	ExpectedNoise []string
	// Minimum positive impact for helpful predicates (-20 to +20 scale)
	MinHelpfulBoost float64
	// Maximum negative impact for noise predicates (-20 to +20 scale)
	MaxNoiseBoost float64
}

// CompressionCheckpoint validates compression behavior at a checkpoint.
type CompressionCheckpoint struct {
	ExpectTriggered         bool     // Should compression have fired?
	MinRatio                float64  // Minimum compression ratio (e.g., 50.0 for 50:1)
	MaxBudgetUtilization    float64  // Maximum token budget usage (e.g., 0.8 for 80%)
	ValidateSummaryContains []string // Key insights that must be in LLM summary
}

// Metrics represents measured performance.
type Metrics struct {
	// CompressionRatio = original_tokens / compressed_tokens
	// Values < 1.0 indicate semantic ENRICHMENT (short message → rich facts)
	// Values > 1.0 indicate actual COMPRESSION (verbose text → compact facts)
	// For the context harness, expect ~0.3-0.5x (enrichment) because
	// short user messages are expanded into structured Mangle facts with
	// extracted topics, file references, error messages, and back-references.
	// True compression happens over session lifetime via fact decay/pruning.
	CompressionRatio float64
	AvgRetrievalPrec   float64
	AvgRetrievalRecall float64
	AvgF1Score         float64
	TokenBudgetViolations int
	AvgCompressionLatency time.Duration
	AvgRetrievalLatency   time.Duration
	PeakMemoryMB       float64
	QualityDegradation float64 // 0.0 = no degradation, 1.0 = complete failure
}

// TestResult captures the outcome of running a scenario.
type TestResult struct {
	Scenario        *Scenario
	ActualMetrics   Metrics
	CheckpointResults []CheckpointResult
	Passed          bool
	FailureReasons  []string
}

// CheckpointResult captures the outcome of a single checkpoint.
type CheckpointResult struct {
	Checkpoint      *Checkpoint
	Retrieved       []string // Fact IDs actually retrieved
	MissingRequired []string // Required facts that were missing
	UnwantedNoise   []string // Noise facts that were retrieved
	Precision       float64
	Recall          float64
	F1Score         float64
	Passed          bool
	FailureReason   string
}

// SimulatorConfig configures the session simulator.
type SimulatorConfig struct {
	MaxTurns       int
	TokenBudget    int
	CompressionEnabled bool
	PagingEnabled  bool
	VectorStoreEnabled bool

	// Engine mode selection
	Mode EngineMode // mock (default) or real
}
