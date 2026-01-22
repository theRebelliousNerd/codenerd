package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// TaskExecutor is the unified interface for task execution.
// It abstracts both the new JIT-driven architecture and the legacy ShardManager,
// enabling incremental migration from the old shard system to the new clean loop.
//
// Migration path:
//  1. Consumers switch from ShardManager.Spawn() to TaskExecutor.Execute()
//  2. TaskExecutor initially wraps ShardManager via LegacyBridge
//  3. Flip to JITExecutor when ready
//  4. Delete LegacyBridge and ShardManager
type TaskExecutor interface {
	// Execute runs a task synchronously and returns the result.
	// The intent parameter is an intent verb (e.g., "/fix", "/test", "/review")
	// that determines the persona, tools, and policies via JIT compilation.
	Execute(ctx context.Context, intent string, task string) (string, error)

	// ExecuteWithContext runs a task with explicit session context and priority.
	// This enables dream mode, shadow execution, and context injection.
	ExecuteWithContext(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error)

	// ExecuteAsync spawns a subagent to handle the task asynchronously.
	// Returns an ID that can be used to track progress and get results.
	ExecuteAsync(ctx context.Context, intent string, task string) (taskID string, err error)

	// GetResult retrieves the result of an async task.
	// Returns empty result and false if the task is still running.
	GetResult(taskID string) (result string, done bool, err error)

	// WaitForResult blocks until the async task completes.
	WaitForResult(ctx context.Context, taskID string) (string, error)
}

// TaskResult represents the result of an async task execution.
type TaskResult struct {
	TaskID    string
	Result    string
	Error     error
	Duration  time.Duration
	Completed bool
}

// JITExecutor implements TaskExecutor using the new JIT-driven architecture.
// It replaces ShardManager by routing all tasks through the clean execution loop.
type JITExecutor struct {
	executor   *Executor
	spawner    *Spawner
	transducer perception.Transducer

	// Results for async tasks (protected by mu)
	mu      sync.RWMutex
	results map[string]*TaskResult
}

// NewJITExecutor creates a TaskExecutor using the new architecture.
func NewJITExecutor(executor *Executor, spawner *Spawner, transducer perception.Transducer) *JITExecutor {
	return &JITExecutor{
		executor:   executor,
		spawner:    spawner,
		transducer: transducer,
		results:    make(map[string]*TaskResult),
	}
}

// Execute runs a task through the clean execution loop.
// For simple tasks, it uses the executor directly.
// For complex tasks that need isolation, it spawns a subagent.
func (j *JITExecutor) Execute(ctx context.Context, intent string, task string) (string, error) {
	return j.ExecuteWithContext(ctx, intent, task, nil, types.PriorityNormal)
}

// ExecuteWithContext runs a task with explicit session context and priority.
func (j *JITExecutor) ExecuteWithContext(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	logging.Session("JITExecutor.ExecuteWithContext: intent=%s task_len=%d priority=%v", intent, len(task), priority)

	// Dream mode tasks are speculative and should always use a subagent
	// to avoid side effects and allow for parallelism.
	if sessionCtx != nil && sessionCtx.DreamMode {
		return j.executeWithSubagent(ctx, intent, task, sessionCtx)
	}

	// Determine if we need a subagent or can use inline execution
	if j.needsSubagent(intent) {
		return j.executeWithSubagent(ctx, intent, task, sessionCtx)
	}

	// Use inline execution for simple tasks
	// NOTE: SetSessionContext is not thread-safe. For true concurrent execution,
	// use ExecuteAsync which spawns isolated subagents.
	// The Executor's session context is only used for inline execution which
	// is typically single-threaded per chat session.
	if sessionCtx != nil {
		j.executor.SetSessionContext(sessionCtx)
	}

	result, err := j.executor.Process(ctx, task)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	return result.Response, nil
}

// ExecuteAsync spawns a subagent to handle the task.
func (j *JITExecutor) ExecuteAsync(ctx context.Context, intent string, task string) (string, error) {
	return j.executeAsyncInternal(ctx, intent, task, nil)
}

// executeAsyncInternal is an internal helper to spawn subagent with context.
func (j *JITExecutor) executeAsyncInternal(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext) (string, error) {
	logging.Session("JITExecutor.ExecuteAsync: intent=%s", intent)

	// Spawn subagent via Spawner
	req := SpawnRequest{
		Name:           j.intentToAgentName(intent),
		Task:           task,
		Type:           SubAgentTypeEphemeral,
		IntentVerb:     intent,
		Timeout:        30 * time.Minute,
		SessionContext: sessionCtx,
	}

	agent, err := j.spawner.Spawn(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to spawn subagent: %w", err)
	}

	taskID := agent.GetID()

	// Track the task for result retrieval
	j.mu.Lock()
	j.results[taskID] = &TaskResult{
		TaskID:    taskID,
		Completed: false,
	}
	j.mu.Unlock()

	return taskID, nil
}

// GetResult retrieves the result of an async task.
func (j *JITExecutor) GetResult(taskID string) (string, bool, error) {
	// Check if subagent exists
	agent, ok := j.spawner.Get(taskID)
	if !ok {
		// Check cached results
		j.mu.RLock()
		result, cached := j.results[taskID]
		j.mu.RUnlock()
		if cached && result.Completed {
			return result.Result, true, result.Error
		}
		return "", false, fmt.Errorf("task not found: %s", taskID)
	}

	// Check if completed
	state := agent.GetState()
	if state == SubAgentStateCompleted || state == SubAgentStateFailed {
		result, resultErr := agent.GetResult()

		// Use the error from GetResult, or create one if state is failed but no error
		var err error
		if resultErr != nil {
			err = resultErr
		} else if state == SubAgentStateFailed {
			err = fmt.Errorf("subagent execution failed")
		}

		// Cache the result
		j.mu.Lock()
		j.results[taskID] = &TaskResult{
			TaskID:    taskID,
			Result:    result,
			Error:     err,
			Completed: true,
		}
		j.mu.Unlock()

		return result, true, err
	}

	return "", false, nil
}

// WaitForResult blocks until the async task completes.
func (j *JITExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			result, done, err := j.GetResult(taskID)
			if done {
				return result, err
			}
		}
	}
}

// needsSubagent determines if a task requires a separate subagent.
// Complex tasks, long-running operations, and certain intents benefit from isolation.
func (j *JITExecutor) needsSubagent(intent string) bool {
	// Intents that typically benefit from subagent isolation
	complexIntents := map[string]bool{
		"/research":  true, // Research can be long-running
		"/implement": true, // Implementation may need multiple turns
		"/refactor":  true, // Refactoring is complex
		"/campaign":  true, // Campaigns always need isolation
	}

	return complexIntents[intent]
}

// executeWithSubagent spawns a subagent and waits for the result.
func (j *JITExecutor) executeWithSubagent(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext) (string, error) {
	taskID, err := j.executeAsyncInternal(ctx, intent, task, sessionCtx)
	if err != nil {
		return "", err
	}

	return j.WaitForResult(ctx, taskID)
}

// intentToAgentName maps intent verbs to agent names for logging and identification.
func (j *JITExecutor) intentToAgentName(intent string) string {
	switch intent {
	case "/fix", "/implement", "/refactor", "/create":
		return "coder"
	case "/test", "/cover", "/verify":
		return "tester"
	case "/review", "/audit", "/check":
		return "reviewer"
	case "/research", "/learn", "/document":
		return "researcher"
	default:
		return "executor"
	}
}
