package perception

// Understanding represents the LLM's interpretation of user intent.
// This is the output of LLM-first classification - the LLM tells us
// what it understands, and the harness uses this to route appropriately.
//
// Philosophy: LLM describes → Harness determines
type Understanding struct {
	// --- Core Understanding (from LLM) ---

	// PrimaryIntent is a one-word summary of what the user wants.
	// Examples: "debug", "implement", "explain", "refactor"
	PrimaryIntent string `json:"primary_intent"`

	// SemanticType describes HOW the user is asking.
	// Examples: causation, mechanism, location, temporal, hypothetical
	SemanticType string `json:"semantic_type"`

	// ActionType describes WHAT the user wants done.
	// Examples: investigate, implement, modify, verify, explain, research
	ActionType string `json:"action_type"`

	// Domain describes the AREA of concern.
	// Examples: testing, security, performance, git, architecture
	Domain string `json:"domain"`

	// Scope describes HOW MUCH of the codebase is involved.
	Scope Scope `json:"scope"`

	// UserConstraints are explicit limitations the user stated.
	// Examples: "don't break tests", "keep it simple", "no external deps"
	UserConstraints []string `json:"user_constraints"`

	// ImplicitAssumptions are things the user assumes but didn't say.
	// Examples: "test was passing before", "using existing patterns"
	ImplicitAssumptions []string `json:"implicit_assumptions"`

	// Confidence is the LLM's self-assessed confidence in this understanding.
	// Range: 0.0 to 1.0
	Confidence float64 `json:"confidence"`

	// --- Signal Flags (from LLM) ---
	Signals Signals `json:"signals"`

	// --- Suggested Approach (from LLM) ---
	SuggestedApproach SuggestedApproach `json:"suggested_approach"`

	// --- Derived Routing (from Harness) ---
	// These are populated by the harness after validation, using Mangle rules.
	Routing *Routing `json:"routing,omitempty"`

	// --- Raw Surface Response (from LLM) ---
	// The natural language response to show the user.
	SurfaceResponse string `json:"surface_response"`
}

// Scope represents the breadth of the user's request.
type Scope struct {
	// Level is the granularity: line, function, file, package, module, codebase
	Level string `json:"level"`

	// Target is the specific target within that level.
	// Examples: "validateToken", "auth.go", "internal/auth"
	Target string `json:"target"`

	// File is the specific file path if known.
	File string `json:"file,omitempty"`

	// Symbol is the specific symbol (function, type, var) if known.
	Symbol string `json:"symbol,omitempty"`
}

// Signals are boolean flags about the nature of the request.
type Signals struct {
	// IsQuestion is true if the user is asking a question, not requesting action.
	IsQuestion bool `json:"is_question"`

	// IsHypothetical is true for "what if" / simulation requests.
	IsHypothetical bool `json:"is_hypothetical"`

	// IsMultiStep is true if this requires multiple distinct phases.
	IsMultiStep bool `json:"is_multi_step"`

	// IsNegated is true if the user said NOT to do something.
	IsNegated bool `json:"is_negated"`

	// RequiresConfirmation is true if the user wants approval before action.
	RequiresConfirmation bool `json:"requires_confirmation"`

	// Urgency indicates priority: low, normal, high, critical
	Urgency string `json:"urgency"`
}

// SuggestedApproach is the LLM's recommendation for how to proceed.
// The harness validates and may override these suggestions.
type SuggestedApproach struct {
	// Mode is the suggested harness mode: normal, tdd, dream, debug, etc.
	Mode string `json:"mode"`

	// PrimaryShard is the suggested primary agent: coder, tester, reviewer, etc.
	PrimaryShard string `json:"primary_shard"`

	// SupportingShards are additional agents that may help.
	SupportingShards []string `json:"supporting_shards,omitempty"`

	// ToolsNeeded are tools the LLM thinks it will need.
	ToolsNeeded []string `json:"tools_needed,omitempty"`

	// ContextNeeded describes what context would help.
	// Examples: "test_output", "function_source", "git_history"
	ContextNeeded []string `json:"context_needed,omitempty"`
}

// Routing is derived by the harness from the Understanding using Mangle rules.
// This represents the harness's decision about how to fulfill the intent.
type Routing struct {
	// Mode is the determined harness mode (may differ from suggested).
	Mode string `json:"mode"`

	// PrimaryShard is the determined primary agent.
	PrimaryShard string `json:"primary_shard"`

	// SupportingShards are determined supporting agents.
	SupportingShards []string `json:"supporting_shards,omitempty"`

	// ContextPriorities maps context categories to priority weights.
	// Higher weight = more important to include in context window.
	ContextPriorities map[string]int `json:"context_priorities,omitempty"`

	// ToolPriorities maps tools to priority weights.
	// Higher weight = more likely to be useful.
	ToolPriorities map[string]int `json:"tool_priorities,omitempty"`

	// BlockedTools are tools that cannot be used (due to constraints).
	BlockedTools []string `json:"blocked_tools,omitempty"`

	// RequiredValidations are checks that must pass before/after execution.
	RequiredValidations []string `json:"required_validations,omitempty"`
}

// UnderstandingEnvelope is the full control packet from LLM classification.
// This replaces the old PiggybackEnvelope for intent classification.
type UnderstandingEnvelope struct {
	// Understanding is the LLM's interpretation of user intent.
	Understanding Understanding `json:"understanding"`

	// SurfaceResponse is the natural language response for the user.
	SurfaceResponse string `json:"surface_response"`
}

// Validate checks that the Understanding contains valid values.
// Returns nil if valid, or an error describing what's invalid.
func (u *Understanding) Validate() error {
	// Validation will be implemented to check against Mangle vocabulary
	// For now, basic presence checks
	if u.PrimaryIntent == "" {
		return &ValidationError{Field: "primary_intent", Message: "required"}
	}
	if u.SemanticType == "" {
		return &ValidationError{Field: "semantic_type", Message: "required"}
	}
	if u.ActionType == "" {
		return &ValidationError{Field: "action_type", Message: "required"}
	}
	if u.Domain == "" {
		return &ValidationError{Field: "domain", Message: "required"}
	}
	if u.Confidence < 0 || u.Confidence > 1 {
		return &ValidationError{Field: "confidence", Message: "must be between 0 and 1"}
	}
	return nil
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error: " + e.Field + " " + e.Message
}

// IsActionRequest returns true if this understanding represents a request
// for the agent to DO something (vs. just explain or answer a question).
func (u *Understanding) IsActionRequest() bool {
	switch u.ActionType {
	case "implement", "modify", "refactor", "verify", "attack", "revert", "configure":
		return true
	default:
		return false
	}
}

// IsReadOnly returns true if this understanding should not modify any files.
func (u *Understanding) IsReadOnly() bool {
	if u.Signals.IsHypothetical {
		return true
	}
	for _, c := range u.UserConstraints {
		if c == "no_changes" || c == "read_only" || c == "dry_run" {
			return true
		}
	}
	switch u.ActionType {
	case "investigate", "explain", "research", "review":
		return true
	default:
		return false
	}
}

// NeedsConfirmation returns true if the harness should confirm before executing.
func (u *Understanding) NeedsConfirmation() bool {
	if u.Signals.RequiresConfirmation {
		return true
	}
	// High-risk actions need confirmation
	switch u.ActionType {
	case "revert", "attack":
		return true
	}
	// Codebase-wide scope needs confirmation
	if u.Scope.Level == "codebase" || u.Scope.Level == "module" {
		return true
	}
	return false
}
