package store

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
)

// CleanupConfig configures automatic cleanup behavior.
type CleanupConfig struct {
	MaxRuntimeHours      float64 // Max runtime hours to keep (default: 336 = 14 days equivalent)
	MaxSizeBytes         int64   // Max storage size in bytes (default: 100MB)
	AutoCleanupThreshold float64 // Trigger cleanup at this % of max (default: 0.8)
	CleanupMode          string  // "runtime" or "size"
}

// DefaultCleanupConfig returns sensible defaults.
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		MaxRuntimeHours:      336,       // 14 days equivalent of runtime
		MaxSizeBytes:         104857600, // 100 MB
		AutoCleanupThreshold: 0.8,
		CleanupMode:          "runtime",
	}
}

// CleanupStats reports cleanup results.
type CleanupStats struct {
	ExecutionsDeleted int
	BytesFreed        int64
	RuntimeHoursFreed float64
	Method            string // Which strategy was used
}

// ShouldAutoCleanup returns true if cleanup should be triggered.
func (s *ToolStore) ShouldAutoCleanup(config CleanupConfig) bool {
	stats, err := s.GetStats()
	if err != nil {
		return false
	}

	switch config.CleanupMode {
	case "size":
		threshold := float64(config.MaxSizeBytes) * config.AutoCleanupThreshold
		return float64(stats.TotalSizeBytes) > threshold
	case "runtime":
		threshold := config.MaxRuntimeHours * config.AutoCleanupThreshold
		return stats.TotalRuntimeHours > threshold
	default:
		// Default to size-based
		threshold := float64(config.MaxSizeBytes) * config.AutoCleanupThreshold
		return float64(stats.TotalSizeBytes) > threshold
	}
}

// CleanupByRuntimeBudget deletes oldest executions to stay within runtime budget.
func (s *ToolStore) CleanupByRuntimeBudget(budgetHours float64) (*CleanupStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := &CleanupStats{Method: "runtime"}

	// Get current total runtime
	var currentHours float64
	row := s.db.QueryRow(`
		SELECT COALESCE(SUM(max_runtime) / 3600000.0, 0) FROM (
			SELECT MAX(session_runtime_ms) as max_runtime
			FROM tool_executions GROUP BY session_id
		)`)
	if err := row.Scan(&currentHours); err != nil {
		return nil, err
	}

	if currentHours <= budgetHours {
		logging.StoreDebug("CleanupByRuntimeBudget: within budget (%.1f <= %.1f hours)", currentHours, budgetHours)
		return stats, nil
	}

	hoursToFree := currentHours - budgetHours
	logging.Store("CleanupByRuntimeBudget: need to free %.1f hours (current: %.1f, budget: %.1f)",
		hoursToFree, currentHours, budgetHours)

	// Batch delete oldest sessions until under budget
	// We sort by MIN(created_at) to delete oldest sessions first.
	// We delete entire sessions because partial deletion doesn't reduce session_runtime_ms (max value).
	batchSize := 500
	for hoursToFree > 0 {
		// Identify sessions to delete
		rows, err := s.db.Query(`
			SELECT session_id, COALESCE(MAX(session_runtime_ms), 0), COUNT(*), COALESCE(SUM(result_size), 0)
			FROM tool_executions
			GROUP BY session_id
			ORDER BY MIN(created_at) ASC
			LIMIT ?`, batchSize)
		if err != nil {
			return stats, err
		}

		var sessionsToDelete []string

		for rows.Next() {
			var sessionID string
			var runtimeMs int64
			var count int
			var size int64

			if err := rows.Scan(&sessionID, &runtimeMs, &count, &size); err != nil {
				continue
			}

			sessionsToDelete = append(sessionsToDelete, sessionID)

			// Update stats
			freedHours := float64(runtimeMs) / 3600000.0
			stats.ExecutionsDeleted += count
			stats.BytesFreed += size
			stats.RuntimeHoursFreed += freedHours
			hoursToFree -= freedHours

			if hoursToFree <= 0 {
				break
			}
		}
		rows.Close()

		if len(sessionsToDelete) == 0 {
			break // No more sessions found
		}

		// Batch delete execution
		placeholders := strings.Repeat("?,", len(sessionsToDelete)-1) + "?"
		query := fmt.Sprintf("DELETE FROM tool_executions WHERE session_id IN (%s)", placeholders)

		args := make([]any, len(sessionsToDelete))
		for i, id := range sessionsToDelete {
			args[i] = id
		}

		_, err = s.db.Exec(query, args...)
		if err != nil {
			logging.Get(logging.CategoryStore).Error("CleanupByRuntimeBudget: failed to delete batch: %v", err)
			return stats, err
		}

		logging.Store("CleanupByRuntimeBudget: deleted batch of %d sessions", len(sessionsToDelete))
	}

	logging.Store("CleanupByRuntimeBudget: deleted %d executions, freed %d bytes",
		stats.ExecutionsDeleted, stats.BytesFreed)

	return stats, nil
}

// CleanupBySizeLimit deletes oldest executions to stay within size limit.
func (s *ToolStore) CleanupBySizeLimit(maxBytes int64) (*CleanupStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := &CleanupStats{Method: "size"}

	// Get current total size
	var currentSize int64
	row := s.db.QueryRow("SELECT COALESCE(SUM(result_size), 0) FROM tool_executions")
	if err := row.Scan(&currentSize); err != nil {
		return nil, err
	}

	if currentSize <= maxBytes {
		logging.StoreDebug("CleanupBySizeLimit: within budget (%d <= %d bytes)", currentSize, maxBytes)
		return stats, nil
	}

	bytesToFree := currentSize - maxBytes
	logging.Store("CleanupBySizeLimit: need to free %d bytes (current: %d, max: %d)",
		bytesToFree, currentSize, maxBytes)

	// Delete oldest executions until under limit
	for currentSize > maxBytes {
		// Get oldest execution
		var id int64
		var resultSize int
		row := s.db.QueryRow(`
			SELECT id, result_size
			FROM tool_executions
			ORDER BY created_at ASC LIMIT 1`)
		if err := row.Scan(&id, &resultSize); err != nil {
			break // No more rows
		}

		// Delete it
		_, err := s.db.Exec("DELETE FROM tool_executions WHERE id = ?", id)
		if err != nil {
			break
		}

		stats.ExecutionsDeleted++
		stats.BytesFreed += int64(resultSize)
		currentSize -= int64(resultSize)
	}

	logging.Store("CleanupBySizeLimit: deleted %d executions, freed %d bytes",
		stats.ExecutionsDeleted, stats.BytesFreed)

	return stats, nil
}

// AutoCleanup runs cleanup if thresholds are exceeded.
func (s *ToolStore) AutoCleanup(config CleanupConfig) (*CleanupStats, error) {
	if !s.ShouldAutoCleanup(config) {
		return &CleanupStats{}, nil
	}

	logging.Store("AutoCleanup triggered (mode=%s)", config.CleanupMode)

	switch config.CleanupMode {
	case "runtime":
		return s.CleanupByRuntimeBudget(config.MaxRuntimeHours)
	case "size":
		return s.CleanupBySizeLimit(config.MaxSizeBytes)
	default:
		return s.CleanupBySizeLimit(config.MaxSizeBytes)
	}
}
