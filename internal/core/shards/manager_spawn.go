package shards

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// SetLimitsEnforcer attaches a limits enforcer for resource constraint checking.
func (sm *ShardManager) SetLimitsEnforcer(enforcer types.LimitsEnforcer) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.limitsEnforcer = enforcer
	logging.ShardsDebug("LimitsEnforcer attached to ShardManager")
}

// SetSpawnQueue attaches a spawn queue for backpressure management.
func (sm *ShardManager) SetSpawnQueue(sq *SpawnQueue) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.spawnQueue = sq
	logging.ShardsDebug("SpawnQueue attached to ShardManager")
}

// StopSpawnQueue stops the spawn queue with a timeout to avoid shutdown hangs.
func (sm *ShardManager) StopSpawnQueue(timeout time.Duration) {
	sm.mu.RLock()
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	if sq == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		_ = sq.Stop()
		close(done)
	}()

	if timeout <= 0 {
		<-done
		return
	}

	select {
	case <-done:
	case <-time.After(timeout):
		logging.Get(logging.CategoryShards).Warn(
			"StopSpawnQueue: timed out after %v; continuing shutdown", timeout,
		)
	}
}

// GetBackpressureStatus returns queue status if a spawn queue is attached.
func (sm *ShardManager) GetBackpressureStatus() *BackpressureStatus {
	sm.mu.RLock()
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	if sq == nil {
		return nil
	}
	status := sq.GetBackpressureStatus()
	return &status
}

// SpawnWithPriority spawns a shard with priority (uses queue if available).
// If no queue is attached, falls back to direct SpawnWithContext.
func (sm *ShardManager) SpawnWithPriority(ctx context.Context, typeName, task string,
	sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {

	typeName = normalizeShardTypeName(typeName)
	if cfg, ok := sm.GetProfile(typeName); ok && cfg.Type == types.ShardTypeSystem {
		// System shards are long-lived; bypass the spawn queue to avoid blocking on completion.
		return sm.SpawnWithContext(ctx, typeName, task, sessionCtx)
	}

	sm.mu.RLock()
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	if sq == nil {
		// No queue, use direct spawn
		return sm.SpawnWithContext(ctx, typeName, task, sessionCtx)
	}

	// Submit to queue and wait
	result, err := sq.SubmitAndWait(ctx, typeName, task, sessionCtx, priority)
	if err != nil {
		return "", err
	}
	if result.Error != nil {
		return result.ShardID, result.Error
	}
	return result.Result, nil
}

// GetActiveShardCount returns the number of currently active shards.
func (sm *ShardManager) GetActiveShardCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.shards)
}

// GetActiveNonSystemShardCount returns the number of active non-system shards.
func (sm *ShardManager) GetActiveNonSystemShardCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	count := 0
	for _, s := range sm.shards {
		if s != nil && s.GetConfig().Type != types.ShardTypeSystem {
			count++
		}
	}
	return count
}

func normalizeShardTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimLeft(typeName, "/")
	return typeName
}

func (sm *ShardManager) Spawn(ctx context.Context, typeName, task string) (string, error) {
	logging.Shards("Spawn: synchronous spawn of %s shard for task", typeName)
	return sm.SpawnWithPriority(ctx, typeName, task, nil, types.PriorityNormal)
}

func (sm *ShardManager) SpawnWithContext(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext) (string, error) {
	timer := logging.StartTimer(logging.CategoryShards, fmt.Sprintf("SpawnWithContext(%s)", typeName))

	id, err := sm.SpawnAsyncWithContext(ctx, typeName, task, sessionCtx)
	if err != nil {
		logging.Get(logging.CategoryShards).Error("SpawnWithContext: async spawn failed for %s: %v", typeName, err)
		return "", err
	}

	logging.ShardsDebug("SpawnWithContext: waiting for shard %s to complete", id)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			res, ok := sm.GetResult(id)
			if ok {
				timer.Stop()
				if res.Error != nil {
					return "", res.Error
				}
				return res.Result, nil
			}
		}
	}
}

