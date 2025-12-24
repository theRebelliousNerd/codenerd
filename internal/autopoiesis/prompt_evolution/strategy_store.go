package prompt_evolution

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"

	"github.com/google/uuid"
)

// StrategyStore manages the database of problem-solving strategies.
// This implements the SPL (System Prompt Learning) pattern where
// strategies are learned, selected, and refined based on execution outcomes.
type StrategyStore struct {
	mu        sync.RWMutex
	db        *sql.DB
	storePath string

	// Cache for frequently used strategies
	cache     map[string][]*Strategy // key: problemType:shardType
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewStrategyStore creates a new strategy store.
func NewStrategyStore(nerdDir string) (*StrategyStore, error) {
	storePath := filepath.Join(nerdDir, "prompts", "strategies.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create strategies directory: %w", err)
	}

	db, err := sql.Open("sqlite3", storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open strategies database: %w", err)
	}

	ss := &StrategyStore{
		db:        db,
		storePath: storePath,
		cache:     make(map[string][]*Strategy),
		cacheTTL:  5 * time.Minute,
	}

	if err := ss.ensureSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	logging.Autopoiesis("StrategyStore initialized: path=%s", storePath)
	return ss, nil
}

// ensureSchema creates the necessary tables.
func (ss *StrategyStore) ensureSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS strategies (
		id TEXT PRIMARY KEY,
		problem_type TEXT NOT NULL,
		shard_type TEXT NOT NULL,
		content TEXT NOT NULL,
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		success_rate REAL DEFAULT 0.5,
		last_used DATETIME,
		last_refined DATETIME,
		version INTEGER DEFAULT 1,
		source TEXT DEFAULT 'generated',
		embedding BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_strategies_type ON strategies(problem_type, shard_type);
	CREATE INDEX IF NOT EXISTS idx_strategies_success ON strategies(success_rate DESC);

	CREATE TABLE IF NOT EXISTS strategy_uses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		strategy_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		success INTEGER NOT NULL,
		used_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (strategy_id) REFERENCES strategies(id)
	);

	CREATE INDEX IF NOT EXISTS idx_uses_strategy ON strategy_uses(strategy_id);
	`

	_, err := ss.db.Exec(schema)
	return err
}

// CreateStrategy inserts a new strategy.
func (ss *StrategyStore) CreateStrategy(strategy *Strategy) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if strategy.ID == "" {
		strategy.ID = uuid.New().String()
	}
	if strategy.CreatedAt.IsZero() {
		strategy.CreatedAt = time.Now()
	}
	if strategy.Version == 0 {
		strategy.Version = 1
	}
	if strategy.Source == "" {
		strategy.Source = "generated"
	}

	_, err := ss.db.Exec(`
		INSERT INTO strategies
		(id, problem_type, shard_type, content, success_count, failure_count,
		 success_rate, last_used, last_refined, version, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strategy.ID, strategy.ProblemType, strategy.ShardType, strategy.Content,
		strategy.SuccessCount, strategy.FailureCount, strategy.SuccessRate,
		strategy.LastUsed, strategy.LastRefined, strategy.Version,
		strategy.Source, strategy.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Invalidate cache
	ss.invalidateCache(string(strategy.ProblemType), strategy.ShardType)

	logging.Autopoiesis("Strategy created: id=%s, problem=%s, shard=%s",
		strategy.ID, strategy.ProblemType, strategy.ShardType)

	return nil
}

