package shards

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// =============================================================================
// SHARD MANAGER
// =============================================================================

// VirtualStoreConsumer interface for agents that need file system access.
type VirtualStoreConsumer interface {
	SetVirtualStore(vs any)
}

// ShardManager orchestrates all shard agents.
type ShardManager struct {
	shards    map[string]types.ShardAgent
	results   map[string]types.ShardResult
	profiles  map[string]types.ShardConfig
	factories map[string]types.ShardFactory
	disabled  map[string]struct{}
	mu        sync.RWMutex

	// Core dependencies to inject into shards
	kernel       types.Kernel
	llmClient    types.LLMClient
	virtualStore any
	// tracingClient TracingClient // Optional: set when llmClient implements TracingClient
	learningStore types.LearningStore

	// Resource limits enforcement
	limitsEnforcer types.LimitsEnforcer

	// SpawnQueue for backpressure management (optional)
	spawnQueue *SpawnQueue

	// Session context for tracing
	sessionID string

	// Workspace paths and callbacks for prompt loading
	nerdDir        string                 // Path to .nerd directory
	promptLoader   types.PromptLoaderFunc // Callback to load agent prompts (avoids import cycle)
	jitRegistrar   types.JITDBRegistrar   // Callback to register agent DBs with JIT compiler
	jitUnregistrar types.JITDBUnregistrar // Callback to unregister agent DBs when shard deactivates
	activeJITDBs   map[string]string      // Tracks which shards have registered JIT DBs (shardID -> typeName)
}

func NewShardManager() *ShardManager {
	logging.Shards("Creating new ShardManager")
	sm := &ShardManager{
		shards:       make(map[string]types.ShardAgent),
		results:      make(map[string]types.ShardResult),
		profiles:     make(map[string]types.ShardConfig),
		factories:    make(map[string]types.ShardFactory),
		activeJITDBs: make(map[string]string),
	}
	logging.ShardsDebug("ShardManager initialized with empty maps")
	return sm
}

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

func (sm *ShardManager) SetParentKernel(k types.Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kernel = k
	logging.ShardsDebug("Parent kernel attached to ShardManager")
}

func (sm *ShardManager) SetVirtualStore(vs any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.virtualStore = vs
	logging.ShardsDebug("VirtualStore attached to ShardManager")
}

