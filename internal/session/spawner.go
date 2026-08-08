// Package session implements the clean execution loop for codeNERD.
package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	appconfig "codenerd/internal/config"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	"codenerd/internal/types"

	"gopkg.in/yaml.v3"
)

var spawnerCounter uint64

// Spawner manages JIT-driven subagent creation and lifecycle.
// It replaces the old ShardFactory pattern with dynamic, config-driven spawning.
type Spawner struct {
	mu sync.RWMutex

	// Core dependencies (shared with all spawned subagents)
	kernel        types.Kernel
	virtualStore  types.VirtualStore
	llmClient     types.LLMClient
	jitCompiler   JITCompiler
	configFactory ConfigFactory
	transducer    perception.Transducer

	// plannerClient is handed to every spawned subagent so a subagent working
	// a reasoning-intensive verb resolves the same tier the session would.
	// Nil keeps subagents entirely on llmClient.
	plannerClient types.LLMClient

	// projectDoc is the workspace's parsed nerd.md. Nil when absent or invalid,
	// which preserves the pre-fix behaviour: subagents run without project
	// instructions. When set, every spawned subagent receives it via
	// Executor.SetProjectDoc so withProjectInstructions can append the rendered
	// prose to the compiled system prompt.
	projectDoc *projectdoc.Document

	// fileContext is the holographic per-file context provider. Nil means
	// subagents run without holographic context, preserving current behaviour.
	// When set, every spawned subagent receives it via
	// Executor.SetFileContextProvider so withFileContext can append the rendered
	// context to the compiled system prompt.
	fileContext FileContextProvider

	// Active subagents
	subagents map[string]*SubAgent

	// Configuration
	maxActiveSubagents int
	tokenBudget        int

	// Pre-reservation tracking for pending spawns
	pendingSpawns int
}

// SpawnerConfig holds configuration for the spawner.
type SpawnerConfig struct {
	MaxActiveSubagents int

	// TokenBudget is the JIT prompt compilation budget for spawned
	// sub-agents. Zero falls back to DefaultTokenBudget. Earlier this
	// was hardcoded to 8192 which silently dropped mandatory atoms from
	// every spawned sub-agent's prompt — they came up amnesiac.
	TokenBudget int
}

// DefaultSpawnerConfig returns sensible defaults.
func DefaultSpawnerConfig() SpawnerConfig {
	return SpawnerConfig{
		MaxActiveSubagents: 10,
		TokenBudget:        DefaultTokenBudget,
	}
}

// NewSpawner creates a new subagent spawner.
func NewSpawner(
	kernel types.Kernel,
	virtualStore types.VirtualStore,
	llmClient types.LLMClient,
	jitCompiler JITCompiler,
	configFactory ConfigFactory,
	transducer perception.Transducer,
	cfg SpawnerConfig,
) *Spawner {
	logging.Session("Creating Spawner (max active: %d)", cfg.MaxActiveSubagents)

	budget := cfg.TokenBudget
	if budget <= 0 {
		budget = DefaultTokenBudget
	}
	return &Spawner{
		kernel:             kernel,
		virtualStore:       virtualStore,
		llmClient:          llmClient,
		jitCompiler:        jitCompiler,
		configFactory:      configFactory,
		transducer:         transducer,
		subagents:          make(map[string]*SubAgent),
		maxActiveSubagents: cfg.MaxActiveSubagents,
		tokenBudget:        budget,
	}
}

// SetPlannerClient installs the high-reasoning client passed to subagents
// spawned from here on. Already-spawned subagents keep the client they were
// built with.
func (s *Spawner) SetPlannerClient(c types.LLMClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plannerClient = c
}

// plannerClientLocked reads the planner slot under the read lock.
func (s *Spawner) currentPlannerClient() types.LLMClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.plannerClient
}

// SetProjectDoc attaches the workspace's parsed nerd.md so every subagent
// spawned from here on inherits the parent Executor's project instructions.
// Mirrors Executor.SetProjectDoc. A nil doc means "no nerd.md" and is a no-op
// for spawned agents, preserving the existing behaviour when the file is absent.
// Already-spawned subagents keep the doc they were built with.
func (s *Spawner) SetProjectDoc(doc *projectdoc.Document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projectDoc = doc
}

// currentProjectDoc reads the project doc slot under the read lock.
func (s *Spawner) currentProjectDoc() *projectdoc.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.projectDoc
}

// SetFileContextProvider attaches the holographic per-file context provider so
// every subagent spawned from here on inherits the parent's context. Mirrors
// SetProjectDoc. A nil provider means subagents run without holographic context,
// preserving current behaviour.
func (s *Spawner) SetFileContextProvider(p FileContextProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fileContext = p
}

