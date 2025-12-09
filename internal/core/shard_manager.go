package core

import (
	"codenerd/internal/logging"
	"codenerd/internal/usage"
	"context"
	"fmt"
	"sort"
	"strings"
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

// StructuredIntent represents the parsed user intent from the perception transducer.
type StructuredIntent struct {
	ID         string // Unique intent ID
	Category   string // /query, /mutation, /instruction
	Verb       string // /explain, /refactor, /debug, /generate
	Target     string // File, symbol, or concept target
	Constraint string // Additional constraints
}

// ShardSummary represents a compressed summary of a prior shard execution.
type ShardSummary struct {
	ShardType string    // "reviewer", "coder", "tester", "researcher"
	Task      string    // Original task (truncated)
	Summary   string    // Compressed output summary
	Timestamp time.Time // When executed
	Success   bool      // Whether it succeeded
}

// SessionContext holds compressed session context for shard injection (Blackboard Pattern).
// This enables shards to understand the full session history without token explosion.
// Extended to include all context types specified in the codeNERD architecture.
type SessionContext struct {
	// ==========================================================================
	// CORE CONTEXT (Original)
	// ==========================================================================
	CompressedHistory string            // Semantically compressed session (from compressor)
	RecentFindings    []string          // Recent reviewer/tester findings
	RecentActions     []string          // Recent shard actions taken
	ActiveFiles       []string          // Files currently in focus
	ExtraContext      map[string]string // Additional context key-values

	// ==========================================================================
	// DREAM MODE (Simulation/Learning)
	// ==========================================================================
	DreamMode bool // When true, shard should ONLY describe what it would do, not execute

	// ==========================================================================
	// WORLD MODEL / EDB FACTS
	// ==========================================================================
	ImpactedFiles      []string // Files transitively affected by current changes (impacted/1)
	CurrentDiagnostics []string // Active errors/warnings from diagnostic/5
	SymbolContext      []string // Relevant symbols in scope (symbol_graph)
	DependencyContext  []string // 1-hop dependencies for target file(s)

	// ==========================================================================
	// USER INTENT & FOCUS
	// ==========================================================================
	UserIntent       *StructuredIntent // Parsed intent from perception transducer
	FocusResolutions []string          // Resolved paths from fuzzy references

	// ==========================================================================
	// CAMPAIGN CONTEXT (Multi-Phase Goals)
	// ==========================================================================
	CampaignActive     bool     // Whether a campaign is in progress
	CampaignPhase      string   // Current phase name/ID
	CampaignGoal       string   // Current phase objective
	TaskDependencies   []string // What this task depends on (blocking tasks)
	LinkedRequirements []string // Requirements/specs this task fulfills

	// ==========================================================================
	// GIT STATE / CHESTERTON'S FENCE
	// ==========================================================================
	GitBranch        string   // Current branch name
	GitModifiedFiles []string // Uncommitted/modified files
	GitRecentCommits []string // Recent commit messages (for Chesterton's Fence)
	GitUnstagedCount int      // Number of unstaged changes

	// ==========================================================================
	// TEST STATE (TDD LOOP)
	// ==========================================================================
	TestState     string   // /passing, /failing, /pending, /unknown
	FailingTests  []string // Names/paths of failing tests
	TDDRetryCount int      // Current TDD repair loop iteration

	// ==========================================================================
	// CROSS-SHARD EXECUTION HISTORY
	// ==========================================================================
	PriorShardOutputs []ShardSummary // Recent shard executions with summaries

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialists)
	// ==========================================================================
	KnowledgeAtoms  []string // Relevant domain expertise facts
	SpecialistHints []string // Hints from specialist knowledge base

	// ==========================================================================
	// AVAILABLE TOOLS (Autopoiesis/Ouroboros)
	// ==========================================================================
	AvailableTools []ToolInfo // Self-generated tools available for execution

	// ==========================================================================
	// CONSTITUTIONAL CONSTRAINTS
	// ==========================================================================
	AllowedActions []string // Permitted actions for this shard
	BlockedActions []string // Explicitly denied actions
	SafetyWarnings []string // Active safety concerns
}