func (sm *ShardManager) SpawnAsync(ctx context.Context, typeName, task string) (string, error) {
	return sm.SpawnAsyncWithContext(ctx, typeName, task, nil)
}

func (sm *ShardManager) SpawnAsyncWithContext(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext) (string, error) {
	typeName = normalizeShardTypeName(typeName)
	logging.Shards("SpawnAsyncWithContext: initiating %s shard", typeName)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	var config types.ShardConfig
	profile, hasProfile := sm.profiles[typeName]
	if hasProfile {
		config = profile
	} else {
		config = DefaultGeneralistConfig(typeName)
		if typeName == "ephemeral" {
			config.Type = types.ShardTypeEphemeral
		}
	}

	if sessionCtx != nil {
		config.SessionContext = sessionCtx
	}

	if sm.limitsEnforcer != nil {
		if config.Type != types.ShardTypeSystem {
			activeNonSystem := 0
			for _, s := range sm.shards {
				if s != nil && s.GetConfig().Type != types.ShardTypeSystem {
					activeNonSystem++
				}
			}
			if err := sm.limitsEnforcer.CheckShardLimit(activeNonSystem); err != nil {
				return "", fmt.Errorf("cannot spawn shard %s: %w", typeName, err)
			}
		}
		if err := sm.limitsEnforcer.CheckMemory(); err != nil {
			return "", fmt.Errorf("cannot spawn shard %s: %w", typeName, err)
		}
	}

	if sm.kernel != nil {
		query := ToolRelevanceQuery{
			ShardType:   typeName,
			TokenBudget: 2000,
		}

		if sessionCtx != nil && sessionCtx.UserIntent != nil && sessionCtx.UserIntent.Verb != "" {
			verb := sessionCtx.UserIntent.Verb
			if len(verb) > 0 && verb[0] == '/' {
				verb = verb[1:]
			}
			query.IntentVerb = verb
			query.TargetFile = sessionCtx.UserIntent.Target
		}

		tools := sm.queryRelevantTools(query)
		if len(tools) > 0 {
			if config.SessionContext == nil {
				config.SessionContext = &types.SessionContext{}
			}
			config.SessionContext.AvailableTools = tools
		}
	}

	factory, hasFactory := sm.factories[typeName]

	if !hasFactory && hasProfile {
		baseType := strings.TrimPrefix(strings.ToLower(config.BaseType), "/")
		if baseType != "" {
			if baseFactory, ok := sm.factories[baseType]; ok {
				factory = baseFactory
				hasFactory = true
			}
		}

		if factory == nil && (config.Type == types.ShardTypePersistent || config.Type == types.ShardTypeUser) {
			factory = sm.factories["researcher"]
		}
	}

	if factory == nil {
		if config.Type == types.ShardTypePersistent {
			factory = sm.factories["researcher"]
		}

		if factory == nil {
			factory = func(id string, config types.ShardConfig) types.ShardAgent {
				return NewBaseShardAgent(id, config)
			}
		}
	}

	sm.spawnCounter++
	seq := sm.spawnCounter
	id := fmt.Sprintf("%s-%d-%d", config.Name, time.Now().UnixNano(), seq)
	agent := factory(id, config)

	if (config.Type == types.ShardTypePersistent || config.Type == types.ShardTypeUser) && sm.nerdDir != "" && sm.promptLoader != nil {
		promptsPath := filepath.Join(sm.nerdDir, "agents", typeName, "prompts.yaml")
		if _, err := os.Stat(promptsPath); err == nil {
			sm.promptLoader(ctx, typeName, sm.nerdDir)
		}
	}

	if (config.Type == types.ShardTypePersistent || config.Type == types.ShardTypeUser) && sm.nerdDir != "" && sm.jitRegistrar != nil {
		dbPath := filepath.Join(sm.nerdDir, "shards", fmt.Sprintf("%s_knowledge.db", strings.ToLower(typeName)))
		if _, statErr := os.Stat(dbPath); statErr == nil {
			if regErr := sm.jitRegistrar(typeName, dbPath); regErr == nil {
				sm.activeJITDBs[id] = typeName
			}
		}
	}

	shardTypeAtom := "/" + typeName
	if sm.kernel != nil {
		// Handle kernel assertion errors
		if err := sm.kernel.Assert(types.Fact{
			Predicate: "active_shard",
			Args:      []interface{}{id, shardTypeAtom},
		}); err != nil {
			logging.Get(logging.CategoryShards).Error("Failed to assert active_shard for %s: %v", id, err)
		}

		if err := sm.kernel.Assert(types.Fact{
			Predicate: "shard_status",
			Args:      []interface{}{id, "/running", task},
		}); err != nil {
			logging.Get(logging.CategoryShards).Error("Failed to assert shard_status for %s: %v", id, err)
		}
	}

	if sm.kernel != nil {
		agent.SetParentKernel(sm.kernel)
	}
	if sm.llmClient != nil {
		// Note: Scheduler logic moved to factory/main to avoid core dependency
		agent.SetLLMClient(sm.llmClient)
	}
	if sm.virtualStore != nil {
		if vsc, ok := agent.(VirtualStoreConsumer); ok {
			vsc.SetVirtualStore(sm.virtualStore)
		}
	}
	if sessionCtx != nil {
		agent.SetSessionContext(sessionCtx)
	}

	sm.shards[id] = agent

	logging.Audit().ShardSpawn(id, typeName)

	sessionID := sm.sessionID
	if sessionID == "" {
		sessionID = "current-session"
	}
	ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), sessionID)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("shard %s panicked: %v", id, r)
				logging.Get(logging.CategoryShards).Error("PANIC RECOVERED in shard %s: %v", id, r)
				logging.Audit().ShardComplete(id, task, 0, false, panicErr.Error())

				if sm.kernel != nil {
					// Handle kernel retraction errors
					if err := sm.kernel.RetractFact(types.Fact{
						Predicate: "active_shard",
						Args:      []interface{}{id, shardTypeAtom},
					}); err != nil {
						logging.Get(logging.CategoryShards).Error("Failed to retract active_shard for %s (panic cleanup): %v", id, err)
					}

					if err := sm.kernel.RetractFact(types.Fact{
						Predicate: "shard_status",
						Args:      []interface{}{id, "/running", task},
					}); err != nil {
						logging.Get(logging.CategoryShards).Error("Failed to retract shard_status for %s (panic cleanup): %v", id, err)
					}
				}
				sm.recordResult(id, "", panicErr)
			}
		}()

		logging.Audit().ShardExecute(id, task)

		execTimeout := config.Timeout
		if execTimeout == 0 {
			execTimeout = 15 * time.Minute
		}
		execCtx, execCancel := context.WithTimeout(ctx, execTimeout)
		defer execCancel()

		// Hint to shared LLM clients (e.g. Codex CLI) what capability tier this shard expects,
		// enabling per-shard reasoning multiplexing without changing the LLMClient interface.
		execCtx = context.WithValue(execCtx, types.CtxKeyModelCapability, config.Model.Capability)

		res, err := agent.Execute(execCtx, task)

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logging.Audit().ShardComplete(id, task, 0, err == nil, errMsg)

		// Record the result before cleanup so synchronous callers don't block on kernel churn.
		sm.recordResult(id, res, err)

		if sm.kernel != nil {
			// Handle kernel retraction errors
			if err := sm.kernel.RetractFact(types.Fact{
				Predicate: "active_shard",
				Args:      []interface{}{id, shardTypeAtom},
			}); err != nil {
				logging.Get(logging.CategoryShards).Error("Failed to retract active_shard for %s: %v", id, err)
			}

			if err := sm.kernel.RetractFact(types.Fact{
				Predicate: "shard_status",
				Args:      []interface{}{id, "/running", task},
			}); err != nil {
				logging.Get(logging.CategoryShards).Error("Failed to retract shard_status for %s: %v", id, err)
			}
		}
	}()

	return id, nil
}

