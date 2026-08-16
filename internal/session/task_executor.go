package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// TaskRequest represents a structured request for task execution.
//
// IntentVerb is the single routing key. It previously shared the struct with
// `Persona` and `ConfigRef`, which every caller populated and no code anywhere
// read — two parallel routing inputs that looked authoritative and bound
// nothing. They are gone: the persona is recoverable from the verb without loss
// (perception.GetShardTypeForVerb for built-ins, UserAgentFromIntentVerb for
// "/consult/<name>" and "/<name>"), and one routing key cannot drift out of
// sync with itself.
type TaskRequest struct {
	IntentVerb string // Canonical intent verb (e.g., /fix, /review, /consult/rustexpert)
	Task       string // The task description
	Target     string // Resolved file or directory the verb acts on; empty when the task is prose rather than a target.
}

// UserAgentFromIntentVerb returns the user-defined agent name a verb addresses,
// or "" when the verb belongs to the built-in taxonomy.
//
// Two shapes reach the executor for a user agent defined in
// .nerd/agents/<name>/prompts.yaml:
//
//	/consult/<name>  chat delegation (cmd/nerd/chat/delegation_routing.go
//	                 personaToIntent) and JITExecutor.SpawnConsultation
//	/<name>          `nerd spawn <name>` and Cortex.SpawnTask, via
//	                 normalizeTaskIntentVerb's bare-identifier branch
//
// The returned name is lower-cased; the JIT compiler's shard-DB registry is
// keyed case-insensitively (internal/prompt/compiler_db.go shardDBKey) so it
// matches the on-disk directory whatever its casing.
func UserAgentFromIntentVerb(verb string) string {
	v := strings.TrimSpace(verb)
	if v == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(v, "/consult/"); ok {
		return strings.ToLower(strings.TrimSpace(after))
	}
	name, ok := strings.CutPrefix(v, "/")
	if !ok {
		return ""
	}
	name = strings.TrimSpace(name)
	// A single bare segment only. Anything with a separator is a structured
	// verb, not an agent name.
	if name == "" || strings.ContainsAny(name, "/ \t") {
		return ""
	}
	return strings.ToLower(name)
}

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
	Execute(ctx context.Context, req TaskRequest) (string, error)

	// ExecuteWithContext runs a task with explicit session context and priority.
	// This enables dream mode, shadow execution, and context injection.
	ExecuteWithContext(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error)

	// ExecuteAsync spawns a subagent to handle the task asynchronously.
	// Returns an ID that can be used to track progress and get results.
	ExecuteAsync(ctx context.Context, req TaskRequest) (taskID string, err error)

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

// normalizeTaskIntentVerb maps CLI/shard-type names onto canonical intent verbs.
// Accepts already-canonical forms ("/fix", "/review") unchanged.
func normalizeTaskIntentVerb(verb string) (string, error) {
	verb = strings.TrimSpace(verb)
	if verb == "" {
		return "", fmt.Errorf("invalid intent verb: empty")
	}
	if strings.HasPrefix(verb, "/") {
		return verb, nil
	}
	// Domain shard types used by `nerd spawn <type>` and Cortex.SpawnTask.
	switch strings.ToLower(verb) {
	case "coder":
		return "/fix", nil
	case "tester":
		return "/test", nil
	case "reviewer":
		return "/review", nil
	case "researcher":
		return "/research", nil
	case "generalist":
		return "/implement", nil
	case "specialist":
		return "/research", nil
	case "tool_generator", "tool-generator", "toolgenerator":
		return "/generate_tool", nil
	case "nemesis":
		return "/review", nil
	case "image_generator", "image-generator", "imagegenerator", "imagen", "image", "nano_banana", "nanobanana":
		// Fail closed: image gen must use Cortex/ShardManager Nano Banana 2
		// (Gemini image client). Mapping to /create would run on the worker
		// Ollama client via JITExecutor — the dual-LLM mis-route FM15 forbids.
		return "", fmt.Errorf("image_generator requires ShardManager image LLM (Nano Banana 2 / gemini-3.1-flash-image), not TaskExecutor worker path")
	default:
		// Bare identifier: treat as /identifier (e.g. user agents).
		if strings.ContainsAny(verb, " \t\n/") {
			return "", fmt.Errorf("invalid intent verb '%s', must start with '/' or be a known shard type", verb)
		}
		return "/" + strings.ToLower(verb), nil
	}
}

