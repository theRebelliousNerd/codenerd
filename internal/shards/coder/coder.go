// Package coder implements the Coder ShardAgent (Type B: Persistent Specialist).
// The Coder Shard is responsible for code writing, modification, and refactoring.
// It is LANGUAGE-AGNOSTIC - language detection is automatic based on file extensions.
// For language-specific expertise, create Type 3 specialists via: nerd define-agent
package coder

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// CoderConfig holds configuration for the coder shard.
type CoderConfig struct {
	MaxEdits     int    // Max edits per task (default: 10)
	StyleProfile string // Code style rules or path to style guide
	SafetyMode   bool   // Block risky edits without confirmation
	MaxRetries   int    // Retry limit for failed edits
	WorkingDir   string // Workspace path
	ImpactCheck  bool   // Check dependency impact before write (default: true)
}

// DefaultCoderConfig returns sensible defaults for coding.
func DefaultCoderConfig() CoderConfig {
	return CoderConfig{
		MaxEdits:     10,
		StyleProfile: "default",
		SafetyMode:   true,
		MaxRetries:   3,
		WorkingDir:   ".",
		ImpactCheck:  true,
	}
}

// =============================================================================
// RESULT TYPES
// =============================================================================

// CodeEdit represents a proposed change to a file.
type CodeEdit struct {
	File       string `json:"file"`
	OldContent string `json:"old_content,omitempty"` // For modifications
	NewContent string `json:"new_content"`           // New content
	Type       string `json:"type"`                  // "create", "modify", "delete"
	Language   string `json:"language"`              // Detected language
	Rationale  string `json:"rationale"`             // Why this change
}

// CoderResult represents the output of a coding task.
type CoderResult struct {
	Summary     string            `json:"summary"`
	Edits       []CodeEdit        `json:"edits"`
	BuildPassed bool              `json:"build_passed"`
	TestsPassed bool              `json:"tests_passed"`
	Diagnostics []core.Diagnostic `json:"diagnostics,omitempty"`
	Facts       []types.Fact      `json:"facts,omitempty"`
	Duration    time.Duration     `json:"duration"`

	// Artifact routing (for Ouroboros integration)
	ArtifactType ArtifactType `json:"artifact_type,omitempty"` // project_code, self_tool, diagnostic
	ToolName     string       `json:"tool_name,omitempty"`     // Name for self-tools
}

// CoderTask represents a parsed coding task.
type CoderTask struct {
	Action      string            // create, modify, refactor, fix, implement
	Target      string            // File path or symbol
	Instruction string            // What to do
	Context     map[string]string // Additional context
}

// =============================================================================
// CODER SHARD
// =============================================================================

// CoderShard is specialized for code writing and modification.
// It is language-agnostic and auto-detects languages from file extensions.
type CoderShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config types.ShardConfig
	state  types.ShardState

	// Coder-specific
	coderConfig CoderConfig

	// Components - each shard has its own kernel
	kernel       *core.RealKernel
	llmClient    types.LLMClient
	virtualStore *core.VirtualStore

	// JIT prompt compilation (optional - falls back to legacy template if nil)
	promptAssembler *articulation.PromptAssembler

	// State tracking
	startTime   time.Time
	editHistory []CodeEdit
	diagnostics []core.Diagnostic

	// Learnings for autopoiesis (in-memory, synced to LearningStore)
	rejectionCount  map[string]int
	acceptanceCount map[string]int
	learningStore   core.LearningStore

	// Policy loading guard (prevents duplicate Decl errors)
	policyLoaded bool
}

// NewCoderShard creates a new Coder shard.
func NewCoderShard() *CoderShard {
	return NewCoderShardWithConfig(DefaultCoderConfig())
}

