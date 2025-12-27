// Package session implements the clean execution loop for codeNERD.
package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ShardManagerInterface defines the minimal interface needed from ShardManager.
// This allows the LegacyBridge to work without importing the shards package directly.
type ShardManagerInterface interface {
	Spawn(ctx context.Context, typeName, task string) (string, error)
	SpawnWithContext(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext) (string, error)
	GetResult(id string) (types.ShardResult, bool)
}

// LegacyBridge wraps ShardManager to implement TaskExecutor.
// This is a temporary bridge to allow incremental migration from the
// old shard-based architecture to the new JIT-driven clean loop.
//
// Usage:
//
//	// Create bridge
//	bridge := session.NewLegacyBridge(shardMgr)
//
//	// Use as TaskExecutor
//	result, err := bridge.Execute(ctx, "/fix", "fix the null pointer")
//
// Once all consumers are migrated, this bridge and ShardManager can be deleted.
type LegacyBridge struct {
	shardMgr   ShardManagerInterface
	sessionCtx *types.SessionContext

	// Track async results
	mu            sync.RWMutex
	pendingTasks  map[string]*legacyTaskInfo
	completedTasks map[string]*TaskResult
}

type legacyTaskInfo struct {
	shardID   string
	shardType string
	task      string
	startTime time.Time
}

// NewLegacyBridge creates a TaskExecutor that wraps the legacy ShardManager.
func NewLegacyBridge(shardMgr ShardManagerInterface) *LegacyBridge {
	return &LegacyBridge{
		shardMgr:       shardMgr,
		pendingTasks:   make(map[string]*legacyTaskInfo),
		completedTasks: make(map[string]*TaskResult),
	}
}

// SetSessionContext sets the session context for shard spawning.
func (l *LegacyBridge) SetSessionContext(ctx *types.SessionContext) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessionCtx = ctx
}

// Execute runs a task via the legacy ShardManager.
// The intent parameter is mapped to a shard type name.
func (l *LegacyBridge) Execute(ctx context.Context, intent string, task string) (string, error) {
	shardType := l.intentToShardType(intent)
	logging.Session("LegacyBridge.Execute: intent=%s -> shardType=%s", intent, shardType)

	l.mu.RLock()
	sessionCtx := l.sessionCtx
	l.mu.RUnlock()

	// Use SpawnWithContext if we have session context
	if sessionCtx != nil {
		return l.shardMgr.SpawnWithContext(ctx, shardType, task, sessionCtx)
	}

	return l.shardMgr.Spawn(ctx, shardType, task)
}

// ExecuteAsync spawns a shard asynchronously.
func (l *LegacyBridge) ExecuteAsync(ctx context.Context, intent string, task string) (string, error) {
	shardType := l.intentToShardType(intent)
	logging.Session("LegacyBridge.ExecuteAsync: intent=%s -> shardType=%s", intent, shardType)

	l.mu.RLock()
	sessionCtx := l.sessionCtx
	l.mu.RUnlock()

	// Spawn the shard (it runs async internally)
	var shardID string
	var err error

	if sessionCtx != nil {
		shardID, err = l.shardMgr.SpawnWithContext(ctx, shardType, task, sessionCtx)
	} else {
		shardID, err = l.shardMgr.Spawn(ctx, shardType, task)
	}

	if err != nil {
		return "", fmt.Errorf("failed to spawn shard: %w", err)
	}

	// Track the task
	l.mu.Lock()
	l.pendingTasks[shardID] = &legacyTaskInfo{
		shardID:   shardID,
		shardType: shardType,
		task:      task,
		startTime: time.Now(),
	}
	l.mu.Unlock()

	return shardID, nil
}

// GetResult retrieves the result of an async task.
func (l *LegacyBridge) GetResult(taskID string) (string, bool, error) {
	// Check completed cache first
	l.mu.RLock()
	if result, ok := l.completedTasks[taskID]; ok {
		l.mu.RUnlock()
		return result.Result, true, result.Error
	}
	l.mu.RUnlock()

	// Check ShardManager for result
	result, ok := l.shardMgr.GetResult(taskID)
	if !ok {
		return "", false, nil // Still running
	}

	// Cache the result
	l.mu.Lock()
	l.completedTasks[taskID] = &TaskResult{
		TaskID:    taskID,
		Result:    result.Result,
		Error:     result.Error,
		Completed: true,
	}
	delete(l.pendingTasks, taskID)
	l.mu.Unlock()

	return result.Result, true, result.Error
}