// SelectStrategies returns the best strategies for a problem type and shard.
func (ss *StrategyStore) SelectStrategies(problemType ProblemType, shardType string, limit int) ([]*Strategy, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if limit <= 0 {
		limit = 3
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s:%s", problemType, shardType)
	if strategies, ok := ss.getCached(cacheKey); ok {
		if len(strategies) > limit {
			return strategies[:limit], nil
		}
		return strategies, nil
	}

	// Query database
	rows, err := ss.db.Query(`
		SELECT id, problem_type, shard_type, content, success_count, failure_count,
		       success_rate, last_used, last_refined, version, source, created_at
		FROM strategies
		WHERE problem_type = ? AND shard_type = ?
		ORDER BY success_rate DESC, success_count DESC
		LIMIT ?`, problemType, shardType, limit*2) // Fetch extra for cache

	if err != nil {
		return nil, fmt.Errorf("failed to select strategies: %w", err)
	}
	defer rows.Close()

	strategies := ss.scanStrategies(rows)

	// Update cache
	ss.setCache(cacheKey, strategies)

	if len(strategies) > limit {
		return strategies[:limit], nil
	}
	return strategies, nil
}

// RecordOutcome records the success or failure of a strategy use.
func (ss *StrategyStore) RecordOutcome(strategyID string, taskID string, success bool) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Record the use
	_, err := ss.db.Exec(`
		INSERT INTO strategy_uses (strategy_id, task_id, success)
		VALUES (?, ?, ?)`, strategyID, taskID, success)
	if err != nil {
		return fmt.Errorf("failed to record use: %w", err)
	}

	// Update strategy statistics
	var successDelta, failureDelta int
	if success {
		successDelta = 1
	} else {
		failureDelta = 1
	}

	_, err = ss.db.Exec(`
		UPDATE strategies
		SET success_count = success_count + ?,
		    failure_count = failure_count + ?,
		    success_rate = CAST(success_count + ? AS REAL) / (success_count + failure_count + 1),
		    last_used = ?
		WHERE id = ?`,
		successDelta, failureDelta, successDelta, time.Now(), strategyID)

	if err != nil {
		return fmt.Errorf("failed to update strategy: %w", err)
	}

	// Invalidate cache for this strategy
	ss.invalidateCacheForStrategy(strategyID)

	logging.AutopoiesisDebug("Strategy outcome recorded: id=%s, success=%v", strategyID, success)
	return nil
}

// RefineStrategy updates a strategy with improved content.
func (ss *StrategyStore) RefineStrategy(strategyID string, newContent string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	_, err := ss.db.Exec(`
		UPDATE strategies
		SET content = ?,
		    version = version + 1,
		    last_refined = ?,
		    source = 'evolved'
		WHERE id = ?`, newContent, time.Now(), strategyID)

	if err != nil {
		return fmt.Errorf("failed to refine strategy: %w", err)
	}

	// Invalidate cache for this strategy
	ss.invalidateCacheForStrategy(strategyID)

	logging.Autopoiesis("Strategy refined: id=%s", strategyID)
	return nil
}