// NewCoderShardWithConfig creates a coder shard with custom config.
func NewCoderShardWithConfig(coderConfig CoderConfig) *CoderShard {
	shard := &CoderShard{
		id:              fmt.Sprintf("coder-%d", time.Now().UnixNano()),
		config:          coreshards.DefaultSpecialistConfig("coder", ""),
		state:           types.ShardStateIdle,
		coderConfig:     coderConfig,
		editHistory:     make([]CodeEdit, 0),
		diagnostics:     make([]core.Diagnostic, 0),
		rejectionCount:  make(map[string]int),
		acceptanceCount: make(map[string]int),
	}
	return shard
}

// =============================================================================
// DEPENDENCY INJECTION
// =============================================================================

// SetLLMClient sets the LLM client for code generation.
func (c *CoderShard) SetLLMClient(client types.LLMClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.llmClient = client
}

// SetSessionContext sets the session context (for dream mode, etc.).
func (c *CoderShard) SetSessionContext(ctx *core.SessionContext) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.SessionContext = ctx
}

// SetParentKernel sets the Mangle kernel for fact storage and policy evaluation.
func (c *CoderShard) SetParentKernel(k types.Kernel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		c.kernel = rk
	} else {
		panic("CoderShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store for action routing.
func (c *CoderShard) SetVirtualStore(vs *core.VirtualStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.virtualStore = vs
}

// SetLearningStore sets the learning store for persistent autopoiesis.
func (c *CoderShard) SetLearningStore(ls core.LearningStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.learningStore = ls
	// Load existing patterns from store
	c.loadLearnedPatterns()
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt compilation.
// If not set or JIT is not ready, the shard falls back to the legacy template.
func (c *CoderShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.promptAssembler = pa
}

// =============================================================================
// SHARD INTERFACE IMPLEMENTATION
// =============================================================================

// GetID returns the shard ID.
func (c *CoderShard) GetID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// GetState returns the current state.
func (c *CoderShard) GetState() types.ShardState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetConfig returns the shard configuration.
func (c *CoderShard) GetConfig() types.ShardConfig {
	return c.config
}

// GetKernel returns the shard's kernel (for fact propagation).
func (c *CoderShard) GetKernel() *core.RealKernel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kernel
}

// Stop stops the shard.
func (c *CoderShard) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = types.ShardStateCompleted
	return nil
}

// =============================================================================
// DREAM MODE (Simulation/Learning)
// =============================================================================

// describeDreamPlan returns a description of what the coder would do WITHOUT executing.
// Used in dream state for learning and simulation.
func (c *CoderShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	fmt.Printf("[CoderShard:%s] DREAM MODE - describing plan without execution\n", c.id)

	// Use LLM to describe the plan
	if c.llmClient == nil {
		return "CoderShard would analyze the task and generate code, but no LLM client available for dream description.", nil
	}

	prompt := fmt.Sprintf(`You are a coding agent in DREAM MODE. Describe what you WOULD do for this task WITHOUT actually doing it.

Task: %s

Provide a structured analysis:
1. **Understanding**: What is being asked?
2. **Files Affected**: What files would I create/modify?
3. **Approach**: Step-by-step what I would do
4. **Tools Needed**: What tools/commands would I use?
5. **Risks**: What could go wrong?
6. **Questions**: What would I need clarified?

Remember: This is a simulation. Describe the plan, don't execute it.`, task)

	response, err := c.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("CoderShard dream analysis failed: %v", err), nil
	}

	return response, nil
}

// =============================================================================
// MAIN EXECUTION
// =============================================================================

