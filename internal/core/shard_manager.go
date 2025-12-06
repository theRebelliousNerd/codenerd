package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// TYPES AND CONSTANTS
// =============================================================================

// ShardType defines the lifecycle model of a shard.
type ShardType string

const (
	ShardTypeEphemeral  ShardType = "ephemeral"  // Type A: Created for a task, dies after
	ShardTypePersistent ShardType = "persistent" // Type B: Persistent, user-defined specialist
	ShardTypeUser       ShardType = "user"       // Alias for Persistent
	ShardTypeSystem     ShardType = "system"     // Type S: Long-running system service
)

// ShardState defines the execution state of a shard.
type ShardState string

const (
	ShardStateIdle      ShardState = "idle"
	ShardStateRunning   ShardState = "running"
	ShardStateCompleted ShardState = "completed"
	ShardStateFailed    ShardState = "failed"
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
	PermissionAskUser   ShardPermission = "ask_user"
	PermissionResearch  ShardPermission = "research"
)

// ModelCapability defines the class of LLM reasoning required.
type ModelCapability string

const (
	CapabilityHighReasoning ModelCapability = "high_reasoning" // e.g. Claude 3.5 Sonnet, GPT-4o
	CapabilityBalanced      ModelCapability = "balanced"       // e.g. Gemini 2.5 Pro
	CapabilityHighSpeed     ModelCapability = "high_speed"     // e.g. Gemini 2.5 Flash, Haiku
)

// ModelConfig defines the LLM requirements for a shard.
type ModelConfig struct {
	Capability ModelCapability
}

// ShardConfig holds configuration for a shard.
type ShardConfig struct {
	Name        string
	Type        ShardType
	Permissions []ShardPermission // Allowed capabilities
	Timeout     time.Duration     // Default execution timeout
	MemoryLimit int               // Abstract memory unit limit
	Model       ModelConfig       // LLM requirements
	KnowledgePath string          // Path to local knowledge DB (Type B only)
}

// DefaultGeneralistConfig returns config for a Type A generalist.
func DefaultGeneralistConfig(name string) ShardConfig {
	return ShardConfig{
		Name:    name,
		Type:    ShardTypeEphemeral,
		Timeout: 5 * time.Minute,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionNetwork,
		},
		Model: ModelConfig{
			Capability: CapabilityBalanced,
		},
	}
}

// DefaultSpecialistConfig returns config for a Type B specialist.
func DefaultSpecialistConfig(name, knowledgePath string) ShardConfig {
	return ShardConfig{
		Name:          name,
		Type:          ShardTypePersistent,
		KnowledgePath: knowledgePath,
		Timeout:       30 * time.Minute,
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionNetwork,
			PermissionBrowser,
			PermissionResearch,
		},
		Model: ModelConfig{
			Capability: CapabilityHighReasoning,
		},
	}
}

// DefaultSystemConfig returns config for a Type S system shard.
func DefaultSystemConfig(name string) ShardConfig {
	return ShardConfig{
		Name:    name,
		Type:    ShardTypeSystem,
		Timeout: 24 * time.Hour, // Long running
		Permissions: []ShardPermission{
			PermissionReadFile,
			PermissionWriteFile,
			PermissionExecCmd,
			PermissionNetwork,
		},
		Model: ModelConfig{
			Capability: CapabilityBalanced,
		},
	}
}

// ShardResult represents the outcome of a shard execution.
type ShardResult struct {
	ShardID   string
	Result    string
	Error     error
	Timestamp time.Time
}

// ShardAgent defines the interface for all agents.
// Renamed from 'Shard' to match usage in registration.go.
type ShardAgent interface {
	Execute(ctx context.Context, task string) (string, error)
	GetID() string
	GetState() ShardState
	GetConfig() ShardConfig
	Stop() error
	
	// Dependency Injection methods
	SetParentKernel(k Kernel)
	SetLLMClient(client LLMClient)
}

// ShardFactory is a function that creates a new shard instance.
type ShardFactory func(id string, config ShardConfig) ShardAgent

