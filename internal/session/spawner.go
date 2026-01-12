// Package session implements the clean execution loop for codeNERD.
package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"

	"gopkg.in/yaml.v3"
)

// Spawner manages JIT-driven subagent creation and lifecycle.
// It replaces the old ShardFactory pattern with dynamic, config-driven spawning.
type Spawner struct {
	mu sync.RWMutex

	// Core dependencies (shared with all spawned subagents)
	kernel        types.Kernel
	virtualStore  types.VirtualStore
	llmClient     types.LLMClient
	jitCompiler   *prompt.JITPromptCompiler
	configFactory *prompt.ConfigFactory
	transducer    perception.Transducer

	// Active subagents
	subagents map[string]*SubAgent

	// Configuration
	maxActiveSubagents int
}

// SpawnerConfig holds configuration for the spawner.
type SpawnerConfig struct {
	MaxActiveSubagents int
}

// DefaultSpawnerConfig returns sensible defaults.
func DefaultSpawnerConfig() SpawnerConfig {
	return SpawnerConfig{
		MaxActiveSubagents: 10,
	}
}

// NewSpawner creates a new subagent spawner.
func NewSpawner(
	kernel types.Kernel,
	virtualStore types.VirtualStore,
	llmClient types.LLMClient,
	jitCompiler *prompt.JITPromptCompiler,
	configFactory *prompt.ConfigFactory,
	transducer perception.Transducer,
	cfg SpawnerConfig,
) *Spawner {
	logging.Session("Creating Spawner (max active: %d)", cfg.MaxActiveSubagents)

	return &Spawner{
		kernel:             kernel,
		virtualStore:       virtualStore,
		llmClient:          llmClient,
		jitCompiler:        jitCompiler,
		configFactory:      configFactory,
		transducer:         transducer,
		subagents:          make(map[string]*SubAgent),
		maxActiveSubagents: cfg.MaxActiveSubagents,
	}
}

// SpawnRequest describes the parameters for spawning a subagent.
type SpawnRequest struct {
	// Name is the subagent name (e.g., "coder", "my-specialist")
	Name string

	// Task is the initial task for the subagent
	Task string

	// Type determines lifecycle behavior (ephemeral, persistent, system)
	Type SubAgentType

	// IntentVerb is used for JIT config generation
	IntentVerb string

	// Timeout for the subagent's execution
	Timeout time.Duration

	// SessionContext provides shared state (e.g., DreamMode, Blackboard)
	SessionContext *types.SessionContext
}

// Spawn creates and starts a new subagent based on the request.
// The subagent's identity, tools, and policies are all JIT-compiled.
func (s *Spawner) Spawn(ctx context.Context, req SpawnRequest) (*SubAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check active limit
	activeCount := s.countActive()
	if activeCount >= s.maxActiveSubagents {
		return nil, fmt.Errorf("max active subagents reached: %d", s.maxActiveSubagents)
	}

	logging.Session("Spawning subagent: %s (type: %s, intent: %s)", req.Name, req.Type, req.IntentVerb)

	// 1. Generate JIT config for this subagent
	agentConfig, err := s.generateConfig(ctx, req)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to generate config for %s: %v", req.Name, err)
		// Continue with empty config - subagent can still function
		agentConfig = &config.AgentConfig{}
	}

	// 2. Build subagent configuration
	subCfg := SubAgentConfig{
		ID:             fmt.Sprintf("%s-%d", req.Name, time.Now().UnixNano()),
		Name:           req.Name,
		Type:           req.Type,
		AgentConfig:    agentConfig,
		Timeout:        req.Timeout,
		MaxTurns:       100,
		SessionContext: req.SessionContext,
	}

	if subCfg.Timeout == 0 {
		subCfg.Timeout = 30 * time.Minute
	}

	// 3. Create subagent
	agent := NewSubAgent(
		subCfg,
		s.kernel,
		s.virtualStore,
		s.llmClient,
		s.jitCompiler,
		s.configFactory,
		s.transducer,
	)

	// 4. Register subagent
	s.subagents[agent.GetID()] = agent

	// 5. Start execution
	go agent.Run(ctx, req.Task)

	logging.Session("Spawned subagent: %s (id: %s)", req.Name, agent.GetID())

	return agent, nil
}

// SpawnForIntent spawns a subagent based on a parsed intent.
// This is the primary entry point for intent-driven spawning.
func (s *Spawner) SpawnForIntent(ctx context.Context, intent perception.Intent, task string) (*SubAgent, error) {
	// Determine subagent type based on intent
	agentType := s.determineAgentType(intent)

	req := SpawnRequest{
		Name:       s.determineAgentName(intent),
		Task:       task,
		Type:       agentType,
		IntentVerb: intent.Verb,
	}

	return s.Spawn(ctx, req)
}

