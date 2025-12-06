package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTraceStore_StoreAndRetrieve(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create test trace
	trace := &ReasoningTrace{
		ID:            "trace_001",
		ShardID:       "shard_coder_123",
		ShardType:     "coder",
		ShardCategory: "ephemeral",
		SessionID:     "session_001",
		TaskContext:   "Fix authentication bug in login handler",
		SystemPrompt:  "You are a code fixing agent.",
		UserPrompt:    "Fix the authentication logic",
		Response:      "Fixed by updating password hash comparison",
		Model:         "claude-sonnet-4",
		TokensUsed:    1500,
		DurationMs:    2500,
		Success:       true,
		QualityScore:  0.9,
		LearningNotes: []string{"Use constant-time comparison", "Check for null values"},
		CreatedAt:     time.Now(),
	}

	// Store the trace
	if err := traceStore.StoreReasoningTrace(trace); err != nil {
		t.Fatalf("Failed to store trace: %v", err)
	}

	// Retrieve by shard type
	traces, err := traceStore.GetShardTraces("coder", 10)
	if err != nil {
		t.Fatalf("Failed to get shard traces: %v", err)
	}

	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace, got %d", len(traces))
	}

	retrieved := traces[0]
	if retrieved.ID != trace.ID {
		t.Errorf("Expected ID %s, got %s", trace.ID, retrieved.ID)
	}
	if retrieved.ShardType != "coder" {
		t.Errorf("Expected shard type 'coder', got %s", retrieved.ShardType)
	}
	if !retrieved.Success {
		t.Error("Expected success=true")
	}
}

func TestTraceStore_FailedTraces(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create successful trace
	successTrace := &ReasoningTrace{
		ID:            "trace_success",
		ShardID:       "shard_001",
		ShardType:     "tester",
		ShardCategory: "ephemeral",
		SessionID:     "session_001",
		TaskContext:   "Run unit tests",
		SystemPrompt:  "You are a testing agent.",
		UserPrompt:    "Run the tests",
		Response:      "All tests passed",
		Success:       true,
		DurationMs:    1000,
		CreatedAt:     time.Now(),
	}

	// Create failed trace
	failedTrace := &ReasoningTrace{
		ID:            "trace_failed",
		ShardID:       "shard_002",
		ShardType:     "tester",
		ShardCategory: "ephemeral",
		SessionID:     "session_001",
		TaskContext:   "Run integration tests",
		SystemPrompt:  "You are a testing agent.",
		UserPrompt:    "Run the integration tests",
		Response:      "Tests failed",
		Success:       false,
		ErrorMessage:  "Database connection timeout",
		DurationMs:    5000,
		CreatedAt:     time.Now(),
	}

	traceStore.StoreReasoningTrace(successTrace)
	traceStore.StoreReasoningTrace(failedTrace)

	// Get failed traces
	failed, err := traceStore.GetFailedShardTraces("tester", 10)
	if err != nil {
		t.Fatalf("Failed to get failed traces: %v", err)
	}

	if len(failed) != 1 {
		t.Fatalf("Expected 1 failed trace, got %d", len(failed))
	}

	if failed[0].ID != "trace_failed" {
		t.Errorf("Expected failed trace ID, got %s", failed[0].ID)
	}
	if failed[0].ErrorMessage != "Database connection timeout" {
		t.Errorf("Expected error message, got %s", failed[0].ErrorMessage)
	}
}

func TestTraceStore_HighQualityTraces(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create traces with different quality scores
	traces := []*ReasoningTrace{
		{
			ID:           "trace_low",
			ShardType:    "coder",
			ShardID:      "shard_001",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "Low quality",
			Success:      true,
			QualityScore: 0.5,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		},
		{
			ID:           "trace_high",
			ShardType:    "coder",
			ShardID:      "shard_002",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "High quality",
			Success:      true,
			QualityScore: 0.95,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		},
	}

	for _, trace := range traces {
		traceStore.StoreReasoningTrace(trace)
	}

	// Get high quality traces (>= 0.8)
	highQuality, err := traceStore.GetHighQualityTraces("coder", 0.8, 10)
	if err != nil {
		t.Fatalf("Failed to get high quality traces: %v", err)
	}

	if len(highQuality) != 1 {
		t.Fatalf("Expected 1 high quality trace, got %d", len(highQuality))
	}

	if highQuality[0].ID != "trace_high" {
		t.Errorf("Expected trace_high, got %s", highQuality[0].ID)
	}
}

func TestTraceStore_UpdateQuality(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create initial trace
	trace := &ReasoningTrace{
		ID:           "trace_001",
		ShardID:      "shard_001",
		ShardType:    "coder",
		SessionID:    "session_001",
		SystemPrompt: "test",
		UserPrompt:   "test",
		Response:     "test",
		Success:      true,
		QualityScore: 0.5,
		DurationMs:   1000,
		CreatedAt:    time.Now(),
	}

	traceStore.StoreReasoningTrace(trace)

	// Update quality
	learningNotes := []string{"Excellent approach", "Could improve error handling"}
	err = traceStore.UpdateTraceQuality("trace_001", 0.95, learningNotes)
	if err != nil {
		t.Fatalf("Failed to update quality: %v", err)
	}

	// Verify update
	traces, _ := traceStore.GetShardTraces("coder", 1)
	if len(traces) == 0 {
		t.Fatal("No traces found after update")
	}

	updated := traces[0]
	if updated.QualityScore != 0.95 {
		t.Errorf("Expected quality score 0.95, got %f", updated.QualityScore)
	}
	if len(updated.LearningNotes) != 2 {
		t.Errorf("Expected 2 learning notes, got %d", len(updated.LearningNotes))
	}
}