func (sm *ShardManager) recordResult(id string, result string, err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.shards, id)

	if typeName, hasJITDB := sm.activeJITDBs[id]; hasJITDB {
		delete(sm.activeJITDBs, id)
		if sm.jitUnregistrar != nil {
			sm.jitUnregistrar(typeName)
		}
	}

	sm.results[id] = types.ShardResult{
		ShardID:   id,
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
}

func (sm *ShardManager) GetResult(id string) (types.ShardResult, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	res, ok := sm.results[id]
	if ok {
		delete(sm.results, id)
	}
	return res, ok
}

func (sm *ShardManager) GetActiveShards() []types.ShardAgent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	active := make([]types.ShardAgent, 0, len(sm.shards))
	for _, s := range sm.shards {
		active = append(active, s)
	}
	return active
}

func (sm *ShardManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	count := len(sm.shards)
	logging.Shards("StopAll: stopping %d active shards", count)
	for id, s := range sm.shards {
		if err := s.Stop(); err != nil {
			logging.Get(logging.CategoryShards).Error("StopAll: failed to stop shard %s: %v", id, err)
		}
	}
	sm.shards = make(map[string]types.ShardAgent)
}

func (sm *ShardManager) StartSystemShards(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryShards, "StartSystemShards")
	defer timer.Stop()

	toStart := make([]string, 0)

	sm.mu.RLock()
	for name, config := range sm.profiles {
		if config.Type == types.ShardTypeSystem {
			if _, disabled := sm.disabled[name]; disabled {
				continue
			}
			if config.StartupMode != "" && config.StartupMode != types.StartupAuto {
				continue
			}
			toStart = append(toStart, name)
		}
	}
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	if sq != nil && sq.IsRunning() {
		return sm.startSystemShardsWithQueue(ctx, toStart, sq)
	}

	for _, name := range toStart {
		_, err := sm.SpawnAsync(ctx, name, "system_start")
		if err != nil {
			logging.Get(logging.CategoryShards).Error("StartSystemShards: failed to start system shard %s: %v", name, err)
		}
	}
	return nil
}

