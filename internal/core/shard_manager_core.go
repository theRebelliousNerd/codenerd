package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/transparency"
)

// =============================================================================
// SHARD MANAGER - CORE STRUCTURE AND BASIC OPERATIONS
// =============================================================================

// ShardFactory is a function that creates a new shard instance.
type ShardFactory func(id string, config ShardConfig) ShardAgent

// PromptLoaderFunc is a callback for loading agent prompts from YAML files.
// Parameters: ctx, agentName, nerdDir
// Returns: number of atoms loaded, error
type PromptLoaderFunc func(context.Context, string, string) (int, error)

// JITDBRegistrar is a callback for registering agent knowledge DBs with the JIT prompt compiler.
// This allows the JIT compiler to load prompt atoms from agent-specific databases.
// Parameters: agentName, dbPath
// Returns: error if registration fails
type JITDBRegistrar func(agentName string, dbPath string) error

// JITDBUnregistrar is a callback for unregistering agent knowledge DBs from the JIT prompt compiler.
// This is called when a shard is deactivated to close the DB connection and free resources.
// Parameters: agentName (the shard type name, not the instance ID)
type JITDBUnregistrar func(agentName string)

// ShardManager orchestrates all shard agents.
type ShardManager struct {
	shards    map[string]ShardAgent
	results   map[string]ShardResult
	profiles  map[string]ShardConfig
	factories map[string]ShardFactory
	disabled  map[string]struct{}
	mu        sync.RWMutex

	// Core dependencies to inject into shards
	kernel        Kernel
	llmClient     LLMClient
	virtualStore  *VirtualStore
	tracingClient TracingClient // Optional: set when llmClient implements TracingClient
	learningStore LearningStore
	transparencyMgr *transparency.TransparencyManager

	// Resource limits enforcement
	limitsEnforcer *LimitsEnforcer

	// SpawnQueue for backpressure management (optional)
	spawnQueue *SpawnQueue

	// Session context for tracing
	sessionID string

	// Workspace paths and callbacks for prompt loading
	nerdDir        string            // Path to .nerd directory
	promptLoader   PromptLoaderFunc  // Callback to load agent prompts (avoids import cycle)
	jitRegistrar   JITDBRegistrar    // Callback to register agent DBs with JIT compiler
	jitUnregistrar JITDBUnregistrar  // Callback to unregister agent DBs when shard deactivates
	activeJITDBs   map[string]string // Tracks which shards have registered JIT DBs (shardID -> typeName)
}

func NewShardManager() *ShardManager {
	logging.Shards("Creating new ShardManager")
	sm := &ShardManager{
		shards:       make(map[string]ShardAgent),
		results:      make(map[string]ShardResult),
		profiles:     make(map[string]ShardConfig),
		factories:    make(map[string]ShardFactory),
		activeJITDBs: make(map[string]string),
	}
	logging.ShardsDebug("ShardManager initialized with empty maps")
	return sm
}

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

func (sm *ShardManager) SetParentKernel(k Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kernel = k
	logging.ShardsDebug("Parent kernel attached to ShardManager")
}

func (sm *ShardManager) SetVirtualStore(vs *VirtualStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.virtualStore = vs
	logging.ShardsDebug("VirtualStore attached to ShardManager")
}

func (sm *ShardManager) SetLLMClient(client LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
	// Check if client supports tracing
	if tc, ok := client.(TracingClient); ok {
		sm.tracingClient = tc
		logging.ShardsDebug("LLM client attached with tracing support")
	} else {
		logging.ShardsDebug("LLM client attached (no tracing support)")
	}
}

func (sm *ShardManager) SetSessionID(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionID = sessionID
	logging.ShardsDebug("Session ID set: %s", sessionID)
}

func (sm *ShardManager) SetNerdDir(nerdDir string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.nerdDir = nerdDir
	logging.ShardsDebug("Nerd directory set: %s", nerdDir)
}

