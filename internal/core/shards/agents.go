package shards

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// BASE IMPLEMENTATION
// =============================================================================

// BaseShardAgent provides common functionality for shards.
type BaseShardAgent struct {
	id     string
	config types.ShardConfig
	state  types.ShardState
	mu     sync.RWMutex

	// Dependencies
	kernel    types.Kernel
	llmClient types.LLMClient
	stopCh    chan struct{}
}

func NewBaseShardAgent(id string, config types.ShardConfig) *BaseShardAgent {
	return &BaseShardAgent{
		id:     id,
		config: config,
		state:  types.ShardStateIdle,
		stopCh: make(chan struct{}),
	}
}

func (b *BaseShardAgent) GetID() string {
	return b.id
}

func (b *BaseShardAgent) GetState() types.ShardState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *BaseShardAgent) SetState(state types.ShardState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = state
}

func (b *BaseShardAgent) GetConfig() types.ShardConfig {
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
	b.state = types.ShardStateCompleted
	return nil
}

func (b *BaseShardAgent) SetParentKernel(k types.Kernel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.kernel = k
}

func (b *BaseShardAgent) SetLLMClient(client types.LLMClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.llmClient = client
}

func (b *BaseShardAgent) SetSessionContext(ctx *types.SessionContext) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.SessionContext = ctx
}

func (b *BaseShardAgent) HasPermission(p types.ShardPermission) bool {
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
func (b *BaseShardAgent) llm() types.LLMClient {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.llmClient
}

// BuildSessionContextPrompt builds comprehensive session context for cross-shard awareness (Blackboard Pattern).
// This is available to all shards including Type U (User-defined) specialists.
// Subclasses can call this to inject session context into their LLM prompts.
func (b *BaseShardAgent) BuildSessionContextPrompt() string {
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
func (b *BaseShardAgent) GetSessionContext() *types.SessionContext {
	return b.config.SessionContext
}

// =============================================================================
// SYSTEM SHARD
// =============================================================================

// SystemShard is a Type 1 (Permanent) shard agent.
// It runs continuously in the background, monitoring the environment
// and maintaining system homeostasis.
type SystemShard struct {
	*BaseShardAgent
	systemPrompt string
}

// NewSystemShard creates a new System Shard.
func NewSystemShard(id string, config types.ShardConfig, systemPrompt string) *SystemShard {
	if systemPrompt == "" {
		systemPrompt = `You are the System Shard (Type 1).
Your Role: The Operating System of the Agent.
Your Duties:
1. Monitor the filesystem for changes (Fact-Based Filesystem).
2. Maintain the integrity of the .nerd/ directory.
3. Prune old logs or temporary files.
4. Alert the Kernel to critical system state changes.

You run in a continuous loop. Report status every heartbeat.`
	}
	return &SystemShard{
		BaseShardAgent: NewBaseShardAgent(id, config),
		systemPrompt:   systemPrompt,
	}
}

// Execute runs the System Shard's continuous loop.
// Unlike Type 2 shards, this does NOT exit after one task.
func (s *SystemShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(types.ShardStateRunning)
	defer s.SetState(types.ShardStateCompleted)

	// Prime with a single LLM call to seed role-specific intent/plan.
	// This makes it a "Real LLM Shard" as requested.
	if llm := s.llm(); llm != nil {
		userPrompt := fmt.Sprintf("System Startup. Task: %s. Status: Online.", task)
		// We ignore the error here to allow the loop to proceed even if LLM is flaky on startup
		// Ideally we'd log this.
		_, _ = llm.CompleteWithSystem(ctx, s.systemPrompt, userPrompt)
	}

	// System Shard Main Loop
	ticker := time.NewTicker(10 * time.Second) // Heartbeat
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "System Shard shutdown", ctx.Err()
		case <-s.stopCh:
			return "System Shard stopped", nil
		case tick := <-ticker.C:
			// Propagate a heartbeat fact to the parent kernel
			if s.kernel != nil {
				_ = s.kernel.Assert(types.Fact{
					Predicate: "system_heartbeat",
					Args:      []interface{}{s.id, tick.Unix()},
				})
			}
		}
	}
}