// Execute performs the coding task.
// Supported formats:
//   - create file:path/to/file.go spec:description
//   - modify file:path/to/file.go instruction:what to change
//   - refactor file:path/to/file.go target:functionName instruction:how
//   - fix file:path/to/file.go error:error message
//   - implement file:path/to/file.go spec:description
func (c *CoderShard) Execute(ctx context.Context, task string) (string, error) {
	timer := logging.StartTimer(logging.CategoryCoder, "Execute")

	logging.Coder("Starting task execution: %s", task)
	fmt.Println("DEBUG: CoderShard.Execute STARTED")

	c.mu.Lock()
	c.state = types.ShardStateRunning
	c.startTime = time.Now()
	c.editHistory = make([]CodeEdit, 0)
	c.diagnostics = make([]core.Diagnostic, 0)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.state = types.ShardStateCompleted
		c.mu.Unlock()
		timer.StopWithInfo()
	}()

	// DREAM MODE: Only describe what we would do, don't execute
	if c.config.SessionContext != nil && c.config.SessionContext.DreamMode {
		logging.CoderDebug("DREAM MODE enabled, describing plan without execution")
		return c.describeDreamPlan(ctx, task)
	}

	logging.CoderDebug("[CoderShard:%s] Initializing for task", c.id)

	// Initialize kernel if not set
	if c.kernel == nil {
		logging.CoderDebug("Creating new RealKernel instance")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		c.kernel = kernel
	}
	// Load coder-specific policy (only once to avoid duplicate Decl errors)
	if !c.policyLoaded {
		logging.CoderDebug("Loading coder.mg policy file")
		fmt.Println("DEBUG: Loading coder.mg")
		_ = c.kernel.LoadPolicyFile("coder.mg")
		c.policyLoaded = true
		fmt.Println("DEBUG: Loaded coder.mg")
	} else {
		fmt.Println("DEBUG: coder.mg already loaded")
	}

	// Parse the task
	parseTimer := logging.StartTimer(logging.CategoryCoder, "ParseTask")
	parsedTask := c.parseTask(task)
	parseTimer.Stop()

	logging.Coder("Parsed task: action=%s, target=%s, instruction=%s",
		parsedTask.Action, parsedTask.Target, parsedTask.Instruction)
	fmt.Println("DEBUG: Parsed task")

	// Check for safety blocks
	if c.coderConfig.SafetyMode && c.coderConfig.ImpactCheck {
		impactTimer := logging.StartTimer(logging.CategoryCoder, "ImpactCheck")
		blocked, reason := c.checkImpact(parsedTask.Target)
		impactTimer.Stop()
		if blocked {
			logging.Coder("Action blocked by impact check: %s", reason)
			c.trackRejection(parsedTask.Action, "impact_blocked")
			return "", fmt.Errorf("action blocked: %s", reason)
		}
		logging.CoderDebug("Impact check passed for target: %s", parsedTask.Target)
	}
	fmt.Println("DEBUG: Impact check passed")

	// Assert task facts to kernel
	logging.CoderDebug("Asserting task facts to kernel")
	fmt.Println("DEBUG: Asserting task facts")
	c.assertTaskFacts(parsedTask)
	fmt.Println("DEBUG: Task facts asserted")

	// Read file context
	contextTimer := logging.StartTimer(logging.CategoryCoder, "ReadFileContext")
	fmt.Println("DEBUG: Reading file context")
	fileContext, err := c.readFileContext(ctx, parsedTask.Target)
	contextTimer.Stop()
	fmt.Println("DEBUG: File context read (len=", len(fileContext), ")")
	if err != nil && parsedTask.Action != "create" {
		logging.Get(logging.CategoryCoder).Error("Failed to read file context: %v", err)
		return "", fmt.Errorf("failed to read file context: %w", err)
	}
	if fileContext != "" {
		logging.CoderDebug("Read file context: %d bytes", len(fileContext))
	}

	// Generate code via LLM
	genTimer := logging.StartTimer(logging.CategoryCoder, "GenerateCode")
	fmt.Println("DEBUG: Calling generateCode")
	result, err := c.generateCode(ctx, parsedTask, fileContext)
	fmt.Println("DEBUG: generateCode returned")
	genTimer.StopWithInfo()
	if err != nil {
		logging.Get(logging.CategoryCoder).Error("Code generation failed: %v", err)
		c.trackRejection(parsedTask.Action, "generation_failed")
		return "", fmt.Errorf("code generation failed: %w", err)
	}
	logging.Coder("Generated %d edits", len(result.Edits))

	// Check for self-tool artifact - route to Ouroboros instead of direct file write
	if result.ArtifactType == ArtifactTypeSelfTool || result.ArtifactType == ArtifactTypeDiagnostic {
		logging.Coder("Detected self-tool artifact (type=%s), routing to Ouroboros", result.ArtifactType)
		return c.routeToOuroboros(ctx, result)
	}

	// Apply edits (normal project code path)
	if len(result.Edits) > 0 {
		applyTimer := logging.StartTimer(logging.CategoryCoder, "ApplyEdits")
		if err := c.applyEdits(ctx, result.Edits); err != nil {
			applyTimer.Stop()
			logging.Get(logging.CategoryCoder).Error("Failed to apply edits: %v", err)
			return "", fmt.Errorf("failed to apply edits: %w", err)
		}
		applyTimer.Stop()
		logging.Coder("Applied %d edits successfully", len(result.Edits))
		c.trackAcceptance(parsedTask.Action)

		// Run build check if safety mode enabled
		if c.coderConfig.SafetyMode {
			buildTimer := logging.StartTimer(logging.CategoryCoder, "BuildCheck")
			result.BuildPassed = c.runBuildCheck(ctx)
			buildTimer.Stop()
			if !result.BuildPassed {
				logging.Coder("Build check FAILED with %d diagnostics", len(c.diagnostics))
				result.Diagnostics = c.diagnostics
			} else {
				logging.Coder("Build check PASSED")
			}
		}
	} else {
		logging.CoderDebug("No edits to apply")
	}

	// Generate facts for propagation
	result.Facts = c.generateFacts(result)
	logging.CoderDebug("Generated %d facts for propagation", len(result.Facts))
	for _, fact := range result.Facts {
		if c.kernel != nil {
			_ = c.kernel.Assert(fact)
		}
	}

	result.Duration = time.Since(c.startTime)

	// Store edit history
	c.mu.Lock()
	c.editHistory = append(c.editHistory, result.Edits...)
	c.mu.Unlock()

	logging.Coder("Task completed: %d edits, duration=%v, build_passed=%v",
		len(result.Edits), result.Duration, result.BuildPassed)

	return c.buildResponse(result), nil
}

