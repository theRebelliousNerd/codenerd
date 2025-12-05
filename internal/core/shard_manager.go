package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ShardType defines the type of shard agent.
type ShardType string

const (
	// Type 1: System Level (Permanent)
	// Lifecycle: Always On
	// Memory: Persistent, High-Performance
	ShardTypeSystem ShardType = "system"

	// Type 2: Ephemeral (LLM Created, Non-Persistent)
	// Lifecycle: Spawn -> Execute Task -> Die
	// Memory: RAM Only
	ShardTypeEphemeral ShardType = "ephemeral"

	// Type 3: Persistent (LLM Created)
	// Lifecycle: Long-running background tasks
	// Memory: Persistent SQLite (Task-Specific)
	ShardTypePersistent ShardType = "persistent"

	// Type 4: User Configured (Persistent)
	// Lifecycle: Explicitly defined by User CLI
	// Memory: Deep Domain Knowledge
	ShardTypeUser ShardType = "user"
)

// ModelCapability defines the reasoning tier required.
type ModelCapability string

const (
	CapabilityHighReasoning ModelCapability = "high_reasoning" // e.g. Gemini 1.5 Pro, Claude Opus
	CapabilityHighSpeed     ModelCapability = "high_speed"     // e.g. Gemini 1.5 Flash, Claude Haiku
	CapabilityBalanced      ModelCapability = "balanced"       // e.g. Gemini 1.5 Flash-8B, Claude Sonnet
)

// ModelConfig defines the LLM configuration for a shard.
type ModelConfig struct {
	Provider   string          // "google", "anthropic", "openai"
	ModelName  string          // "gemini-1.5-pro", "claude-3-opus"
	Capability ModelCapability // The abstract capability level
}

// ShardState represents the lifecycle state of a shard.
type ShardState string

const (
	ShardStateIdle      ShardState = "idle"
	ShardStateSpawning  ShardState = "spawning"
	ShardStateRunning   ShardState = "running"
	ShardStateCompleted ShardState = "completed"
	ShardStateFailed    ShardState = "failed"
	ShardStateSleeping  ShardState = "sleeping"  // For persistent specialists
	ShardStateHydrating ShardState = "hydrating" // Loading knowledge shard
)

// ShardPermission defines what a shard is allowed to do.
type ShardPermission string

const (
	PermissionReadFile  ShardPermission = "read_file"
	PermissionWriteFile ShardPermission = "write_file"
	PermissionExecCmd   ShardPermission = "exec_cmd"
	PermissionNetwork   ShardPermission = "network"
	PermissionBrowser   ShardPermission = "browser"
	PermissionCodeGraph ShardPermission = "code_graph"
	PermissionResearch  ShardPermission = "research"
	PermissionAskUser   ShardPermission = "ask_user"
)

type ShardConfig struct {
	Name          string
	Type          ShardType
	Permissions   []ShardPermission
	KnowledgePath string // Path to SQLite knowledge shard (for specialists)
	Timeout       time.Duration
	MemoryLimit   int         // Max facts in working memory
	Model         ModelConfig // Purpose-Driven Model Selection
}

// DefaultGeneralistConfig returns config for ephemeral generalists.
func DefaultGeneralistConfig(name string) ShardConfig {
	return ShardConfig{
		Name: name,
		Type: ShardTypeEphemeral,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionExecCmd,
		},
		Timeout:     5 * time.Minute,
		MemoryLimit: 1000,
		Model: ModelConfig{
			Capability: CapabilityHighSpeed, // Default to speed for ephemeral tasks
		},
	}
}

// DefaultSpecialistConfig returns config for persistent specialists.
func DefaultSpecialistConfig(name, knowledgePath string) ShardConfig {
	return ShardConfig{
		Name: name,
		Type: ShardTypeUser,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionExecCmd,
			PermissionCodeGraph,
			PermissionResearch,
		},
		KnowledgePath: knowledgePath,
		Timeout:       30 * time.Minute,
		MemoryLimit:   10000,
		Model: ModelConfig{
			Capability: CapabilityHighReasoning, // Experts need high reasoning
		},
	}
}