func (sm *ShardManager) SetPromptLoader(loader PromptLoaderFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.promptLoader = loader
	logging.ShardsDebug("Prompt loader callback set")
}

// SetJITRegistrar sets the callback for registering agent DBs with the JIT prompt compiler.
// This enables the JIT compiler to access agent-specific prompt atoms during compilation.
func (sm *ShardManager) SetJITRegistrar(registrar JITDBRegistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitRegistrar = registrar
	logging.ShardsDebug("JIT registrar callback set")
}

// SetJITUnregistrar sets the callback for unregistering agent DBs from the JIT prompt compiler.
// This is called when shards are deactivated to close DB connections and free resources.
func (sm *ShardManager) SetJITUnregistrar(unregistrar JITDBUnregistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitUnregistrar = unregistrar
	logging.ShardsDebug("JIT unregistrar callback set")
}

func (sm *ShardManager) SetLearningStore(store LearningStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.learningStore = store
	logging.ShardsDebug("LearningStore attached to ShardManager")
}

func (sm *ShardManager) SetTransparencyManager(tm *transparency.TransparencyManager) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transparencyMgr = tm
	logging.ShardsDebug("TransparencyManager attached to ShardManager")
}

// SetLimitsEnforcer attaches a limits enforcer for resource constraint checking.
func (sm *ShardManager) SetLimitsEnforcer(enforcer *LimitsEnforcer) {
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

// =============================================================================
// PROFILE AND FACTORY MANAGEMENT
// =============================================================================

// RegisterShard registers a factory for a given shard type.
func (sm *ShardManager) RegisterShard(typeName string, factory ShardFactory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.factories[typeName] = factory
	logging.Shards("Registered shard factory: %s (total factories: %d)", typeName, len(sm.factories))
}

// DefineProfile registers a shard configuration profile.
func (sm *ShardManager) DefineProfile(name string, config ShardConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.profiles[name] = config
	logging.Shards("Defined shard profile: %s (type: %s, total profiles: %d)", name, config.Type, len(sm.profiles))
}

// GetProfile retrieves a profile by name.
func (sm *ShardManager) GetProfile(name string) (ShardConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	cfg, ok := sm.profiles[name]
	return cfg, ok
}

// ListAvailableShards returns information about all available shards.
// This includes system shards, LLM-created specialists, user-created specialists, and ephemeral shards.
func (sm *ShardManager) ListAvailableShards() []ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var shards []ShardInfo

	// 1. Add registered factories (ephemeral and system shards)
	for name := range sm.factories {
		shardType := ShardTypeEphemeral
		// Check if it's a system shard
		if name == "perception_firewall" || name == "executive_policy" ||
			name == "constitution_gate" || name == "tactile_router" ||
			name == "session_planner" || name == "world_model_ingestor" {
			shardType = ShardTypeSystem
		}
		shards = append(shards, ShardInfo{
			Name: name,
			Type: shardType,
		})
	}

	// 2. Add defined profiles (specialists - both LLM-created and user-created)
	for name, profile := range sm.profiles {
		// Skip if already added via factory
		alreadyAdded := false
		for _, s := range shards {
			if s.Name == name {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			continue
		}

		shards = append(shards, ShardInfo{
			Name:         name,
			Type:         profile.Type,
			HasKnowledge: profile.KnowledgePath != "",
		})
	}

	return shards
}

// GetRunningShardByConfigName returns the first active shard whose ShardConfig.Name matches.
// This is primarily used to interact with long-running system shards (Type S) that expose
// additional methods beyond the core ShardAgent interface.
func (sm *ShardManager) GetRunningShardByConfigName(name string) (ShardAgent, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, shard := range sm.shards {
		if shard == nil {
			continue
		}
		cfg := shard.GetConfig()
		if cfg.Name == name {
			return shard, true
		}
	}
	return nil, false
}

// =============================================================================
// ACTIVE SHARD TRACKING
// =============================================================================

// GetActiveShardCount returns the number of currently active shards.
func (sm *ShardManager) GetActiveShardCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.shards)
}