// ShardConfig holds configuration for a shard.
type ShardConfig struct {
	Name          string
	Type          ShardType
	Permissions   []ShardPermission // Allowed capabilities
	Timeout       time.Duration     // Default execution timeout
	MemoryLimit   int               // Abstract memory unit limit
	Model         ModelConfig       // LLM requirements
	KnowledgePath string            // Path to local knowledge DB (Type B only)

	// Tool associations (for specialist shards)
	Tools           []string          // List of tool names this shard can use
	ToolPreferences map[string]string // Action -> preferred tool mapping

	// Session context (Blackboard Pattern)
	SessionContext *SessionContext // Compressed session context for LLM injection
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
	SetSessionContext(ctx *SessionContext) // For dream mode and session state
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
	virtualStore  *VirtualStore
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

// VirtualStoreConsumer interface for agents that need file system access.
type VirtualStoreConsumer interface {
	SetVirtualStore(vs *VirtualStore)
}

func (sm *ShardManager) SetParentKernel(k Kernel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.kernel = k
}

func (sm *ShardManager) SetVirtualStore(vs *VirtualStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.virtualStore = vs
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
		"perception_firewall":  true,
		"constitution_gate":    true,
		"executive_policy":     true,
		"cost_guard":           true,
		"tactile_router":       true,
		"session_planner":      true,
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

// queryToolsFromKernel queries the Mangle kernel for registered tools.
// Uses predicates: tool_registered, tool_description, tool_binary_path
// This enables dynamic, Mangle-governed tool discovery for agents.
func (sm *ShardManager) queryToolsFromKernel() []ToolInfo {
	if sm.kernel == nil {
		return nil
	}

	// Query all registered tools
	registeredFacts, err := sm.kernel.Query("tool_registered")
	if err != nil || len(registeredFacts) == 0 {
		return nil
	}

	// Build a map of tool names for lookup
	toolNames := make([]string, 0, len(registeredFacts))
	for _, fact := range registeredFacts {
		if len(fact.Args) >= 1 {
			if name, ok := fact.Args[0].(string); ok {
				toolNames = append(toolNames, name)
			}
		}
	}

	if len(toolNames) == 0 {
		return nil
	}

	// Query descriptions and binary paths
	descFacts, _ := sm.kernel.Query("tool_description")
	pathFacts, _ := sm.kernel.Query("tool_binary_path")

	// Build lookup maps
	descriptions := make(map[string]string)
	for _, fact := range descFacts {
		if len(fact.Args) >= 2 {
			if name, ok := fact.Args[0].(string); ok {
				if desc, ok := fact.Args[1].(string); ok {
					descriptions[name] = desc
				}
			}
		}
	}

	binaryPaths := make(map[string]string)
	for _, fact := range pathFacts {
		if len(fact.Args) >= 2 {
			if name, ok := fact.Args[0].(string); ok {
				if path, ok := fact.Args[1].(string); ok {
					binaryPaths[name] = path
				}
			}
		}
	}

	// Build ToolInfo slice
	tools := make([]ToolInfo, 0, len(toolNames))
	for _, name := range toolNames {
		tools = append(tools, ToolInfo{
			Name:        name,
			Description: descriptions[name],
			BinaryPath:  binaryPaths[name],
		})
	}

	return tools
}

// =============================================================================
// INTELLIGENT TOOL ROUTING (§40)
// =============================================================================
// Routes Ouroboros-generated tools to shards based on capabilities, intent,
// domain matching, and usage history.

// ToolRelevanceQuery holds parameters for intelligent tool discovery.
type ToolRelevanceQuery struct {
	ShardType   string // e.g., "coder", "tester", "reviewer", "researcher"
	IntentVerb  string // e.g., "implement", "test", "review", "research"
	TargetFile  string // Target file path (for domain detection)
	TokenBudget int    // Max tokens for tool descriptions (0 = default 2000)
}

// queryRelevantTools queries Mangle for tools relevant to this shard and context.
// Falls back to queryToolsFromKernel if intelligent routing fails.
func (sm *ShardManager) queryRelevantTools(query ToolRelevanceQuery) []ToolInfo {
	if sm.kernel == nil {
		return nil
	}

	// Set up routing context facts
	sm.assertToolRoutingContext(query)

	// Query derived relevant_tool predicate
	// Format: relevant_tool(/shardType, ToolName)
	shardAtom := "/" + query.ShardType
	relevantFacts, err := sm.kernel.Query("relevant_tool")
	if err != nil || len(relevantFacts) == 0 {
		// Fallback to all tools if derivation fails (with budget trimming)
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	// Filter to tools relevant for this shard type
	relevantToolNames := make([]string, 0)
	for _, fact := range relevantFacts {
		if len(fact.Args) >= 2 {
			// Check if shard type matches
			factShardType, _ := fact.Args[0].(string)
			toolName, _ := fact.Args[1].(string)
			if factShardType == shardAtom && toolName != "" {
				relevantToolNames = append(relevantToolNames, toolName)
			}
		}
	}

	if len(relevantToolNames) == 0 {
		// No relevant tools derived - fallback to all (with budget trimming)
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	// Get full tool info for relevant tools
	allTools := sm.queryToolsFromKernel()
	if allTools == nil {
		return nil
	}

	// Filter to only relevant tools
	relevantSet := make(map[string]bool)
	for _, name := range relevantToolNames {
		relevantSet[name] = true
	}

	tools := make([]ToolInfo, 0)
	for _, tool := range allTools {
		if relevantSet[tool.Name] {
			tools = append(tools, tool)
		}
	}

	// Sort by priority score (if available)
	sm.sortToolsByPriority(tools, query.ShardType)

	// Apply token budget trimming
	return sm.trimToTokenBudget(tools, query.TokenBudget)
}

// assertToolRoutingContext sets up Mangle facts for tool relevance derivation.
func (sm *ShardManager) assertToolRoutingContext(query ToolRelevanceQuery) {
	if sm.kernel == nil {
		return
	}

	// Retract old context (avoid stale facts)
	_ = sm.kernel.Retract("current_shard_type")
	_ = sm.kernel.Retract("current_intent")

	// Assert current shard type (with / prefix for Mangle atom)
	shardAtom := "/" + query.ShardType
	_ = sm.kernel.Assert(Fact{
		Predicate: "current_shard_type",
		Args:      []interface{}{shardAtom},
	})

	// Assert current intent if available
	if query.IntentVerb != "" {
		// Create a synthetic intent ID for routing purposes
		intentID := "routing_context"
		verbAtom := "/" + query.IntentVerb
		_ = sm.kernel.Assert(Fact{
			Predicate: "current_intent",
			Args:      []interface{}{intentID},
		})
		// Ensure user_intent fact exists for derivation rules
		_ = sm.kernel.Assert(Fact{
			Predicate: "user_intent",
			Args:      []interface{}{intentID, "/mutation", verbAtom, query.TargetFile, "_"},
		})
	}

	// Assert current time for recency calculations
	// Note: Use int64 for Unix timestamps - Mangle rules compare these against integer expiration times
	_ = sm.kernel.Assert(Fact{
		Predicate: "current_time",
		Args:      []interface{}{int64(time.Now().Unix())},
	})
}

// sortToolsByPriority sorts tools by their Mangle-derived priority score.
func (sm *ShardManager) sortToolsByPriority(tools []ToolInfo, shardType string) {
	if sm.kernel == nil || len(tools) == 0 {
		return
	}

	// Query priority scores
	shardAtom := "/" + shardType
	baseRelevanceFacts, _ := sm.kernel.Query("tool_base_relevance")

	// Build score map
	scores := make(map[string]float64)
	for _, fact := range baseRelevanceFacts {
		if len(fact.Args) >= 3 {
			factShardType, _ := fact.Args[0].(string)
			toolName, _ := fact.Args[1].(string)
			score, _ := fact.Args[2].(float64)
			if factShardType == shardAtom {
				scores[toolName] = score
			}
		}
	}

	// Sort by score descending
	sort.Slice(tools, func(i, j int) bool {
		scoreI := scores[tools[i].Name]
		scoreJ := scores[tools[j].Name]
		return scoreI > scoreJ
	})
}

// trimToTokenBudget limits tools to fit within context window budget.
func (sm *ShardManager) trimToTokenBudget(tools []ToolInfo, budget int) []ToolInfo {
	if budget <= 0 {
		budget = 2000 // Default: ~2000 tokens for tools section
	}

	result := make([]ToolInfo, 0)
	tokensUsed := 0

	for _, tool := range tools {
		// Estimate tokens: name + description + binary path + overhead
		toolTokens := estimateTokens(tool.Name) +
			estimateTokens(tool.Description) +
			estimateTokens(tool.BinaryPath) + 20 // formatting overhead

		if tokensUsed+toolTokens <= budget {
			result = append(result, tool)
			tokensUsed += toolTokens
		} else {
			break // Budget exhausted
		}
	}

	return result
}

// estimateTokens provides rough token estimate (1 token per 4 chars).
func estimateTokens(s string) int {
	return len(s) / 4
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
	Name         string    `json:"name"`
	Type         ShardType `json:"type"`
	Description  string    `json:"description,omitempty"`
	HasKnowledge bool      `json:"has_knowledge"`
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
	return sm.SpawnWithContext(ctx, typeName, task, nil)
}

// SpawnWithContext creates and executes a shard with session context (Blackboard Pattern).
// The sessionCtx provides compressed history and recent findings for cross-shard awareness.
func (sm *ShardManager) SpawnWithContext(ctx context.Context, typeName, task string, sessionCtx *SessionContext) (string, error) {
	id, err := sm.SpawnAsyncWithContext(ctx, typeName, task, sessionCtx)
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
	return sm.SpawnAsyncWithContext(ctx, typeName, task, nil)
}

// SpawnAsyncWithContext creates and executes a shard asynchronously with session context.
func (sm *ShardManager) SpawnAsyncWithContext(ctx context.Context, typeName, task string, sessionCtx *SessionContext) (string, error) {
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

	// Inject session context into config (Blackboard Pattern)
	if sessionCtx != nil {
		config.SessionContext = sessionCtx
	}

	// Populate available tools from Mangle kernel (Intelligent Tool Routing §40)
	// Uses Mangle predicates: tool_registered, tool_description, tool_binary_path,
	// tool_capability, shard_capability_affinity, relevant_tool
	if sm.kernel != nil {
		// Build tool relevance query with shard context
		query := ToolRelevanceQuery{
			ShardType:   typeName,
			TokenBudget: 2000, // Default budget
		}

		// Extract intent verb from session context if available
		if sessionCtx != nil && sessionCtx.UserIntent != nil && sessionCtx.UserIntent.Verb != "" {
			// Strip leading "/" from verb atom if present
			verb := sessionCtx.UserIntent.Verb
			if len(verb) > 0 && verb[0] == '/' {
				verb = verb[1:]
			}
			query.IntentVerb = verb
			query.TargetFile = sessionCtx.UserIntent.Target
		}

		// Use intelligent routing to get relevant tools
		tools := sm.queryRelevantTools(query)
		if len(tools) > 0 {
			if config.SessionContext == nil {
				config.SessionContext = &SessionContext{}
			}
			config.SessionContext.AvailableTools = tools
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
	if sm.virtualStore != nil {
		if vsc, ok := agent.(VirtualStoreConsumer); ok {
			vsc.SetVirtualStore(sm.virtualStore)
		}
	}
	// Inject session context (for dream mode, etc.)
	if sessionCtx != nil {
		agent.SetSessionContext(sessionCtx)
	}

	sm.shards[id] = agent

	// Audit: Shard spawned
	logging.Audit().ShardSpawn(id, typeName)

	// Determine shard category for tracing
	shardCategory := sm.categorizeShardType(typeName, config.Type)

	// Set tracing context before execution
	if sm.tracingClient != nil {
		sm.tracingClient.SetShardContext(id, typeName, shardCategory, sm.sessionID, task)
	}

	// Wrap context with shard metadata for usage tracking
	ctx = usage.WithShardContext(ctx, config.Name, string(config.Type), "current-session") // TODO: pass actual session ID

	// 4. Execute Async
	go func() {
		// Audit: Shard execution started
		logging.Audit().ShardExecute(id, task)
		startTime := time.Now()
		res, err := agent.Execute(ctx, task)
		duration := time.Since(startTime)
		
		// Audit: Shard execution completed
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		logging.Audit().ShardComplete(id, task, duration.Milliseconds(), err == nil, errMsg)
		
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
	sm.mu.Lock()
	defer sm.mu.Unlock()
	res, ok := sm.results[id]
	if ok {
		// clean up to prevent unbounded growth
		delete(sm.results, id)
	}
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
			Args:      []interface{}{name, string(cfg.Type)},
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
			logging.Shards("Failed to start system shard %s: %v", name, err)
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

// =============================================================================
// REVIEWER FEEDBACK INTERFACE (Validation Triggers)
// =============================================================================
// These methods allow the main agent to interact with the reviewer feedback
// system for validating suspect reviews and learning from user feedback.

// ReviewerFeedbackProvider defines the interface for reviewer validation.
// This allows the main agent to check if reviews need validation without
// importing the reviewer package directly.
type ReviewerFeedbackProvider interface {
	NeedsValidation(reviewID string) bool
	GetSuspectReasons(reviewID string) []string
	AcceptFinding(reviewID, file string, line int)
	RejectFinding(reviewID, file string, line int, reason string)
	GetAccuracyReport(reviewID string) string
}

// reviewerFeedbackProvider holds the current reviewer instance if available.
var reviewerFeedbackProvider ReviewerFeedbackProvider

// SetReviewerFeedbackProvider registers a reviewer feedback provider.
// Called by registration.go when creating reviewer shards.
func SetReviewerFeedbackProvider(provider ReviewerFeedbackProvider) {
	reviewerFeedbackProvider = provider
}

// CheckReviewNeedsValidation queries whether a review is suspect.
// Returns true if the review has signs of inaccuracy and should be spot-checked.
func (sm *ShardManager) CheckReviewNeedsValidation(reviewID string) bool {
	if reviewerFeedbackProvider == nil {
		// Fallback: check kernel directly for reviewer_needs_validation
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("reviewer_needs_validation")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 && fact.Args[0] == reviewID {
						return true
					}
				}
			}
		}
		return false
	}
	return reviewerFeedbackProvider.NeedsValidation(reviewID)
}

// GetReviewSuspectReasons returns reasons why a review is flagged as suspect.
func (sm *ShardManager) GetReviewSuspectReasons(reviewID string) []string {
	if reviewerFeedbackProvider == nil {
		// Fallback: check kernel directly
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("review_suspect")
			if err == nil {
				var reasons []string
				for _, fact := range facts {
					if len(fact.Args) >= 2 && fact.Args[0] == reviewID {
						if reason, ok := fact.Args[1].(string); ok {
							reasons = append(reasons, reason)
						}
					}
				}
				return reasons
			}
		}
		return nil
	}
	return reviewerFeedbackProvider.GetSuspectReasons(reviewID)
}

// AcceptReviewFinding marks a finding as accepted by the user.
func (sm *ShardManager) AcceptReviewFinding(reviewID, file string, line int) {
	if reviewerFeedbackProvider != nil {
		reviewerFeedbackProvider.AcceptFinding(reviewID, file, line)
	}
}

// RejectReviewFinding marks a finding as rejected by the user.
// The reason helps the system learn from the rejection.
func (sm *ShardManager) RejectReviewFinding(reviewID, file string, line int, reason string) {
	if reviewerFeedbackProvider != nil {
		reviewerFeedbackProvider.RejectFinding(reviewID, file, line, reason)
	}
}

// GetReviewAccuracyReport returns accuracy statistics for a review session.
func (sm *ShardManager) GetReviewAccuracyReport(reviewID string) string {
	if reviewerFeedbackProvider == nil {
		return "Review feedback provider not available"
	}
	return reviewerFeedbackProvider.GetAccuracyReport(reviewID)
}