// =============================================================================
// BASE IMPLEMENTATION
// =============================================================================

// BaseShardAgent provides common functionality for shards.
type BaseShardAgent struct {
	id     string
	config ShardConfig
	state  ShardState
	mu     sync.RWMutex
	
	// Dependencies
	kernel    Kernel
	llmClient LLMClient
	stopCh    chan struct{}
}

func NewBaseShardAgent(id string, config ShardConfig) *BaseShardAgent {
	return &BaseShardAgent{
		id:     id,
		config: config,
		state:  ShardStateIdle,
		stopCh: make(chan struct{}),
	}
}

func (b *BaseShardAgent) GetID() string {
	return b.id
}

func (b *BaseShardAgent) GetState() ShardState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *BaseShardAgent) SetState(state ShardState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = state
}

func (b *BaseShardAgent) GetConfig() ShardConfig {
	return b.config
}

func (b *BaseShardAgent) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.stopCh:
		// already closed
	default:
		close(b.stopCh)
	}
	b.state = ShardStateCompleted
	return nil
}

func (b *BaseShardAgent) SetParentKernel(k Kernel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.kernel = k
}

func (b *BaseShardAgent) SetLLMClient(client LLMClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.llmClient = client
}

func (b *BaseShardAgent) HasPermission(p ShardPermission) bool {
	for _, perm := range b.config.Permissions {
		if perm == p {
			return true
		}
	}
	return false
}

// Execute is a placeholder; specific shards must embed BaseShardAgent and implement this.
func (b *BaseShardAgent) Execute(ctx context.Context, task string) (string, error) {
	return "BaseShardAgent execution", nil
}

// Helper for subclasses to access LLM
func (b *BaseShardAgent) llm() LLMClient {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.llmClient
}

// =============================================================================
// SHARD MANAGER
// =============================================================================

// ShardManager orchestrates all shard agents.
type ShardManager struct {
	shards    map[string]ShardAgent
	results   map[string]ShardResult
	profiles  map[string]ShardConfig
	factories map[string]ShardFactory
	disabled  map[string]struct{}
	mu        sync.RWMutex
	
	// Core dependencies to inject into shards
	kernel    Kernel
	llmClient LLMClient
	learningStore LearningStore
}

func NewShardManager() *ShardManager {
	return &ShardManager{
		shards:    make(map[string]ShardAgent),
		results:   make(map[string]ShardResult),
		profiles:  make(map[string]ShardConfig),
		factories: make(map[string]ShardFactory),
	}
}

func (sm *ShardManager) SetParentKernel(k Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kernel = k
}

func (sm *ShardManager) SetLLMClient(client LLMClient) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.llmClient = client
}

func (sm *ShardManager) SetLearningStore(store LearningStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.learningStore = store
}

// RegisterShard registers a factory for a given shard type.
func (sm *ShardManager) RegisterShard(typeName string, factory ShardFactory) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.factories[typeName] = factory
}

// DefineProfile registers a shard configuration profile.
func (sm *ShardManager) DefineProfile(name string, config ShardConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.profiles[name] = config
}

// GetProfile retrieves a profile by name.
func (sm *ShardManager) GetProfile(name string) (ShardConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	cfg, ok := sm.profiles[name]
	return cfg, ok
}

// Spawn creates and executes a shard synchronously.
func (sm *ShardManager) Spawn(ctx context.Context, typeName, task string) (string, error) {
	id, err := sm.SpawnAsync(ctx, typeName, task)
	if err != nil {
		return "", err
	}

	// Wait for result
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			res, ok := sm.GetResult(id)
			if ok {
				if res.Error != nil {
					return "", res.Error
				}
				return res.Result, nil
			}
		}
	}
}