// GetActiveNonSystemShardCount returns the number of active non-system shards.
// This is used for concurrency limits so background system shards do not
// reduce the available user/task shard budget.
func (sm *ShardManager) GetActiveNonSystemShardCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	count := 0
	for _, s := range sm.shards {
		if s != nil && s.GetConfig().Type != ShardTypeSystem {
			count++
		}
	}
	return count
}

// GetActiveShards returns all currently active shards.
func (sm *ShardManager) GetActiveShards() []ShardAgent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	active := make([]ShardAgent, 0, len(sm.shards))
	for _, s := range sm.shards {
		active = append(active, s)
	}
	return active
}

// =============================================================================
// RESULT MANAGEMENT
// =============================================================================

func (sm *ShardManager) GetResult(id string) (ShardResult, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	res, ok := sm.results[id]
	if ok {
		// clean up to prevent unbounded growth
		delete(sm.results, id)
	}
	return res, ok
}

func (sm *ShardManager) recordResult(id string, result string, err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clean up shard
	delete(sm.shards, id)
	logging.ShardsDebug("recordResult: shard %s removed from active shards (remaining: %d)", id, len(sm.shards))

	// Unregister JIT DB if this shard had one registered
	if typeName, hasJITDB := sm.activeJITDBs[id]; hasJITDB {
		delete(sm.activeJITDBs, id)
		if sm.jitUnregistrar != nil {
			sm.jitUnregistrar(typeName)
			logging.ShardsDebug("recordResult: unregistered JIT DB for shard %s (type: %s)", id, typeName)
		}
	}

	sm.results[id] = ShardResult{
		ShardID:   id,
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
	logging.ShardsDebug("recordResult: result stored for shard %s (success=%v, resultLen=%d)", id, err == nil, len(result))
}

// =============================================================================
// LIFECYCLE MANAGEMENT
// =============================================================================

// StopAll stops all active shards.
func (sm *ShardManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	count := len(sm.shards)
	logging.Shards("StopAll: stopping %d active shards", count)
	for id, s := range sm.shards {
		logging.ShardsDebug("StopAll: stopping shard %s", id)
		if err := s.Stop(); err != nil {
			logging.Get(logging.CategoryShards).Error("StopAll: failed to stop shard %s: %v", id, err)
		}
	}
	// Clear shards
	sm.shards = make(map[string]ShardAgent)
	logging.Shards("StopAll: all shards stopped and cleared")
}

// DisableSystemShard prevents a system shard from auto-starting.
func (sm *ShardManager) DisableSystemShard(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.disabled == nil {
		sm.disabled = make(map[string]struct{})
	}
	sm.disabled[name] = struct{}{}
	logging.Shards("DisableSystemShard: disabled system shard %s", name)
}

// =============================================================================
// FACT CONVERSION
// =============================================================================

func (sm *ShardManager) ToFacts() []Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	facts := make([]Fact, 0)

	for name, cfg := range sm.profiles {
		facts = append(facts, Fact{
			Predicate: "shard_profile",
			Args:      []interface{}{name, string(cfg.Type)},
		})
	}

	return facts
}

// =============================================================================
// SYSTEM SHARD MANAGEMENT
// =============================================================================

