package transparency

import (
	"fmt"
	"strings"

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
func formatArg(arg interface{}) string {
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

// explainRule provides a human-readable explanation for a rule name.
func explainRule(ruleName string) string {
	explanations := map[string]string{
		"strategy_selector":    "action selection strategy",
		"permission_gate":      "safety permission check",
		"commit_barrier":       "commit safety barrier",
		"transitive_impact":    "transitive dependency analysis",
		"focus_threshold":      "focus confidence check",
		"refactoring_guard":    "refactoring safety check",
		"tdd_loop":             "test-driven development loop",
		"spreading_activation": "context spreading activation",
		"abductive_repair":     "abductive reasoning repair",
		"shard_delegation":     "shard task delegation",
		"activation_rules":     "context activation",
	}

	if explanation, ok := explanations[ruleName]; ok {
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
func QuickExplain(predicate string, args []interface{}) string {
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
	Operation     string        // What operation was performed
	Duration      string        // How long it took
	FilesAffected []string      // Files that were modified/read
	RulesApplied  []string      // Mangle rules that were triggered
	Outcome       string        // Success/failure/partial
	Details       string        // Additional details
	NextSteps     []string      // Suggested follow-up actions
}

// FormatOperationSummary formats an operation summary for display.
func FormatOperationSummary(summary *OperationSummary) string {
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
