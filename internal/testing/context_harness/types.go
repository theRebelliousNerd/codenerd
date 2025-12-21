package context_harness

import (
	"time"
)

// Scenario represents a complete test scenario for the context system.
type Scenario struct {
	Name        string
	Description string
	Turns       []Turn
	Checkpoints []Checkpoint
	ExpectedMetrics Metrics
}

// Turn represents a single interaction in a coding session.
type Turn struct {
	TurnID   int
	Speaker  string // "user" or "assistant"
	Message  string
	Intent   string // "debug", "implement", "test", "refactor", etc.
	Metadata TurnMetadata
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
}

// Metrics represents measured performance.
type Metrics struct {
	CompressionRatio   float64 // original_tokens / compressed_tokens
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
}
