package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/usage"
)

// =============================================================================
// SHARD SPAWNING AND EXECUTION
// =============================================================================

// Spawn creates and executes a shard synchronously.
// Routes through SpawnQueue for backpressure management if queue is attached.
func (sm *ShardManager) Spawn(ctx context.Context, typeName, task string) (string, error) {
	logging.Shards("Spawn: synchronous spawn of %s shard for task", typeName)
	logging.ShardsDebug("Spawn: task=%s", task)
	return sm.SpawnWithPriority(ctx, typeName, task, nil, PriorityNormal)
}

// SpawnWithContext creates and executes a shard with session context (Blackboard Pattern).
// The sessionCtx provides compressed history and recent findings for cross-shard awareness.
func (sm *ShardManager) SpawnWithContext(ctx context.Context, typeName, task string, sessionCtx *SessionContext) (string, error) {
	timer := logging.StartTimer(logging.CategoryShards, fmt.Sprintf("SpawnWithContext(%s)", typeName))
	hasContext := sessionCtx != nil
	logging.Shards("SpawnWithContext: spawning %s shard (hasSessionContext=%v)", typeName, hasContext)

	id, err := sm.SpawnAsyncWithContext(ctx, typeName, task, sessionCtx)
	if err != nil {
		logging.Get(logging.CategoryShards).Error("SpawnWithContext: async spawn failed for %s: %v", typeName, err)
		return "", err
	}

	logging.ShardsDebug("SpawnWithContext: waiting for shard %s to complete", id)

	// Wait for result
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Get(logging.CategoryShards).Warn("SpawnWithContext: context canceled while waiting for shard %s", id)
			return "", ctx.Err()
		case <-ticker.C:
			res, ok := sm.GetResult(id)
			if ok {
				timer.Stop()
				if res.Error != nil {
					logging.Get(logging.CategoryShards).Error("SpawnWithContext: shard %s failed: %v", id, res.Error)
					return "", res.Error
				}
				logging.Shards("SpawnWithContext: shard %s completed successfully (output_len=%d)", id, len(res.Result))
				return res.Result, nil
			}
		}
	}
}

// SpawnAsync creates and executes a shard asynchronously.
func (sm *ShardManager) SpawnAsync(ctx context.Context, typeName, task string) (string, error) {
	logging.ShardsDebug("SpawnAsync: async spawn of %s shard", typeName)
	return sm.SpawnAsyncWithContext(ctx, typeName, task, nil)
}

// SpawnWithPriority spawns a shard with priority (uses queue if available).
// If no queue is attached, falls back to direct SpawnWithContext.
func (sm *ShardManager) SpawnWithPriority(ctx context.Context, typeName, task string,
	sessionCtx *SessionContext, priority SpawnPriority) (string, error) {

	typeName = normalizeShardTypeName(typeName)

	sm.mu.RLock()
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	if sq == nil {
		// No queue, use direct spawn
		return sm.SpawnWithContext(ctx, typeName, task, sessionCtx)
	}

	// Submit to queue
	req := SpawnRequest{
		TypeName:   typeName,
		Task:       task,
		SessionCtx: sessionCtx,
		Priority:   priority,
	}

	result, err := sq.SubmitAndWait(ctx, req)
	if err != nil {
		return "", err
	}
	if result.Error != nil {
		return result.ShardID, result.Error
	}
	return result.Result, nil
}

