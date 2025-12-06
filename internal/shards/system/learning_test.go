// learning_test.go tests the autopoiesis learning infrastructure for system shards.
package system

import (
	"codenerd/internal/store"
	"os"
	"path/filepath"
	"testing"
)

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

	// Create base shard
	base := NewBaseSystemShard("test_shard", StartupAuto)
	base.SetLearningStore(ls)

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
	base2.SetLearningStore(ls)

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
	perception := NewPerceptionFirewallShard()
	perception.SetLearningStore(ls)

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
	perception2.SetLearningStore(ls)

	// Verify patterns loaded
	if count, ok := perception2.patternSuccess["explain:query"]; !ok || count != 3 {
		t.Errorf("Expected pattern to be loaded with count 3, got %d", count)
	}
}