// currentFileContext reads the file context slot under the read lock.
func (s *Spawner) currentFileContext() FileContextProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fileContext
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
	// Phase 1: Check capacity & pre-reserve (lock held briefly)
	s.mu.Lock()
	activeCount := s.countActive() + s.pendingSpawns
	if activeCount >= s.maxActiveSubagents {
		s.mu.Unlock()
		return nil, fmt.Errorf("max active subagents reached: %d", s.maxActiveSubagents)
	}
	s.pendingSpawns++
	s.mu.Unlock()

	var success bool
	defer func() {
		if !success {
			s.mu.Lock()
			s.pendingSpawns--
			s.mu.Unlock()
		}
	}()

	logging.Session("Spawning subagent: %s (type: %s, intent: %s)", req.Name, req.Type, req.IntentVerb)

	// Phase 2: Generate JIT config (no lock - may involve IO/LLM calls)
	EffectiveAgentRuntimeConfig, err := s.generateConfig(ctx, req)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to generate config for %s: %v", req.Name, err)
		// Continue with empty config - subagent can still function
		EffectiveAgentRuntimeConfig = &config.EffectiveAgentRuntimeConfig{}
	}

	// Phase 3: Build subagent configuration
	subCfg := SubAgentConfig{
		ID:                          fmt.Sprintf("%s-%d-%d", req.Name, time.Now().UnixNano(), atomic.AddUint64(&spawnerCounter, 1)),
		Name:                        req.Name,
		Type:                        req.Type,
		EffectiveAgentRuntimeConfig: EffectiveAgentRuntimeConfig,
		IntentVerb:                  req.IntentVerb,
		Timeout:                     req.Timeout,
		MaxTurns:                    100,
		SessionContext:              req.SessionContext,
	}

	if subCfg.Timeout == 0 {
		subCfg.Timeout = appconfig.GetLLMTimeouts().ShardExecutionTimeout
	}

	// Phase 4: Create subagent
	agent := NewSubAgent(
		subCfg,
		s.kernel,
		s.virtualStore,
		s.llmClient,
		s.jitCompiler,
		s.configFactory,
		s.transducer,
	)
	agent.SetPlannerClient(s.currentPlannerClient())
	// Forward the parent's nerd.md project instructions. Executor.SetProjectDoc
	// is nil-safe and withProjectInstructions is a no-op when doc is nil, so
	// this preserves the existing behaviour when no nerd.md is present.
	if doc := s.currentProjectDoc(); doc != nil {
		agent.executor.SetProjectDoc(doc)
	}
	if fc := s.currentFileContext(); fc != nil {
		agent.executor.SetFileContextProvider(fc)
	}

	// Phase 5: Register subagent (lock held briefly)
	s.mu.Lock()
	s.pendingSpawns--
	// Re-check capacity after config generation (double-check active count only)
	activeCount = s.countActive()
	if activeCount >= s.maxActiveSubagents {
		s.mu.Unlock()
		return nil, fmt.Errorf("max active subagents reached during spawn: %d", s.maxActiveSubagents)
	}
	s.subagents[agent.GetID()] = agent
	success = true
	s.mu.Unlock()

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
	EffectiveAgentRuntimeConfig, err := s.loadSpecialistConfig(ctx, name)
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
		ID:                          fmt.Sprintf("%s-%d-%d", name, time.Now().UnixNano(), atomic.AddUint64(&spawnerCounter, 1)),
		Name:                        name,
		Type:                        SubAgentTypePersistent, // Specialists are persistent
		EffectiveAgentRuntimeConfig: EffectiveAgentRuntimeConfig,
		IntentVerb:                  "/consult/" + name,
		Timeout:                     appconfig.GetLLMTimeouts().ShardExecutionTimeout,
		MaxTurns:                    100,
	}

	// Create and start. s.mu is already held for writing here, so read the
	// planner slot directly rather than through currentPlannerClient().
	agent := NewSubAgent(
		subCfg,
		s.kernel,
		s.virtualStore,
		s.llmClient,
		s.jitCompiler,
		s.configFactory,
		s.transducer,
	)
	agent.SetPlannerClient(s.plannerClient)
	// Forward nerd.md to specialists as well. s.mu is already held for writing,
	// so read the slot directly rather than via currentProjectDoc() to avoid
	// a redundant RLock (and to mirror the plannerClient pattern above).
	if s.projectDoc != nil {
		agent.executor.SetProjectDoc(s.projectDoc)
	}
	if s.fileContext != nil {
		agent.executor.SetFileContextProvider(s.fileContext)
	}

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
		if agent.GetName() != name {
			continue
		}
		state := agent.GetState()
		if state != SubAgentStateCompleted && state != SubAgentStateFailed {
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
		state := agent.GetState()
		if state != SubAgentStateCompleted && state != SubAgentStateFailed {
			count++
		}
	}
	return count
}

