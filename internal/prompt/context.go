package prompt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"sync"
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

	// HasReflectionHits indicates System 2 reflection recall is present
	HasReflectionHits bool

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

	// ReservedTokensFallbackRatio is the ratio used to calculate reserved tokens when budget is exceeded
	ReservedTokensFallbackRatio int

	// =========================================================================
	// Semantic Search
	// =========================================================================

	// SemanticQuery is the query text for vector-based atom retrieval
	SemanticQuery string

	// SemanticTopK is the number of semantic results to consider
	SemanticTopK int

	// VectorWeight is the weight of vector scores in combined calculation (0.0-1.0)
	// If HasVectorWeight is true, this overrides the AtomSelector's default vector weight.
	VectorWeight float64

	// HasVectorWeight indicates if VectorWeight is explicitly set for this context
	HasVectorWeight bool

	// =========================================================================
	// External References (opaque to avoid circular imports)
	// =========================================================================

	// SessionContext holds the current session context
	// Type: *core.SessionContext
	SessionContext any

	// UserIntent holds the parsed user intent
	// Type: *core.StructuredIntent
	UserIntent any

	// Kernel holds a reference to the Mangle kernel for queries
	// Type: *core.RealKernel
	Kernel any

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

	// =========================================================================
	// Dynamic Capability Injection
	// =========================================================================

	// AvailableSpecialists is a formatted list of domain specialists available
	// for knowledge consultation. Populated at runtime from .nerd/agents.json.
	// Used by the capability/knowledge_discovery atom template.
	AvailableSpecialists string

	// AvailableTools is the runtime-resolved list of tool names this turn is
	// permitted to invoke (e.g. ["write_file", "edit_file", "run_command"]).
	// Populated from EffectiveAgentRuntimeConfig.AllowedTools so prompt atoms
	// can reference the actually-allowed surface via {{available_tools}}
	// rather than hardcoding tool names in Go strings or atom YAML.
	AvailableTools []string

	// PreviousAttemptNoToolCall is set by the session executor when the model's
	// previous turn produced narrative-only text for an intent that the kernel
	// derived intent_requires_tool_call/1 for. Activates the
	// world_state "no_tool_call_retry" so the JIT selects the
	// system/tool_nudge/no_tool_call_retry atom.
	PreviousAttemptNoToolCall bool
}

// NewCompilationContext creates a new CompilationContext with defaults.
// Note: TokenBudget should be overridden by callers from config.ContextWindow.MaxTokens.
func NewCompilationContext() *CompilationContext {
	return &CompilationContext{
		OperationalMode:             "/active",
		TokenBudget:                 200000, // 200k tokens default - callers should override from config
		ReservedTokens:              8000,   // Reserve 8k for response
		ReservedTokensFallbackRatio: 10,     // Default 10 fallback ratio
		SemanticTopK:                20,     // Top 20 semantic results
		ActivationThreshold:         0.5,    // Default activation threshold
	}
}

// NewCompilationContextWithBudget creates a new CompilationContext with a specific token budget.
// Use this when creating compilation context with config.ContextWindow.MaxTokens.
func NewCompilationContextWithBudget(tokenBudget int) *CompilationContext {
	cc := NewCompilationContext()
	if tokenBudget > 0 {
		cc.TokenBudget = tokenBudget
	}
	return cc
}