// StartSystemShards starts all registered system shards (Type S).
// Uses the spawn queue with PriorityCritical to queue shards when limits are reached
// instead of failing immediately.
func (sm *ShardManager) StartSystemShards(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryShards, "StartSystemShards")
	defer timer.Stop()

	// Collect system shards to start
	toStart := make([]string, 0)

	sm.mu.RLock()
	for name, config := range sm.profiles {
		if config.Type == ShardTypeSystem {
			// Skip if disabled
			if _, disabled := sm.disabled[name]; disabled {
				logging.ShardsDebug("StartSystemShards: skipping disabled shard %s", name)
				continue
			}
			toStart = append(toStart, name)
		}
	}
	sq := sm.spawnQueue
	sm.mu.RUnlock()

	logging.Shards("StartSystemShards: starting %d system shards", len(toStart))

	// If spawn queue is available, use it for queuing instead of failing
	if sq != nil && sq.IsRunning() {
		return sm.startSystemShardsWithQueue(ctx, toStart, sq)
	}

	// Fallback: direct spawn (may hit limits)
	started := 0
	for _, name := range toStart {
		_, err := sm.SpawnAsync(ctx, name, "system_start")
		if err != nil {
			logging.Get(logging.CategoryShards).Error("StartSystemShards: failed to start system shard %s: %v", name, err)
		} else {
			started++
			logging.ShardsDebug("StartSystemShards: started system shard %s", name)
		}
	}

	logging.Shards("StartSystemShards: started %d/%d system shards", started, len(toStart))
	return nil
}

// startSystemShardsWithQueue uses the spawn queue to queue system shards.
// This allows system shards to start sequentially when LLM concurrency limits are reached.
func (sm *ShardManager) startSystemShardsWithQueue(ctx context.Context, toStart []string, sq *SpawnQueue) error {
	started := 0
	queued := 0

	for _, name := range toStart {
		req := SpawnRequest{
			ID:          fmt.Sprintf("system-%s-%d", name, time.Now().UnixNano()),
			TypeName:    name,
			Task:        "system_start",
			Priority:    PriorityCritical,
			SubmittedAt: time.Now(),
			Ctx:         ctx,
			Detached:    true, // Critical: Don't block worker waiting for infinite system shard
		}

		// Submit to queue - don't wait for result, let them process asynchronously
		resultCh, err := sq.Submit(ctx, req)
		if err != nil {
			logging.Get(logging.CategoryShards).Error("StartSystemShards: queue rejected shard %s: %v", name, err)
			continue
		}

		queued++
		logging.ShardsDebug("StartSystemShards: queued system shard %s", name)

		// Start a goroutine to track completion
		go func(shardName string, ch <-chan SpawnResult) {
			select {
			case result := <-ch:
				if result.Error != nil {
					logging.Get(logging.CategoryShards).Error("StartSystemShards: system shard %s failed: %v", shardName, result.Error)
				} else {
					logging.ShardsDebug("StartSystemShards: system shard %s started (waited %v)", shardName, result.Queued)
				}
			case <-ctx.Done():
				logging.Get(logging.CategoryShards).Error("StartSystemShards: context cancelled waiting for shard %s", shardName)
			}
		}(name, resultCh)

		started++
	}

	logging.Shards("StartSystemShards: queued %d/%d system shards (will start as capacity allows)", queued, len(toStart))
	return nil
}

// =============================================================================
// BACKPRESSURE STATUS
// =============================================================================

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

// =============================================================================
// UTILITIES
// =============================================================================

// categorizeShardType determines the shard category based on type name and config.
func (sm *ShardManager) categorizeShardType(typeName string, shardType ShardType) string {
	// System shards (built-in, always-on)
	systemShards := map[string]bool{
		"perception_firewall":  true,
		"constitution_gate":    true,
		"executive_policy":     true,
		"cost_guard":           true,
		"tactile_router":       true,
		"session_planner":      true,
		"world_model_ingestor": true,
	}
	if systemShards[typeName] || shardType == ShardTypeSystem {
		return "system"
	}

	// Ephemeral shards (built-in factories)
	ephemeralShards := map[string]bool{
		"coder":      true,
		"tester":     true,
		"reviewer":   true,
		"researcher": true,
	}
	if ephemeralShards[typeName] || shardType == ShardTypeEphemeral {
		return "ephemeral"
	}

	// Everything else is a specialist (LLM-created or user-created)
	return "specialist"
}

// VirtualStoreConsumer interface for agents that need file system access.
type VirtualStoreConsumer interface {
	SetVirtualStore(vs *VirtualStore)
}
