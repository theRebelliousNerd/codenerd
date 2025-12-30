// Package autopoiesis implements self-modification capabilities for codeNERD.
// Autopoiesis (from Greek: self-creation) enables the system to:
// 1. Detect when tasks require campaign orchestration (complex multi-phase work)
// 2. Generate new tools when existing capabilities are insufficient
// 3. Create persistent agents when ongoing monitoring/learning is needed
package autopoiesis

import (
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
// CORE TYPES
// =============================================================================

// Config holds configuration for the autopoiesis system
type Config struct {
	ToolsDir         string  // Directory for generated tools
	AgentsDir        string  // Directory for agent definitions
	MinConfidence    float64 // Minimum confidence to trigger autopoiesis
	MinToolConfidence float64 // Minimum confidence to trigger tool generation
	EnableToolGeneration bool  // Master switch for Ouroboros tool generation
	MaxToolsPerSession   int   // Hard cap on tools per session (0 = unlimited)
	ToolGenerationCooldown time.Duration // Cooldown between tool generations (0 = none)
	EnableLLM        bool    // Whether to use LLM for analysis
	TargetOS         string  // Target GOOS for tool compilation
	TargetArch       string  // Target GOARCH for tool compilation
	WorkspaceRoot    string  // Absolute workspace root for module replacement
	MaxLearningFacts int     // Maximum number of learning event facts to keep
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