// GetStrategy retrieves a strategy by ID.
func (ss *StrategyStore) GetStrategy(strategyID string) (*Strategy, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	row := ss.db.QueryRow(`
		SELECT id, problem_type, shard_type, content, success_count, failure_count,
		       success_rate, last_used, last_refined, version, source, created_at
		FROM strategies
		WHERE id = ?`, strategyID)

	var s Strategy
	var lastUsed, lastRefined, createdAt sql.NullTime

	err := row.Scan(
		&s.ID, &s.ProblemType, &s.ShardType, &s.Content,
		&s.SuccessCount, &s.FailureCount, &s.SuccessRate,
		&lastUsed, &lastRefined, &s.Version, &s.Source, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if lastUsed.Valid {
		s.LastUsed = lastUsed.Time
	}
	if lastRefined.Valid {
		s.LastRefined = lastRefined.Time
	}
	if createdAt.Valid {
		s.CreatedAt = createdAt.Time
	}

	return &s, nil
}

// GetStrategiesNeedingRefinement returns strategies that should be refined.
// This includes strategies with low success rates after significant use.
func (ss *StrategyStore) GetStrategiesNeedingRefinement(minUses, limit int) ([]*Strategy, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	rows, err := ss.db.Query(`
		SELECT id, problem_type, shard_type, content, success_count, failure_count,
		       success_rate, last_used, last_refined, version, source, created_at
		FROM strategies
		WHERE (success_count + failure_count) >= ?
		  AND success_rate < 0.6
		ORDER BY success_rate ASC, (success_count + failure_count) DESC
		LIMIT ?`, minUses, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ss.scanStrategies(rows), nil
}

// GetAllStrategies returns all strategies with optional filtering.
func (ss *StrategyStore) GetAllStrategies(problemType string, shardType string) ([]*Strategy, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, problem_type, shard_type, content, success_count, failure_count,
		       success_rate, last_used, last_refined, version, source, created_at
		FROM strategies
		WHERE 1=1`
	args := []interface{}{}

	if problemType != "" {
		query += " AND problem_type = ?"
		args = append(args, problemType)
	}
	if shardType != "" {
		query += " AND shard_type = ?"
		args = append(args, shardType)
	}

	query += " ORDER BY success_rate DESC"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ss.scanStrategies(rows), nil
}

// GetStats returns statistics about the strategy store.
func (ss *StrategyStore) GetStats() (total int, avgSuccessRate float64, err error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	row := ss.db.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(success_rate), 0.5)
		FROM strategies`)

	err = row.Scan(&total, &avgSuccessRate)
	return
}

// DeleteStrategy removes a strategy.
func (ss *StrategyStore) DeleteStrategy(strategyID string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Delete uses first
	_, err := ss.db.Exec(`DELETE FROM strategy_uses WHERE strategy_id = ?`, strategyID)
	if err != nil {
		return err
	}

	_, err = ss.db.Exec(`DELETE FROM strategies WHERE id = ?`, strategyID)
	if err != nil {
		return err
	}

	ss.invalidateCacheForStrategy(strategyID)
	logging.Autopoiesis("Strategy deleted: id=%s", strategyID)
	return nil
}

// scanStrategies scans rows into Strategy slices.
func (ss *StrategyStore) scanStrategies(rows *sql.Rows) []*Strategy {
	var strategies []*Strategy

	for rows.Next() {
		var s Strategy
		var lastUsed, lastRefined, createdAt sql.NullTime

		err := rows.Scan(
			&s.ID, &s.ProblemType, &s.ShardType, &s.Content,
			&s.SuccessCount, &s.FailureCount, &s.SuccessRate,
			&lastUsed, &lastRefined, &s.Version, &s.Source, &createdAt,
		)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to scan strategy: %v", err)
			continue
		}

		if lastUsed.Valid {
			s.LastUsed = lastUsed.Time
		}
		if lastRefined.Valid {
			s.LastRefined = lastRefined.Time
		}
		if createdAt.Valid {
			s.CreatedAt = createdAt.Time
		}

		strategies = append(strategies, &s)
	}

	return strategies
}

// Cache helpers
func (ss *StrategyStore) getCached(key string) ([]*Strategy, bool) {
	if time.Since(ss.cacheTime) > ss.cacheTTL {
		ss.cache = make(map[string][]*Strategy)
		return nil, false
	}
	strategies, ok := ss.cache[key]
	return strategies, ok
}

func (ss *StrategyStore) setCache(key string, strategies []*Strategy) {
	ss.cache[key] = strategies
	ss.cacheTime = time.Now()
}

func (ss *StrategyStore) invalidateCache(problemType, shardType string) {
	key := fmt.Sprintf("%s:%s", problemType, shardType)
	delete(ss.cache, key)
}

func (ss *StrategyStore) invalidateCacheForStrategy(strategyID string) {
	// Clear entire cache since we don't track which keys contain which strategies
	ss.cache = make(map[string][]*Strategy)
}

// Close closes the database connection.
func (ss *StrategyStore) Close() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.db != nil {
		return ss.db.Close()
	}
	return nil
}