// =============================================================================
// TASK PARSING
// =============================================================================

// parseTask extracts action and parameters from task string.
func (c *CoderShard) parseTask(task string) CoderTask {
	parsed := CoderTask{
		Action:  "modify",
		Context: make(map[string]string),
	}

	parts := strings.Fields(task)
	if len(parts) == 0 {
		return parsed
	}

	// First token may be the action
	action := strings.ToLower(parts[0])
	switch action {
	case "create", "new", "add":
		parsed.Action = "create"
		parts = parts[1:]
	case "modify", "edit", "change", "update":
		parsed.Action = "modify"
		parts = parts[1:]
	case "refactor", "restructure":
		parsed.Action = "refactor"
		parts = parts[1:]
	case "fix", "repair", "patch":
		parsed.Action = "fix"
		parts = parts[1:]
	case "implement", "write":
		parsed.Action = "implement"
		parts = parts[1:]
	case "delete", "remove":
		parsed.Action = "delete"
		parts = parts[1:]
	default:
		// First token might be a file path
		if strings.Contains(action, "/") || strings.Contains(action, ".") {
			parsed.Target = action
			parts = parts[1:]
		}
	}

	// Parse key:value pairs
	var instructions []string
	for _, part := range parts {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.ToLower(kv[0])
			value := kv[1]

			switch key {
			case "file", "path", "target":
				parsed.Target = c.resolvePath(value)
			case "spec", "specification", "description":
				parsed.Instruction = value
			case "instruction", "change", "do":
				parsed.Instruction = value
			case "error", "issue", "bug":
				parsed.Context["error"] = value
			case "findings":
				parsed.Context["findings"] = value
			case "test_errors":
				parsed.Context["test_errors"] = value
			default:
				parsed.Context[key] = value
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - could be file path or instruction
			if parsed.Target == "" && (strings.Contains(part, "/") || strings.Contains(part, ".")) {
				parsed.Target = c.resolvePath(part)
			} else {
				instructions = append(instructions, part)
			}
		}
	}

	// Combine remaining parts as instruction if not set
	if parsed.Instruction == "" && len(instructions) > 0 {
		parsed.Instruction = strings.Join(instructions, " ")
	}

	return parsed
}