// DefaultSystemConfig returns config for Type 1 (permanent) system shards.
func DefaultSystemConfig(name string) ShardConfig {
	return ShardConfig{
		Name: name,
		Type: ShardTypeSystem,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionAskUser,
		},
		Timeout:     24 * time.Hour, // Permanent shards
		MemoryLimit: 5000,
		Model: ModelConfig{
			Capability: CapabilityBalanced,
		},
	}
}

// ShardAgent defines the interface for a shard agent.
type ShardAgent interface {
	Execute(ctx context.Context, task string) (string, error)
	GetID() string
	GetState() ShardState
	GetConfig() ShardConfig
	Stop() error
}

// ShardResult represents the result of a shard execution.
type ShardResult struct {
	ShardID   string
	Task      string
	Output    string
	Error     error
	Duration  time.Duration
	Facts     []Fact // Facts to propagate back to parent
	Timestamp time.Time
}

// BaseShardAgent implements common shard functionality.
type BaseShardAgent struct {
	mu sync.RWMutex

	id           string
	config       ShardConfig
	state        ShardState
	kernel       *RealKernel
	virtualStore *VirtualStore
	llmClient    LLMClient

	startTime time.Time
	stopCh    chan struct{}
}

// NewBaseShardAgent creates a new base shard agent.
func NewBaseShardAgent(id string, config ShardConfig) *BaseShardAgent {
	return &BaseShardAgent{
		id:     id,
		config: config,
		state:  ShardStateIdle,
		kernel: NewRealKernel(),
		stopCh: make(chan struct{}),
	}
}

// GetID returns the shard ID.
func (s *BaseShardAgent) GetID() string {
	return s.id
}

// GetState returns the current state.
func (s *BaseShardAgent) GetState() ShardState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetConfig returns the shard configuration.
func (s *BaseShardAgent) GetConfig() ShardConfig {
	return s.config
}

// SetLLMClient wires an LLM client into the shard (used by LLM-enabled shards).
func (s *BaseShardAgent) SetLLMClient(client LLMClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llmClient = client
}

// llm returns the configured LLM client (may be nil).
func (s *BaseShardAgent) llm() LLMClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.llmClient
}

