package autopoiesis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/tools/research"
	"codenerd/internal/types"
)

// =============================================================================
// AUTOPOIESIS ORCHESTRATOR
// =============================================================================
// The main coordinator for all self-modification capabilities.

// Orchestrator coordinates all autopoiesis capabilities
type Orchestrator struct {
	mu          sync.RWMutex
	config      Config
	complexity  *ComplexityAnalyzer
	toolGen     *ToolGenerator
	persistence *PersistenceAnalyzer
	agentCreate *AgentCreator
	ouroboros   ToolSynthesizer // The Ouroboros Loop for tool self-generation
	client      LLMClient

	// Kernel Integration - Bridge to Mangle Logic Core
	kernel KernelInterface // The Mangle kernel for fact assertion/query

	// Feedback and Learning System
	evaluator *QualityEvaluator // Assess tool output quality
	patterns  *PatternDetector  // Detect recurring issues
	refiner   *ToolRefiner      // Improve suboptimal tools
	learnings *LearningStore    // Persist learnings
	profiles  *ProfileStore     // Tool-specific quality profiles

	// Reasoning Trace and Logging System
	traces      *TraceCollector // Capture reasoning during generation
	logInjector *LogInjector    // Inject mandatory logging into tools

	// JIT Prompt Compilation (Phase 5) - using interfaces to avoid import cycles
	promptAssembler PromptAssembler // JIT-aware prompt assembler
	jitCompiler     JITCompiler     // JIT prompt compiler

	// Gemini advanced features (nil if not Gemini or features unavailable)
	grounding *research.GroundingHelper // Google Search / URL Context grounding
	thinking  *research.ThinkingHelper  // Thinking mode metadata capture

	// Tool generation throttling (session-local)
	toolsGenerated int
	lastToolGen    time.Time
}

// llmClientWrapper adapts the local LLMClient interface to types.LLMClient
// for use with research.GroundingHelper and research.ThinkingHelper.
// It proxies grounding/thinking interfaces to the underlying client when available.
type llmClientWrapper struct {
	client LLMClient
}

func (w *llmClientWrapper) Complete(ctx context.Context, prompt string) (string, error) {
	return w.client.Complete(ctx, prompt)
}

func (w *llmClientWrapper) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return w.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
}

func (w *llmClientWrapper) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Not used by grounding/thinking helpers, but needed for interface compliance
	// The autopoiesis package doesn't use tool calling directly
	return nil, fmt.Errorf("CompleteWithTools not supported in autopoiesis wrapper")
}

// GroundingController implementation - proxies to underlying client if available

func (w *llmClientWrapper) GetLastGroundingSources() []string {
	if gp, ok := w.client.(types.GroundingProvider); ok {
		return gp.GetLastGroundingSources()
	}
	return nil
}

func (w *llmClientWrapper) IsGoogleSearchEnabled() bool {
	if gp, ok := w.client.(types.GroundingProvider); ok {
		return gp.IsGoogleSearchEnabled()
	}
	return false
}

func (w *llmClientWrapper) IsURLContextEnabled() bool {
	if gp, ok := w.client.(types.GroundingProvider); ok {
		return gp.IsURLContextEnabled()
	}
	return false
}

func (w *llmClientWrapper) SetEnableGoogleSearch(enable bool) {
	if gc, ok := w.client.(types.GroundingController); ok {
		gc.SetEnableGoogleSearch(enable)
	}
}

func (w *llmClientWrapper) SetEnableURLContext(enable bool) {
	if gc, ok := w.client.(types.GroundingController); ok {
		gc.SetEnableURLContext(enable)
	}
}

func (w *llmClientWrapper) SetURLContextURLs(urls []string) {
	if gc, ok := w.client.(types.GroundingController); ok {
		gc.SetURLContextURLs(urls)
	}
}

// ThinkingProvider implementation - proxies to underlying client if available

func (w *llmClientWrapper) GetLastThoughtSummary() string {
	if tp, ok := w.client.(types.ThinkingProvider); ok {
		return tp.GetLastThoughtSummary()
	}
	return ""
}

func (w *llmClientWrapper) GetLastThinkingTokens() int {
	if tp, ok := w.client.(types.ThinkingProvider); ok {
		return tp.GetLastThinkingTokens()
	}
	return 0
}

