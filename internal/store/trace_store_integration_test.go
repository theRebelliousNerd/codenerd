//go:build integration
package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/store"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/suite"
)

type TraceStoreSuite struct {
	suite.Suite
	dbPath  string
	db      *sql.DB
	store   *store.TraceStore
	tempDir string
}

func (s *TraceStoreSuite) SetupSuite() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "trace_store_integration_test")
	s.Require().NoError(err)

	s.dbPath = filepath.Join(s.tempDir, "traces.db")
	s.db, err = sql.Open("sqlite3", s.dbPath)
	s.Require().NoError(err)

	// Initialize the store
	s.store, err = store.NewTraceStore(s.db, s.dbPath)
	s.Require().NoError(err)
}

func (s *TraceStoreSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
	if s.tempDir != "" {
		os.RemoveAll(s.tempDir)
	}
}

func (s *TraceStoreSuite) SetupTest() {
	// Clean up table before each test
	_, err := s.db.Exec("DELETE FROM reasoning_traces")
	s.Require().NoError(err)
}

func (s *TraceStoreSuite) TestStoreAndRetrieveTrace() {
	trace := &store.ReasoningTrace{
		ID:            "trace-1",
		ShardID:       "shard-a",
		ShardType:     "planner",
		ShardCategory: "system",
		SessionID:     "session-1",
		TaskContext:   "plan the day",
		SystemPrompt:  "be helpful",
		UserPrompt:    "what to do?",
		Response:      "sleep",
		Model:         "gpt-4",
		TokensUsed:    100,
		DurationMs:    500,
		Success:       true,
		QualityScore:  0.9,
		CreatedAt:     time.Now().UTC(),
	}

	err := s.store.StoreReasoningTrace(trace)
	s.Require().NoError(err)

	// Retrieve specifically by session
	traces, err := s.store.GetTracesBySession("session-1")
	s.Require().NoError(err)
	s.Require().Len(traces, 1)

	retrieved := traces[0]
	s.Equal(trace.ID, retrieved.ID)
	s.Equal(trace.ShardType, retrieved.ShardType)
	s.Equal(trace.Response, retrieved.Response)
	s.Equal(trace.Success, retrieved.Success)
}

func (s *TraceStoreSuite) TestGetShardTraces() {
	// Insert traces for different shards
	s.insertTrace("t1", "shard-a", "planner", true, 0.8)
	s.insertTrace("t2", "shard-a", "planner", true, 0.9)
	s.insertTrace("t3", "shard-b", "coder", true, 0.95)

	traces, err := s.store.GetShardTraces("planner", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 2)
	for _, t := range traces {
		s.Equal("planner", t.ShardType)
	}
}

func (s *TraceStoreSuite) TestGetFailedShardTraces() {
	s.insertTrace("t1", "shard-a", "planner", true, 0.8)
	s.insertTrace("t2", "shard-a", "planner", false, 0.0)
	s.insertTrace("t3", "shard-a", "planner", false, 0.0)

	traces, err := s.store.GetFailedShardTraces("planner", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 2)
	for _, t := range traces {
		s.False(t.Success)
	}
}

func (s *TraceStoreSuite) TestGetSimilarTaskTraces() {
	s.insertTraceWithContext("t1", "planner", "write golang code")
	s.insertTraceWithContext("t2", "planner", "write python code")
	s.insertTraceWithContext("t3", "planner", "debug java")

	traces, err := s.store.GetSimilarTaskTraces("planner", "code", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 2) // golang code, python code

	traces, err = s.store.GetSimilarTaskTraces("planner", "golang", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 1)
}

func (s *TraceStoreSuite) TestGetHighQualityTraces() {
	s.insertTrace("t1", "shard-a", "planner", true, 0.5)
	s.insertTrace("t2", "shard-a", "planner", true, 0.9)
	s.insertTrace("t3", "shard-a", "planner", true, 0.95)
	s.insertTrace("t4", "shard-a", "planner", false, 0.0) // Failed shouldn't count

	traces, err := s.store.GetHighQualityTraces("planner", 0.9, 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 2)
	s.True(traces[0].QualityScore >= 0.9)
	s.True(traces[1].QualityScore >= 0.9)
}

func (s *TraceStoreSuite) TestUpdateTraceQuality() {
	s.insertTrace("t1", "shard-a", "planner", true, 0.5)

	err := s.store.UpdateTraceQuality("t1", 0.99, []string{"great job"})
	s.Require().NoError(err)

	traces, err := s.store.GetTracesBySession("session-default")
	s.Require().NoError(err)
	s.Require().Len(traces, 1)
	s.Equal(0.99, traces[0].QualityScore)
	s.Len(traces[0].LearningNotes, 1)
	s.Equal("great job", traces[0].LearningNotes[0])
}

func (s *TraceStoreSuite) TestGetTraceStats() {
	s.insertTrace("t1", "shard-a", "planner", true, 1.0)
	s.insertTrace("t2", "shard-b", "coder", false, 0.0)

	stats, err := s.store.GetTraceStats()
	s.Require().NoError(err)

	s.Equal(int64(2), stats["total_traces"])
	s.Equal(0.5, stats["success_rate"]) // 1 success out of 2
}