// =============================================================================
// PIGGYBACK PROTOCOL ROUTING
// =============================================================================

// routeControlPacketToKernel processes the control_packet and routes data to the kernel.
// This handles mangle_updates, memory_operations, and self_correction signals.
func (c *CoderShard) routeControlPacketToKernel(control *articulation.ControlPacket) {
	if control == nil {
		return
	}

	c.mu.RLock()
	kernel := c.kernel
	c.mu.RUnlock()

	if kernel == nil {
		logging.CoderDebug("No kernel available for control packet routing")
		return
	}

	// 1. Assert mangle_updates as facts
	if len(control.MangleUpdates) > 0 {
		logging.CoderDebug("Routing %d mangle_updates to kernel", len(control.MangleUpdates))
		for _, atomStr := range control.MangleUpdates {
			if fact := parseMangleAtomCoder(atomStr); fact != nil {
				if err := kernel.Assert(*fact); err != nil {
					logging.Get(logging.CategoryCoder).Warn("Failed to assert mangle_update %q: %v", atomStr, err)
				}
			}
		}
	}

	// 2. Process memory_operations (route to LearningStore)
	if len(control.MemoryOperations) > 0 {
		logging.CoderDebug("Processing %d memory_operations", len(control.MemoryOperations))
		c.processMemoryOperations(control.MemoryOperations)
	}

	// 3. Track self-correction for autopoiesis
	if control.SelfCorrection != nil && control.SelfCorrection.Triggered {
		logging.Coder("Self-correction triggered: %s", control.SelfCorrection.Hypothesis)
		selfCorrFact := core.Fact{
			Predicate: "self_correction_triggered",
			Args:      []interface{}{c.id, control.SelfCorrection.Hypothesis, time.Now().Unix()},
		}
		_ = kernel.Assert(selfCorrFact)
	}

	// 4. Log reasoning trace for debugging/learning
	if control.ReasoningTrace != "" {
		logging.CoderDebug("Reasoning trace: %.200s...", control.ReasoningTrace)
	}
}

// processMemoryOperations handles memory_operations from the control packet.
func (c *CoderShard) processMemoryOperations(ops []articulation.MemoryOperation) {
	c.mu.RLock()
	ls := c.learningStore
	c.mu.RUnlock()

	for _, op := range ops {
		switch op.Op {
		case "store_vector":
			if ls != nil {
				_ = ls.Save("coder_memory", op.Key, []any{op.Value}, "")
			}
			logging.CoderDebug("Memory store_vector: %s", op.Key)
		case "promote_to_long_term":
			if ls != nil {
				_ = ls.Save("coder_long_term", op.Key, []any{op.Value}, "")
			}
			logging.CoderDebug("Memory promote_to_long_term: %s", op.Key)
		case "note":
			logging.CoderDebug("Memory note: %s = %s", op.Key, op.Value)
		case "forget":
			if ls != nil {
				_ = ls.DecayConfidence("coder_memory", 0.0)
			}
			logging.CoderDebug("Memory forget: %s", op.Key)
		}
	}
}

// parseMangleAtomCoder attempts to parse a string into a Mangle fact.
func parseMangleAtomCoder(atomStr string) *core.Fact {
	atomStr = strings.TrimSpace(atomStr)
	if atomStr == "" {
		return nil
	}

	fact, err := core.ParseFactString(atomStr)
	if err != nil {
		logging.Get(logging.CategoryCoder).Warn("Failed to parse mangle atom %q: %v", atomStr, err)
		return nil
	}
	return &fact
}
