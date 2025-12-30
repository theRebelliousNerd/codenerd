package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// CleanupConfig configures automatic cleanup behavior.
type CleanupConfig struct {
	MaxRuntimeHours      float64 // Max runtime hours to keep (default: 336 = 14 days equivalent)
	MaxSizeBytes         int64   // Max storage size in bytes (default: 100MB)
	AutoCleanupThreshold float64 // Trigger cleanup at this % of max (default: 0.8)
	CleanupMode          string  // "runtime", "size", or "smart"
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

// ToolStatsSummary provides per-tool statistics for LLM cleanup.
type ToolStatsSummary struct {
	ToolName       string
	Count          int
	SuccessRate    float64
	AvgDurationMs  float64
	TotalSizeBytes int64
	AvgReferences  float64
	LastReferenced string // Human-readable
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

	// Delete oldest executions until under budget
	// We delete by oldest created_at, which correlates with oldest runtime
	for currentHours > budgetHours {
		// Get oldest execution
		var id int64
		var resultSize int
		var runtimeMs int64
		row := s.db.QueryRow(`
			SELECT id, result_size, session_runtime_ms
			FROM tool_executions
			ORDER BY created_at ASC LIMIT 1`)
		if err := row.Scan(&id, &resultSize, &runtimeMs); err != nil {
			break // No more rows
		}

		// Delete it
		_, err := s.db.Exec("DELETE FROM tool_executions WHERE id = ?", id)
		if err != nil {
			break
		}

		stats.ExecutionsDeleted++
		stats.BytesFreed += int64(resultSize)

		// Recalculate current hours
		row = s.db.QueryRow(`
			SELECT COALESCE(SUM(max_runtime) / 3600000.0, 0) FROM (
				SELECT MAX(session_runtime_ms) as max_runtime
				FROM tool_executions GROUP BY session_id
			)`)
		if err := row.Scan(&currentHours); err != nil {
			break
		}
	}

	stats.RuntimeHoursFreed = hoursToFree
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

// GetToolStatsSummary generates per-tool statistics for LLM cleanup decisions.
func (s *ToolStore) GetToolStatsSummary() ([]ToolStatsSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT
			tool_name,
			COUNT(*) as count,
			AVG(CASE WHEN success = 1 THEN 100.0 ELSE 0.0 END) as success_rate,
			AVG(duration_ms) as avg_duration,
			SUM(result_size) as total_size,
			AVG(reference_count) as avg_refs,
			MAX(last_referenced) as last_ref
		FROM tool_executions
		GROUP BY tool_name
		ORDER BY count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ToolStatsSummary
	for rows.Next() {
		var s ToolStatsSummary
		var lastRef *string
		if err := rows.Scan(&s.ToolName, &s.Count, &s.SuccessRate,
			&s.AvgDurationMs, &s.TotalSizeBytes, &s.AvgReferences, &lastRef); err != nil {
			continue
		}
		if lastRef != nil {
			s.LastReferenced = *lastRef
		} else {
			s.LastReferenced = "never"
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

// LLMCleanupRecommendation is the expected JSON output from LLM cleanup.
type LLMCleanupRecommendation struct {
	DeleteStale struct {
		BeforeRuntimeHours float64 `json:"before_runtime_hours"`
		MinReferences      int     `json:"min_references"`
	} `json:"delete_stale"`
	DeleteFailed struct {
		OlderThanHours float64  `json:"older_than_hours"`
		Exceptions     []string `json:"exceptions"`
	} `json:"delete_failed"`
	DeleteLargeUnused struct {
		SizeThresholdKB int `json:"size_threshold_kb"`
	} `json:"delete_large_unused"`
	DeleteRedundant struct {
		ToolName    string `json:"tool_name"`
		KeepNewestN int    `json:"keep_newest_n"`
	} `json:"delete_redundant"`
	EstimatedFreedMB float64 `json:"estimated_freed_mb"`
}

// CleanupIntelligent uses LLM to decide what to cleanup.
func (s *ToolStore) CleanupIntelligent(ctx context.Context, llm types.LLMClient) (*CleanupStats, error) {
	stats := &CleanupStats{Method: "smart"}

	// Get tool statistics
	toolStats, err := s.GetToolStatsSummary()
	if err != nil {
		return nil, fmt.Errorf("failed to get tool stats: %w", err)
	}

	// Get overall stats
	overallStats, err := s.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get overall stats: %w", err)
	}

	// Build prompt
	var sb strings.Builder
	sb.WriteString("Analyze tool execution history and recommend cleanup:\n\n")
	sb.WriteString("## Statistics by Tool\n")
	for _, ts := range toolStats {
		sb.WriteString(fmt.Sprintf("- %s: %d executions, %.0f%% success, last referenced: %s, avg refs: %.1f, size: %.1fKB\n",
			ts.ToolName, ts.Count, ts.SuccessRate, ts.LastReferenced, ts.AvgReferences, float64(ts.TotalSizeBytes)/1024))
	}
	sb.WriteString(fmt.Sprintf("\n## Storage Status\nTotal: %.1fMB, Runtime: %.1f hours\n\n",
		float64(overallStats.TotalSizeBytes)/(1024*1024), overallStats.TotalRuntimeHours))
	sb.WriteString(`## Cleanup Criteria
1. Redundant: Multiple executions with similar results (keep newest)
2. Stale: Never referenced by LLM, older than 10 runtime hours
3. Failed: Error executions older than 5 runtime hours (unless only record of that tool)
4. Large: Results > 50KB with reference_count = 0

Recommend which executions to DELETE. Output ONLY valid JSON:
{
  "delete_stale": {"before_runtime_hours": 10, "min_references": 0},
  "delete_failed": {"older_than_hours": 5, "exceptions": []},
  "delete_large_unused": {"size_threshold_kb": 50},
  "delete_redundant": {"tool_name": "", "keep_newest_n": 3},
  "estimated_freed_mb": 0
}`)

	// Call LLM
	response, err := llm.Complete(ctx, sb.String())
	if err != nil {
		return nil, fmt.Errorf("LLM cleanup failed: %w", err)
	}

	// Parse recommendation
	var rec LLMCleanupRecommendation
	// Extract JSON from response (may be wrapped in markdown)
	jsonStr := response
	if idx := strings.Index(response, "{"); idx >= 0 {
		jsonStr = response[idx:]
	}
	if idx := strings.LastIndex(jsonStr, "}"); idx >= 0 {
		jsonStr = jsonStr[:idx+1]
	}

	if err := json.Unmarshal([]byte(jsonStr), &rec); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to parse LLM cleanup recommendation: %v", err)
		// Fall back to safe defaults
		rec.DeleteStale.BeforeRuntimeHours = 10
		rec.DeleteStale.MinReferences = 0
		rec.DeleteFailed.OlderThanHours = 5
		rec.DeleteLargeUnused.SizeThresholdKB = 50
	}

	// Execute cleanup based on recommendations
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete stale (never referenced, old runtime)
	if rec.DeleteStale.BeforeRuntimeHours > 0 {
		cutoffMs := int64(rec.DeleteStale.BeforeRuntimeHours * 3600000)
		result, err := s.db.Exec(`
			DELETE FROM tool_executions
			WHERE reference_count <= ? AND session_runtime_ms < ?`,
			rec.DeleteStale.MinReferences, cutoffMs)
		if err == nil {
			if n, _ := result.RowsAffected(); n > 0 {
				stats.ExecutionsDeleted += int(n)
				logging.Store("CleanupIntelligent: deleted %d stale executions", n)
			}
		}
	}

	// Delete failed (old errors)
	if rec.DeleteFailed.OlderThanHours > 0 {
		cutoffMs := int64(rec.DeleteFailed.OlderThanHours * 3600000)
		query := `DELETE FROM tool_executions WHERE success = 0 AND session_runtime_ms < ?`
		if len(rec.DeleteFailed.Exceptions) > 0 {
			placeholders := make([]string, len(rec.DeleteFailed.Exceptions))
			args := make([]interface{}, 1+len(rec.DeleteFailed.Exceptions))
			args[0] = cutoffMs
			for i, exc := range rec.DeleteFailed.Exceptions {
				placeholders[i] = "?"
				args[i+1] = exc
			}
			query += fmt.Sprintf(" AND tool_name NOT IN (%s)", strings.Join(placeholders, ","))
			result, err := s.db.Exec(query, args...)
			if err == nil {
				if n, _ := result.RowsAffected(); n > 0 {
					stats.ExecutionsDeleted += int(n)
					logging.Store("CleanupIntelligent: deleted %d failed executions", n)
				}
			}
		} else {
			result, err := s.db.Exec(query, cutoffMs)
			if err == nil {
				if n, _ := result.RowsAffected(); n > 0 {
					stats.ExecutionsDeleted += int(n)
					logging.Store("CleanupIntelligent: deleted %d failed executions", n)
				}
			}
		}
	}

	// Delete large unused
	if rec.DeleteLargeUnused.SizeThresholdKB > 0 {
		thresholdBytes := rec.DeleteLargeUnused.SizeThresholdKB * 1024
		result, err := s.db.Exec(`
			DELETE FROM tool_executions
			WHERE result_size > ? AND reference_count = 0`,
			thresholdBytes)
		if err == nil {
			if n, _ := result.RowsAffected(); n > 0 {
				stats.ExecutionsDeleted += int(n)
				logging.Store("CleanupIntelligent: deleted %d large unused executions", n)
			}
		}
	}

	// Delete redundant (keep newest N per tool)
	if rec.DeleteRedundant.ToolName != "" && rec.DeleteRedundant.KeepNewestN > 0 {
		result, err := s.db.Exec(`
			DELETE FROM tool_executions
			WHERE tool_name = ? AND id NOT IN (
				SELECT id FROM tool_executions
				WHERE tool_name = ?
				ORDER BY created_at DESC
				LIMIT ?
			)`,
			rec.DeleteRedundant.ToolName, rec.DeleteRedundant.ToolName, rec.DeleteRedundant.KeepNewestN)
		if err == nil {
			if n, _ := result.RowsAffected(); n > 0 {
				stats.ExecutionsDeleted += int(n)
				logging.Store("CleanupIntelligent: deleted %d redundant %s executions", n, rec.DeleteRedundant.ToolName)
			}
		}
	}

	// Calculate bytes freed
	newStats, _ := s.GetStats()
	if newStats != nil {
		stats.BytesFreed = overallStats.TotalSizeBytes - newStats.TotalSizeBytes
		stats.RuntimeHoursFreed = overallStats.TotalRuntimeHours - newStats.TotalRuntimeHours
	}

	logging.Store("CleanupIntelligent: total deleted %d executions, freed %d bytes",
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