// SetState sets the shard state.
func (s *BaseShardAgent) SetState(state ShardState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// Stop stops the shard.
func (s *BaseShardAgent) Stop() error {
	close(s.stopCh)
	s.SetState(ShardStateCompleted)
	return nil
}

// HasPermission checks if the shard has a specific permission.
func (s *BaseShardAgent) HasPermission(perm ShardPermission) bool {
	for _, p := range s.config.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// Execute executes a task (to be overridden by specific implementations).
func (s *BaseShardAgent) Execute(ctx context.Context, task string) (string, error) {
	s.mu.Lock()
	s.state = ShardStateRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	// Default implementation just returns the task
	// Specific shard types override this
	return fmt.Sprintf("Executed task: %s", task), nil
}

// LearningStore interface for Autopoiesis persistence (§8.3).
type LearningStore interface {
	Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error
	Load(shardType string) ([]ShardLearning, error)
	DecayConfidence(shardType string, decayFactor float64) error
	Close() error
}

// ShardLearning represents a persisted learning.
type ShardLearning struct {
	FactPredicate string
	FactArgs      []any
	Confidence    float64
}

// ShardManager acts as the Hypervisor for ShardAgents.
// Implements Cortex 1.5.0 §7.0 Sharding (Scalability Layer)
type ShardManager struct {
	mu sync.RWMutex

	// Registered shard types
	shardFactories map[ShardType]func(string, ShardConfig) ShardAgent

	// Active shards
	activeShards map[string]ShardAgent

	// Completed results (for retrieval)
	results map[string]*ShardResult

	// Shard profiles for specialists (§9.1)
	profiles map[string]ShardConfig

	// Counter for generating unique IDs
	counter uint64

	// Parent kernel for fact propagation
	parentKernel Kernel

	// Concurrency control
	maxConcurrent int
	semaphore     chan struct{}

	// System shard lifecycle management (Type 1)
	systemShardCancels map[string]context.CancelFunc

	// Shared LLM client for shards (optional)
	llmClient LLMClient

	// Learning store for Autopoiesis (§8.3)
	learningStore LearningStore

	// Current campaign ID (for learning attribution)
	currentCampaignID string
}

// NewShardManager creates a new shard manager.
func NewShardManager() *ShardManager {
	sm := &ShardManager{
		shardFactories: make(map[ShardType]func(string, ShardConfig) ShardAgent),
		activeShards:   make(map[string]ShardAgent),
		results:        make(map[string]*ShardResult),
		profiles:       make(map[string]ShardConfig),
		maxConcurrent:  10,
		semaphore:      make(chan struct{}, 10),
	}

	// Register default shard factories
	sm.registerDefaultFactories()
	// Note: System shard profiles are now registered via shards.RegisterAllShardFactories()
	// which includes the full implementations in internal/shards/system/

	return sm
}

// System shard identifiers (Type 1 permanent shards).
// These are now implemented in internal/shards/system/ with full functionality.
const (
	SystemShardPerception = "perception_firewall"
	SystemShardWorldModel = "world_model_ingestor"
	SystemShardExecutive  = "executive_policy"
	SystemShardSafety     = "constitution_gate"
	SystemShardRouter     = "tactile_router"
	SystemShardPlanner    = "session_planner"
)

// NewShardManagerWithConfig creates a shard manager with custom concurrency.
func NewShardManagerWithConfig(maxConcurrent int) *ShardManager {
	sm := NewShardManager()
	sm.maxConcurrent = maxConcurrent
	sm.semaphore = make(chan struct{}, maxConcurrent)
	return sm
}

// registerDefaultFactories registers the built-in shard factories.
func (sm *ShardManager) registerDefaultFactories() {
	// Type 1: System Level (Permanent)
	sm.shardFactories[ShardTypeSystem] = func(id string, config ShardConfig) ShardAgent {
		// Pass default empty prompt, NewSystemShard handles defaults
		return NewSystemShard(id, config, "")
	}

	// Type 2: Ephemeral (LLM Created, Non-Persistent)
	sm.shardFactories[ShardTypeEphemeral] = func(id string, config ShardConfig) ShardAgent {
		return NewBaseShardAgent(id, config)
	}

	// Type 3: Persistent (LLM Created)
	sm.shardFactories[ShardTypePersistent] = func(id string, config ShardConfig) ShardAgent {
		return NewBaseShardAgent(id, config)
	}

	// Type 4: User Configured (Persistent Expert)
	sm.shardFactories[ShardTypeUser] = func(id string, config ShardConfig) ShardAgent {
		// Default to base agent if no specific factory is registered for "user"
		// In practice, the profile name (e.g., "RustExpert") acts as the type key
		return NewBaseShardAgent(id, config)
	}

	// ========================
	// Backwards-compatible aliases for legacy shard type strings
	// ========================
	sm.shardFactories["generalist"] = sm.shardFactories[ShardTypeEphemeral]
}

// SetParentKernel sets the parent kernel for fact propagation.
func (sm *ShardManager) SetParentKernel(kernel Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.parentKernel = kernel
}

// SetLLMClient sets the shared LLM client provided to spawned shards.
func (sm *ShardManager) SetLLMClient(client LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
}

// SetLearningStore sets the learning store for Autopoiesis (§8.3).
func (sm *ShardManager) SetLearningStore(store LearningStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.learningStore = store
}

// SetCampaignID sets the current campaign ID for learning attribution.
func (sm *ShardManager) SetCampaignID(campaignID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentCampaignID = campaignID
}

// RegisterShard registers a custom shard type with a factory function.
func (sm *ShardManager) RegisterShard(shardType ShardType, factory func(string, ShardConfig) ShardAgent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.shardFactories[shardType] = factory
}

// DefineProfile defines a specialist shard profile (§9.1).
func (sm *ShardManager) DefineProfile(name string, config ShardConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.profiles[name] = config
}

// GetProfile retrieves a shard profile.
func (sm *ShardManager) GetProfile(name string) (ShardConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	config, ok := sm.profiles[name]
	return config, ok
}

// generateID generates a unique shard ID.
func (sm *ShardManager) generateID(shardType string) string {
	id := atomic.AddUint64(&sm.counter, 1)
	return fmt.Sprintf("shard-%s-%d", shardType, id)
}

// defaultSystemShardConfigs returns the map of system shard names for testing.
func defaultSystemShardConfigs() map[string]struct{} {
	return map[string]struct{}{
		SystemShardPerception: {},
		SystemShardWorldModel: {},
		SystemShardExecutive:  {},
		SystemShardSafety:     {},
		SystemShardRouter:     {},
		SystemShardPlanner:    {},
	}
}

// Spawn spawns a new shard agent and executes the task.
// This implements the Hypervisor pattern from §7.0.
func (sm *ShardManager) Spawn(ctx context.Context, shardType string, task string) (string, error) {
	sm.mu.RLock()
	factory, ok := sm.shardFactories[ShardType(shardType)]
	profile, hasProfile := sm.profiles[shardType]
	sm.mu.RUnlock()

	if !ok && !hasProfile {
		return "", fmt.Errorf("shard type not found: %s", shardType)
	}

	// Acquire semaphore for concurrency control
	select {
	case sm.semaphore <- struct{}{}:
		defer func() { <-sm.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Create shard configuration
	config := profile
	if !hasProfile {
		config = DefaultGeneralistConfig(shardType)
	}

	// Generate unique ID
	id := sm.generateID(shardType)

	// Create shard agent
	var shard ShardAgent
	if ok {
		shard = factory(id, config)
	} else {
		shard = NewBaseShardAgent(id, config)
	}
	sm.injectLLMClient(shard)

	// Hydrate with prior learnings (Autopoiesis §8.3)
	// Extract base shard type (e.g., "coder" from "shard-coder-123")
	baseShardType := shardType
	if idx := strings.LastIndex(shardType, "-"); idx > 0 {
		baseShardType = shardType[:idx]
	}
	sm.hydrateWithLearnings(shard, baseShardType)

	// Register as active
	sm.mu.Lock()
	sm.activeShards[id] = shard
	sm.mu.Unlock()

	// Execute in goroutine with timeout
	resultCh := make(chan *ShardResult, 1)
	go func() {
		startTime := time.Now()
		output, err := shard.Execute(ctx, task)
		duration := time.Since(startTime)

		// Try to get facts from shard for learning propagation
		var facts []Fact
		if factProvider, ok := shard.(interface{ GetKernel() *RealKernel }); ok {
			if kernel := factProvider.GetKernel(); kernel != nil {
				facts = kernel.GetAllFacts()
			}
		}

		result := &ShardResult{
			ShardID:   id,
			Task:      task,
			Output:    output,
			Error:     err,
			Duration:  duration,
			Facts:     facts,
			Timestamp: time.Now(),
		}

		// Process learnings from result (Autopoiesis §8.3)
		sm.processLearnings(baseShardType, result)

		// Store result
		sm.mu.Lock()
		sm.results[id] = result
		delete(sm.activeShards, id)
		sm.mu.Unlock()

		resultCh <- result
	}()

	// Wait for result with timeout
	select {
	case result := <-resultCh:
		if result.Error != nil {
			return "", result.Error
		}
		return result.Output, nil

	case <-ctx.Done():
		// Attempt to stop the shard
		_ = shard.Stop()
		return "", ctx.Err()

	case <-time.After(config.Timeout):
		_ = shard.Stop()
		return "", fmt.Errorf("shard timeout after %v", config.Timeout)
	}
}

// SpawnAsync spawns a shard asynchronously and returns immediately.
func (sm *ShardManager) SpawnAsync(ctx context.Context, shardType string, task string) (string, error) {
	sm.mu.RLock()
	factory, ok := sm.shardFactories[ShardType(shardType)]
	profile, hasProfile := sm.profiles[shardType]
	sm.mu.RUnlock()

	if !ok && !hasProfile {
		return "", fmt.Errorf("shard type not found: %s", shardType)
	}

	config := profile
	if !hasProfile {
		config = DefaultGeneralistConfig(shardType)
	}

	id := sm.generateID(shardType)

	var shard ShardAgent
	if ok {
		shard = factory(id, config)
	} else {
		shard = NewBaseShardAgent(id, config)
	}
	sm.injectLLMClient(shard)

	sm.mu.Lock()
	sm.activeShards[id] = shard
	sm.mu.Unlock()

	// Execute in background
	go func() {
		// Acquire semaphore
		sm.semaphore <- struct{}{}
		defer func() { <-sm.semaphore }()

		startTime := time.Now()
		output, err := shard.Execute(ctx, task)
		duration := time.Since(startTime)

		result := &ShardResult{
			ShardID:   id,
			Task:      task,
			Output:    output,
			Error:     err,
			Duration:  duration,
			Timestamp: time.Now(),
		}

		sm.mu.Lock()
		sm.results[id] = result
		delete(sm.activeShards, id)
		sm.mu.Unlock()
	}()

	return id, nil
}

// GetResult retrieves the result of a completed shard.
func (sm *ShardManager) GetResult(shardID string) (*ShardResult, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result, ok := sm.results[shardID]
	return result, ok
}

// injectLLMClient wires the shared LLM client into shards that support it.
func (sm *ShardManager) injectLLMClient(shard ShardAgent) {
	sm.mu.RLock()
	client := sm.llmClient
	sm.mu.RUnlock()
	if client == nil {
		return
	}
	if llmAware, ok := shard.(interface{ SetLLMClient(LLMClient) }); ok {
		llmAware.SetLLMClient(client)
	}
}

// processLearnings extracts and persists promote_to_long_term facts from a shard result.
// This implements the Autopoiesis feedback loop (§8.3).
func (sm *ShardManager) processLearnings(shardType string, result *ShardResult) {
	sm.mu.RLock()
	store := sm.learningStore
	campaignID := sm.currentCampaignID
	sm.mu.RUnlock()

	if store == nil || result == nil {
		return
	}

	for _, fact := range result.Facts {
		if fact.Predicate == "promote_to_long_term" && len(fact.Args) >= 2 {
			// Args format: [predicate_name, ...args]
			factPredicate, ok := fact.Args[0].(string)
			if !ok {
				continue
			}

			// Extract remaining args
			factArgs := fact.Args[1:]

			// Validate against parent kernel (Constitution) if available
			if sm.parentKernel != nil {
				// Check if this learning would violate safety rules
				// Query: contradict_safety(FactPredicate, FactArgs)?
				// For now, allow all learnings (Constitution validation can be added)
			}

			// Persist the learning
			if err := store.Save(shardType, factPredicate, factArgs, campaignID); err != nil {
				// Log error but don't fail the shard execution
				fmt.Printf("[ShardManager] Failed to persist learning: %v\n", err)
			}
		}
	}
}

// hydrateWithLearnings loads prior learnings into a shard's kernel.
// This allows shards to benefit from patterns learned in previous sessions.
func (sm *ShardManager) hydrateWithLearnings(shard ShardAgent, shardType string) {
	sm.mu.RLock()
	store := sm.learningStore
	sm.mu.RUnlock()

	if store == nil {
		return
	}

	// Get the shard's kernel if it has one
	kernelGetter, ok := shard.(interface{ GetKernel() *RealKernel })
	if !ok {
		return
	}
	kernel := kernelGetter.GetKernel()
	if kernel == nil {
		return
	}

	// Load learnings for this shard type
	learnings, err := store.Load(shardType)
	if err != nil {
		fmt.Printf("[ShardManager] Failed to load learnings: %v\n", err)
		return
	}

	// Assert each learning as a fact in the kernel
	for _, learning := range learnings {
		// Reconstruct the fact from the learning
		args := make([]interface{}, 0, len(learning.FactArgs)+1)
		args = append(args, learning.FactPredicate)
		args = append(args, learning.FactArgs...)

		_ = kernel.Assert(Fact{
			Predicate: "learned_" + learning.FactPredicate,
			Args:      learning.FactArgs,
		})

		// Also assert as a weighted preference fact
		if learning.Confidence >= 0.7 {
			_ = kernel.Assert(Fact{
				Predicate: "strong_preference",
				Args:      []interface{}{learning.FactPredicate, learning.FactArgs},
			})
		}
	}
}

// GetActiveShards returns all currently active shards.
func (sm *ShardManager) GetActiveShards() []ShardAgent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shards := make([]ShardAgent, 0, len(sm.activeShards))
	for _, shard := range sm.activeShards {
		shards = append(shards, shard)
	}
	return shards
}

// StopAll stops all active shards.
func (sm *ShardManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, cancel := range sm.systemShardCancels {
		cancel()
	}
	sm.systemShardCancels = make(map[string]context.CancelFunc)

	for _, shard := range sm.activeShards {
		_ = shard.Stop()
	}
}

// StartSystemShards starts all registered system shards.
func (sm *ShardManager) StartSystemShards(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.systemShardCancels == nil {
		sm.systemShardCancels = make(map[string]context.CancelFunc)
	}

	for name, config := range sm.profiles {
		if config.Type == ShardTypeSystem {
			// Check if already running
			if _, exists := sm.systemShardCancels[name]; exists {
				continue
			}

			// Create a cancellable context for this shard
			shardCtx, cancel := context.WithCancel(ctx)
			sm.systemShardCancels[name] = cancel

			// Spawn manually to keep track of it, but bypass standard activeShards limit if needed?
			// Use SpawnAsync logic but tailored for system shards
			factory, ok := sm.shardFactories[ShardType(name)]
			// Fallback to generic system factory if specific one not found
			if !ok {
				factory = sm.shardFactories[ShardTypeSystem]
			}

			// If still no factory, skip
			if factory == nil {
				cancel()
				delete(sm.systemShardCancels, name)
				continue
			}

			id := sm.generateID(name)
			shard := factory(id, config)
			sm.activeShards[id] = shard // Add to active shards map?
			// System shards might need special handling in activeShards if they are permanent.
			// The current Spawn implementation removes from activeShards on completion.
			// System shards don't complete until simplified.

			// We need to inject LLM client too (can't call injectLLMClient since we are holding lock)
			// Copy-paste inject logic or unlock briefly (risky).
			// Better: extract inject logic to method that takes shard and client, doesn't lock.
			if client := sm.llmClient; client != nil {
				if llmAware, ok := shard.(interface{ SetLLMClient(LLMClient) }); ok {
					llmAware.SetLLMClient(client)
				}
			}

			go func(s ShardAgent, c context.Context, sid string) {
				_, _ = s.Execute(c, "Maintain system homeostasis")

				sm.mu.Lock()
				delete(sm.activeShards, sid)
				sm.mu.Unlock()
			}(shard, shardCtx, id)
		}
	}
	return nil
}

// DisableSystemShard stops a specific system shard.
func (sm *ShardManager) DisableSystemShard(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cancel, ok := sm.systemShardCancels[name]; ok {
		cancel()
		delete(sm.systemShardCancels, name)
	}
}

// PropagateFactsToParent propagates facts from a shard result to the parent kernel.
func (sm *ShardManager) PropagateFactsToParent(result *ShardResult) error {
	sm.mu.RLock()
	kernel := sm.parentKernel
	sm.mu.RUnlock()

	if kernel == nil {
		return nil
	}

	for _, fact := range result.Facts {
		if err := kernel.Assert(fact); err != nil {
			return err
		}
	}

	// Also record the delegation result
	return kernel.Assert(Fact{
		Predicate: "delegation_result",
		Args:      []interface{}{result.ShardID, result.Output, result.Error == nil},
	})
}

// ========== Shard Facts ==========

// ToFacts converts the shard manager state to Mangle facts.
func (sm *ShardManager) ToFacts() []Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	facts := make([]Fact, 0)

	// Active shard facts
	for id, shard := range sm.activeShards {
		config := shard.GetConfig()
		facts = append(facts, Fact{
			Predicate: "active_shard",
			Args:      []interface{}{id, "/" + string(config.Type), "/" + string(shard.GetState())},
		})
	}

	// Profile facts
	for name, config := range sm.profiles {
		facts = append(facts, Fact{
			Predicate: "shard_profile",
			Args:      []interface{}{name, "/" + string(config.Type), config.KnowledgePath},
		})
	}

	return facts
}