func TestTraceStore_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create multiple traces
	for i := 0; i < 5; i++ {
		trace := &ReasoningTrace{
			ID:            string(rune('a' + i)),
			ShardID:       "shard_001",
			ShardType:     "coder",
			ShardCategory: "ephemeral",
			SessionID:     "session_001",
			SystemPrompt:  "test",
			UserPrompt:    "test",
			Response:      "test",
			Success:       i%2 == 0, // 3 success, 2 failure
			DurationMs:    int64(1000 * (i + 1)),
			CreatedAt:     time.Now(),
		}
		traceStore.StoreReasoningTrace(trace)
	}

	// Get stats
	stats, err := traceStore.GetTraceStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	totalTraces, ok := stats["total_traces"].(int64)
	if !ok || totalTraces != 5 {
		t.Errorf("Expected 5 total traces, got %v", stats["total_traces"])
	}

	successRate, ok := stats["success_rate"].(float64)
	if !ok || successRate != 0.6 { // 3/5 = 0.6
		t.Errorf("Expected success rate 0.6, got %v", stats["success_rate"])
	}
}

func TestTraceStore_SimilarTasks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create traces with different task contexts
	traces := []*ReasoningTrace{
		{
			ID:           "trace_auth_1",
			ShardType:    "coder",
			ShardID:      "shard_001",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "test",
			TaskContext:  "Fix authentication bug in login handler",
			Success:      true,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		},
		{
			ID:           "trace_auth_2",
			ShardType:    "coder",
			ShardID:      "shard_002",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "test",
			TaskContext:  "Update authentication token expiry",
			Success:      true,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		},
		{
			ID:           "trace_ui",
			ShardType:    "coder",
			ShardID:      "shard_003",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "test",
			TaskContext:  "Refactor UI component styling",
			Success:      true,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		},
	}

	for _, trace := range traces {
		traceStore.StoreReasoningTrace(trace)
	}

	// Find authentication-related tasks
	similar, err := traceStore.GetSimilarTaskTraces("coder", "authentication", 10)
	if err != nil {
		t.Fatalf("Failed to get similar tasks: %v", err)
	}

	if len(similar) != 2 {
		t.Fatalf("Expected 2 authentication traces, got %d", len(similar))
	}

	// Verify both are authentication-related
	for _, trace := range similar {
		if trace.ID != "trace_auth_1" && trace.ID != "trace_auth_2" {
			t.Errorf("Unexpected trace ID in results: %s", trace.ID)
		}
	}
}

func TestTraceStore_CleanupOldTraces(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()
	db := store.db

	// Insert old trace directly with SQL to set proper created_at timestamp
	oldTimestamp := time.Now().AddDate(0, 0, -40).Format("2006-01-02 15:04:05")
	_, err = db.Exec(`
		INSERT INTO reasoning_traces
		(id, shard_id, shard_type, shard_category, session_id, system_prompt, user_prompt, response, success, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"trace_old", "shard_001", "coder", "ephemeral", "session_001", "test", "test", "test", true, 1000, oldTimestamp)
	if err != nil {
		t.Fatalf("Failed to insert old trace: %v", err)
	}

	// Insert recent trace normally
	recentTrace := &ReasoningTrace{
		ID:           "trace_recent",
		ShardType:    "coder",
		ShardID:      "shard_002",
		SessionID:    "session_001",
		ShardCategory: "ephemeral",
		SystemPrompt: "test",
		UserPrompt:   "test",
		Response:     "test",
		Success:      true,
		DurationMs:   1000,
		CreatedAt:    time.Now(),
	}
	traceStore.StoreReasoningTrace(recentTrace)

	// Cleanup traces older than 30 days
	deleted, err := traceStore.CleanupOldTraces(30)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 trace deleted, got %d", deleted)
	}

	// Verify only recent trace remains
	traces, _ := traceStore.GetShardTraces("coder", 10)
	if len(traces) != 1 {
		t.Fatalf("Expected 1 trace remaining, got %d", len(traces))
	}

	if traces[0].ID != "trace_recent" {
		t.Errorf("Expected recent trace to remain, got %s", traces[0].ID)
	}
}

func TestTraceStore_GetFailurePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_traces.db")

	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	traceStore := store.GetTraceStore()

	// Create failed traces with different error patterns
	errors := []string{
		"Database connection timeout",
		"Database connection timeout",
		"File not found",
		"Permission denied",
		"Database connection timeout",
	}

	for i, errMsg := range errors {
		trace := &ReasoningTrace{
			ID:           string(rune('a' + i)),
			ShardType:    "coder",
			ShardID:      "shard_001",
			SessionID:    "session_001",
			SystemPrompt: "test",
			UserPrompt:   "test",
			Response:     "test",
			Success:      false,
			ErrorMessage: errMsg,
			DurationMs:   1000,
			CreatedAt:    time.Now(),
		}
		traceStore.StoreReasoningTrace(trace)
	}

	patterns, err := traceStore.GetFailurePatterns(10)
	if err != nil {
		t.Fatalf("Failed to get failure patterns: %v", err)
	}

	if patterns["Database connection timeout"] != 3 {
		t.Errorf("Expected 3 timeout errors, got %d", patterns["Database connection timeout"])
	}

	if patterns["File not found"] != 1 {
		t.Errorf("Expected 1 file not found error, got %d", patterns["File not found"])
	}
}
