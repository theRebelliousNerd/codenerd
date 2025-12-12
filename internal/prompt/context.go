package prompt

import (
	"fmt"
)

// CompilationContext holds all dimensions for prompt atom selection.
// This structure captures the 10 contextual tiers that determine
// which atoms should be included in a compiled prompt.
//
// The context flows from the kernel's current state and is used
// to select the most relevant prompt atoms for the current situation.
type CompilationContext struct {
	// =========================================================================
	// Tier 1: Operational Mode
	// The high-level mode the agent is operating in.
	// =========================================================================

	// OperationalMode: /active, /dream, /debugging, /creative, /scaffolding, /shadow, /tdd_repair
	OperationalMode string

	// =========================================================================
	// Tier 2: Campaign Phase
	// Multi-phase goal orchestration state.
	// =========================================================================

	// CampaignPhase: /planning, /decomposing, /validating, /active, /completed, /paused, /failed
	CampaignPhase string

	// CampaignID is the unique identifier for the active campaign
	CampaignID string

	// CampaignName is the human-readable name of the campaign
	CampaignName string

	// =========================================================================
	// Tier 3: Build Taxonomy
	// The architectural layer being worked on.
	// =========================================================================

	// BuildLayer: /scaffold, /domain_core, /data_layer, /service, /transport, /integration
	BuildLayer string

	// =========================================================================
	// Tier 4: Init Phase
	// Project initialization phase (for /init command).
	// =========================================================================

	// InitPhase: /migration, /setup, /scanning, /analysis, /profile, /facts, /agents, /kb_agent, /kb_complete
	InitPhase string

	// =========================================================================
	// Tier 5: Northstar Phase
	// High-level planning/vision phase.
	// =========================================================================

	// NorthstarPhase: /doc_ingestion, /problem, /vision, /requirements, /architecture, /roadmap, /validation
	NorthstarPhase string

	// =========================================================================
	// Tier 6: Ouroboros Stage
	// Self-improvement/tool-generation stage.
	// =========================================================================

	// OuroborosStage: /detection, /specification, /safety_check, /simulation, /codegen, /testing, /deployment
	OuroborosStage string

	// =========================================================================
	// Tier 7: Intent Verb
	// The type of action being performed.
	// =========================================================================

	// IntentVerb: /fix, /debug, /refactor, /test, /review, /create, /research, /explain
	IntentVerb string

	// IntentTarget is what the intent is operating on (file, function, etc.)
	IntentTarget string

	// =========================================================================
	// Tier 8: Shard Type
	// The type of shard being configured.
	// =========================================================================

	// ShardType: /coder, /tester, /reviewer, /researcher, /librarian, /planner, /custom
	ShardType string

	// ShardID is the unique identifier for the shard
	ShardID string

	// ShardInstanceID is the unique identifier for this shard instance (may be ephemeral).
	// Example: "coder-123", "campaign_abc_planner".
	// This is NOT used for shard DB lookup; ShardID should be the stable agent name.
	ShardInstanceID string

	// ShardName is the human-readable name of the shard
	ShardName string

	// =========================================================================
	// Tier 9: World Model State
	// Current state of the codebase/environment.
	// =========================================================================

	// FailingTestCount is the number of currently failing tests
	FailingTestCount int

	// DiagnosticCount is the number of active diagnostics/errors
	DiagnosticCount int

	// IsLargeRefactor indicates a large-scale refactoring operation
	IsLargeRefactor bool

	// HasSecurityIssues indicates security vulnerabilities detected
	HasSecurityIssues bool

	// HasNewFiles indicates new files have been created this session
	HasNewFiles bool

	// IsHighChurn indicates high file modification frequency
	IsHighChurn bool

	// =========================================================================
	// Tier 10: Language & Framework
	// Technology stack context.
	// =========================================================================

	// Language: /go, /python, /typescript, /rust, /java, /javascript, /mangle
	Language string

	// Frameworks: [/bubbletea, /gin], etc.
	Frameworks []string

	// =========================================================================
	// Budget Configuration
	// =========================================================================

	// TokenBudget is the maximum tokens allowed for the compiled prompt
	TokenBudget int

	// ReservedTokens is tokens reserved for response/output
	ReservedTokens int

	// =========================================================================
	// Semantic Search
	// =========================================================================

	// SemanticQuery is the query text for vector-based atom retrieval
	SemanticQuery string

	// SemanticTopK is the number of semantic results to consider
	SemanticTopK int

	// =========================================================================
	// External References (opaque to avoid circular imports)
	// =========================================================================

	// SessionContext holds the current session context
	// Type: *core.SessionContext
	SessionContext interface{}

	// UserIntent holds the parsed user intent
	// Type: *core.StructuredIntent
	UserIntent interface{}

	// Kernel holds a reference to the Mangle kernel for queries
	// Type: *core.RealKernel
	Kernel interface{}

	// =========================================================================
	// Activation Scores (from Compression System)
	// =========================================================================

	// ActivatedFacts maps fact string representation to activation score (0.0-1.0).
	// Used to boost atoms related to highly-activated facts.
	// Populated by the compression system's GetActivationScores().
	ActivatedFacts map[string]float64

	// ActivationThreshold is the minimum score for a fact to be considered "hot".
	// Default: 0.5
	ActivationThreshold float64
}