// WorldStates returns the world state strings for atom matching.
// These are derived from the boolean/numeric world model fields.
func (cc *CompilationContext) WorldStates() []string {
	// Optimization: Pre-allocate with max possible capacity (8) to avoid reallocations
	states := make([]string, 0, 8)

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

	if cc.HasReflectionHits {
		states = append(states, "reflection_hits")
	}

	if cc.PreviousAttemptNoToolCall {
		states = append(states, "no_tool_call_retry")
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

// WithVectorWeight sets the vector weight and returns the context.
func (cc *CompilationContext) WithVectorWeight(weight float64) *CompilationContext {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	cc.VectorWeight = weight
	cc.HasVectorWeight = true
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
	if cc.AvailableTools != nil {
		clone.AvailableTools = make([]string, len(cc.AvailableTools))
		copy(clone.AvailableTools, cc.AvailableTools)
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
		"CompilationContext{mode=%s, campaign=%s, campaign_id=%s, shard=%s, lang=%s, intent=%s, budget=%d, world_states=%v}",
		cc.OperationalMode,
		cc.CampaignPhase,
		cc.CampaignID,
		cc.ShardType,
		cc.Language,
		cc.IntentVerb,
		cc.AvailableTokens(),
		cc.WorldStates(),
	)
}

// ToContextFacts generates Mangle facts representing this context.
// These facts are formatted for the compile_context(Dimension, Value) schema
// as declared in schemas.mg Section 45 and used by policy.mg for atom selection.

func (cc *CompilationContext) ToContextFacts() []any {
	return cc.GenerateFacts(FactStyle{
		Predicate:  "compile_context",
		UseShort:   false,
		ForceAtoms: false,
		AddDot:     true,
	})
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
			Values:      []string{"failing_tests", "diagnostics", "large_refactor", "security_issues", "new_files", "high_churn", "reflection_hits", "no_tool_call_retry"},
		},
	}
}

// hashBufferPool is a sync.Pool for bytes.Buffer used in CompilationContext.Hash
// to reduce GC pressure during high-frequency compilation requests.
var hashBufferPool = sync.Pool{
	New: func() any {
		buf := new(bytes.Buffer)
		buf.Grow(256)
		return buf
	},
}

// Hash generates a stable, versioned cache identity for every field that can
// affect atom collection, selection, budgeting, or rendering. Set-like slices
// are sorted and deduplicated without mutating the caller's context.
func (cc *CompilationContext) Hash() string {
	if cc == nil {
		return "nil"
	}

	// Optimization: Use bytes.Buffer instead of strings.Builder to avoid
	// string->[]byte allocation when passing to sha256.Sum256.
	buf := hashBufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		hashBufferPool.Put(buf)
	}()

	// Length-prefix each value so user-controlled strings cannot create
	// delimiter collisions (for example, ["a,b", "c"] vs ["a", "b,c"]).
	write := func(name, value string) {
		buf.WriteString(name)
		buf.WriteByte('=')
		buf.WriteString(strconv.Itoa(len(value)))
		buf.WriteByte(':')
		buf.WriteString(value)
		buf.WriteByte(';')
	}
	writeInt := func(name string, value int) {
		write(name, strconv.Itoa(value))
	}
	writeBool := func(name string, value bool) {
		write(name, strconv.FormatBool(value))
	}
	writeFloat := func(name string, value float64) {
		write(name, strconv.FormatFloat(value, 'g', -1, 64))
	}
	writeSet := func(name string, values []string) {
		canonical := canonicalStringSet(values)
		writeInt(name+"_count", len(canonical))
		for i, value := range canonical {
			write(name+"_"+strconv.Itoa(i), value)
		}
	}

	write("schema", "compilation-context-v2")
	write("operational_mode", cc.OperationalMode)
	write("campaign_phase", cc.CampaignPhase)
	write("campaign_id", cc.CampaignID)
	write("campaign_name", cc.CampaignName)
	write("build_layer", cc.BuildLayer)
	write("init_phase", cc.InitPhase)
	write("northstar_phase", cc.NorthstarPhase)
	write("ouroboros_stage", cc.OuroborosStage)
	write("intent_verb", cc.IntentVerb)
	write("intent_target", cc.IntentTarget)
	write("shard_type", cc.ShardType)
	write("shard_id", cc.ShardID)
	write("shard_instance_id", cc.ShardInstanceID)
	write("shard_name", cc.ShardName)
	write("language", cc.Language)
	writeSet("framework", cc.Frameworks)

	writeInt("failing_test_count", cc.FailingTestCount)
	writeInt("diagnostic_count", cc.DiagnosticCount)
	writeBool("large_refactor", cc.IsLargeRefactor)
	writeBool("security_issues", cc.HasSecurityIssues)
	writeBool("new_files", cc.HasNewFiles)
	writeBool("high_churn", cc.IsHighChurn)
	writeBool("reflection_hits", cc.HasReflectionHits)
	writeBool("previous_attempt_no_tool_call", cc.PreviousAttemptNoToolCall)

	writeInt("token_budget", cc.TokenBudget)
	writeInt("reserved_tokens", cc.ReservedTokens)
	writeInt("reserved_tokens_fallback_ratio", cc.ReservedTokensFallbackRatio)
	write("semantic_query", cc.SemanticQuery)
	writeInt("semantic_top_k", cc.SemanticTopK)
	if cc.HasVectorWeight {
		writeFloat("vector_weight", cc.VectorWeight)
	}
	writeFloat("activation_threshold", cc.ActivationThreshold)
	write("available_specialists", cc.AvailableSpecialists)
	writeSet("available_tool", cc.AvailableTools)

	// Hash the content
	hash := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(hash[:])
}