// presetIntentForTask builds the pre-classified intent for a delegated task.
// Returns nil when no verb is known, in which case the executor falls back to
// perceiving the task text.
func presetIntentForTask(intentVerb, task, target string) *perception.Intent {
	intentVerb = strings.TrimSpace(intentVerb)
	if intentVerb == "" || !strings.HasPrefix(intentVerb, "/") {
		return nil
	}
	return &perception.Intent{
		Category:   categoryForIntentVerb(intentVerb),
		Verb:       intentVerb,
		Target:     target,
		Constraint: "",
		// The routing layer already decided this verb; the executor must not
		// second-guess it.
		Confidence: 1.0,
		Response:   "",
		IsQuestion: false,
	}
}

// categoryForIntentVerb maps an intent verb to its category for preset
// intents. Mirrors the perception taxonomy defaults; unknown verbs (including
// /consult/<specialist>) are queries, which is the safe default — queries
// never trigger the mutation-only machinery.
func categoryForIntentVerb(verb string) string {
	switch verb {
	case "/fix", "/refactor", "/create", "/write", "/delete", "/implement",
		"/test", "/git", "/migrate", "/optimize", "/document", "/format",
		"/scaffold", "/campaign", "/assault", "/init", "/generate_tool", "/commit":
		return "/mutation"
	default:
		return "/query"
	}
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
func (j *JITExecutor) Execute(ctx context.Context, req TaskRequest) (string, error) {
	return j.ExecuteWithContext(ctx, req, nil, types.PriorityNormal)
}

// ExecuteWithContext runs a task with explicit session context and priority.
func (j *JITExecutor) ExecuteWithContext(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	// Normalize IntentVerb: callers (CLI `nerd spawn <shard-type>`, Cortex.SpawnTask)
	// often pass bare shard names ("tester", "reviewer") rather than Mangle verbs
	// ("/test", "/review"). Only "coder" was special-cased before — other domain
	// shards hard-failed with "must start with '/'".
	normalized, nerr := normalizeTaskIntentVerb(req.IntentVerb)
	if nerr != nil {
		return "", nerr
	}
	if normalized != req.IntentVerb {
		logging.Get(logging.CategorySession).Warn(
			"TaskExecutor mapped shard/intent %q → %q", req.IntentVerb, normalized,
		)
		req.IntentVerb = normalized
	}

	logging.Session("JITExecutor.ExecuteWithContext: intent=%s task_len=%d priority=%v", req.IntentVerb, len(req.Task), priority)

	// Propagate the caller's priority to the API scheduler. Without this the
	// priority parameter was accepted and dropped — user-initiated shard work
	// (PriorityHigh from the chat turn) queued for LLM slots at the same
	// priority as background learning.
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := types.SpawnPriorityFromContext(ctx); !ok {
		ctx = types.WithSpawnPriority(ctx, priority)
	}

	// Dream mode tasks are speculative and should always use a subagent
	// to avoid side effects and allow for parallelism.
	if sessionCtx != nil && sessionCtx.DreamMode {
		return j.executeWithSubagent(ctx, req, sessionCtx)
	}

	// Determine if we need a subagent or can use inline execution
	if j.needsSubagent(req.IntentVerb) {
		return j.executeWithSubagent(ctx, req, sessionCtx)
	}

	// Inline execution runs on an ISOLATED clone of the session executor.
	// Running directly on the shared executor (the old behavior) raced on
	// SetSessionContext and appended every delegated task to the session's
	// conversation history, contaminating later turns.
	exec := j.executor.CloneForTask()
	if sessionCtx != nil {
		exec.SetSessionContext(sessionCtx)
	}

	inlineTask := strings.TrimSpace(req.Task)
	if req.IntentVerb != "" {
		intentWord := strings.TrimPrefix(strings.TrimSpace(req.IntentVerb), "/")
		if intentWord != "" && (inlineTask == "" || !strings.HasPrefix(inlineTask, intentWord+" ")) {
			if inlineTask == "" {
				inlineTask = intentWord
			} else {
				inlineTask = intentWord + " " + inlineTask
			}
		}
	}

	// The routing layer already classified this task — run with the preset
	// intent instead of re-perceiving the synthetic task string.
	result, err := exec.ProcessWithIntent(ctx, inlineTask, presetIntentForTask(req.IntentVerb, inlineTask, req.Target))
	if err != nil {
		// Still surface any partial response text for diagnostics, but never
		// treat hollow/tool failure as success for CLI one-shots.
		if result != nil && strings.TrimSpace(result.Response) != "" {
			return result.Response, fmt.Errorf("execution failed: %w", err)
		}
		return "", fmt.Errorf("execution failed: %w", err)
	}
	if result.Error != nil {
		return result.Response, result.Error
	}

	return result.Response, nil
}

// ExecuteAsync spawns a subagent to handle the task.
func (j *JITExecutor) ExecuteAsync(ctx context.Context, req TaskRequest) (string, error) {
	return j.executeAsyncInternal(ctx, req, nil)
}

// SpawnConsultation implements shards.ConsultationSpawner.
func (j *JITExecutor) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	req := TaskRequest{
		IntentVerb: fmt.Sprintf("/consult/%s", specialistName),
		Task:       task,
	}
	return j.executeAsyncInternal(ctx, req, nil)
}

