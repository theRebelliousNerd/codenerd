package core

import (
	"context"
	"fmt"
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

	return sm
}

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
		return NewBaseShardAgent(id, config)
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
		return &CoderShard{BaseShardAgent: NewBaseShardAgent(id, config)}
	}

	// ========================
	// Backwards-compatible aliases for legacy shard type strings
	// ========================
	sm.shardFactories["generalist"] = sm.shardFactories[ShardTypeEphemeral]

	sm.shardFactories["coder"] = func(id string, config ShardConfig) ShardAgent {
		return &CoderShard{BaseShardAgent: NewBaseShardAgent(id, config)}
	}

	sm.shardFactories["researcher"] = func(id string, config ShardConfig) ShardAgent {
		return &ResearcherShard{BaseShardAgent: NewBaseShardAgent(id, config)}
	}

	sm.shardFactories["reviewer"] = func(id string, config ShardConfig) ShardAgent {
		return &ReviewerShard{BaseShardAgent: NewBaseShardAgent(id, config)}
	}

	sm.shardFactories["tester"] = func(id string, config ShardConfig) ShardAgent {
		return &TesterShard{BaseShardAgent: NewBaseShardAgent(id, config)}
	}
}

// SetParentKernel sets the parent kernel for fact propagation.
func (sm *ShardManager) SetParentKernel(kernel Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.parentKernel = kernel
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

		result := &ShardResult{
			ShardID:   id,
			Task:      task,
			Output:    output,
			Error:     err,
			Duration:  duration,
			Timestamp: time.Now(),
		}

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

	for _, shard := range sm.activeShards {
		_ = shard.Stop()
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

// ========== Specialized Shard Implementations ==========

// CoderShard is specialized for code writing and modification.
type CoderShard struct {
	*BaseShardAgent
}

func (s *CoderShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(ShardStateRunning)
	defer s.SetState(ShardStateCompleted)

	// Coder shard logic
	// Would integrate with code graph, AST analysis, etc.
	return fmt.Sprintf("Coder shard executed: %s", task), nil
}

// ResearcherShard is specialized for deep research tasks.
type ResearcherShard struct {
	*BaseShardAgent
}

func (s *ResearcherShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(ShardStateRunning)
	defer s.SetState(ShardStateCompleted)

	// Researcher shard logic
	// Would integrate with scraper service, knowledge base
	return fmt.Sprintf("Researcher shard executed: %s", task), nil
}

// ReviewerShard is specialized for code review tasks.
type ReviewerShard struct {
	*BaseShardAgent
}

func (s *ReviewerShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(ShardStateRunning)
	defer s.SetState(ShardStateCompleted)

	// Reviewer shard logic
	// Would analyze code for best practices, security, etc.
	return fmt.Sprintf("Reviewer shard executed: %s", task), nil
}

// TesterShard is specialized for test generation and execution.
type TesterShard struct {
	*BaseShardAgent
}

func (s *TesterShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(ShardStateRunning)
	defer s.SetState(ShardStateCompleted)

	// Tester shard logic
	// Would integrate with TDD loop
	return fmt.Sprintf("Tester shard executed: %s", task), nil
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
