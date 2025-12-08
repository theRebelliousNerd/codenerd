// Package coder implements the Coder ShardAgent (Type B: Persistent Specialist).
// The Coder Shard is responsible for code writing, modification, and refactoring.
// It is LANGUAGE-AGNOSTIC - language detection is automatic based on file extensions.
// For language-specific expertise, create Type 3 specialists via: nerd define-agent
package coder

import (
	"codenerd/internal/core"
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
	Facts       []core.Fact       `json:"facts,omitempty"`
	Duration    time.Duration     `json:"duration"`
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
	config core.ShardConfig
	state  core.ShardState

	// Coder-specific
	coderConfig CoderConfig

	// Components - each shard has its own kernel
	kernel       *core.RealKernel
	llmClient    core.LLMClient
	virtualStore *core.VirtualStore

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
		config:          core.DefaultSpecialistConfig("coder", ""),
		state:           core.ShardStateIdle,
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
func (c *CoderShard) SetLLMClient(client core.LLMClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.llmClient = client
}

// SetParentKernel sets the Mangle kernel for fact storage and policy evaluation.
func (c *CoderShard) SetParentKernel(k core.Kernel) {
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
func (c *CoderShard) GetState() core.ShardState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetConfig returns the shard configuration.
func (c *CoderShard) GetConfig() core.ShardConfig {
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
	c.state = core.ShardStateCompleted
	return nil
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
	c.mu.Lock()
	c.state = core.ShardStateRunning
	c.startTime = time.Now()
	c.editHistory = make([]CodeEdit, 0)
	c.diagnostics = make([]core.Diagnostic, 0)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.state = core.ShardStateCompleted
		c.mu.Unlock()
	}()

	fmt.Printf("[CoderShard:%s] Starting task: %s\n", c.id, task)

	// Initialize kernel if not set
	if c.kernel == nil {
		c.kernel = core.NewRealKernel()
	}
	// Load coder-specific policy (only once to avoid duplicate Decl errors)
	if !c.policyLoaded {
		_ = c.kernel.LoadPolicyFile("coder.mg")
		c.policyLoaded = true
	}

	// Parse the task
	parsedTask := c.parseTask(task)

	// Check for safety blocks
	if c.coderConfig.SafetyMode && c.coderConfig.ImpactCheck {
		blocked, reason := c.checkImpact(parsedTask.Target)
		if blocked {
			c.trackRejection(parsedTask.Action, "impact_blocked")
			return "", fmt.Errorf("action blocked: %s", reason)
		}
	}

	// Assert task facts to kernel
	c.assertTaskFacts(parsedTask)

	// Read file context
	fileContext, err := c.readFileContext(ctx, parsedTask.Target)
	if err != nil && parsedTask.Action != "create" {
		return "", fmt.Errorf("failed to read file context: %w", err)
	}

	// Generate code via LLM
	result, err := c.generateCode(ctx, parsedTask, fileContext)
	if err != nil {
		c.trackRejection(parsedTask.Action, "generation_failed")
		return "", fmt.Errorf("code generation failed: %w", err)
	}

	// Apply edits
	if len(result.Edits) > 0 {
		if err := c.applyEdits(ctx, result.Edits); err != nil {
			return "", fmt.Errorf("failed to apply edits: %w", err)
		}
		c.trackAcceptance(parsedTask.Action)

		// Run build check if safety mode enabled
		if c.coderConfig.SafetyMode {
			result.BuildPassed = c.runBuildCheck(ctx)
			if !result.BuildPassed {
				result.Diagnostics = c.diagnostics
			}
		}
	}

	// Generate facts for propagation
	result.Facts = c.generateFacts(result)
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
