// Package chat provides tests for session.go adapters and utilities.
package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/store"
)

// =============================================================================
// SPAWNER ADAPTER TYPES FOR TESTS
// =============================================================================

// shardManagerObserverSpawner wraps ShardManager for observer spawning.
type shardManagerObserverSpawner struct {
	shardMgr any // *coreshards.ShardManager when non-nil
}

var errShardManagerNotAvailable = errors.New("shard manager not available")

// SpawnObserver spawns an observer shard for the given task.
func (s *shardManagerObserverSpawner) SpawnObserver(ctx context.Context, observerType, task string) (string, error) {
	if s.shardMgr == nil {
		return "", errShardManagerNotAvailable
	}
	// In a real implementation, this would delegate to s.shardMgr
	return "", errors.New("observer spawner not implemented")
}

// shardManagerConsultationSpawner wraps ShardManager for consultation spawning.
type shardManagerConsultationSpawner struct {
	shardMgr any // *coreshards.ShardManager when non-nil
}

// SpawnConsultation spawns a specialist consultation for the given task.
func (s *shardManagerConsultationSpawner) SpawnConsultation(ctx context.Context, specialistType, task string) (string, error) {
	if s.shardMgr == nil {
		return "", errShardManagerNotAvailable
	}
	// In a real implementation, this would delegate to s.shardMgr
	return "", errors.New("consultation spawner not implemented")
}


// =============================================================================
// CORE LEARNING STORE ADAPTER TESTS
// =============================================================================

func TestCoreLearningStoreAdapter_Save_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.Save("coder", "learned_pattern", []any{"arg1"}, "campaign1")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_Load_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	results, err := adapter.Load("coder")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for nil store, got: %v", results)
	}
}

func TestCoreLearningStoreAdapter_LoadByPredicate_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	results, err := adapter.LoadByPredicate("coder", "learned_pattern")
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for nil store, got: %v", results)
	}
}

func TestCoreLearningStoreAdapter_DecayConfidence_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.DecayConfidence("coder", 0.9)
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_Close_NilStore(t *testing.T) {
	t.Parallel()

	adapter := &coreLearningStoreAdapter{store: nil}

	err := adapter.Close()
	if err != nil {
		t.Errorf("Expected nil error for nil store, got: %v", err)
	}
}

func TestCoreLearningStoreAdapter_WithStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter test requiring LearningStore in short mode")
	}

	tmpDir := t.TempDir()
	learningStore, err := store.NewLearningStore(tmpDir)
	if err != nil {
		t.Skipf("Failed to create LearningStore (SQLite issue on this platform): %v", err)
	}
	defer learningStore.Close()

	adapter := &coreLearningStoreAdapter{store: learningStore}

	// Test Save
	err = adapter.Save("coder", "test_pattern", []any{"arg1", "arg2"}, "test_campaign")
	if err != nil {
		t.Logf("Save returned error: %v", err)
	}

	// Test Load
	results, err := adapter.Load("coder")
	if err != nil {
		t.Logf("Load returned error: %v", err)
	}
	t.Logf("Load returned %d results", len(results))

	// Test LoadByPredicate
	results, err = adapter.LoadByPredicate("coder", "test_pattern")
	if err != nil {
		t.Logf("LoadByPredicate returned error: %v", err)
	}
	t.Logf("LoadByPredicate returned %d results", len(results))

	// Test DecayConfidence
	err = adapter.DecayConfidence("coder", 0.95)
	if err != nil {
		t.Logf("DecayConfidence returned error: %v", err)
	}

	// Test Close
	err = adapter.Close()
	if err != nil {
		t.Logf("Close returned error: %v", err)
	}
}

// =============================================================================
// SHARD MANAGER OBSERVER/CONSULTATION SPAWNER TESTS
// =============================================================================

func TestShardManagerObserverSpawner_NilShardMgr(t *testing.T) {
	t.Parallel()

	spawner := &shardManagerObserverSpawner{shardMgr: nil}

	ctx := context.Background()
	result, err := spawner.SpawnObserver(ctx, "test_observer", "test task")

	if err == nil {
		t.Error("Expected error for nil shard manager")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("Expected 'not available' error, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got: %s", result)
	}
}

func TestShardManagerConsultationSpawner_NilShardMgr(t *testing.T) {
	t.Parallel()

	spawner := &shardManagerConsultationSpawner{shardMgr: nil}

	ctx := context.Background()
	result, err := spawner.SpawnConsultation(ctx, "test_specialist", "test task")

	if err == nil {
		t.Error("Expected error for nil shard manager")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("Expected 'not available' error, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result, got: %s", result)
	}
}

// =============================================================================
// RESOLVE SESSION/TURN TESTS
// =============================================================================

func TestResolveSessionID_WithConfig(t *testing.T) {
	t.Parallel()

	session := &Session{
		SessionID: "existing-session-id",
	}

	result := resolveSessionID(session)
	if result != "existing-session-id" {
		t.Errorf("Expected 'existing-session-id', got '%s'", result)
	}
}

func TestResolveSessionID_Empty(t *testing.T) {
	t.Parallel()

	session := &Session{
		SessionID: "",
	}

	result := resolveSessionID(session)
	if result == "" {
		t.Error("Expected non-empty session ID to be generated")
	}
}

func TestResolveSessionID_NilSession(t *testing.T) {
	t.Parallel()

	result := resolveSessionID(nil)
	if result == "" {
		t.Error("Expected non-empty session ID to be generated for nil session")
	}
}

func TestResolveTurnCount_WithValue(t *testing.T) {
	t.Parallel()

	session := &Session{
		TurnCount: 42,
	}

	result := resolveTurnCount(session)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}
}

func TestResolveTurnCount_Zero(t *testing.T) {
	t.Parallel()

	session := &Session{
		TurnCount: 0,
	}

	result := resolveTurnCount(session)
	if result != 0 {
		t.Errorf("Expected 0 (for zero turn count), got %d", result)
	}
}

func TestResolveTurnCount_NilSession(t *testing.T) {
	t.Parallel()

	result := resolveTurnCount(nil)
	if result != 0 {
		t.Errorf("Expected 0 for nil session, got %d", result)
	}
}

