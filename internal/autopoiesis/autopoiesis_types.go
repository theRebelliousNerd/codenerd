// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
package autopoiesis

import (
	"context"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// KERNEL INTERFACE - Bridge to Mangle Logic Core
// =============================================================================

// KernelFact is an alias to types.KernelFact.
// This represents a fact that can be asserted to the kernel.
type KernelFact = types.KernelFact

// KernelInterface is an alias to types.KernelInterface.
// This defines the interface for interacting with the Mangle kernel,
// allowing autopoiesis to assert facts and query for derived actions.
type KernelInterface = types.KernelInterface

// =============================================================================
// JIT INTERFACES - Avoid Import Cycles
// =============================================================================

// PromptAssembler is an interface for JIT prompt compilation.
// Implemented by articulation.PromptAssembler to avoid import cycles.
type PromptAssembler interface {
	AssembleSystemPrompt(ctx context.Context, pc interface{}) (string, error)
	JITReady() bool
}

// JITCompiler is an interface for the JIT prompt compiler.
// Implemented by prompt.JITPromptCompiler to avoid import cycles.
type JITCompiler interface {
	Compile(ctx context.Context, cc interface{}) (interface{}, error)
}

// =============================================================================
// OUROBOROS INTERFACES & TYPES
// =============================================================================

// ToolSynthesizer defines the interface for the Ouroboros tool generation loop.
// This allows mocking the complex tool generation process for testing.
type ToolSynthesizer interface {
	// Execute runs the Ouroboros loop to generate a tool.
	Execute(ctx context.Context, need *ToolNeed) *LoopResult

	// GenerateToolFromCode generates a tool from pre-existing code.
	GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string)

	// SetOnToolRegistered sets a callback for when a tool is registered.
	SetOnToolRegistered(callback ToolRegisteredCallback)

	// GetStats returns current loop statistics.
	GetStats() OuroborosStats

	// ListTools returns all registered tools.
	ListTools() []types.ToolInfo

	// GetTool returns info about a specific tool.
	GetTool(name string) (*types.ToolInfo, bool)

	// ExecuteTool runs a registered tool with the given input
	ExecuteTool(ctx context.Context, toolName string, input string) (string, error)

	// GetRuntimeTool returns the internal RuntimeTool handle.
	// Used by Orchestrator for direct access to registry.
	GetRuntimeTool(name string) (*RuntimeTool, bool)

	// ListRuntimeTools returns all registered runtime tools.
	ListRuntimeTools() []*RuntimeTool

	// CheckToolSafety validates tool code without compiling.
	CheckToolSafety(code string) *SafetyReport

	// SetLearningsContext updates the learnings context for the tool generator.
	SetLearningsContext(ctx string)
}

// ToolRegisteredCallback is called when a tool is successfully registered.
// This allows the Orchestrator to propagate facts to the parent kernel.
type ToolRegisteredCallback func(tool *RuntimeTool)

// RuntimeTool represents a compiled tool ready for execution
type RuntimeTool struct {
	Name         string
	Description  string
	BinaryPath   string
	Hash         string
	Schema       ToolSchema
	RegisteredAt time.Time
	ExecuteCount int64
}

// LoopResult contains the result of a complete Ouroboros Loop execution
type LoopResult struct {
	Success       bool
	ToolName      string
	Stage         LoopStage
	Error         string
	SafetyReport  *SafetyReport
	CompileResult *CompileResult
	ToolHandle    *RuntimeTool
	Duration      time.Duration
}

// CompileResult contains compilation output
type CompileResult struct {
	Success     bool
	OutputPath  string
	Hash        string // SHA-256 of compiled binary
	CompileTime time.Duration
	Errors      []string
	Warnings    []string
}

// LoopStage identifies where in the loop we are
type LoopStage int

const (
	StageDetection LoopStage = iota
	StageSpecification
	StageSafetyCheck
	StageThunderdome // NEW: Adversarial testing phase
	StageCompilation
	StageRegistration
	StageExecution
	StageComplete
	StageSimulation // New stage
	StagePanic      // New stage
)

func (s LoopStage) String() string {
	switch s {
	case StageDetection:
		return "detection"
	case StageSpecification:
		return "specification"
	case StageSafetyCheck:
		return "safety_check"
	case StageThunderdome:
		return "thunderdome"
	case StageCompilation:
		return "compilation"
	case StageRegistration:
		return "registration"
	case StageExecution:
		return "execution"
	case StageComplete:
		return "complete"
	case StageSimulation:
		return "simulation"
	case StagePanic:
		return "panic"
	default:
		return "unknown"
	}
}