// SpawnAsync creates and executes a shard asynchronously.
func (sm *ShardManager) SpawnAsync(ctx context.Context, typeName, task string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. Resolve Config
	var config ShardConfig
	profile, hasProfile := sm.profiles[typeName]
	if hasProfile {
		config = profile
	} else {
		// Default to ephemeral generalist if unknown profile
		config = DefaultGeneralistConfig(typeName)
		if typeName == "ephemeral" {
			config.Type = ShardTypeEphemeral
		}
	}

	// 2. Resolve Factory
	// First check if there is a factory matching the typeName (e.g. "coder", "researcher")
	factory, hasFactory := sm.factories[typeName]
	if !hasFactory && hasProfile {
		// If using a profile (e.g. "RustExpert"), it might map to a base type factory (e.g. "researcher")
		// For Type B specialists, we usually default to "researcher" or "coder" based on profile config?
		// Currently the system assumes profile name == factory name for built-ins.
		// For dynamic agents like "RustExpert", we need to know the base class.
		// TODO: Add 'BaseType' to ShardConfig. For now assume "researcher" if TypePersistent and unknown factory.
		if config.Type == ShardTypePersistent {
			factory = sm.factories["researcher"]
		}
	}

	if factory == nil {
		// Fallback logic based on type
		if config.Type == ShardTypePersistent {
			// Assuming 'researcher' is the base implementation for specialists
			factory = sm.factories["researcher"]
		}

		// Final fallback
		if factory == nil {
			// Fallback to basic agent
			factory = func(id string, config ShardConfig) ShardAgent {
				return NewBaseShardAgent(id, config)
			}
		}
	}

	// 3. Create Shard Instance
	id := fmt.Sprintf("%s-%d", config.Name, time.Now().UnixNano())
	agent := factory(id, config)

	// Inject dependencies
	if sm.kernel != nil {
		agent.SetParentKernel(sm.kernel)
	}
	if sm.llmClient != nil {
		agent.SetLLMClient(sm.llmClient)
	}

	sm.shards[id] = agent

	// 4. Execute Async
	go func() {
		res, err := agent.Execute(ctx, task)
		sm.recordResult(id, res, err)
	}()

	return id, nil
}

func (sm *ShardManager) recordResult(id string, result string, err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Clean up shard
	delete(sm.shards, id)

	sm.results[id] = ShardResult{
		ShardID:   id,
		Result:    result,
		Error:     err,
		Timestamp: time.Now(),
	}
}

func (sm *ShardManager) GetResult(id string) (ShardResult, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	res, ok := sm.results[id]
	return res, ok
}

func (sm *ShardManager) GetActiveShards() []ShardAgent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	active := make([]ShardAgent, 0, len(sm.shards))
	for _, s := range sm.shards {
		active = append(active, s)
	}
	return active
}

func (sm *ShardManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, s := range sm.shards {
		s.Stop()
	}
	// Clear shards
	sm.shards = make(map[string]ShardAgent)
}

func (sm *ShardManager) ToFacts() []Fact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	facts := make([]Fact, 0)
	
	for name, cfg := range sm.profiles {
		facts = append(facts, Fact{
			Predicate: "shard_profile",
			Args: []interface{}{name, string(cfg.Type)},
		})
	}
	
	return facts
}

// StartSystemShards starts all registered system shards (Type S).
func (sm *ShardManager) StartSystemShards(ctx context.Context) error {
	// Collect system shards to start
	toStart := make([]string, 0)
	
	sm.mu.RLock()
	for name, config := range sm.profiles {
		if config.Type == ShardTypeSystem {
			// Skip if disabled
			if _, disabled := sm.disabled[name]; disabled {
				continue
			}
			toStart = append(toStart, name)
		}
	}
	sm.mu.RUnlock()

	for _, name := range toStart {
		// SpawnAsync handles locking internally
		// We use the profile name as the type name
		_, err := sm.SpawnAsync(ctx, name, "system_start")
		if err != nil {
			fmt.Printf("Failed to start system shard %s: %v\n", name, err)
		}
	}
	
	return nil
}

// DisableSystemShard prevents a system shard from auto-starting.
func (sm *ShardManager) DisableSystemShard(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.disabled == nil {
		sm.disabled = make(map[string]struct{})
	}
	sm.disabled[name] = struct{}{}
}