// executeAsyncInternal is an internal helper to spawn subagent with context.
func (j *JITExecutor) executeAsyncInternal(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext) (string, error) {
	logging.Session("JITExecutor.ExecuteAsync: intent=%s", req.IntentVerb)

	// Spawn subagent via Spawner. Timeout comes from the central LLM timeout
	// config (user-tunable) instead of a hardcoded magic number.
	spawnReq := SpawnRequest{
		Name:           j.intentToAgentName(req.IntentVerb),
		Task:           req.Task,
		Type:           SubAgentTypeEphemeral,
		IntentVerb:     req.IntentVerb,
		IntentTarget:   req.Target,
		Timeout:        appconfig.GetLLMTimeouts().ShardExecutionTimeout,
		SessionContext: sessionCtx,
	}
	// Spawner.Spawn() creates the agent. We must manually start it after tracking
	// its ID to prevent a TOCTOU race where a very fast execution completes and
	// caches its true result before ExecuteAsync initializes it to false.
	agent, err := j.spawner.Spawn(ctx, spawnReq)
	if err != nil {
		return "", fmt.Errorf("failed to spawn subagent: %w", err)
	}

	taskID := agent.GetID()

	// Track the task for result retrieval BEFORE starting execution
	j.mu.Lock()
	j.results[taskID] = &TaskResult{
		TaskID:    taskID,
		Completed: false,
	}
	j.mu.Unlock()

	// Start execution
	go agent.Run(ctx, spawnReq.Task)

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
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Reap the subagent to prevent zombie processes burning LLM tokens.
			// Without this, the agent continues running even though nobody is
			// waiting for its result.
			if j.spawner != nil {
				if err := j.spawner.Stop(taskID); err != nil {
					logging.SessionDebug("WaitForResult: failed to stop subagent %s on cancellation: %v", taskID, err)
				} else {
					logging.Session("WaitForResult: stopped subagent %s on context cancellation", taskID)
				}
			}
			return "", ctx.Err()
		case <-ticker.C:
			result, done, err := j.GetResult(taskID)
			if err != nil && !done {
				return "", err
			}
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
func (j *JITExecutor) executeWithSubagent(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext) (string, error) {
	taskID, err := j.executeAsyncInternal(ctx, req, sessionCtx)
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
	case "/attack":
		return "nemesis"
	case "/legislate":
		return "legislator"
	case "/plan":
		return "planner"
	}
	// /consult/<persona> → <persona>
	if after, ok := strings.CutPrefix(intent, "/consult/"); ok {
		return after
	}
	return "executor"
}