func (w *llmClientWrapper) IsThinkingEnabled() bool {
	if tp, ok := w.client.(types.ThinkingProvider); ok {
		return tp.IsThinkingEnabled()
	}
	return false
}

func (w *llmClientWrapper) GetThinkingLevel() string {
	if tp, ok := w.client.(types.ThinkingProvider); ok {
		return tp.GetThinkingLevel()
	}
	return ""
}

// ThoughtSignatureProvider implementation - proxies to underlying client if available

func (w *llmClientWrapper) GetLastThoughtSignature() string {
	if tsp, ok := w.client.(types.ThoughtSignatureProvider); ok {
		return tsp.GetLastThoughtSignature()
	}
	return ""
}

// DefaultConfig returns default configuration
func DefaultConfig(workspaceRoot string) Config {
	toolDefaults := internalconfig.DefaultToolGenerationConfig()

	cfg := Config{
		ToolsDir:               filepath.Join(workspaceRoot, ".nerd", "tools"),
		AgentsDir:              filepath.Join(workspaceRoot, ".nerd", "agents"),
		MinConfidence:          0.6,
		MinToolConfidence:      0.75,
		EnableToolGeneration:   true,
		MaxToolsPerSession:     3,
		ToolGenerationCooldown: 0,
		EnableLLM:              true,
		TargetOS:               toolDefaults.TargetOS,
		TargetArch:             toolDefaults.TargetArch,
		WorkspaceRoot:          workspaceRoot,
		// Safety: Explicit gas limit for Ouroboros self-generated logic.
		// Prevents infinite recursion in self-modifying autopoiesis loops.
		// This bounds the number of learning_event facts the kernel will retain.
		MaxLearningFacts: 1000,
	}

	if userCfg, err := internalconfig.LoadUserConfig(internalconfig.DefaultUserConfigPath()); err == nil && userCfg != nil {
		tg := userCfg.GetToolGenerationConfig()
		if tg.TargetOS != "" {
			cfg.TargetOS = tg.TargetOS
		}
		if tg.TargetArch != "" {
			cfg.TargetArch = tg.TargetArch
		}
	}

	return cfg
}

// NewOrchestrator creates a new autopoiesis orchestrator
func NewOrchestrator(client LLMClient, config Config) *Orchestrator {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "NewOrchestrator")
	defer timer.Stop()

	logging.Autopoiesis("Initializing Autopoiesis Orchestrator")
	logging.AutopoiesisDebug("Config: ToolsDir=%s, AgentsDir=%s, MinConfidence=%.2f",
		config.ToolsDir, config.AgentsDir, config.MinConfidence)
	logging.AutopoiesisDebug("Target: OS=%s, Arch=%s", config.TargetOS, config.TargetArch)

	// Create Ouroboros config from autopoiesis config
	ouroborosConfig := OuroborosConfig{
		ToolsDir:        config.ToolsDir,
		CompiledDir:     filepath.Join(config.ToolsDir, ".compiled"),
		MaxToolSize:     100 * 1024, // 100KB
		CompileTimeout:  300 * time.Second,
		ExecuteTimeout:  300 * time.Second,
		AllowNetworking: false,
		AllowFileSystem: true,
		AllowExec:       true,
		TargetOS:        config.TargetOS,
		TargetArch:      config.TargetArch,
		WorkspaceRoot:   config.WorkspaceRoot,
		// Adversarial Co-Evolution (Thunderdome)
		EnableThunderdome: true,
		ThunderdomeConfig: DefaultThunderdomeConfig(),
		MaxPanicRetries:   2,
	}

	logging.AutopoiesisDebug("Creating ToolGenerator")
	toolGen := NewToolGenerator(client, config.ToolsDir)

	// Note: JIT components will be attached later via SetJITComponents if available

	learningsDir := filepath.Join(config.ToolsDir, ".learnings")
	tracesDir := filepath.Join(config.ToolsDir, ".traces")
	profilesDir := filepath.Join(config.ToolsDir, ".profiles")

	logging.AutopoiesisDebug("Creating ProfileStore: %s", profilesDir)
	profileStore := NewProfileStore(profilesDir)

	logging.AutopoiesisDebug("Creating subsystems: ComplexityAnalyzer, PersistenceAnalyzer, AgentCreator")

	orch := &Orchestrator{
		config:      config,
		complexity:  NewComplexityAnalyzer(client),
		toolGen:     toolGen,
		persistence: NewPersistenceAnalyzer(client),
		agentCreate: NewAgentCreator(client, config.AgentsDir),
		ouroboros:   NewOuroborosLoop(client, ouroborosConfig),
		client:      client,

		// Initialize feedback and learning system
		evaluator: NewQualityEvaluator(client, profileStore),
		patterns:  NewPatternDetector(),
		refiner:   NewToolRefiner(client, toolGen),
		learnings: NewLearningStore(learningsDir),
		profiles:  profileStore,

		// Initialize reasoning trace and logging system
		traces:      NewTraceCollector(tracesDir, client),
		logInjector: NewLogInjector(DefaultLoggingRequirements()),
	}

	// Initialize Gemini advanced features helpers
	// The LLMClient interface in autopoiesis is a subset of types.LLMClient,
	// so we wrap it to satisfy the full interface for the research helpers.
	wrapper := &llmClientWrapper{client: client}
	orch.grounding = research.NewGroundingHelper(wrapper)
	orch.thinking = research.NewThinkingHelper(wrapper)

	// Enable Google Search grounding for research-intensive tool generation
	if orch.grounding.IsGroundingAvailable() {
		orch.grounding.EnableGoogleSearch()
		logging.Autopoiesis("Gemini grounding enabled for autopoiesis (Google Search active)")
	}
	if orch.thinking.IsThinkingAvailable() {
		logging.AutopoiesisDebug("Gemini thinking mode active for autopoiesis (level=%s)", orch.thinking.GetThinkingLevel())
	}

	// Wire Ouroboros callback to propagate tool registration facts to parent kernel
	orch.ouroboros.SetOnToolRegistered(func(tool *RuntimeTool) {
		logging.AutopoiesisDebug("Ouroboros callback: tool %s registered, asserting to kernel", tool.Name)
		orch.assertToolRegistered(tool)
	})

	logging.Autopoiesis("Autopoiesis Orchestrator initialized successfully")
	return orch
}