// OuroborosStats tracks loop statistics
type OuroborosStats struct {
	ToolsGenerated   int
	ToolsCompiled    int
	ToolsRejected    int
	SafetyViolations int
	ExecutionCount   int
	Panics           int
	LastGeneration   time.Time
	// Adversarial Co-Evolution stats
	ThunderdomeRuns     int // Number of Thunderdome battles
	ThunderdomeKills    int // Tools killed by PanicMaker
	ThunderdomeSurvived int // Tools that survived
}

// =============================================================================
// CORE TYPES
// =============================================================================

// Config holds configuration for the autopoiesis system
type Config struct {
	ToolsDir               string        // Directory for generated tools
	AgentsDir              string        // Directory for agent definitions
	MinConfidence          float64       // Minimum confidence to trigger autopoiesis
	MinToolConfidence      float64       // Minimum confidence to trigger tool generation
	EnableToolGeneration   bool          // Master switch for Ouroboros tool generation
	MaxToolsPerSession     int           // Hard cap on tools per session (0 = unlimited)
	ToolGenerationCooldown time.Duration // Cooldown between tool generations (0 = none)
	EnableLLM              bool          // Whether to use LLM for analysis
	TargetOS               string        // Target GOOS for tool compilation
	TargetArch             string        // Target GOARCH for tool compilation
	WorkspaceRoot          string        // Absolute workspace root for module replacement
	MaxLearningFacts       int           // Maximum number of learning event facts to keep
}

// AnalysisResult contains the complete autopoiesis analysis
type AnalysisResult struct {
	// Complexity analysis
	Complexity      ComplexityResult
	NeedsCampaign   bool
	SuggestedPhases []string

	// Tool generation
	ToolNeeds []ToolNeed

	// Persistence analysis
	Persistence     PersistenceResult
	NeedsPersistent bool
	SuggestedAgents []AgentSpec

	// Actions to take
	Actions []AutopoiesisAction

	// Metadata
	AnalyzedAt time.Time
	InputHash  string
}

// AutopoiesisAction represents an action the system should take
type AutopoiesisAction struct {
	Type        ActionType
	Priority    float64
	Description string
	Payload     any // Type-specific payload
}

// ActionType defines types of autopoiesis actions
type ActionType int

const (
	ActionNone ActionType = iota
	ActionStartCampaign
	ActionGenerateTool
	ActionCreateAgent
	ActionDelegateToShard
)

// String returns the string representation of an action type
func (at ActionType) String() string {
	switch at {
	case ActionStartCampaign:
		return "start_campaign"
	case ActionGenerateTool:
		return "generate_tool"
	case ActionCreateAgent:
		return "create_agent"
	case ActionDelegateToShard:
		return "delegate_to_shard"
	default:
		return "none"
	}
}

// CampaignPayload contains data for starting a campaign
type CampaignPayload struct {
	Phases         []string
	EstimatedFiles int
	Reasons        []string
}

// QuickResult is a lightweight analysis result for real-time decisions
type QuickResult struct {
	NeedsCampaign   bool
	NeedsPersistent bool
	NeedsTool       bool
	ComplexityLevel ComplexityLevel
	TopAction       *AutopoiesisAction
}

// =============================================================================
// AGENT TYPES
// =============================================================================

// AgentMemory represents the persistent memory for an agent
type AgentMemory struct {
	AgentName   string                 `json:"agent_name"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Learnings   []Learning             `json:"learnings"`
	Preferences map[string]interface{} `json:"preferences"`
	Patterns    []LearnedPattern       `json:"patterns"`
}

// Learning represents something the agent has learned
type Learning struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // "preference", "pattern", "feedback"
	Content    string    `json:"content"`
	Source     string    `json:"source"`
	Confidence float64   `json:"confidence"`
	LearnedAt  time.Time `json:"learned_at"`
	LastUsed   time.Time `json:"last_used"`
	UseCount   int       `json:"use_count"`
}

// LearnedPattern represents a pattern the agent has identified
type LearnedPattern struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Examples    []string  `json:"examples"`
	Confidence  float64   `json:"confidence"`
	DetectedAt  time.Time `json:"detected_at"`
}
