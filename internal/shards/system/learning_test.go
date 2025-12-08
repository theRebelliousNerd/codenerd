// learning_test.go tests the autopoiesis learning infrastructure for system shards.
package system

import (
	"codenerd/internal/core"
	"codenerd/internal/store"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// learningStoreAdapter wraps store.LearningStore to implement core.LearningStore interface.
type learningStoreAdapter struct {
	store *store.LearningStore
}

func (a *learningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *learningStoreAdapter) Load(shardType string) ([]core.ShardLearning, error) {
	learnings, err := a.store.Load(shardType)
	if err != nil {
		return nil, err
	}
	result := make([]core.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = core.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
}

func (a *learningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]core.ShardLearning, error) {
	learnings, err := a.store.LoadByPredicate(shardType, predicate)
	if err != nil {
		return nil, err
	}
	result := make([]core.ShardLearning, len(learnings))
	for i, l := range learnings {
		result[i] = core.ShardLearning{
			FactPredicate: l.FactPredicate,
			FactArgs:      l.FactArgs,
			Confidence:    l.Confidence,
		}
	}
	return result, nil
}

func (a *learningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *learningStoreAdapter) Close() error {
	return a.store.Close()
}

func TestBaseSystemShard_LearningInfrastructure(t *testing.T) {
	// Create temp directory for test database
	tempDir := t.TempDir()
	learningPath := filepath.Join(tempDir, ".nerd", "shards")

	// Create learning store
	ls, err := store.NewLearningStore(learningPath)
	if err != nil {
		t.Fatalf("Failed to create learning store: %v", err)
	}
	defer ls.Close()

	// Create base shard with adapter
	adapter := &learningStoreAdapter{store: ls}
	base := NewBaseSystemShard("test_shard", StartupAuto)
	base.SetLearningStore(adapter)

	// Test tracking success
	base.trackSuccess("test_pattern_1")
	base.trackSuccess("test_pattern_1")
	base.trackSuccess("test_pattern_1")

	// Verify in-memory tracking
	if count, ok := base.patternSuccess["test_pattern_1"]; !ok || count != 3 {
		t.Errorf("Expected success count 3, got %d", count)
	}

	// Test tracking failure
	base.trackFailure("test_pattern_2", "error_reason")
	base.trackFailure("test_pattern_2", "error_reason")

	key := "test_pattern_2:error_reason"
	if count, ok := base.patternFailure[key]; !ok || count != 2 {
		t.Errorf("Expected failure count 2, got %d", count)
	}

	// Test tracking correction
	base.trackCorrection("original", "corrected")
	base.trackCorrection("original", "corrected")

	corrKey := "original→corrected"
	if count, ok := base.corrections[corrKey]; !ok || count != 2 {
		t.Errorf("Expected correction count 2, got %d", count)
	}

	// Test persistence
	err = base.persistLearning()
	if err != nil {
		t.Fatalf("Failed to persist learning: %v", err)
	}

	// Verify persistence by loading in new shard
	base2 := NewBaseSystemShard("test_shard", StartupAuto)
	base2.SetLearningStore(adapter)

	// Check that patterns were loaded
	if count, ok := base2.patternSuccess["test_pattern_1"]; !ok || count != 3 {
		t.Errorf("Expected loaded success count 3, got %d", count)
	}

	if count, ok := base2.patternFailure[key]; !ok || count != 3 {
		t.Errorf("Expected loaded failure count 3, got %d", count)
	}

	if count, ok := base2.corrections[corrKey]; !ok || count != 3 {
		t.Errorf("Expected loaded correction count 3, got %d", count)
	}
}

func TestPerceptionFirewall_LearningTracking(t *testing.T) {
	shard := NewPerceptionFirewallShard()

	// Track successful parse
	shard.trackSuccess("create:mutation")
	shard.trackSuccess("create:mutation")
	shard.trackSuccess("create:mutation")

	// Track ambiguous parse
	shard.trackFailure("ambiguous:search", "low_confidence")
	shard.trackFailure("ambiguous:search", "low_confidence")

	// Get learned patterns
	patterns := shard.GetLearnedPatterns()

	if len(patterns["successful"]) == 0 {
		t.Error("Expected successful patterns to be tracked")
	}

	if len(patterns["failed"]) == 0 {
		t.Error("Expected failed patterns to be tracked")
	}
}

func TestExecutivePolicy_OutcomeTracking(t *testing.T) {
	shard := NewExecutivePolicyShard()

	// Track successful action
	shard.RecordActionOutcome("write_file", "tdd_next_action", true, "")
	shard.RecordActionOutcome("write_file", "tdd_next_action", true, "")
	shard.RecordActionOutcome("write_file", "tdd_next_action", true, "")

	// Track failed action
	shard.RecordActionOutcome("deploy", "campaign_next_action", false, "permission_denied")
	shard.RecordActionOutcome("deploy", "campaign_next_action", false, "permission_denied")

	// Get learned patterns
	patterns := shard.GetLearnedPatterns()

	if len(patterns["successful"]) == 0 {
		t.Error("Expected successful patterns to be tracked")
	}

	if len(patterns["failed"]) == 0 {
		t.Error("Expected failed patterns to be tracked")
	}
}

func TestPerceptionFirewall_IntentCorrection(t *testing.T) {
	shard := NewPerceptionFirewallShard()

	// Simulate user correction
	original := Intent{
		Verb:     "search",
		Category: "query",
		Target:   "main.go",
	}

	corrected := Intent{
		Verb:     "explain",
		Category: "query",
		Target:   "main.go",
	}

	shard.RecordCorrection(original, corrected)
	shard.RecordCorrection(original, corrected)

	// Verify correction tracking
	patterns := shard.GetLearnedPatterns()
	if len(patterns["corrections"]) == 0 {
		t.Error("Expected corrections to be tracked")
	}
}

func TestLearningStore_Integration(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	learningPath := filepath.Join(tempDir, ".nerd", "shards")

	// Create learning store
	ls, err := store.NewLearningStore(learningPath)
	if err != nil {
		t.Fatalf("Failed to create learning store: %v", err)
	}
	defer ls.Close()

	// Create perception shard with learning
	adapter := &learningStoreAdapter{store: ls}
	perception := NewPerceptionFirewallShard()
	perception.SetLearningStore(adapter)

	// Track patterns
	perception.trackSuccess("explain:query")
	perception.trackSuccess("explain:query")
	perception.trackSuccess("explain:query")

	// Persist
	err = perception.persistLearning()
	if err != nil {
		t.Fatalf("Failed to persist: %v", err)
	}

	// Verify database file exists
	dbPath := filepath.Join(learningPath, "perception_firewall_learnings.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected learning database to be created")
	}

	// Load in new shard
	perception2 := NewPerceptionFirewallShard()
	perception2.SetLearningStore(adapter)

	// Verify patterns loaded
	if count, ok := perception2.patternSuccess["explain:query"]; !ok || count != 3 {
		t.Errorf("Expected pattern to be loaded with count 3, got %d", count)
	}
}