// SetJITComponents attaches the JIT components using interfaces.
// This enables context-aware prompt generation for tool generation stages.
func (o *Orchestrator) SetJITComponents(jitCompiler JITCompiler, assembler PromptAssembler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.jitCompiler = jitCompiler
	o.promptAssembler = assembler

	// Wire the assembler to ToolGenerator and ToolRefiner
	if assembler != nil {
		o.toolGen.SetPromptAssembler(assembler)
		o.refiner.SetPromptAssembler(assembler)
		logging.Autopoiesis("JIT prompt compiler attached to autopoiesis orchestrator")
	}
}

// GetOuroborosLoop returns the Ouroboros Loop for tool self-generation.
// This implements core.ToolGenerator interface for routing coder shard self-tools.
func (o *Orchestrator) GetOuroborosLoop() ToolSynthesizer {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.ouroboros
}

// CompileTool compiles an existing tool source file.
// It reads the file from disk, runs it through safety checks, and compiles/registers it.
func (o *Orchestrator) CompileTool(ctx context.Context, toolName string) (*RuntimeTool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. Resolve tool path
	toolPath := filepath.Join(o.config.ToolsDir, toolName+".go")

	// 2. Read source code
	codeBytes, err := os.ReadFile(toolPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tool source for %s: %w", toolName, err)
	}

	// 3. Delegate to Ouroboros for Safety -> Write -> Compile -> Register
	// We use "Manual Compilation" as purpose to distinguish in logs if needed.
	success, _, _, errMsg := o.ouroboros.GenerateToolFromCode(
		ctx,
		toolName,
		"Manual Compilation", // Purpose
		string(codeBytes),
		1.0,   // Confidence (manual override implies high confidence)
		1.0,   // Priority
		false, // isDiagnostic
	)

	if !success {
		return nil, fmt.Errorf("compilation failed: %s", errMsg)
	}

	// 4. Retrieve registered tool to return
	// We access the registry directly through Ouroboros via interface
	if tool, exists := o.ouroboros.GetRuntimeTool(toolName); exists {
		return tool, nil
	}

	return nil, fmt.Errorf("tool compiled successfully but not found in registry")
}