// NewCompilationContext creates a new CompilationContext with defaults.
func NewCompilationContext() *CompilationContext {
	return &CompilationContext{
		OperationalMode:     "/active",
		TokenBudget:         100000, // Default 100k tokens
		ReservedTokens:      8000,   // Reserve 8k for response
		SemanticTopK:        20,     // Top 20 semantic results
		ActivationThreshold: 0.5,    // Default activation threshold
	}
}

// WorldStates returns the world state strings for atom matching.
// These are derived from the boolean/numeric world model fields.
func (cc *CompilationContext) WorldStates() []string {
	var states []string

	if cc.FailingTestCount > 0 {
		states = append(states, "failing_tests")
	}

	if cc.DiagnosticCount > 0 {
		states = append(states, "diagnostics")
	}

	if cc.IsLargeRefactor {
		states = append(states, "large_refactor")
	}

	if cc.HasSecurityIssues {
		states = append(states, "security_issues")
	}

	if cc.HasNewFiles {
		states = append(states, "new_files")
	}

	if cc.IsHighChurn {
		states = append(states, "high_churn")
	}

	return states
}

// AvailableTokens returns the tokens available for prompt content.
func (cc *CompilationContext) AvailableTokens() int {
	available := cc.TokenBudget - cc.ReservedTokens
	if available < 0 {
		return 0
	}
	return available
}

// WithOperationalMode sets the operational mode and returns the context.
func (cc *CompilationContext) WithOperationalMode(mode string) *CompilationContext {
	cc.OperationalMode = mode
	return cc
}

// WithCampaign sets campaign context and returns the context.
func (cc *CompilationContext) WithCampaign(id, name, phase string) *CompilationContext {
	cc.CampaignID = id
	cc.CampaignName = name
	cc.CampaignPhase = phase
	return cc
}

// WithShard sets shard context and returns the context.
func (cc *CompilationContext) WithShard(shardType, shardID, shardName string) *CompilationContext {
	cc.ShardType = shardType
	cc.ShardID = shardID
	cc.ShardName = shardName
	return cc
}

// WithLanguage sets language context and returns the context.
func (cc *CompilationContext) WithLanguage(language string, frameworks ...string) *CompilationContext {
	cc.Language = language
	cc.Frameworks = frameworks
	return cc
}

// WithIntent sets intent context and returns the context.
func (cc *CompilationContext) WithIntent(verb, target string) *CompilationContext {
	cc.IntentVerb = verb
	cc.IntentTarget = target
	return cc
}

// WithTokenBudget sets the token budget and returns the context.
func (cc *CompilationContext) WithTokenBudget(budget, reserved int) *CompilationContext {
	cc.TokenBudget = budget
	cc.ReservedTokens = reserved
	return cc
}

// WithSemanticQuery sets the semantic search query and returns the context.
func (cc *CompilationContext) WithSemanticQuery(query string, topK int) *CompilationContext {
	cc.SemanticQuery = query
	if topK > 0 {
		cc.SemanticTopK = topK
	}
	return cc
}

// Clone creates a deep copy of the compilation context.
func (cc *CompilationContext) Clone() *CompilationContext {
	clone := *cc

	// Deep copy slices
	if cc.Frameworks != nil {
		clone.Frameworks = make([]string, len(cc.Frameworks))
		copy(clone.Frameworks, cc.Frameworks)
	}

	return &clone
}

// Validate checks the context for consistency.
func (cc *CompilationContext) Validate() error {
	if cc.TokenBudget <= 0 {
		return fmt.Errorf("token budget must be positive, got %d", cc.TokenBudget)
	}

	if cc.ReservedTokens < 0 {
		return fmt.Errorf("reserved tokens cannot be negative, got %d", cc.ReservedTokens)
	}

	if cc.ReservedTokens >= cc.TokenBudget {
		return fmt.Errorf("reserved tokens (%d) must be less than budget (%d)",
			cc.ReservedTokens, cc.TokenBudget)
	}

	return nil
}