func (s *TraceStoreSuite) TestCleanupOldTraces() {
	s.insertTrace("t1", "shard-a", "planner", true, 1.0)
	s.insertTrace("t2", "shard-a", "planner", true, 1.0)

	// Manually age t1
	oldTime := time.Now().AddDate(0, 0, -10).Format("2006-01-02 15:04:05")
	_, err := s.db.Exec("UPDATE reasoning_traces SET created_at = ? WHERE id = 't1'", oldTime)
	s.Require().NoError(err)

	// Cleanup older than 5 days
	deleted, err := s.store.CleanupOldTraces(5)
	s.Require().NoError(err)
	s.Equal(int64(1), deleted)

	traces, err := s.store.GetTracesBySession("session-default")
	s.Require().NoError(err)
	s.Require().Len(traces, 1)
	s.Equal("t2", traces[0].ID)
}

func (s *TraceStoreSuite) TestGetRecentTraces() {
	// Insert 3 traces
	s.insertTrace("t1", "shard-a", "planner", true, 0.8)
	// Sleep briefly to ensure timestamps differ if needed, but DB uses CURRENT_TIMESTAMP
	// which is in seconds. So we might need to manually adjust if we want strict ordering verification
	// or rely on insert order if resolution is high enough.
	// Actually sqlite CURRENT_TIMESTAMP is seconds.
	// Let's manually set created_at for deterministic ordering
	s.insertTrace("t2", "shard-b", "coder", true, 0.9)
	s.insertTrace("t3", "shard-c", "reviewer", true, 0.95)

	// Update timestamps to be sequential
	s.db.Exec("UPDATE reasoning_traces SET created_at = ? WHERE id = 't1'", time.Now().Add(-3*time.Second).Format("2006-01-02 15:04:05"))
	s.db.Exec("UPDATE reasoning_traces SET created_at = ? WHERE id = 't2'", time.Now().Add(-2*time.Second).Format("2006-01-02 15:04:05"))
	s.db.Exec("UPDATE reasoning_traces SET created_at = ? WHERE id = 't3'", time.Now().Add(-1*time.Second).Format("2006-01-02 15:04:05"))

	traces, err := s.store.GetRecentTraces(2)
	s.Require().NoError(err)
	s.Require().Len(traces, 2)
	s.Equal("t3", traces[0].ID) // Most recent first
	s.Equal("t2", traces[1].ID)
}

func (s *TraceStoreSuite) TestGetTracesByCategory() {
	s.insertTraceWithCategory("t1", "system")
	s.insertTraceWithCategory("t2", "system")
	s.insertTraceWithCategory("t3", "ephemeral")

	traces, err := s.store.GetTracesByCategory("system", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 2)
	for _, t := range traces {
		s.Equal("system", t.ShardCategory)
	}

	traces, err = s.store.GetTracesByCategory("ephemeral", 10)
	s.Require().NoError(err)
	s.Require().Len(traces, 1)
}

func (s *TraceStoreSuite) TestGetLearningInsights() {
	// Insert traces to generate insights
	// 5 successful traces, 5 failed traces
	for i := 0; i < 5; i++ {
		s.insertTrace("s"+string(rune('0'+i)), "shard-a", "planner", true, 1.0)
	}
	for i := 0; i < 5; i++ {
		s.insertTrace("f"+string(rune('0'+i)), "shard-a", "planner", false, 0.0)
	}

	insights, err := s.store.GetLearningInsights("planner", 7)
	s.Require().NoError(err)

	s.Equal(int64(10), insights["recent_trace_count"])
	s.Equal(0.5, insights["recent_success_rate"]) // 50% success
}

// Helper to insert a simple trace
func (s *TraceStoreSuite) insertTrace(id, shardID, shardType string, success bool, score float64) {
	trace := &store.ReasoningTrace{
		ID:            id,
		ShardID:       shardID,
		ShardType:     shardType,
		ShardCategory: "system",
		SessionID:     "session-default",
		TaskContext:   "context",
		SystemPrompt:  "sys",
		UserPrompt:    "user",
		Response:      "resp",
		Success:       success,
		QualityScore:  score,
		DurationMs:    100,
	}
	s.Require().NoError(s.store.StoreReasoningTrace(trace))
}

func (s *TraceStoreSuite) insertTraceWithContext(id, shardType, context string) {
	trace := &store.ReasoningTrace{
		ID:            id,
		ShardID:       "shard-x",
		ShardType:     shardType,
		ShardCategory: "system",
		SessionID:     "session-default",
		TaskContext:   context,
		SystemPrompt:  "sys",
		UserPrompt:    "user",
		Response:      "resp",
		Success:       true,
		DurationMs:    100,
	}
	s.Require().NoError(s.store.StoreReasoningTrace(trace))
}

func (s *TraceStoreSuite) insertTraceWithCategory(id, category string) {
	trace := &store.ReasoningTrace{
		ID:            id,
		ShardID:       "shard-x",
		ShardType:     "planner",
		ShardCategory: category,
		SessionID:     "session-default",
		TaskContext:   "context",
		SystemPrompt:  "sys",
		UserPrompt:    "user",
		Response:      "resp",
		Success:       true,
		DurationMs:    100,
	}
	s.Require().NoError(s.store.StoreReasoningTrace(trace))
}

func TestTraceStoreSuite(t *testing.T) {
	suite.Run(t, new(TraceStoreSuite))
}