func (sm *ShardManager) startSystemShardsWithQueue(ctx context.Context, toStart []string, sq *SpawnQueue) error {
	for _, name := range toStart {
		_, err := sq.Submit(ctx, name, "system_start", nil, types.PriorityCritical, time.Time{}, true)
		if err != nil {
			logging.Get(logging.CategoryShards).Error("StartSystemShards: queue rejected shard %s: %v", name, err)
			continue
		}
	}
	return nil
}

func (sm *ShardManager) DisableSystemShard(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.disabled == nil {
		sm.disabled = make(map[string]struct{})
	}
	sm.disabled[name] = struct{}{}
}

// ResultToFacts converts a shard execution result into kernel-loadable facts.
func (sm *ShardManager) ResultToFacts(shardID, shardType, task, result string, err error) []types.Fact {
	facts := make([]types.Fact, 0, 5)
	timestamp := time.Now().Unix()

	facts = append(facts, types.Fact{
		Predicate: "shard_executed",
		Args:      []interface{}{shardID, shardType, task, timestamp},
	})

	facts = append(facts, types.Fact{
		Predicate: "last_shard_execution",
		Args:      []interface{}{shardID, shardType, task},
	})

	if err != nil {
		facts = append(facts, types.Fact{
			Predicate: "shard_error",
			Args:      []interface{}{shardID, err.Error()},
		})
	} else {
		facts = append(facts, types.Fact{
			Predicate: "shard_success",
			Args:      []interface{}{shardID},
		})

		output := result
		if len(output) > 4000 {
			output = output[:4000] + "... (truncated)"
		}
		facts = append(facts, types.Fact{
			Predicate: "shard_output",
			Args:      []interface{}{shardID, output},
		})

		summary := sm.extractSummary(shardType, result)
		facts = append(facts, types.Fact{
			Predicate: "recent_shard_context",
			Args:      []interface{}{shardType, task, summary, timestamp},
		})
	}

	return facts
}

func (sm *ShardManager) extractSummary(shardType, result string) string {
	prefix := fmt.Sprintf("[%s] ", shardType)
	if len(result) > 200 {
		return prefix + result[:200]
	}
	return prefix + result
}