// String returns a human-readable summary of the context.
func (cc *CompilationContext) String() string {
	return fmt.Sprintf(
		"CompilationContext{mode=%s, campaign=%s, shard=%s, lang=%s, intent=%s, budget=%d}",
		cc.OperationalMode,
		cc.CampaignPhase,
		cc.ShardType,
		cc.Language,
		cc.IntentVerb,
		cc.AvailableTokens(),
	)
}

// ToContextFacts generates Mangle facts representing this context.
// These facts are formatted for the compile_context(Dimension, Value) schema
// as declared in schemas.mg Section 45 and used by policy.mg for atom selection.
func (cc *CompilationContext) ToContextFacts() []string {
	var facts []string

	// Helper to add compile_context facts for non-empty values.
	// Format: compile_context(/dimension, /value). or compile_context(/dimension, "string").
	addFact := func(dimension, value string) {
		if value == "" {
			return
		}
		// Ensure dimension and value start with / for atom constants
		if !hasPrefix(dimension, "/") {
			dimension = "/" + dimension
		}
		// Values that look like name constants (start with /) stay as-is
		// Others get quoted as strings
		if hasPrefix(value, "/") {
			facts = append(facts, fmt.Sprintf("compile_context(%s, %s).", dimension, value))
		} else {
			facts = append(facts, fmt.Sprintf("compile_context(%s, \"%s\").", dimension, value))
		}
	}

	// Core context dimensions (per schemas.mg Section 45)
	addFact("operational_mode", cc.OperationalMode)
	addFact("campaign_phase", cc.CampaignPhase)
	addFact("build_layer", cc.BuildLayer)
	addFact("init_phase", cc.InitPhase)
	addFact("northstar_phase", cc.NorthstarPhase)
	addFact("ouroboros_stage", cc.OuroborosStage)
	addFact("intent_verb", cc.IntentVerb)
	addFact("shard_type", cc.ShardType)
	addFact("language", cc.Language)

	// Multi-value dimensions
	for _, fw := range cc.Frameworks {
		addFact("framework", fw)
	}

	for _, ws := range cc.WorldStates() {
		addFact("world_state", ws)
	}

	return facts
}

// hasPrefix checks if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ContextDimension represents a single dimension of context.
type ContextDimension struct {
	Name        string
	Description string
	Values      []string // Possible values for this dimension
}

// AllContextDimensions returns all context dimensions with their possible values.
func AllContextDimensions() []ContextDimension {
	return []ContextDimension{
		{
			Name:        "operational_mode",
			Description: "High-level operational mode",
			Values:      []string{"/active", "/dream", "/debugging", "/creative", "/scaffolding", "/shadow", "/tdd_repair"},
		},
		{
			Name:        "campaign_phase",
			Description: "Campaign orchestration phase",
			Values:      []string{"/planning", "/decomposing", "/validating", "/active", "/completed", "/paused", "/failed"},
		},
		{
			Name:        "build_layer",
			Description: "Build taxonomy layer",
			Values:      []string{"/scaffold", "/domain_core", "/data_layer", "/service", "/transport", "/integration"},
		},
		{
			Name:        "init_phase",
			Description: "Project initialization phase",
			Values:      []string{"/migration", "/setup", "/scanning", "/analysis", "/profile", "/facts", "/agents", "/kb_agent", "/kb_complete"},
		},
		{
			Name:        "northstar_phase",
			Description: "Northstar planning phase",
			Values:      []string{"/doc_ingestion", "/problem", "/vision", "/requirements", "/architecture", "/roadmap", "/validation"},
		},
		{
			Name:        "ouroboros_stage",
			Description: "Ouroboros self-improvement stage",
			Values:      []string{"/detection", "/specification", "/safety_check", "/simulation", "/codegen", "/testing", "/deployment"},
		},
		{
			Name:        "intent_verb",
			Description: "User intent action type",
			Values:      []string{"/fix", "/debug", "/refactor", "/test", "/review", "/create", "/research", "/explain"},
		},
		{
			Name:        "shard_type",
			Description: "Shard agent type",
			Values:      []string{"/coder", "/tester", "/reviewer", "/researcher", "/librarian", "/planner", "/custom"},
		},
		{
			Name:        "language",
			Description: "Programming language",
			Values:      []string{"/go", "/python", "/typescript", "/rust", "/java", "/javascript", "/mangle"},
		},
		{
			Name:        "world_state",
			Description: "World model state indicators",
			Values:      []string{"failing_tests", "diagnostics", "large_refactor", "security_issues", "new_files", "high_churn"},
		},
	}
}