// SpawnAsyncWithContext creates and executes a shard asynchronously with session context.
func (sm *ShardManager) SpawnAsyncWithContext(ctx context.Context, typeName, task string, sessionCtx *SessionContext) (string, error) {
	typeName = normalizeShardTypeName(typeName)
	logging.Shards("SpawnAsyncWithContext: initiating %s shard", typeName)
	logging.ShardsDebug("SpawnAsyncWithContext: task=%s, hasSessionContext=%v", task, sessionCtx != nil)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. Resolve Config (needed to know shard type for enforcement)
	var config ShardConfig
	profile, hasProfile := sm.profiles[typeName]
	if hasProfile {
		config = profile
		logging.ShardsDebug("SpawnAsyncWithContext: using profile config for %s (type=%s)", typeName, config.Type)
	} else {
		// Default to ephemeral generalist if unknown profile
		config = DefaultGeneralistConfig(typeName)
		if typeName == "ephemeral" {
			config.Type = ShardTypeEphemeral
		}
		logging.ShardsDebug("SpawnAsyncWithContext: using default generalist config for %s", typeName)
	}

	// Inject session context into config (Blackboard Pattern)
	if sessionCtx != nil {
		config.SessionContext = sessionCtx
	}

	// ENFORCEMENT: Check concurrent shard limit for non-system shards only.
	// System shards are essential background services and do not consume the
	// user-facing concurrency budget.
	if sm.limitsEnforcer != nil {
		if config.Type != ShardTypeSystem {
			activeNonSystem := 0
			for _, s := range sm.shards {
				if s != nil && s.GetConfig().Type != ShardTypeSystem {
					activeNonSystem++
				}
			}
			if err := sm.limitsEnforcer.CheckShardLimit(activeNonSystem); err != nil {
				logging.Get(logging.CategoryShards).Error("SpawnAsyncWithContext: shard limit enforcement blocked spawn: %v", err)
				return "", fmt.Errorf("cannot spawn shard %s: %w", typeName, err)
			}
		}
		// Always check memory limits before spawning.
		if err := sm.limitsEnforcer.CheckMemory(); err != nil {
			logging.Get(logging.CategoryShards).Error("SpawnAsyncWithContext: memory limit enforcement blocked spawn: %v", err)
			return "", fmt.Errorf("cannot spawn shard %s: %w", typeName, err)
		}
	}

	// Populate available tools from Mangle kernel (Intelligent Tool Routing §40)
	// Uses Mangle predicates: tool_registered, tool_description, tool_binary_path,
	// tool_capability, shard_capability_affinity, relevant_tool
	if sm.kernel != nil {
		// Build tool relevance query with shard context
		query := ToolRelevanceQuery{
			ShardType:   typeName,
			TokenBudget: 2000, // Default budget
		}

		// Extract intent verb from session context if available
		if sessionCtx != nil && sessionCtx.UserIntent != nil && sessionCtx.UserIntent.Verb != "" {
			// Strip leading "/" from verb atom if present
			verb := sessionCtx.UserIntent.Verb
			if len(verb) > 0 && verb[0] == '/' {
				verb = verb[1:]
			}
			query.IntentVerb = verb
			query.TargetFile = sessionCtx.UserIntent.Target
		}

		// Use intelligent routing to get relevant tools
		tools := sm.queryRelevantTools(query)
		if len(tools) > 0 {
			if config.SessionContext == nil {
				config.SessionContext = &SessionContext{}
			}
			config.SessionContext.AvailableTools = tools
		}
	}

	// 2. Resolve Factory
	// First check if there is a factory matching the typeName (e.g. "coder", "researcher")
	factory, hasFactory := sm.factories[typeName]
	logging.ShardsDebug("SpawnAsyncWithContext: factory lookup for %s: hasFactory=%v", typeName, hasFactory)

	if !hasFactory && hasProfile {
		// If using a profile (e.g. "RustExpert"), it might map to a base type factory (e.g. "researcher")
		baseType := strings.TrimPrefix(strings.ToLower(config.BaseType), "/")
		if baseType != "" {
			if baseFactory, ok := sm.factories[baseType]; ok {
				factory = baseFactory
				hasFactory = true
				logging.ShardsDebug("SpawnAsyncWithContext: using base type factory %s for profile %s", baseType, typeName)
			} else {
				logging.ShardsDebug("SpawnAsyncWithContext: base type factory %s not found for profile %s", baseType, typeName)
			}
		}

		// Default to researcher for persistent/user profiles without a matching factory.
		if factory == nil && (config.Type == ShardTypePersistent || config.Type == ShardTypeUser) {
			factory = sm.factories["researcher"]
			logging.ShardsDebug("SpawnAsyncWithContext: using researcher factory as fallback for %s", typeName)
		}
	}

	if factory == nil {
		// Fallback logic based on type
		if config.Type == ShardTypePersistent {
			// Assuming 'researcher' is the base implementation for specialists
			factory = sm.factories["researcher"]
			logging.ShardsDebug("SpawnAsyncWithContext: final fallback to researcher factory for %s", typeName)
		}

		// Final fallback
		if factory == nil {
			// Fallback to basic agent
			logging.ShardsDebug("SpawnAsyncWithContext: using BaseShardAgent fallback for %s", typeName)
			factory = func(id string, config ShardConfig) ShardAgent {
				return NewBaseShardAgent(id, config)
			}
		}
	}

	// 3. Create Shard Instance
	id := fmt.Sprintf("%s-%d", config.Name, time.Now().UnixNano())
	logging.Shards("SpawnAsyncWithContext: creating shard instance id=%s", id)
	agent := factory(id, config)

	// Load prompt atoms for Type B (Persistent) and Type U (User-defined) shards
	// Uses callback to avoid import cycle with internal/prompt package
	if (config.Type == ShardTypePersistent || config.Type == ShardTypeUser) && sm.nerdDir != "" && sm.promptLoader != nil {
		promptsPath := filepath.Join(sm.nerdDir, "agents", typeName, "prompts.yaml")

		// Check if prompts.yaml exists
		if _, err := os.Stat(promptsPath); err == nil {
			logging.ShardsDebug("SpawnAsyncWithContext: loading prompt atoms for %s from %s", typeName, promptsPath)

			// Load prompt atoms via callback (embeddings are skipped by the callback for speed)
			count, loadErr := sm.promptLoader(ctx, typeName, sm.nerdDir)
			if loadErr != nil {
				// Log warning but continue - not a fatal error
				logging.Get(logging.CategoryShards).Warn("SpawnAsyncWithContext: failed to load prompts for %s: %v", typeName, loadErr)
			} else if count > 0 {
				logging.Shards("SpawnAsyncWithContext: loaded %d prompt atoms for %s", count, typeName)
			} else {
				logging.ShardsDebug("SpawnAsyncWithContext: no prompt atoms loaded for %s", typeName)
			}
		} else {
			logging.ShardsDebug("SpawnAsyncWithContext: no prompts.yaml found for %s", typeName)
		}
	}

	// Register agent's knowledge DB with JIT prompt compiler
	// This allows the JIT compiler to load prompt_atoms from the agent's unified knowledge.db
	if (config.Type == ShardTypePersistent || config.Type == ShardTypeUser) && sm.nerdDir != "" && sm.jitRegistrar != nil {
		dbPath := filepath.Join(sm.nerdDir, "shards", fmt.Sprintf("%s_knowledge.db", strings.ToLower(typeName)))
		if _, statErr := os.Stat(dbPath); statErr == nil {
			if regErr := sm.jitRegistrar(typeName, dbPath); regErr != nil {
				logging.Get(logging.CategoryShards).Warn("SpawnAsyncWithContext: failed to register JIT DB for %s: %v", typeName, regErr)
			} else {
				logging.ShardsDebug("SpawnAsyncWithContext: registered JIT DB for %s at %s", typeName, dbPath)
				// Track that this shard instance has a registered JIT DB for cleanup
				sm.activeJITDBs[id] = typeName
			}
		} else {
			logging.ShardsDebug("SpawnAsyncWithContext: no knowledge DB found for %s at %s", typeName, dbPath)
		}
	}

	// Assert active_shard for spreading activation rules
	// This allows policy.mg rules to derive injectable_context atoms based on which shard is running
	// The assertion happens BEFORE execution so context can be gathered during shard initialization
	shardTypeAtom := "/" + typeName
	if sm.kernel != nil {
		if err := sm.kernel.Assert(Fact{
			Predicate: "active_shard",
			Args:      []interface{}{id, shardTypeAtom},
		}); err != nil {
			logging.ShardsDebug("SpawnAsyncWithContext: failed to assert active_shard: %v", err)
		} else {
			logging.ShardsDebug("SpawnAsyncWithContext: asserted active_shard(%s, %s)", id, shardTypeAtom)
		}

		// Fix 15.2: Assert shard_status for observability
		if err := sm.kernel.Assert(Fact{
			Predicate: "shard_status",
			Args:      []interface{}{id, "/running", task},
		}); err != nil {
			logging.ShardsDebug("SpawnAsyncWithContext: failed to assert shard_status: %v", err)
		}
	}

	// Inject dependencies
	depsInjected := []string{}
	if sm.kernel != nil {
		agent.SetParentKernel(sm.kernel)
		depsInjected = append(depsInjected, "kernel")

		// POWER-USER-FEATURE: Inject shard-specific policy if defined
		// This allows specialist shards to have custom permissions or constraints
		if config.Policy != "" {
			sm.kernel.AppendPolicy(config.Policy)
			logging.Shards("SpawnAsyncWithContext: appended shard-specific policy for %s (%d bytes)", typeName, len(config.Policy))
			depsInjected = append(depsInjected, "policy")
		}
	}
	if sm.llmClient != nil {
		// Wrap LLM client with APIScheduler for cooperative slot management
		// This ensures shards release their API slot between LLM calls
		scheduler := GetAPIScheduler()
		scheduler.RegisterShard(id, typeName)
		client := sm.llmClient
		if scheduled, ok := client.(*ScheduledLLMCall); ok {
			client = scheduled.Client
		}
		if disabler, ok := client.(semaphoreDisabler); ok {
			disabler.DisableSemaphore()
		}
		scheduledClient := &ScheduledLLMCall{
			Scheduler: scheduler,
			ShardID:   id,
			Client:    client,
		}
		agent.SetLLMClient(scheduledClient)
		depsInjected = append(depsInjected, "llmClient(scheduled)")
	}
	if sm.virtualStore != nil {
		if vsc, ok := agent.(VirtualStoreConsumer); ok {
			vsc.SetVirtualStore(sm.virtualStore)
			depsInjected = append(depsInjected, "virtualStore")
		}
	}
	// Inject session context (for dream mode, etc.)
	if sessionCtx != nil {
		agent.SetSessionContext(sessionCtx)
		depsInjected = append(depsInjected, "sessionContext")
	}
	logging.ShardsDebug("SpawnAsyncWithContext: dependencies injected: %v", depsInjected)

	sm.shards[id] = agent
	logging.ShardsDebug("SpawnAsyncWithContext: shard %s added to active shards (total active: %d)", id, len(sm.shards))

	// Audit: Shard spawned
	logging.Audit().ShardSpawn(id, typeName)

	// Determine shard category for tracing
	shardCategory := sm.categorizeShardType(typeName, config.Type)

	// Set tracing context before execution
	if sm.tracingClient != nil {
		sm.tracingClient.SetShardContext(id, typeName, shardCategory, sm.sessionID, task)
	}

	// Wrap context with shard metadata for usage tracking
	sessionID := sm.sessionID
	if sessionID == "" {
		sessionID = "current-session"
	}
	ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), sessionID)

	// 4. Execute Async
	logging.Shards("SpawnAsyncWithContext: launching goroutine for shard %s execution", id)
	go func() {
		// Transparency: Start tracking
		if sm.transparencyMgr != nil {
			sm.transparencyMgr.StartShard(id, typeName, task)
		}

		// Panic recovery - shard panics should not crash the system
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("shard %s panicked: %v", id, r)
				logging.Get(logging.CategoryShards).Error("PANIC RECOVERED in shard %s: %v", id, r)
				logging.Audit().ShardComplete(id, task, 0, false, panicErr.Error())

				// Transparency: End tracking (failed)
				if sm.transparencyMgr != nil {
					sm.transparencyMgr.EndShard(id, true)
				}

				// Cleanup: retract active_shard fact
				if sm.kernel != nil {
					_ = sm.kernel.RetractFact(Fact{
						Predicate: "active_shard",
						Args:      []interface{}{id, shardTypeAtom},
					})
					// Fix 15.2: Retract shard_status
					_ = sm.kernel.RetractFact(Fact{
						Predicate: "shard_status",
						Args:      []interface{}{id, "/running", task},
					})
				}
				// Clear tracing context
				if sm.tracingClient != nil {
					sm.tracingClient.ClearShardContext()
				}
				// Record the panic as an error result
				sm.recordResult(id, "", panicErr)
			}
		}()

		// Audit: Shard execution started
		logging.Audit().ShardExecute(id, task)
		logging.ShardsDebug("Shard %s: execution starting (task=%s)", id, task)

		// Apply shard execution timeout from ShardConfig
		execTimeout := config.Timeout
		if execTimeout == 0 {
			execTimeout = 15 * time.Minute // Fallback default
		}
		execCtx, execCancel := context.WithTimeout(ctx, execTimeout)
		defer execCancel()

		logging.ShardsDebug("Shard %s: execution timeout=%v", id, execTimeout)

		startTime := time.Now()
		res, err := agent.Execute(execCtx, task)
		duration := time.Since(startTime)

		// Log specific timeout information
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			logging.Get(logging.CategoryShards).Warn("Shard %s: execution timed out after %v", id, execTimeout)
		}

		// Log execution result
		if err != nil {
			logging.Get(logging.CategoryShards).Error("Shard %s: execution failed after %v: %v", id, duration, err)
		} else {
			logging.Shards("Shard %s: execution completed in %v (output_len=%d)", id, duration, len(res))
		}

		// Audit: Shard execution completed
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logging.Audit().ShardComplete(id, task, duration.Milliseconds(), err == nil, errMsg)

		// Transparency: End tracking
		if sm.transparencyMgr != nil {
			sm.transparencyMgr.EndShard(id, err != nil)
		}

		// Retract active_shard fact to prevent stale facts from accumulating
		// This cleanup ensures the kernel state accurately reflects which shards are running
		if sm.kernel != nil {
			if retractErr := sm.kernel.RetractFact(Fact{
				Predicate: "active_shard",
				Args:      []interface{}{id, shardTypeAtom},
			}); retractErr != nil {
				logging.ShardsDebug("Shard %s: failed to retract active_shard: %v", id, retractErr)
			} else {
				logging.ShardsDebug("Shard %s: retracted active_shard(%s, %s)", id, id, shardTypeAtom)
			}

			// Fix 15.2: Retract shard_status
			_ = sm.kernel.RetractFact(Fact{
				Predicate: "shard_status",
				Args:      []interface{}{id, "/running", task},
			})
		}

		// Clear tracing context after execution
		if sm.tracingClient != nil {
			sm.tracingClient.ClearShardContext()
		}

		// Unregister shard from APIScheduler
		GetAPIScheduler().UnregisterShard(id)

		sm.recordResult(id, res, err)
	}()

	logging.ShardsDebug("SpawnAsyncWithContext: goroutine launched for shard %s, returning id", id)
	return id, nil
}
