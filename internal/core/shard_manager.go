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
	kernel        Kernel
	llmClient     LLMClient
	tracingClient TracingClient // Optional: set when llmClient implements TracingClient
	learningStore LearningStore

	// Session context for tracing
	sessionID string
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
	// Check if client supports tracing
	if tc, ok := client.(TracingClient); ok {
		sm.tracingClient = tc
	}
}

func (sm *ShardManager) SetSessionID(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionID = sessionID
}

// categorizeShardType determines the shard category based on type name and config.
func (sm *ShardManager) categorizeShardType(typeName string, shardType ShardType) string {
	// System shards (built-in, always-on)
	systemShards := map[string]bool{
		"perception_firewall": true,
		"constitution_gate":   true,
		"executive_policy":    true,
		"cost_guard":          true,
		"tactile_router":      true,
		"session_planner":     true,
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

// ShardInfo contains information about an available shard for selection.
type ShardInfo struct {
	Name        string    `json:"name"`
	Type        ShardType `json:"type"`
	Description string    `json:"description,omitempty"`
	HasKnowledge bool     `json:"has_knowledge"`
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

	// Determine shard category for tracing
	shardCategory := sm.categorizeShardType(typeName, config.Type)

	// Set tracing context before execution
	if sm.tracingClient != nil {
		sm.tracingClient.SetShardContext(id, typeName, shardCategory, sm.sessionID, task)
	}

	// 4. Execute Async
	go func() {
		res, err := agent.Execute(ctx, task)
		// Clear tracing context after execution
		if sm.tracingClient != nil {
			sm.tracingClient.ClearShardContext()
		}
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

// =============================================================================
// SHARD RESULT TO FACTS CONVERSION (Cross-Turn Context Propagation)
// =============================================================================
// These methods convert shard execution results into Mangle facts that can be
// loaded into the kernel for persistence across conversation turns.

// ResultToFacts converts a shard execution result into kernel-loadable facts.
// This is the key bridge between shard execution and kernel state.
func (sm *ShardManager) ResultToFacts(shardID, shardType, task, result string, err error) []Fact {
	facts := make([]Fact, 0, 5)
	timestamp := time.Now().Unix()

	// Core execution fact
	facts = append(facts, Fact{
		Predicate: "shard_executed",
		Args:      []interface{}{shardID, shardType, task, timestamp},
	})

	// Track last execution for quick reference
	facts = append(facts, Fact{
		Predicate: "last_shard_execution",
		Args:      []interface{}{shardID, shardType, task},
	})

	if err != nil {
		// Record failure
		facts = append(facts, Fact{
			Predicate: "shard_error",
			Args:      []interface{}{shardID, err.Error()},
		})
	} else {
		// Record success
		facts = append(facts, Fact{
			Predicate: "shard_success",
			Args:      []interface{}{shardID},
		})

		// Store output (truncate if too long for kernel)
		output := result
		if len(output) > 4000 {
			output = output[:4000] + "... (truncated)"
		}
		facts = append(facts, Fact{
			Predicate: "shard_output",
			Args:      []interface{}{shardID, output},
		})

		// Create compressed context for LLM injection
		summary := sm.extractSummary(shardType, result)
		facts = append(facts, Fact{
			Predicate: "recent_shard_context",
			Args:      []interface{}{shardType, task, summary, timestamp},
		})

		// Parse shard-specific structured facts
		structuredFacts := sm.parseShardSpecificFacts(shardID, shardType, result)
		facts = append(facts, structuredFacts...)
	}

	return facts
}

// extractSummary creates a compressed summary of shard output for context injection.
func (sm *ShardManager) extractSummary(shardType, result string) string {
	// Extract key information based on shard type
	lines := splitLines(result)

	switch shardType {
	case "reviewer":
		// Look for summary lines in review output
		for _, line := range lines {
			if contains(line, "PASSED") || contains(line, "FAILED") ||
				contains(line, "critical") || contains(line, "error") ||
				contains(line, "warning") || contains(line, "info") {
				return truncateString(line, 200)
			}
		}
	case "tester":
		// Look for test summary
		for _, line := range lines {
			if contains(line, "PASS") || contains(line, "FAIL") ||
				contains(line, "ok") || contains(line, "---") {
				return truncateString(line, 200)
			}
		}
	case "coder":
		// Look for completion indicators
		for _, line := range lines {
			if contains(line, "created") || contains(line, "modified") ||
				contains(line, "wrote") || contains(line, "updated") {
				return truncateString(line, 200)
			}
		}
	}

	// Default: first meaningful line
	for _, line := range lines {
		trimmed := trimSpace(line)
		if len(trimmed) > 10 {
			return truncateString(trimmed, 200)
		}
	}

	return truncateString(result, 200)
}

// parseShardSpecificFacts extracts structured facts from shard-specific output formats.
func (sm *ShardManager) parseShardSpecificFacts(shardID, shardType, result string) []Fact {
	var facts []Fact

	switch shardType {
	case "reviewer":
		facts = append(facts, sm.parseReviewerOutput(shardID, result)...)
	case "tester":
		facts = append(facts, sm.parseTesterOutput(shardID, result)...)
	}

	return facts
}

// parseReviewerOutput extracts structured facts from reviewer shard output.
func (sm *ShardManager) parseReviewerOutput(shardID, result string) []Fact {
	var facts []Fact

	// Parse summary counts (e.g., "0 critical, 0 errors, 0 warnings, 5 info")
	critical, errors, warnings, info := 0, 0, 0, 0

	lines := splitLines(result)
	for _, line := range lines {
		lower := toLower(line)

		// Try to extract counts
		if contains(lower, "critical") {
			critical = extractCount(line, "critical")
		}
		if contains(lower, "error") && !contains(lower, "errors:") {
			errors = extractCount(line, "error")
		}
		if contains(lower, "warning") {
			warnings = extractCount(line, "warning")
		}
		if contains(lower, "info") && !contains(lower, "information") {
			info = extractCount(line, "info")
		}
	}

	// Add summary fact
	facts = append(facts, Fact{
		Predicate: "review_summary",
		Args:      []interface{}{shardID, critical, errors, warnings, info},
	})

	return facts
}

// parseTesterOutput extracts structured facts from tester shard output.
func (sm *ShardManager) parseTesterOutput(shardID, result string) []Fact {
	var facts []Fact

	// Parse test summary (e.g., "10 passed, 2 failed, 1 skipped")
	total, passed, failed, skipped := 0, 0, 0, 0

	lines := splitLines(result)
	for _, line := range lines {
		lower := toLower(line)

		if contains(lower, "pass") {
			passed = extractCount(line, "pass")
		}
		if contains(lower, "fail") {
			failed = extractCount(line, "fail")
		}
		if contains(lower, "skip") {
			skipped = extractCount(line, "skip")
		}
	}

	total = passed + failed + skipped
	if total > 0 {
		facts = append(facts, Fact{
			Predicate: "test_summary",
			Args:      []interface{}{shardID, total, passed, failed, skipped},
		})
	}

	return facts
}

// Helper functions for string manipulation (avoid importing strings in hot path)

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractCount(line, keyword string) int {
	// Simple extraction: find number before keyword
	lower := toLower(line)
	idx := findSubstring(lower, toLower(keyword))
	if idx < 0 {
		return 0
	}

	// Look backward for digits
	numEnd := idx
	for numEnd > 0 && (line[numEnd-1] == ' ' || line[numEnd-1] == ':') {
		numEnd--
	}

	numStart := numEnd
	for numStart > 0 && line[numStart-1] >= '0' && line[numStart-1] <= '9' {
		numStart--
	}

	if numStart < numEnd {
		num := 0
		for i := numStart; i < numEnd; i++ {
			num = num*10 + int(line[i]-'0')
		}
		return num
	}

	return 0
}