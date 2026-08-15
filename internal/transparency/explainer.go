package transparency

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/mangle"
)

// Explainer builds human-readable explanations from derivation traces.
// This provides the "why" behind codeNERD's decisions in user-friendly format.
type Explainer struct {
	maxDepth    int
	showDetails bool
}

// NewExplainer creates a new explainer with default settings.
func NewExplainer() *Explainer {
	return &Explainer{
		maxDepth:    5,
		showDetails: true,
	}
}

// SetMaxDepth configures the maximum depth of explanation.
func (e *Explainer) SetMaxDepth(depth int) {
	e.maxDepth = depth
}

// SetShowDetails configures whether to show technical details.
func (e *Explainer) SetShowDetails(show bool) {
	e.showDetails = show
}

// ExplainTrace generates a human-readable explanation from a derivation trace.
func (e *Explainer) ExplainTrace(trace *mangle.DerivationTrace) string {
	if trace == nil || len(trace.RootNodes) == 0 {
		return "No derivation found for this query."
	}

	var sb strings.Builder

	// Header
	sb.WriteString("## Explanation\n\n")

	// Query summary
	sb.WriteString(fmt.Sprintf("**Query**: `%s`\n\n", trace.Query))

	// For each root derivation
	for i, root := range trace.RootNodes {
		if len(trace.RootNodes) > 1 {
			sb.WriteString(fmt.Sprintf("### Result %d\n\n", i+1))
		}

		// Explain this derivation tree
		e.explainNode(&sb, root, 0)
		sb.WriteString("\n")
	}

	// Summary
	if e.showDetails {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("*%d facts examined in %v*\n", len(trace.AllNodes), trace.Duration))
	}

	return sb.String()
}

// explainNode recursively explains a derivation node.
func (e *Explainer) explainNode(sb *strings.Builder, node *mangle.DerivationNode, depth int) {
	if depth > e.maxDepth {
		sb.WriteString(strings.Repeat("  ", depth))
		sb.WriteString("*... (more premises omitted)*\n")
		return
	}

	indent := strings.Repeat("  ", depth)

	// Format the fact nicely
	factStr := formatFactForHuman(node.Fact)

	if node.Source == mangle.SourceEDB {
		// Base fact (EDB) - explain as observed truth
		sb.WriteString(fmt.Sprintf("%s- `%s` **(base fact)**\n", indent, factStr))
	} else {
		// Derived fact (IDB) - explain the rule that produced it
		ruleExplanation := explainRule(node.RuleName)
		sb.WriteString(fmt.Sprintf("%s- `%s`\n", indent, factStr))
		if ruleExplanation != "" {
			sb.WriteString(fmt.Sprintf("%s  *derived via %s*\n", indent, ruleExplanation))
		}

		// Show premises (children)
		if len(node.Children) > 0 {
			sb.WriteString(fmt.Sprintf("%s  **Because:**\n", indent))
			for _, child := range node.Children {
				e.explainNode(sb, child, depth+1)
			}
		}
	}
}

// formatFactForHuman converts a fact to a human-readable string.
func formatFactForHuman(fact mangle.Fact) string {
	if len(fact.Args) == 0 {
		return fact.Predicate
	}

	args := make([]string, len(fact.Args))
	for i, arg := range fact.Args {
		args[i] = formatArg(arg)
	}

	return fmt.Sprintf("%s(%s)", fact.Predicate, strings.Join(args, ", "))
}

