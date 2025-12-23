package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"

	_ "github.com/mattn/go-sqlite3"
)

// ToolStore persists tool execution results to SQLite for debugging and context.
// Separate from LocalStore to avoid bloating the knowledge base.
//
// Storage location: .nerd/tools.db
//
// Cleanup strategies:
// - Runtime hours: Keep N hours of accumulated runtime (not calendar time)
// - Size-based: Cap total storage, FIFO deletion when exceeded
// - LLM-based: Intelligent cleanup via /cleanup-tools --smart
type ToolStore struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// ToolExecution represents a single tool execution record.
type ToolExecution struct {
	ID               int64
	CallID           string
	SessionID        string
	ToolName         string
	Action           string // Original action that triggered the tool
	Input            string // Tool input/arguments (JSON)
	Result           string // Full result (no truncation)
	Error            string // Error message if failed
	Success          bool
	DurationMs       int64
	ResultSize       int
	CreatedAt        time.Time
	SessionRuntimeMs int64   // Accumulated session runtime when executed
	UsefulnessScore  float64 // LLM-assigned usefulness (0.0-1.0)
	LastReferenced   *time.Time
	ReferenceCount   int
}

// ToolStoreStats provides storage statistics.
type ToolStoreStats struct {
	TotalExecutions   int
	TotalSizeBytes    int64
	TotalRuntimeHours float64
	SuccessCount      int
	FailureCount      int
	ToolBreakdown     map[string]int // Count by tool name
}

// NewToolStore creates a new tool execution store at the given path.
func NewToolStore(dbPath string) (*ToolStore, error) {
	logging.StoreDebug("Initializing ToolStore at path: %s", dbPath)

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to create ToolStore directory %s: %v", dir, err)
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open ToolStore database at %s: %v", dbPath, err)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &ToolStore{db: db, dbPath: dbPath}
	if err := store.initialize(); err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to initialize ToolStore schema: %v", err)
		db.Close()
		return nil, err
	}

	logging.Store("ToolStore initialized at %s", dbPath)
	return store, nil
}

// initialize creates the database schema.
func (s *ToolStore) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tool_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		call_id TEXT UNIQUE NOT NULL,
		session_id TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		action TEXT,
		input TEXT,
		result TEXT NOT NULL,
		error TEXT,
		success INTEGER NOT NULL DEFAULT 1,
		duration_ms INTEGER NOT NULL,
		result_size INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		session_runtime_ms INTEGER,
		usefulness_score REAL DEFAULT 0.5,
		last_referenced DATETIME,
		reference_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_tool_executions_session ON tool_executions(session_id);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_tool ON tool_executions(tool_name);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_created ON tool_executions(created_at);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_usefulness ON tool_executions(usefulness_score);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_size ON tool_executions(result_size);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Store persists a tool execution record.
func (s *ToolStore) Store(exec ToolExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	successInt := 0
	if exec.Success {
		successInt = 1
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO tool_executions
		(call_id, session_id, tool_name, action, input, result, error, success,
		 duration_ms, result_size, session_runtime_ms, usefulness_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.CallID, exec.SessionID, exec.ToolName, exec.Action, exec.Input,
		exec.Result, exec.Error, successInt, exec.DurationMs, exec.ResultSize,
		exec.SessionRuntimeMs, exec.UsefulnessScore,
	)

	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to store tool execution %s: %v", exec.CallID, err)
		return err
	}

	logging.ToolsDebug("Stored tool execution: %s (tool=%s, size=%d bytes)", exec.CallID, exec.ToolName, exec.ResultSize)
	return nil
}

// GetByCallID retrieves a tool execution by its call ID.
func (s *ToolStore) GetByCallID(callID string) (*ToolExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`
		SELECT id, call_id, session_id, tool_name, action, input, result, error,
		       success, duration_ms, result_size, created_at, session_runtime_ms,
		       usefulness_score, last_referenced, reference_count
		FROM tool_executions WHERE call_id = ?`, callID)

	return s.scanExecution(row)
}

// GetBySession retrieves all tool executions for a session.
func (s *ToolStore) GetBySession(sessionID string) ([]ToolExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, call_id, session_id, tool_name, action, input, result, error,
		       success, duration_ms, result_size, created_at, session_runtime_ms,
		       usefulness_score, last_referenced, reference_count
		FROM tool_executions WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExecutions(rows)
}