// WaitForResult blocks until the async task completes.
func (l *LegacyBridge) WaitForResult(ctx context.Context, taskID string) (string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			result, done, err := l.GetResult(taskID)
			if done {
				return result, err
			}
		}
	}
}

// intentToShardType maps intent verbs to legacy shard type names.
func (l *LegacyBridge) intentToShardType(intent string) string {
	// Strip leading slash if present
	intent = strings.TrimPrefix(intent, "/")

	// Map common intent verbs to shard types
	switch intent {
	case "fix", "implement", "refactor", "create", "modify", "add", "update":
		return "coder"
	case "test", "cover", "verify", "validate":
		return "tester"
	case "review", "audit", "check", "analyze", "inspect":
		return "reviewer"
	case "research", "learn", "document", "explore", "find":
		return "researcher"
	default:
		// If it looks like a shard type name, use it directly
		if intent == "coder" || intent == "tester" || intent == "reviewer" || intent == "researcher" {
			return intent
		}
		// Default to coder for unknown intents
		return "coder"
	}
}

// Cleanup removes old completed results from the cache.
func (l *LegacyBridge) Cleanup() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Keep results for 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	removed := 0

	for taskID, result := range l.completedTasks {
		if result.Completed {
			// Check if task info exists for age check
			if info, ok := l.pendingTasks[taskID]; ok && info.startTime.Before(cutoff) {
				delete(l.completedTasks, taskID)
				removed++
			}
		}
	}

	return removed
}

// MigrationHelper provides utilities to help migrate code from ShardManager to TaskExecutor.
type MigrationHelper struct{}

// ShardTypeToIntent converts a legacy shard type name to an intent verb.
// Use this when migrating code from ShardManager.Spawn() to TaskExecutor.Execute().
//
// Example migration:
//
//	// BEFORE
//	result, err := shardMgr.Spawn(ctx, "coder", task)
//
//	// AFTER
//	intent := MigrationHelper{}.ShardTypeToIntent("coder") // returns "/fix"
//	result, err := taskExecutor.Execute(ctx, intent, task)
func (MigrationHelper) ShardTypeToIntent(shardType string) string {
	shardType = strings.TrimPrefix(shardType, "/")
	shardType = strings.ToLower(shardType)

	switch shardType {
	case "coder":
		return "/fix" // Default coder intent
	case "tester":
		return "/test"
	case "reviewer":
		return "/review"
	case "researcher":
		return "/research"
	default:
		return "/" + shardType
	}
}

// InferIntentFromTask tries to determine the appropriate intent verb from a task description.
// This is a heuristic helper for migration; prefer explicit intent verbs when possible.
func (MigrationHelper) InferIntentFromTask(task string) string {
	taskLower := strings.ToLower(task)

	// Test-related keywords
	if strings.Contains(taskLower, "test") || strings.Contains(taskLower, "coverage") ||
		strings.Contains(taskLower, "verify") {
		return "/test"
	}

	// Review-related keywords
	if strings.Contains(taskLower, "review") || strings.Contains(taskLower, "audit") ||
		strings.Contains(taskLower, "check") || strings.Contains(taskLower, "analyze") {
		return "/review"
	}

	// Research-related keywords
	if strings.Contains(taskLower, "research") || strings.Contains(taskLower, "find") ||
		strings.Contains(taskLower, "document") || strings.Contains(taskLower, "explore") {
		return "/research"
	}

	// Fix-related keywords (common for coder)
	if strings.Contains(taskLower, "fix") || strings.Contains(taskLower, "bug") ||
		strings.Contains(taskLower, "error") || strings.Contains(taskLower, "broken") {
		return "/fix"
	}

	// Implement-related keywords
	if strings.Contains(taskLower, "implement") || strings.Contains(taskLower, "create") ||
		strings.Contains(taskLower, "add") || strings.Contains(taskLower, "build") {
		return "/implement"
	}

	// Refactor-related keywords
	if strings.Contains(taskLower, "refactor") || strings.Contains(taskLower, "restructure") ||
		strings.Contains(taskLower, "reorganize") || strings.Contains(taskLower, "clean") {
		return "/refactor"
	}

	// Default to /fix for general coding tasks
	return "/fix"
}