// SpawnSpecialist spawns a user-defined specialist agent.
// Specialists have their configs loaded from .nerd/agents/{name}/
func (s *Spawner) SpawnSpecialist(ctx context.Context, name string, task string) (*SubAgent, error) {
	// Load specialist config from filesystem
	agentConfig, err := s.loadSpecialistConfig(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to load specialist %s: %w", name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check active limit
	activeCount := s.countActive()
	if activeCount >= s.maxActiveSubagents {
		return nil, fmt.Errorf("max active subagents reached: %d", s.maxActiveSubagents)
	}

	// Build config
	subCfg := SubAgentConfig{
		ID:          fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		Name:        name,
		Type:        SubAgentTypePersistent, // Specialists are persistent
		AgentConfig: agentConfig,
		Timeout:     30 * time.Minute,
		MaxTurns:    100,
	}

	// Create and start
	agent := NewSubAgent(
		subCfg,
		s.kernel,
		s.virtualStore,
		s.llmClient,
		s.jitCompiler,
		s.configFactory,
		s.transducer,
	)

	s.subagents[agent.GetID()] = agent
	go agent.Run(ctx, task)

	logging.Session("Spawned specialist: %s (id: %s)", name, agent.GetID())

	return agent, nil
}

// Get returns a subagent by ID.
func (s *Spawner) Get(id string) (*SubAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.subagents[id]
	return agent, ok
}

// GetByName returns the first active subagent with the given name.
func (s *Spawner) GetByName(name string) (*SubAgent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, agent := range s.subagents {
		if agent.GetName() == name && agent.GetState() == SubAgentStateRunning {
			return agent, true
		}
	}
	return nil, false
}

// Stop stops a subagent by ID.
func (s *Spawner) Stop(id string) error {
	s.mu.Lock()
	agent, ok := s.subagents[id]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("subagent not found: %s", id)
	}

	return agent.Stop()
}

// StopAll stops all active subagents.
func (s *Spawner) StopAll() {
	s.mu.Lock()
	agents := make([]*SubAgent, 0, len(s.subagents))
	for _, agent := range s.subagents {
		agents = append(agents, agent)
	}
	s.mu.Unlock()

	for _, agent := range agents {
		if agent.GetState() == SubAgentStateRunning {
			_ = agent.Stop()
		}
	}
}

// Cleanup removes completed subagents from tracking.
func (s *Spawner) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for id, agent := range s.subagents {
		state := agent.GetState()
		if state == SubAgentStateCompleted || state == SubAgentStateFailed {
			delete(s.subagents, id)
			removed++
		}
	}

	if removed > 0 {
		logging.SessionDebug("Cleaned up %d completed subagents", removed)
	}

	return removed
}

// ListActive returns all currently running subagents.
func (s *Spawner) ListActive() []*SubAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	active := make([]*SubAgent, 0)
	for _, agent := range s.subagents {
		if agent.GetState() == SubAgentStateRunning {
			active = append(active, agent)
		}
	}
	return active
}

// GetMetrics returns metrics for all subagents.
func (s *Spawner) GetMetrics() []SubAgentMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make([]SubAgentMetrics, 0, len(s.subagents))
	for _, agent := range s.subagents {
		metrics = append(metrics, agent.GetMetrics())
	}
	return metrics
}

// countActive returns the number of running subagents (caller must hold lock).
func (s *Spawner) countActive() int {
	count := 0
	for _, agent := range s.subagents {
		if agent.GetState() == SubAgentStateRunning {
			count++
		}
	}
	return count
}

// generateConfig creates a JIT config for the subagent.
func (s *Spawner) generateConfig(ctx context.Context, req SpawnRequest) (*config.AgentConfig, error) {
	if s.configFactory == nil {
		return &config.AgentConfig{}, nil
	}

	intentVerb := req.IntentVerb
	if intentVerb == "" {
		intentVerb = "/general"
	}

	// First compile a minimal prompt to get compilation result
	compilationCtx := &prompt.CompilationContext{
		IntentVerb:      intentVerb,
		OperationalMode: "/active",
		TokenBudget:     8192, // Default token budget for JIT compilation
	}

	// If dream mode, pass it to compilation context to potentially select different persona/skills
	if req.SessionContext != nil && req.SessionContext.DreamMode {
		compilationCtx.OperationalMode = "/dream"
	}

	compileResult, err := s.jitCompiler.Compile(ctx, compilationCtx)
	if err != nil {
		return nil, fmt.Errorf("JIT compilation failed: %w", err)
	}

	return s.configFactory.Generate(ctx, compileResult, intentVerb)
}

// loadSpecialistConfig loads a specialist's config from the filesystem.
func (s *Spawner) loadSpecialistConfig(ctx context.Context, name string) (*config.AgentConfig, error) {
	// Try to load from .nerd/agents/{name}/config.yaml
	configPath := filepath.Join(".nerd", "agents", name, "config.yaml")
	logging.SessionDebug("Loading specialist config for: %s from %s", name, configPath)

	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg config.AgentConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse specialist config at %s: %w", configPath, err)
		}
		logging.SessionDebug("Successfully loaded specialist config for %s", name)
		return &cfg, nil
	} else if !os.IsNotExist(err) {
		// Log read errors other than NotExist
		logging.Session("Error reading specialist config for %s: %v", name, err)
	} else {
		logging.SessionDebug("Specialist config not found for %s, falling back to JIT generation", name)
	}

	if s.configFactory == nil {
		return &config.AgentConfig{}, nil
	}

	// Use specialist name as intent for now
	return s.configFactory.Generate(ctx, nil, "/"+name)
}

// determineAgentType maps intents to subagent types.
func (s *Spawner) determineAgentType(intent perception.Intent) SubAgentType {
	// Most tasks are ephemeral
	switch intent.Category {
	case "/system":
		return SubAgentTypeSystem
	default:
		return SubAgentTypeEphemeral
	}
}

// determineAgentName maps intents to subagent names.
func (s *Spawner) determineAgentName(intent perception.Intent) string {
	// Map common verbs to agent names
	switch intent.Verb {
	case "/fix", "/implement", "/refactor", "/create":
		return "coder"
	case "/test", "/cover", "/verify":
		return "tester"
	case "/review", "/audit", "/check":
		return "reviewer"
	case "/research", "/learn", "/document":
		return "researcher"
	default:
		return "executor"
	}
}