func canonicalStringSet(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	canonical := append([]string(nil), values...)
	sort.Strings(canonical)
	result := canonical[:0]
	for _, value := range canonical {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

// FactStyle defines the formatting strategy for context facts.
type FactStyle struct {
	Predicate  string
	UseShort   bool
	ForceAtoms bool
	AddDot     bool
}

func (cc *CompilationContext) GenerateFacts(style FactStyle) []any {
	capacity := 9 + len(cc.Frameworks) + 7
	facts := make([]any, 0, capacity)

	var fb factBuilder

	add := func(longDim, shortDim, val string) {
		if val == "" {
			return
		}

		dim := longDim
		if style.UseShort {
			dim = shortDim
		}

		fb.Reset()

		fb.WriteString(style.Predicate)
		fb.WriteByte('(')

		// Dimension is always an atom
		if !fb.writeAtom(dim) {
			return
		}

		fb.WriteString(", ")

		if style.ForceAtoms {
			if !fb.writeAtom(val) {
				return
			}
		} else {
			if len(val) > 0 && val[0] == '/' {
				if !fb.writeAtom(val) {
					return
				}
			} else {
				// Quote as string
				fb.writeStringLiteral(val)
			}
		}

		fb.WriteByte(')')
		if style.AddDot {
			fb.WriteByte('.')
		}
		facts = append(facts, fb.String())
	}

	add("operational_mode", "mode", cc.OperationalMode)
	add("campaign_phase", "phase", cc.CampaignPhase)
	add("build_layer", "layer", cc.BuildLayer)
	add("init_phase", "init_phase", cc.InitPhase)
	add("northstar_phase", "northstar_phase", cc.NorthstarPhase)
	add("ouroboros_stage", "ouroboros_stage", cc.OuroborosStage)
	add("intent_verb", "intent", cc.IntentVerb)
	add("shard_type", "shard", cc.ShardType)
	add("language", "lang", cc.Language)

	for _, fw := range cc.Frameworks {
		add("framework", "framework", fw)
	}

	if cc.FailingTestCount > 0 {
		add("world_state", "state", "failing_tests")
	}
	if cc.DiagnosticCount > 0 {
		add("world_state", "state", "diagnostics")
	}
	if cc.IsLargeRefactor {
		add("world_state", "state", "large_refactor")
	}
	if cc.HasSecurityIssues {
		add("world_state", "state", "security_issues")
	}
	if cc.HasNewFiles {
		add("world_state", "state", "new_files")
	}
	if cc.IsHighChurn {
		add("world_state", "state", "high_churn")
	}
	if cc.HasReflectionHits {
		add("world_state", "state", "reflection_hits")
	}
	if cc.PreviousAttemptNoToolCall {
		add("world_state", "state", "no_tool_call_retry")
	}

	return facts
}
