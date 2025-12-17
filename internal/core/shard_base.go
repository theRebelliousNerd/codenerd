package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/types"
)

// =============================================================================
// BASE SHARD AGENT IMPLEMENTATION
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

func (b *BaseShardAgent) SetSessionContext(ctx *SessionContext) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.SessionContext = ctx
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

// buildSessionContextPrompt builds comprehensive session context for cross-shard awareness (Blackboard Pattern).
// This is available to all shards including Type U (User-defined) specialists.
// Subclasses can call this to inject session context into their LLM prompts.
func (b *BaseShardAgent) buildSessionContextPrompt() string {
	if b.config.SessionContext == nil {
		return ""
	}

	var sb strings.Builder
	ctx := b.config.SessionContext

	// ==========================================================================
	// CURRENT DIAGNOSTICS
	// ==========================================================================
	if len(ctx.CurrentDiagnostics) > 0 {
		sb.WriteString("\nCURRENT BUILD/LINT ISSUES:\n")
		for _, diag := range ctx.CurrentDiagnostics {
			sb.WriteString(fmt.Sprintf("  %s\n", diag))
		}
	}

	// ==========================================================================
	// TEST STATE
	// ==========================================================================
	if ctx.TestState == "/failing" || len(ctx.FailingTests) > 0 {
		sb.WriteString("\nTEST STATE: FAILING\n")
		if ctx.TDDRetryCount > 0 {
			sb.WriteString(fmt.Sprintf("  TDD Retry: %d\n", ctx.TDDRetryCount))
		}
		for _, test := range ctx.FailingTests {
			sb.WriteString(fmt.Sprintf("  - %s\n", test))
		}
	}

	// ==========================================================================
	// RECENT FINDINGS (from reviewer/tester)
	// ==========================================================================
	if len(ctx.RecentFindings) > 0 {
		sb.WriteString("\nRECENT FINDINGS:\n")
		for _, finding := range ctx.RecentFindings {
			sb.WriteString(fmt.Sprintf("  - %s\n", finding))
		}
	}

	// ==========================================================================
	// IMPACTED FILES
	// ==========================================================================
	if len(ctx.ImpactedFiles) > 0 {
		sb.WriteString("\nIMPACTED FILES:\n")
		for _, file := range ctx.ImpactedFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", file))
		}
	}

	// ==========================================================================
	// GIT CONTEXT
	// ==========================================================================
	if ctx.GitBranch != "" || len(ctx.GitRecentCommits) > 0 {
		sb.WriteString("\nGIT CONTEXT:\n")
		if ctx.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("  Branch: %s\n", ctx.GitBranch))
		}
		if len(ctx.GitRecentCommits) > 0 {
			sb.WriteString("  Recent commits:\n")
			for _, commit := range ctx.GitRecentCommits {
				sb.WriteString(fmt.Sprintf("    - %s\n", commit))
			}
		}
	}

	// ==========================================================================
	// CAMPAIGN CONTEXT
	// ==========================================================================
	if ctx.CampaignActive {
		sb.WriteString("\nCAMPAIGN CONTEXT:\n")
		if ctx.CampaignPhase != "" {
			sb.WriteString(fmt.Sprintf("  Phase: %s\n", ctx.CampaignPhase))
		}
		if ctx.CampaignGoal != "" {
			sb.WriteString(fmt.Sprintf("  Goal: %s\n", ctx.CampaignGoal))
		}
	}

	// ==========================================================================
	// PRIOR SHARD OUTPUTS
	// ==========================================================================
	if len(ctx.PriorShardOutputs) > 0 {
		sb.WriteString("\nPRIOR SHARD RESULTS:\n")
		for _, output := range ctx.PriorShardOutputs {
			status := "SUCCESS"
			if !output.Success {
				status = "FAILED"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s - %s\n",
				output.ShardType, status, output.Task, output.Summary))
		}
	}

	// ==========================================================================
	// RECENT SESSION ACTIONS
	// ==========================================================================
	if len(ctx.RecentActions) > 0 {
		sb.WriteString("\nSESSION ACTIONS:\n")
		for _, action := range ctx.RecentActions {
			sb.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialist Hints)
	// ==========================================================================
	if len(ctx.KnowledgeAtoms) > 0 || len(ctx.SpecialistHints) > 0 {
		sb.WriteString("\nDOMAIN KNOWLEDGE:\n")
		for _, atom := range ctx.KnowledgeAtoms {
			sb.WriteString(fmt.Sprintf("  - %s\n", atom))
		}
		for _, hint := range ctx.SpecialistHints {
			sb.WriteString(fmt.Sprintf("  - HINT: %s\n", hint))
		}
	}

	// ==========================================================================
	// SAFETY CONSTRAINTS
	// ==========================================================================
	if len(ctx.BlockedActions) > 0 || len(ctx.SafetyWarnings) > 0 {
		sb.WriteString("\nSAFETY CONSTRAINTS:\n")
		for _, blocked := range ctx.BlockedActions {
			sb.WriteString(fmt.Sprintf("  BLOCKED: %s\n", blocked))
		}
		for _, warning := range ctx.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("  WARNING: %s\n", warning))
		}
	}

	// ==========================================================================
	// COMPRESSED SESSION HISTORY
	// ==========================================================================
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 1500 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetSessionContext returns the session context for subclasses.
func (b *BaseShardAgent) GetSessionContext() *SessionContext {
	return b.config.SessionContext
}

// =============================================================================
// TYPE ALIASES AND RE-EXPORTS
// =============================================================================

// Type aliases to internal/types to avoid import cycles
type ShardType = types.ShardType
type ShardState = types.ShardState
type ShardPermission = types.ShardPermission
type ModelCapability = types.ModelCapability
type ModelConfig = types.ModelConfig
type ShardConfig = types.ShardConfig
type ShardResult = types.ShardResult
type ShardInfo = types.ShardInfo
type ShardLearning = types.ShardLearning
type StructuredIntent = types.StructuredIntent
type ShardSummary = types.ShardSummary
type SessionContext = types.SessionContext
type ShardAgent = types.ShardAgent
type SpawnPriority = types.SpawnPriority

// Re-export constants from internal/types
const (
	ShardTypeEphemeral  = types.ShardTypeEphemeral
	ShardTypePersistent = types.ShardTypePersistent
	ShardTypeUser       = types.ShardTypeUser
	ShardTypeSystem     = types.ShardTypeSystem

	ShardStateIdle      = types.ShardStateIdle
	ShardStateRunning   = types.ShardStateRunning
	ShardStateCompleted = types.ShardStateCompleted
	ShardStateFailed    = types.ShardStateFailed

	PermissionReadFile  = types.PermissionReadFile
	PermissionWriteFile = types.PermissionWriteFile
	PermissionExecCmd   = types.PermissionExecCmd
	PermissionNetwork   = types.PermissionNetwork
	PermissionBrowser   = types.PermissionBrowser
	PermissionCodeGraph = types.PermissionCodeGraph
	PermissionAskUser   = types.PermissionAskUser
	PermissionResearch  = types.PermissionResearch

	CapabilityHighReasoning = types.CapabilityHighReasoning
	CapabilityBalanced      = types.CapabilityBalanced
	CapabilityHighSpeed     = types.CapabilityHighSpeed

	PriorityLow      = types.PriorityLow
	PriorityNormal   = types.PriorityNormal
	PriorityHigh     = types.PriorityHigh
	PriorityCritical = types.PriorityCritical
)
