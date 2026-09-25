package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/observation"
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
	// Constraint carries the routing layer's requirements (acceptance
	// criteria, must/must-not rules). The delegation boundary used to drop
	// it: shards received a bare target noun and analyzed instead of acting.
	Constraint string
}

// TaskText returns the agent-facing task description: the task plus any
// routing-layer constraint. Callers that already merged the constraint into
// Task are left unchanged (Contains guard) so cmd flattening and direct
// TaskRequest users compose without duplication.
func (r TaskRequest) TaskText() string {
	task := strings.TrimSpace(r.Task)
	if c := strings.TrimSpace(r.Constraint); c != "" && !strings.Contains(task, c) {
		if task != "" {
			task += "\n\nConstraints:\n" + c
		} else {
			task = c
		}
	}
	return task
}

// UserAgentFromIntentVerb returns the user-defined agent name a verb addresses,
// or "" when the verb belongs to the built-in taxonomy.
//
// Two shapes reach the executor for a user agent defined in
// .nerd/agents/<name>/prompts.yaml:
//
//	/consult/<name>  JITExecutor.SpawnConsultation and campaign specialists
//	/<name>          any bare name that is not a persona (chat delegation,
//	                 `nerd spawn <name>`, Cortex.SpawnTask), via intentFor
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
	TaskID string
	Result string
	Error  error
	// Observed is the raw observation of the run: the same output, plus the
	// write set, build verdict and test verdict the executor measured. Result
	// stays the string it always was because the surfaces that print it to a
	// user are right to want prose; this is what a PARENT AGENT reads.
	Observed  observation.Return
	Duration  time.Duration
	Completed bool
}

// imageGeneratorNames fail closed: image generation must use the Cortex
// ShardManager's image client (Nano Banana 2, gemini-3.1-flash-image). Run
// here it would land on the worker client via the JIT executor -- the
// dual-LLM mis-route FM15 forbids.
var imageGeneratorNames = map[string]bool{
	"image_generator": true, "image-generator": true, "imagegenerator": true,
	"imagen": true, "image": true, "nano_banana": true, "nanobanana": true,
}

// intentFor is the intent verb a request runs, and the one place a persona
// becomes a verb (sweep finding F6). Chat, the delegation verifier and this
// executor each kept a persona table and they disagreed (nemesis ran /attack
// from chat and /review from here); now every caller passes what it has and
// this asks the kernel's persona_verb table (policy/delegation.mg).
//
//   - "/consult/<name>" and a verb the taxonomy knows ("/fix") are unchanged.
//   - A persona, bare or slashed ("coder", "/coder" from a delegate_task
//     fact), becomes its persona_verb verb.
//   - Any other slashed single name is unchanged; any other bare name is a
//     user agent, "/<name>".
func (j *JITExecutor) intentFor(verb string) (string, error) {
	verb = strings.TrimSpace(verb)
	if verb == "" {
		return "", fmt.Errorf("invalid intent verb: empty")
	}
	if strings.HasPrefix(verb, "/consult/") {
		return verb, nil
	}
	name := strings.ToLower(strings.TrimPrefix(verb, "/"))
	if imageGeneratorNames[name] {
		return "", fmt.Errorf("%s requires ShardManager image LLM (Nano Banana 2 / gemini-3.1-flash-image), not TaskExecutor worker path", verb)
	}
	slashed := strings.HasPrefix(verb, "/")
	if slashed && perception.GetShardTypeForVerb(verb) != "" {
		return verb, nil
	}
	if strings.ContainsAny(name, " \t\n/") {
		if slashed {
			return verb, nil
		}
		return "", fmt.Errorf("invalid intent verb '%s', must start with '/' or be a persona or agent name", verb)
	}
	mapped, err := j.personaVerb(name)
	if err != nil {
		return "", err
	}
	switch {
	case mapped != "":
		return mapped, nil
	case slashed:
		return verb, nil
	default:
		return "/" + name, nil
	}
}

// personaVerb is persona_verb's verb for name, or "" when name is no persona.
func (j *JITExecutor) personaVerb(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	k := j.kernel()
	if k == nil {
		return "", fmt.Errorf("no kernel to map %q to a verb", name)
	}
	rows, err := k.Query("persona_verb")
	if err != nil {
		return "", fmt.Errorf("query persona_verb: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("the kernel holds no persona_verb table (policy/delegation.mg); %q cannot be mapped", name)
	}
	for _, f := range rows {
		if len(f.Args) == 2 && strings.TrimPrefix(types.ExtractString(f.Args[0]), "/") == name {
			return types.ExtractString(f.Args[1]), nil
		}
	}
	return "", nil
}

// kernel is the session executor's kernel, which holds the persona and
// isolation tables.
func (j *JITExecutor) kernel() types.Kernel {
	if j.executor == nil {
		return nil
	}
	return j.executor.kernel
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
	// completedOrder is the FIFO eviction order for completed results.
	// In-flight entries are never listed here and are never evicted.
	completedOrder []string
}

// maxCachedResults bounds completed async results. Every production task is
// retrieved through WaitForResult, which hands the caller the string directly,
// so the cache is a late-poller convenience, not the durable record — and an
// unbounded one retains every delegated response for the process lifetime.
const maxCachedResults = 256

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
	observed, err := j.executeObserved(ctx, req, sessionCtx, priority)
	return observed.Output, err
}