// GetRecent retrieves the N most recent tool executions.
func (s *ToolStore) GetRecent(limit int) ([]ToolExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, call_id, session_id, tool_name, action, input, result, error,
		       success, duration_ms, result_size, created_at, session_runtime_ms,
		       usefulness_score, last_referenced, reference_count
		FROM tool_executions ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExecutions(rows)
}

// GetRecentByTool retrieves the N most recent executions of a specific tool.
func (s *ToolStore) GetRecentByTool(toolName string, limit int) ([]ToolExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, call_id, session_id, tool_name, action, input, result, error,
		       success, duration_ms, result_size, created_at, session_runtime_ms,
		       usefulness_score, last_referenced, reference_count
		FROM tool_executions WHERE tool_name = ? ORDER BY created_at DESC LIMIT ?`, toolName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExecutions(rows)
}

// IncrementReference updates reference tracking when a tool result is used.
func (s *ToolStore) IncrementReference(callID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE tool_executions
		SET reference_count = reference_count + 1,
		    last_referenced = CURRENT_TIMESTAMP
		WHERE call_id = ?`, callID)

	return err
}

// GetStats returns storage statistics.
func (s *ToolStore) GetStats() (*ToolStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ToolStoreStats{
		ToolBreakdown: make(map[string]int),
	}

	// Total count and size
	row := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(result_size), 0),
		       SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END),
		       SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END)
		FROM tool_executions`)
	if err := row.Scan(&stats.TotalExecutions, &stats.TotalSizeBytes,
		&stats.SuccessCount, &stats.FailureCount); err != nil {
		return nil, err
	}

	// Runtime hours (sum of max runtime per session)
	row = s.db.QueryRow(`
		SELECT COALESCE(SUM(max_runtime) / 3600000.0, 0) FROM (
			SELECT MAX(session_runtime_ms) as max_runtime
			FROM tool_executions GROUP BY session_id
		)`)
	if err := row.Scan(&stats.TotalRuntimeHours); err != nil {
		return nil, err
	}

	// Tool breakdown
	rows, err := s.db.Query(`SELECT tool_name, COUNT(*) FROM tool_executions GROUP BY tool_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			continue
		}
		stats.ToolBreakdown[name] = count
	}

	return stats, nil
}

// Close closes the database connection.
func (s *ToolStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		logging.Store("Closing ToolStore at %s", s.dbPath)
		return s.db.Close()
	}
	return nil
}

// scanExecution scans a single row into a ToolExecution.
func (s *ToolStore) scanExecution(row *sql.Row) (*ToolExecution, error) {
	var exec ToolExecution
	var successInt int
	var createdAt string
	var lastRefStr sql.NullString

	err := row.Scan(
		&exec.ID, &exec.CallID, &exec.SessionID, &exec.ToolName,
		&exec.Action, &exec.Input, &exec.Result, &exec.Error,
		&successInt, &exec.DurationMs, &exec.ResultSize, &createdAt,
		&exec.SessionRuntimeMs, &exec.UsefulnessScore, &lastRefStr,
		&exec.ReferenceCount,
	)
	if err != nil {
		return nil, err
	}

	exec.Success = successInt == 1
	exec.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	if lastRefStr.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", lastRefStr.String)
		exec.LastReferenced = &t
	}

	return &exec, nil
}

// scanExecutions scans multiple rows into ToolExecution slice.
func (s *ToolStore) scanExecutions(rows *sql.Rows) ([]ToolExecution, error) {
	var executions []ToolExecution

	for rows.Next() {
		var exec ToolExecution
		var successInt int
		var createdAt string
		var lastRefStr sql.NullString

		err := rows.Scan(
			&exec.ID, &exec.CallID, &exec.SessionID, &exec.ToolName,
			&exec.Action, &exec.Input, &exec.Result, &exec.Error,
			&successInt, &exec.DurationMs, &exec.ResultSize, &createdAt,
			&exec.SessionRuntimeMs, &exec.UsefulnessScore, &lastRefStr,
			&exec.ReferenceCount,
		)
		if err != nil {
			continue
		}

		exec.Success = successInt == 1
		exec.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if lastRefStr.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", lastRefStr.String)
			exec.LastReferenced = &t
		}

		executions = append(executions, exec)
	}

	return executions, nil
}