func (sm *ShardManager) SetLLMClient(client types.LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
	// Tracing support would check interface here
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

func (sm *ShardManager) SetPromptLoader(loader types.PromptLoaderFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.promptLoader = loader
	logging.ShardsDebug("Prompt loader callback set")
}

func (sm *ShardManager) SetJITRegistrar(registrar types.JITDBRegistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitRegistrar = registrar
	logging.ShardsDebug("JIT registrar callback set")
}

func (sm *ShardManager) SetJITUnregistrar(unregistrar types.JITDBUnregistrar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.jitUnregistrar = unregistrar
	logging.ShardsDebug("JIT unregistrar callback set")
}

func (sm *ShardManager) categorizeShardType(typeName string, shardType types.ShardType) string {
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
	if systemShards[typeName] || shardType == types.ShardTypeSystem {
		return "system"
	}

	// Ephemeral shards (built-in factories)
	ephemeralShards := map[string]bool{
		"coder":      true,
		"tester":     true,
		"reviewer":   true,
		"researcher": true,
	}
	if ephemeralShards[typeName] || shardType == types.ShardTypeEphemeral {
		return "ephemeral"
	}

	// Everything else is a specialist (LLM-created or user-created)
	return "specialist"
}

func (sm *ShardManager) SetLearningStore(store types.LearningStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.learningStore = store
	logging.ShardsDebug("LearningStore attached to ShardManager")
}

// queryToolsFromKernel queries the Mangle kernel for registered tools.
func (sm *ShardManager) queryToolsFromKernel() []types.ToolInfo {
	if sm.kernel == nil {
		logging.ShardsDebug("queryToolsFromKernel: no kernel available")
		return nil
	}

	logging.ShardsDebug("queryToolsFromKernel: querying tool_registered predicate")

	// Query all registered tools
	registeredFacts, err := sm.kernel.Query("tool_registered")
	if err != nil || len(registeredFacts) == 0 {
		logging.ShardsDebug("queryToolsFromKernel: no registered tools found")
		return nil
	}
	logging.ShardsDebug("queryToolsFromKernel: found %d registered tools", len(registeredFacts))

	// Build a map of tool names for lookup
	toolNames := make([]string, 0, len(registeredFacts))
	for _, fact := range registeredFacts {
		if len(fact.Args) >= 1 {
			if name, ok := fact.Args[0].(string); ok {
				toolNames = append(toolNames, name)
			}
		}
	}

	if len(toolNames) == 0 {
		return nil
	}

	// Query descriptions and binary paths
	descFacts, _ := sm.kernel.Query("tool_description")
	pathFacts, _ := sm.kernel.Query("tool_binary_path")

	// Build lookup maps
	descriptions := make(map[string]string)
	for _, fact := range descFacts {
		if len(fact.Args) >= 2 {
			if name, ok := fact.Args[0].(string); ok {
				if desc, ok := fact.Args[1].(string); ok {
					descriptions[name] = desc
				}
			}
		}
	}

	binaryPaths := make(map[string]string)
	for _, fact := range pathFacts {
		if len(fact.Args) >= 2 {
			if name, ok := fact.Args[0].(string); ok {
				if path, ok := fact.Args[1].(string); ok {
					binaryPaths[name] = path
				}
			}
		}
	}

	// Build ToolInfo slice
	tools := make([]types.ToolInfo, 0, len(toolNames))
	for _, name := range toolNames {
		tools = append(tools, types.ToolInfo{
			Name:        name,
			Description: descriptions[name],
			BinaryPath:  binaryPaths[name],
		})
	}

	return tools
}

// ToolRelevanceQuery holds parameters for intelligent tool discovery.
type ToolRelevanceQuery struct {
	ShardType   string
	IntentVerb  string
	TargetFile  string
	TokenBudget int
}

func (sm *ShardManager) queryRelevantTools(query ToolRelevanceQuery) []types.ToolInfo {
	if sm.kernel == nil {
		return nil
	}

	sm.assertToolRoutingContext(query)

	shardAtom := normalizeMangleAtom(query.ShardType)
	relevantFacts, err := sm.kernel.Query("relevant_tool")
	if err != nil || len(relevantFacts) == 0 {
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	relevantToolNames := make([]string, 0)
	for _, fact := range relevantFacts {
		if len(fact.Args) >= 2 {
			factShardType, _ := fact.Args[0].(string)
			toolName, _ := fact.Args[1].(string)
			if factShardType == shardAtom && toolName != "" {
				relevantToolNames = append(relevantToolNames, toolName)
			}
		}
	}

	if len(relevantToolNames) == 0 {
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	allTools := sm.queryToolsFromKernel()
	if allTools == nil {
		return nil
	}

	relevantSet := make(map[string]bool)
	for _, name := range relevantToolNames {
		relevantSet[name] = true
	}

	tools := make([]types.ToolInfo, 0)
	for _, tool := range allTools {
		if relevantSet[tool.Name] {
			tools = append(tools, tool)
		}
	}

	sm.sortToolsByPriority(tools, query.ShardType)

	return sm.trimToTokenBudget(tools, query.TokenBudget)
}

func (sm *ShardManager) assertToolRoutingContext(query ToolRelevanceQuery) {
	if sm.kernel == nil {
		return
	}

	_ = sm.kernel.Retract("current_shard_type")
	_ = sm.kernel.Retract("current_intent")
	_ = sm.kernel.Retract("current_time")

	shardAtom := normalizeMangleAtom(query.ShardType)
	_ = sm.kernel.Assert(types.Fact{
		Predicate: "current_shard_type",
		Args:      []interface{}{shardAtom},
	})

	if query.IntentVerb != "" {
		intentID := "/tool_routing_context"
		verbAtom := normalizeMangleAtom(query.IntentVerb)
		_ = sm.kernel.RetractFact(types.Fact{Predicate: "user_intent", Args: []interface{}{intentID}})
		_ = sm.kernel.Assert(types.Fact{
			Predicate: "current_intent",
			Args:      []interface{}{intentID},
		})
		_ = sm.kernel.Assert(types.Fact{
			Predicate: "user_intent",
			Args:      []interface{}{intentID, "/routing", verbAtom, query.TargetFile, "_"},
		})
	}

	_ = sm.kernel.Assert(types.Fact{
		Predicate: "current_time",
		Args:      []interface{}{int64(time.Now().Unix())},
	})
}

func (sm *ShardManager) sortToolsByPriority(tools []types.ToolInfo, shardType string) {
	if sm.kernel == nil || len(tools) == 0 {
		return
	}

	shardAtom := normalizeMangleAtom(shardType)
	baseRelevanceFacts, _ := sm.kernel.Query("tool_base_relevance")

	scores := make(map[string]float64)
	for _, fact := range baseRelevanceFacts {
		if len(fact.Args) >= 3 {
			factShardType, _ := fact.Args[0].(string)
			toolName, _ := fact.Args[1].(string)
			score, _ := fact.Args[2].(float64)
			if factShardType == shardAtom {
				scores[toolName] = score
			}
		}
	}

	sort.Slice(tools, func(i, j int) bool {
		scoreI := scores[tools[i].Name]
		scoreJ := scores[tools[j].Name]
		return scoreI > scoreJ
	})
}

func normalizeMangleAtom(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "/")
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	if value == "" {
		return ""
	}
	return "/" + value
}

func normalizeShardTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimLeft(typeName, "/")
	return typeName
}

func (sm *ShardManager) trimToTokenBudget(tools []types.ToolInfo, budget int) []types.ToolInfo {
	if budget <= 0 {
		budget = 2000
	}

	result := make([]types.ToolInfo, 0)
	tokensUsed := 0

	for _, tool := range tools {
		toolTokens := estimateTokens(tool.Name) +
			estimateTokens(tool.Description) +
			estimateTokens(tool.BinaryPath) + 20

		if tokensUsed+toolTokens <= budget {
			result = append(result, tool)
			tokensUsed += toolTokens
		} else {
			break
		}
	}

	return result
}

func estimateTokens(s string) int {
	return len(s) / 4
}

func (sm *ShardManager) RegisterShard(typeName string, factory types.ShardFactory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.factories[typeName] = factory
	logging.Shards("Registered shard factory: %s", typeName)
}

func (sm *ShardManager) DefineProfile(name string, config types.ShardConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.profiles[name] = config
	logging.Shards("Defined shard profile: %s (type: %s)", name, config.Type)
}

func (sm *ShardManager) GetProfile(name string) (types.ShardConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	cfg, ok := sm.profiles[name]
	return cfg, ok
}

// ShardInfo contains information about an available shard for selection.
type ShardInfo struct {
	Name         string          `json:"name"`
	Type         types.ShardType `json:"type"`
	Description  string          `json:"description,omitempty"`
	HasKnowledge bool            `json:"has_knowledge"`
}

func (sm *ShardManager) ListAvailableShards() []ShardInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var shards []ShardInfo

	for name := range sm.factories {
		shardType := types.ShardTypeEphemeral
		if name == "perception_firewall" || name == "executive_policy" ||
			name == "constitution_gate" || name == "tactile_router" ||
			name == "session_planner" || name == "world_model_ingestor" {
			shardType = types.ShardTypeSystem
		}
		shards = append(shards, ShardInfo{
			Name: name,
			Type: shardType,
		})
	}

	for name, profile := range sm.profiles {
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

func (sm *ShardManager) GetRunningShardByConfigName(name string) (types.ShardAgent, bool) {
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

	id := fmt.Sprintf("%s-%d", config.Name, time.Now().UnixNano())
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
		_ = sm.kernel.Assert(types.Fact{
			Predicate: "active_shard",
			Args:      []interface{}{id, shardTypeAtom},
		})
		_ = sm.kernel.Assert(types.Fact{
			Predicate: "shard_status",
			Args:      []interface{}{id, "/running", task},
		})
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
					_ = sm.kernel.RetractFact(types.Fact{
						Predicate: "active_shard",
						Args:      []interface{}{id, shardTypeAtom},
					})
					_ = sm.kernel.RetractFact(types.Fact{
						Predicate: "shard_status",
						Args:      []interface{}{id, "/running", task},
					})
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

		res, err := agent.Execute(execCtx, task)

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logging.Audit().ShardComplete(id, task, 0, err == nil, errMsg)

		if sm.kernel != nil {
			_ = sm.kernel.RetractFact(types.Fact{
				Predicate: "active_shard",
				Args:      []interface{}{id, shardTypeAtom},
			})
			_ = sm.kernel.RetractFact(types.Fact{
				Predicate: "shard_status",
				Args:      []interface{}{id, "/running", task},
			})
		}

		sm.recordResult(id, res, err)
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

func (sm *ShardManager) ToFacts() []types.Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	facts := make([]types.Fact, 0)

	for name, cfg := range sm.profiles {
		facts = append(facts, types.Fact{
			Predicate: "shard_profile",
			Args:      []interface{}{name, string(cfg.Type)},
		})
	}

	return facts
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

		// structuredFacts := sm.parseShardSpecificFacts(shardID, shardType, result)
		// facts = append(facts, structuredFacts...)
	}

	return facts
}

func (sm *ShardManager) extractSummary(shardType, result string) string {
	if len(result) > 200 {
		return result[:200]
	}
	return result
}

// ReviewerFeedbackProvider interface usage
var reviewerFeedbackProvider types.ReviewerFeedbackProvider

func SetReviewerFeedbackProvider(provider types.ReviewerFeedbackProvider) {
	reviewerFeedbackProvider = provider
}

func (sm *ShardManager) CheckReviewNeedsValidation(reviewID string) bool {
	if reviewerFeedbackProvider == nil {
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("reviewer_needs_validation")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 && fact.Args[0] == reviewID {
						return true
					}
				}
			}
		}
		return false
	}
	return reviewerFeedbackProvider.NeedsValidation(reviewID)
}