// generateConfig creates a JIT config for the subagent.
func (s *Spawner) generateConfig(ctx context.Context, req SpawnRequest) (*config.EffectiveAgentRuntimeConfig, error) {
	if s.configFactory == nil {
		return &config.EffectiveAgentRuntimeConfig{}, nil
	}

	intentVerb := req.IntentVerb
	if intentVerb == "" {
		intentVerb = "/general"
	}

	// First compile a minimal prompt to get compilation result.
	// TokenBudget comes from spawner config (set by the chat session
	// from UserConfig.ContextWindow.MaxTokens). Hardcoded 8192 was the
	// pre-fix bottleneck that silently stripped mandatory atoms.
	budget := s.tokenBudget
	if budget <= 0 {
		budget = DefaultTokenBudget
	}
	compilationCtx := &prompt.CompilationContext{
		IntentVerb:      intentVerb,
		OperationalMode: "/active",
		TokenBudget:     budget,
	}

	// If dream mode, pass it to compilation context to potentially select different persona/skills
	if req.SessionContext != nil && req.SessionContext.DreamMode {
		compilationCtx.OperationalMode = "/dream"
	}

	var compileResult *prompt.CompilationResult
	if s.jitCompiler == nil {
		logging.Get(logging.CategorySession).Warn("JIT compiler unavailable, using empty subagent config")
		return &config.EffectiveAgentRuntimeConfig{}, nil
	}

	compileResult, err := s.jitCompiler.Compile(ctx, compilationCtx)
	if err != nil {
		// Fallback strategy: Retry once with baseline context, then return empty config
		logging.Get(logging.CategorySession).Warn("JIT compilation failed, retrying with baseline: %v", err)

		// Retry with minimal context (baseline fallback)
		baselineCtx := &prompt.CompilationContext{
			IntentVerb:      "/general",
			OperationalMode: "/active",
			TokenBudget:     4096, // Reduced budget for fallback
		}
		compileResult, err = s.jitCompiler.Compile(ctx, baselineCtx)
		if err != nil {
			// Final fallback: return empty config, subagent will use defaults
			logging.Get(logging.CategorySession).Warn("JIT baseline compilation also failed, using empty config: %v", err)
			return &config.EffectiveAgentRuntimeConfig{}, nil
		}
	}

	return s.configFactory.Generate(ctx, compileResult, intentVerb)
}

// maxSpecialistConfigSize is the maximum allowed size for a specialist config YAML file.
// Prevents DoS via oversized configs that cause the YAML parser to consume excessive CPU/memory.
const maxSpecialistConfigSize = 1 << 20 // 1MB

// loadSpecialistConfig loads a specialist's config from the filesystem.
func (s *Spawner) loadSpecialistConfig(ctx context.Context, name string) (*config.EffectiveAgentRuntimeConfig, error) {
	// Guard against path traversal: reject names containing ".." or path separators.
	// Without this, a name like "../../etc/passwd" would escape .nerd/agents/.
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("invalid specialist name %q: contains path traversal characters", name)
	}

	// Try to load from .nerd/agents/{name}/config.yaml
	configPath := filepath.Join(".nerd", "agents", name, "config.yaml")
	logging.SessionDebug("Loading specialist config for: %s from %s", name, configPath)

	// Use virtualStore.ReadRaw() for consistency with architecture if available
	var data []byte
	var err error
	if s.virtualStore != nil {
		data, err = s.virtualStore.ReadRaw(configPath)
	} else {
		// Fallback to os.ReadFile when virtualStore is not set
		data, err = os.ReadFile(configPath)
	}
	if err == nil {
		// Reject oversized configs to prevent YAML parser DoS
		if len(data) > maxSpecialistConfigSize {
			return nil, fmt.Errorf("specialist config for %q exceeds maximum size (%d > %d bytes)",
				name, len(data), maxSpecialistConfigSize)
		}

		var cfg config.EffectiveAgentRuntimeConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse specialist config at %s: %w", configPath, err)
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid specialist config at %s: %w", configPath, err)
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
		return &config.EffectiveAgentRuntimeConfig{}, nil
	}

	// Specialist fallback path: when no on-disk config exists for `name`, we still
	// need a runtime config so the specialist can boot with default tools/policies
	// drawn from its intent atom. ConfigFactory.Generate rejects a nil
	// CompilationResult (it dereferences result.Prompt), so we pass a minimal
	// non-nil shell instead. The identity prompt is intentionally empty here —
	// the specialist will be driven by whatever ConfigAtom the factory resolves
	// for "/<name>".
	return s.configFactory.Generate(ctx, &prompt.CompilationResult{Prompt: ""}, "/"+name)
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