// GenerateDefaultStrategies creates initial strategies for all problem types.
// This seeds the strategy database with baseline strategies.
func (ss *StrategyStore) GenerateDefaultStrategies() error {
	defaults := getDefaultStrategies()

	for _, s := range defaults {
		// Check if already exists
		existing, _ := ss.GetStrategy(s.ID)
		if existing != nil {
			continue
		}

		if err := ss.CreateStrategy(s); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to create default strategy: %v", err)
		}
	}

	logging.Autopoiesis("Default strategies seeded: %d strategies", len(defaults))
	return nil
}

// getDefaultStrategies returns baseline strategies for each problem type.
func getDefaultStrategies() []*Strategy {
	return []*Strategy{
		{
			ID:          "default-debugging-coder",
			ProblemType: ProblemDebugging,
			ShardType:   "/coder",
			Content: `**Debugging Strategy:**
1. First, reproduce the issue by understanding the exact steps
2. Read the relevant error messages and stack traces carefully
3. Identify the specific file and line where the error originates
4. Check for common issues: nil pointers, uninitialized variables, wrong types
5. Add targeted logging if needed to trace execution
6. Fix the root cause, not just the symptoms
7. Verify the fix doesn't break existing tests`,
			Source: "manual",
		},
		{
			ID:          "default-feature-coder",
			ProblemType: ProblemFeatureCreation,
			ShardType:   "/coder",
			Content: `**Feature Creation Strategy:**
1. Understand the full requirements before writing code
2. Check for existing similar patterns in the codebase
3. Design the interface/API first, implementation second
4. Follow existing code conventions and patterns
5. Write the core functionality first, then edge cases
6. Add appropriate error handling from the start
7. Include tests alongside the implementation`,
			Source: "manual",
		},
		{
			ID:          "default-refactoring-coder",
			ProblemType: ProblemRefactoring,
			ShardType:   "/coder",
			Content: `**Refactoring Strategy:**
1. Ensure tests exist before refactoring
2. Identify all callers/dependents of code being changed
3. Make incremental, testable changes
4. Maintain backwards compatibility unless explicitly breaking
5. Update all affected callers when changing interfaces
6. Run tests after each change
7. Document breaking changes clearly`,
			Source: "manual",
		},
		{
			ID:          "default-testing-tester",
			ProblemType: ProblemTesting,
			ShardType:   "/tester",
			Content: `**Testing Strategy:**
1. Understand what behavior needs to be tested
2. Write tests for the happy path first
3. Add edge cases and error scenarios
4. Use table-driven tests for multiple inputs
5. Mock external dependencies appropriately
6. Ensure tests are deterministic (no flaky tests)
7. Name tests clearly to describe what they verify`,
			Source: "manual",
		},
		{
			ID:          "default-concurrency-coder",
			ProblemType: ProblemConcurrency,
			ShardType:   "/coder",
			Content: `**Concurrency Strategy:**
1. Identify shared state that needs protection
2. Use the smallest scope of locking possible
3. Prefer channels over shared memory when possible
4. Always handle goroutine lifecycle (start, stop, cleanup)
5. Use context for cancellation and timeouts
6. Test with race detector enabled
7. Document concurrency guarantees in comments`,
			Source: "manual",
		},
		{
			ID:          "default-errorhandling-coder",
			ProblemType: ProblemErrorHandling,
			ShardType:   "/coder",
			Content: `**Error Handling Strategy:**
1. Check all error returns - never ignore errors
2. Wrap errors with context using fmt.Errorf
3. Return early on errors (guard clauses)
4. Log errors at the point of handling, not everywhere
5. Use sentinel errors for expected conditions
6. Handle all error cases, even unlikely ones
7. Make error messages actionable and specific`,
			Source: "manual",
		},
	}
}

// ExportStrategies exports all strategies as JSON.
func (ss *StrategyStore) ExportStrategies() (string, error) {
	strategies, err := ss.GetAllStrategies("", "")
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(strategies, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
