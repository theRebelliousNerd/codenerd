package context_harness

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// SessionSimulator simulates a coding session and measures context system performance.
type SessionSimulator struct {
	kernel  *core.Kernel
	config  SimulatorConfig
	metrics *MetricsCollector
}

// NewSessionSimulator creates a new session simulator.
func NewSessionSimulator(kernel *core.Kernel, config SimulatorConfig) *SessionSimulator {
	return &SessionSimulator{
		kernel:  kernel,
		config:  config,
		metrics: NewMetricsCollector(),
	}
}

// RunScenario executes a complete test scenario.
func (s *SessionSimulator) RunScenario(ctx context.Context, scenario *Scenario) (*TestResult, error) {
	result := &TestResult{
		Scenario:          scenario,
		CheckpointResults: make([]CheckpointResult, 0, len(scenario.Checkpoints)),
		Passed:            true,
		FailureReasons:    make([]string, 0),
	}

	// Execute each turn
	for i, turn := range scenario.Turns {
		// Simulate the turn (user message → assistant response)
		if err := s.executeTurn(ctx, &turn); err != nil {
			return nil, fmt.Errorf("turn %d failed: %w", i, err)
		}

		// Check if we have a checkpoint after this turn
		for _, checkpoint := range scenario.Checkpoints {
			if checkpoint.AfterTurn == i {
				cpResult := s.validateCheckpoint(ctx, &checkpoint)
				result.CheckpointResults = append(result.CheckpointResults, cpResult)

				if !cpResult.Passed {
					result.Passed = false
					result.FailureReasons = append(result.FailureReasons,
						fmt.Sprintf("Checkpoint at turn %d failed: %s", i, cpResult.FailureReason))
				}
			}
		}
	}

	// Finalize metrics
	result.ActualMetrics = s.metrics.Finalize()

	// Validate against expected metrics
	if !s.meetsExpectations(&result.ActualMetrics, &scenario.ExpectedMetrics) {
		result.Passed = false
		result.FailureReasons = append(result.FailureReasons,
			"Metrics did not meet expectations")
	}

	return result, nil
}

// executeTurn simulates a single turn in the session.
func (s *SessionSimulator) executeTurn(ctx context.Context, turn *Turn) error {
	// Create session context for this turn
	sessionCtx := &types.SessionContext{
		Turn:        turn.TurnID,
		Intent:      turn.Intent,
		FilesInContext: turn.Metadata.FilesReferenced,
	}

	// Measure compression
	compressionStart := time.Now()

	// Simulate compression: Turn → Facts
	// In real implementation, this would call the actual Compressor
	// For now, we'll use the kernel's fact store
	if s.config.CompressionEnabled {
		// TODO: Call actual compression logic
		// facts := s.compressor.Compress(turn)
		// s.kernel.AddFacts(facts)
	}

	compressionLatency := time.Since(compressionStart)
	s.metrics.RecordCompression(len(turn.Message), 0, compressionLatency) // TODO: measure compressed size

	// Measure retrieval
	retrievalStart := time.Now()

	// Simulate spreading activation retrieval
	// In real implementation, this would call the actual Activation engine
	if turn.Metadata.IsQuestionReferringBack {
		// TODO: Call actual spreading activation
		// facts := s.kernel.SpreadingActivation(turn.Message, s.config.TokenBudget)
	}

	retrievalLatency := time.Since(retrievalStart)
	s.metrics.RecordRetrieval(0, 0, retrievalLatency) // TODO: measure precision/recall

	return nil
}

// validateCheckpoint tests retrieval accuracy at a checkpoint.
func (s *SessionSimulator) validateCheckpoint(ctx context.Context, checkpoint *Checkpoint) CheckpointResult {
	result := CheckpointResult{
		Checkpoint: checkpoint,
		Passed:     true,
	}

	// TODO: Execute actual spreading activation with checkpoint.Query
	// For now, return mock result
	// retrieved := s.kernel.Query(checkpoint.Query, s.config.TokenBudget)

	retrieved := []string{} // TODO: replace with actual retrieval
	result.Retrieved = retrieved

	// Calculate precision and recall
	mustRetrieveSet := toSet(checkpoint.MustRetrieve)
	shouldAvoidSet := toSet(checkpoint.ShouldAvoid)
	retrievedSet := toSet(retrieved)

	// Missing required facts
	result.MissingRequired = setDifference(mustRetrieveSet, retrievedSet)

	// Unwanted noise
	result.UnwantedNoise = setIntersection(shouldAvoidSet, retrievedSet)

	// Calculate metrics
	if len(checkpoint.MustRetrieve) > 0 {
		result.Recall = 1.0 - (float64(len(result.MissingRequired)) / float64(len(checkpoint.MustRetrieve)))
	} else {
		result.Recall = 1.0
	}

	relevantRetrieved := len(checkpoint.MustRetrieve) - len(result.MissingRequired)
	totalRetrieved := len(retrieved)

	if totalRetrieved > 0 {
		result.Precision = float64(relevantRetrieved) / float64(totalRetrieved)
	} else {
		result.Precision = 0.0
	}

	if result.Precision+result.Recall > 0 {
		result.F1Score = 2 * (result.Precision * result.Recall) / (result.Precision + result.Recall)
	}

	// Check if metrics meet thresholds
	if result.Recall < checkpoint.MinRecall {
		result.Passed = false
		result.FailureReason = fmt.Sprintf("Recall %.2f < required %.2f", result.Recall, checkpoint.MinRecall)
	}

	if result.Precision < checkpoint.MinPrecision {
		result.Passed = false
		if result.FailureReason != "" {
			result.FailureReason += "; "
		}
		result.FailureReason += fmt.Sprintf("Precision %.2f < required %.2f", result.Precision, checkpoint.MinPrecision)
	}

	return result
}

// meetsExpectations checks if actual metrics meet expected thresholds.
func (s *SessionSimulator) meetsExpectations(actual, expected *Metrics) bool {
	if actual.CompressionRatio < expected.CompressionRatio {
		return false
	}
	if actual.AvgRetrievalRecall < expected.AvgRetrievalRecall {
		return false
	}
	if actual.AvgRetrievalPrec < expected.AvgRetrievalPrec {
		return false
	}
	if actual.TokenBudgetViolations > expected.TokenBudgetViolations {
		return false
	}
	return true
}

// Helper functions for set operations
func toSet(items []string) map[string]bool {
	set := make(map[string]bool)
	for _, item := range items {
		set[item] = true
	}
	return set
}

func setDifference(a, b map[string]bool) []string {
	diff := make([]string, 0)
	for item := range a {
		if !b[item] {
			diff = append(diff, item)
		}
	}
	return diff
}

func setIntersection(a, b map[string]bool) []string {
	intersection := make([]string, 0)
	for item := range a {
		if b[item] {
			intersection = append(intersection, item)
		}
	}
	return intersection
}