// formatArg formats a single argument for human readability.
func formatArg(arg any) string {
	switch v := arg.(type) {
	case string:
		// Remove atom prefix if present for cleaner display
		if strings.HasPrefix(v, "/") {
			return v
		}
		return fmt.Sprintf("\"%s\"", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ruleGlossary maps a Mangle rule name to English.
//
// The keys are HEAD PREDICATES, because that is what ends up in
// DerivationNode.RuleName: mangle's ProofTreeTracer indexes rules by head
// predicate and sets RuleSpec.Name = headPred (internal/mangle/proof_tree.go).
// The previous glossary was keyed by the symbolic names in
// internal/core/defaults/policy/trace_logic.mg's rule_metadata/2 facts
// ("strategy_selector", "permission_gate", …). None of those strings is a
// predicate, so not one key ever matched and every derived fact fell through
// to the generic "rule 'x'" line — the explainer's whole reason to exist.
//
// Entries are drawn from actual rule heads in internal/core/defaults/**.mg;
// TestRuleGlossary_EveryEntry_ShouldExistInMangleCorpus fails if one drifts
// out of the corpus.
var ruleGlossary = map[string]string{
	// Executive / action selection
	"next_action":       "action selection strategy",
	"next_coder_action": "coder action selection",
	"next_tester_action": "tester action selection",
	"tester_action":     "tester workflow step",
	"reviewer_action":   "reviewer workflow step",
	"active_strategy":   "active execution strategy",
	"coder_strategy":    "coder strategy selection",
	"route_decision":    "request routing decision",
	"delegate_task":     "shard task delegation",
	"activate_shard":    "shard activation",
	"should_halt":       "halt condition",

	// Constitutional safety
	"permitted":                 "safety permission gate",
	"prohibited":                "constitutional prohibition",
	"base_prohibited":           "base constitutional prohibition",
	"dangerous_content":         "dangerous content detection",
	"permission_denied":         "permission refusal",
	"block_commit":              "commit safety barrier",
	"block_action":              "action safety barrier",
	"coder_block_action":        "coder action barrier",
	"deny_edit":                 "edit denial",
	"edit_unsafe":               "unsafe edit detection",
	"edit_warning":              "edit risk warning",
	"element_edit_blocked":      "code element edit barrier",
	"panic_state":               "projected panic state",
	"system_invariant_violated": "system invariant violation",
	"requires_approval":         "human approval requirement",
	"escalation_needed":         "escalation trigger",
	"escalation_required":       "mandatory escalation",
	"is_honeypot":               "honeypot detection",
	"honeypot_detected":         "honeypot detection",
	"file_has_security_sensitive": "security-sensitive file detection",

	// Impact / quality
	"impact_warning":        "impact warning",
	"impact_graph":          "transitive dependency analysis",
	"breaking_change_risk":  "breaking change risk analysis",
	"quality_violation_detected": "quality rule violation",
	"tdd_violation":         "test-driven development violation",
	"has_test_coverage":     "test coverage check",
	"test_framework":        "test framework detection",
	"test_priority":         "test prioritization",
	"impacted_test":         "impacted test selection",
	"review_suspect":        "suspect review finding",
	"reviewer_needs_validation": "review validation requirement",
	"suppressed_finding":    "finding suppression",
	"raw_finding":           "raw review finding",
	"root_cause":            "root cause analysis",
	"needs_self_healing":    "self-healing trigger",

	// Perception / clarification
	"detected_language":      "language detection",
	"detected_interrogative": "question detection",
	"detected_modal":         "modal verb detection",
	"clarification_needed":   "focus confidence check",
	"clarification_question": "clarification question selection",
	"clarification_option":   "clarification option",

	// Context / JIT
	"activation":           "context spreading activation",
	"potential_score":      "context potential scoring",
	"boost":                "relevance boost",
	"context_relevant":     "context relevance",
	"include_in_context":   "context inclusion",
	"exclude_from_context": "context exclusion",
	"injectable_context":   "injectable context selection",
	"context_priority":     "context prioritization",
	"relevant_context":     "context relevance",
	"selected_atom":        "prompt atom selection",
	"mandatory_atom":       "mandatory prompt atom",
	"relevant_tool":        "tool relevance",
	"mcp_tool_selected":    "MCP tool selection",
	"shard_context_atom":   "shard context atom selection",

	// Campaign / phases
	"campaign_blocked":     "campaign block",
	"campaign_risk_block":  "campaign risk block",
	"phase_blocked":        "phase block",
	"current_phase":        "current phase derivation",
	"current_ooda_phase":   "OODA phase derivation",
	"ooda_phase":           "OODA phase derivation",
	"milestone_reached":    "milestone detection",
	"replan_needed":        "replanning trigger",
	"goal_requires_campaign": "campaign requirement",

	// Learning / memory
	"promote_to_long_term": "long-term memory promotion",
	"learning_signal":      "learning signal extraction",
	"quality_signal":       "quality signal extraction",
}

// explainRule provides a human-readable explanation for a rule name.
func explainRule(ruleName string) string {
	if explanation, ok := ruleGlossary[ruleName]; ok {
		return explanation
	}
	if ruleName != "" {
		return fmt.Sprintf("rule '%s'", ruleName)
	}
	return ""
}

// ExplainFact generates an explanation for why a specific fact holds.
func (e *Explainer) ExplainFact(trace *mangle.DerivationTrace, factPredicate string) string {
	if trace == nil {
		return "No trace available."
	}

	// Find nodes matching this predicate
	var relevant []*mangle.DerivationNode
	for _, node := range trace.AllNodes {
		if node.Fact.Predicate == factPredicate {
			relevant = append(relevant, node)
		}
	}

	if len(relevant) == 0 {
		return fmt.Sprintf("No derivation found for `%s`.", factPredicate)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Why `%s` holds\n\n", factPredicate))

	for i, node := range relevant {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		e.explainNode(&sb, node, 0)
	}

	return sb.String()
}

// ExplainDecision generates a narrative explanation for a decision.
func (e *Explainer) ExplainDecision(action string, trace *mangle.DerivationTrace) string {
	if trace == nil {
		return fmt.Sprintf("Decided to **%s**.\n\n*No detailed explanation available.*", action)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Decision: %s\n\n", action))

	// Find the next_action derivation
	var actionNode *mangle.DerivationNode
	for _, node := range trace.RootNodes {
		if node.Fact.Predicate == "next_action" {
			actionNode = node
			break
		}
	}

	if actionNode == nil {
		sb.WriteString("This action was selected based on the current system state.\n")
		return sb.String()
	}

	sb.WriteString("### Reasoning Chain\n\n")

	// Build narrative from the derivation tree
	e.buildNarrative(&sb, actionNode, 0)

	return sb.String()
}

// buildNarrative creates a flowing narrative from a derivation tree.
func (e *Explainer) buildNarrative(sb *strings.Builder, node *mangle.DerivationNode, depth int) {
	if depth > 3 {
		return
	}

	fact := formatFactForHuman(node.Fact)

	switch depth {
	case 0:
		sb.WriteString(fmt.Sprintf("**Conclusion**: `%s`\n\n", fact))
		if len(node.Children) > 0 {
			sb.WriteString("**Based on:**\n")
		}
	case 1:
		sb.WriteString(fmt.Sprintf("- %s\n", fact))
	default:
		indent := strings.Repeat("  ", depth-1)
		sb.WriteString(fmt.Sprintf("%s- %s\n", indent, fact))
	}

	for _, child := range node.Children {
		e.buildNarrative(sb, child, depth+1)
	}
}

// QuickExplain provides a one-liner explanation for common predicates.
func QuickExplain(predicate string, args []any) string {
	switch predicate {
	case "next_action":
		if len(args) > 0 {
			return fmt.Sprintf("Next action will be: %v", args[0])
		}
	case "permitted":
		if len(args) > 0 {
			return fmt.Sprintf("Action '%v' is permitted by safety rules", args[0])
		}
	case "user_intent":
		if len(args) >= 3 {
			return fmt.Sprintf("User wants to %v on %v", args[1], args[2])
		}
	case "clarification_needed":
		if len(args) > 0 {
			return fmt.Sprintf("Need clarification about: %v", args[0])
		}
	case "test_state":
		if len(args) >= 2 {
			return fmt.Sprintf("Test %v is in state: %v", args[0], args[1])
		}
	case "impacted":
		if len(args) > 0 {
			return fmt.Sprintf("File '%v' may be impacted by changes", args[0])
		}
	case "context_atom":
		if len(args) > 0 {
			return fmt.Sprintf("'%v' is relevant to current context", args[0])
		}
	}

	// Default format
	if len(args) == 0 {
		return predicate
	}
	return fmt.Sprintf("%s: %v", predicate, args)
}

// OperationSummary holds summary data for a completed operation.
type OperationSummary struct {
	Operation     string    // What operation was performed
	Duration      string    // How long it took
	FilesAffected []string  // Files that were modified/read
	RulesApplied  []string  // Mangle rules that were triggered
	Outcome       string    // Success/failure/partial
	Details       string    // Additional details
	NextSteps     []string  // Suggested follow-up actions
	Source        string    // Producer identity (shard ID, action ID)
	CompletedAt   time.Time // When the operation finished
}

// StatusLine renders a one-line form for status tables.
func (s *OperationSummary) StatusLine() string {
	var sb strings.Builder
	if !s.CompletedAt.IsZero() {
		sb.WriteString("[" + s.CompletedAt.Format("15:04:05") + "] ")
	}
	sb.WriteString(s.Operation)
	if s.Outcome != "" {
		sb.WriteString(": " + s.Outcome)
	}
	if s.Duration != "" {
		sb.WriteString(" (" + s.Duration + ")")
	}
	return sb.String()
}

// FormatOperationSummary formats an operation summary for display.
func FormatOperationSummary(summary *OperationSummary) string {
	if summary == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s Complete\n\n", summary.Operation))

	if summary.Duration != "" {
		sb.WriteString(fmt.Sprintf("**Duration**: %s\n", summary.Duration))
	}

	sb.WriteString(fmt.Sprintf("**Outcome**: %s\n\n", summary.Outcome))

	if len(summary.FilesAffected) > 0 {
		sb.WriteString("### Files Affected\n")
		for _, f := range summary.FilesAffected {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	if summary.Details != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", summary.Details))
	}

	if len(summary.NextSteps) > 0 {
		sb.WriteString("### Suggested Next Steps\n")
		for _, step := range summary.NextSteps {
			sb.WriteString(fmt.Sprintf("- %s\n", step))
		}
	}

	return sb.String()
}