// ExecuteObserved implements ObservedTaskExecutor: the same run, returning what
// the executor measured about it rather than only the prose.
func (j *JITExecutor) ExecuteObserved(ctx context.Context, req TaskRequest) (observation.Return, error) {
	return j.executeObserved(ctx, req, nil, types.PriorityNormal)
}

// ExecuteObservedWithContext implements ObservedTaskExecutor for the callers
// that also need a session context and a priority — the chat's delegation and
// continuation path — without making them give up the structured return to get
// them.
func (j *JITExecutor) ExecuteObservedWithContext(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (observation.Return, error) {
	return j.executeObserved(ctx, req, sessionCtx, priority)
}

// executeObserved is the one implementation both entry points share. Splitting
// it in two so each could "just" return what its caller wanted is how the two
// paths would drift, and a structured return that disagreed with the string
// beside it would be worse than no structured return at all.
func (j *JITExecutor) executeObserved(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (observation.Return, error) {
	// Normalize IntentVerb: callers (CLI `nerd spawn <shard-type>`, Cortex.SpawnTask)
	// often pass bare shard names ("tester", "reviewer") rather than Mangle verbs
	// ("/test", "/review"). Only "coder" was special-cased before — other domain
	// shards hard-failed with "must start with '/'".
	normalized, nerr := j.intentFor(req.IntentVerb)
	if nerr != nil {
		return observation.Return{}, nerr
	}
	if normalized != req.IntentVerb {
		logging.Get(logging.CategorySession).Warn(
			"TaskExecutor mapped shard/intent %q → %q", req.IntentVerb, normalized,
		)
		req.IntentVerb = normalized
	}

	taskText := req.TaskText()
	logging.Session("JITExecutor.ExecuteWithContext: intent=%s task_len=%d priority=%v", req.IntentVerb, len(taskText), priority)

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

	// Whether the verb runs isolated is the kernel's (verb_isolated).
	isolated, err := j.runsIsolated(req.IntentVerb)
	if err != nil {
		return observation.Return{}, err
	}
	if isolated {
		return j.executeWithSubagent(ctx, req, sessionCtx)
	}

	// Inline execution runs on an ISOLATED clone of the session executor.
	// Running directly on the shared executor (the old behavior) raced on
	// SetSessionContext and appended every delegated task to the session's
	// conversation history, contaminating later turns.
	exec := j.executor.CloneForTask()
	// The clone runs this one task; its working archives die with it.
	defer exec.RetireWorkingScopes()
	if sessionCtx != nil {
		exec.SetSessionContext(sessionCtx)
	}

	inlineTask := strings.TrimSpace(taskText)
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
	// intent instead of re-perceiving the synthetic task string. The preset
	// carries the constraint structurally too: retrieval and prompt assembly
	// read intent.Constraint, so text alone would leave them blind.
	preset := presetIntentForTask(req.IntentVerb, inlineTask, req.Target)
	if preset != nil {
		preset.Constraint = strings.TrimSpace(req.Constraint)
	}
	result, err := exec.ProcessWithIntent(ctx, inlineTask, preset)
	observed := withTurnWrites(observedReturn(agentName(req.IntentVerb), inlineTask, result), exec.workspaceForVerification(), result)
	if err != nil {
		// Still surface any partial response text for diagnostics, but never
		// treat hollow/tool failure as success for CLI one-shots.
		if observed.Failure == "" {
			observed.Failure = err.Error()
		}
		if result == nil || strings.TrimSpace(result.Response) == "" {
			observed.Output = ""
		}
		return observed, fmt.Errorf("execution failed: %w", err)
	}
	if result.Error != nil {
		return observed, result.Error
	}

	return observed, nil
}

// ExecuteAsync spawns a subagent to handle the task.
func (j *JITExecutor) ExecuteAsync(ctx context.Context, req TaskRequest) (string, error) {
	return j.executeAsyncInternal(ctx, req, nil)
}

// SpawnConsultation implements shards.ConsultationSpawner: it runs the
// consultation to completion in an isolated subagent and returns the
// specialist's ANSWER, not a task ID. The interface contract returns response
// text — the ConsultationManager parses the return as the specialist's reply —
// so an async task ID here would silently corrupt every consultation.
func (j *JITExecutor) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	req := TaskRequest{
		IntentVerb: "/consult/" + strings.ToLower(strings.TrimSpace(specialistName)),
		Task:       task,
	}
	observed, err := j.executeWithSubagent(ctx, req, nil)
	return observed.Output, err
}

// executeAsyncInternal is an internal helper to spawn subagent with context.
func (j *JITExecutor) executeAsyncInternal(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext) (string, error) {
	logging.Session("JITExecutor.ExecuteAsync: intent=%s", req.IntentVerb)

	// Spawn subagent via Spawner. No Timeout: the task runs under the caller's
	// context (the user's --timeout, when set) and stops when the working
	// policy derives a stall, never on a clock of the harness's own (the
	// 30-minute shard ceiling was removed 2026-09-19).
	spawnReq := SpawnRequest{
		Name:           agentName(req.IntentVerb),
		Task:           req.TaskText(),
		Type:           SubAgentTypeEphemeral,
		IntentVerb:     req.IntentVerb,
		IntentTarget:   req.Target,
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
	observed, done, err := j.getObserved(taskID)
	return observed.Output, done, err
}

// getObserved is the one completion check GetResult and waitObserved share.
//
// Not two, and not an exported second accessor beside GetResult: two readers of
// one subagent's completion that cached independently would each see a
// different answer the moment either raced the other to the state transition.
func (j *JITExecutor) getObserved(taskID string) (observation.Return, bool, error) {
	// Check if subagent exists
	agent, ok := j.spawner.Get(taskID)
	if !ok {
		// Check cached results
		j.mu.RLock()
		result, cached := j.results[taskID]
		j.mu.RUnlock()
		if cached && result.Completed {
			return result.Observed, true, result.Error
		}
		return observation.Return{}, false, fmt.Errorf("task not found: %s", taskID)
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

		// The subagent captured its structured return at the moment its
		// ExecutionResult was still in scope; this reads that back rather than
		// rebuilding anything from the string, which is what makes "changed"
		// and "verified" facts here instead of guesses.
		observed := agent.ObservedReturn()
		observed.Output = result
		if observed.Failure == "" && err != nil {
			observed.Failure = err.Error()
		}
		if observed.Agent == "" {
			observed.Agent = agent.GetName()
		}

		// Cache the result, then release the spawner entry: the cache is now
		// the durable home, so the registry need not hold the agent anymore.
		j.cacheCompletedResult(taskID, observed, err)
		j.spawner.Remove(taskID)

		return observed, true, err
	}

	return observation.Return{}, false, nil
}

// cacheCompletedResult records a finished task under the completed-results
// bound, evicting the oldest completed entries first. In-flight entries are
// never listed for eviction. Repeat calls for one task refresh the entry
// without duplicating its eviction slot. Result is observed.Output, so the
// string and structured views of one completion cannot disagree.
func (j *JITExecutor) cacheCompletedResult(taskID string, observed observation.Return, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if prev, ok := j.results[taskID]; !ok || !prev.Completed {
		j.completedOrder = append(j.completedOrder, taskID)
	}
	j.results[taskID] = &TaskResult{
		TaskID:    taskID,
		Result:    observed.Output,
		Error:     err,
		Observed:  observed,
		Completed: true,
	}
	for len(j.completedOrder) > maxCachedResults {
		oldest := j.completedOrder[0]
		j.completedOrder[0] = ""
		j.completedOrder = j.completedOrder[1:]
		if r, ok := j.results[oldest]; ok && r.Completed {
			delete(j.results, oldest)
		}
	}
}

// WaitForResult blocks until the async task completes.
func (j *JITExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	observed, err := j.waitObserved(ctx, taskID)
	return observed.Output, err
}

// waitObserved is the polling loop both waiters share.
func (j *JITExecutor) waitObserved(ctx context.Context, taskID string) (observation.Return, error) {
	if ctx == nil {
		return observation.Return{}, fmt.Errorf("context is nil")
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
			return observation.Return{}, ctx.Err()
		case <-ticker.C:
			result, done, err := j.getObserved(taskID)
			if err != nil && !done {
				return observation.Return{}, err
			}
			if done {
				return result, err
			}
		}
	}
}

// runsIsolated reports whether the kernel runs verb as an isolated subagent
// (verb_isolated, policy/delegation.mg).
func (j *JITExecutor) runsIsolated(verb string) (bool, error) {
	k := j.kernel()
	if k == nil {
		return false, fmt.Errorf("no kernel to ask whether %s runs isolated", verb)
	}
	rows, err := k.Query("verb_isolated")
	if err != nil {
		return false, fmt.Errorf("query verb_isolated: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) == 1 && types.ExtractString(f.Args[0]) == verb {
			return true, nil
		}
	}
	return false, nil
}

// executeWithSubagent spawns a subagent and waits for the result.
func (j *JITExecutor) executeWithSubagent(ctx context.Context, req TaskRequest, sessionCtx *types.SessionContext) (observation.Return, error) {
	taskID, err := j.executeAsyncInternal(ctx, req, sessionCtx)
	if err != nil {
		return observation.Return{}, err
	}

	return j.waitObserved(ctx, taskID)
}

// agentName labels a request's subagent and its return: the persona the
// taxonomy maps the verb to (verb_def), the agent's name for
// "/consult/<name>", and "executor" otherwise. It replaced two Go tables
// (JITExecutor.intentToAgentName, Spawner.determineAgentName) that disagreed
// with the taxonomy and with each other (sweep finding F6).
func agentName(verb string) string {
	if after, ok := strings.CutPrefix(verb, "/consult/"); ok {
		return after
	}
	if shard := strings.TrimPrefix(perception.GetShardTypeForVerb(verb), "/"); shard != "" && shard != "none" {
		return shard
	}
	return "executor"
}
