package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/mangle"
)

// LLMTransducer implements LLM-first intent classification.
// The LLM understands user intent; the harness routes based on that understanding.
//
// Philosophy: LLM describes → Harness determines
type LLMTransducer struct {
	client LLMClient
	kernel RoutingKernel
	prompt string
}

// RoutingKernel is the interface for querying routing rules from Mangle.
type RoutingKernel interface {
	// QueryRouting queries the routing schema for affinity scores.
	// predicate is one of: mode_from_semantic, context_affinity_semantic, etc.
	// arg is the semantic type, action type, or domain.
	// Returns a list of (target, weight) pairs.
	QueryRouting(ctx context.Context, predicate string, arg string) ([]RoutingMatch, error)

	// ValidateField checks if a value is valid for a given field.
	// field is one of: semantic_type, action_type, domain, scope_level, mode
	ValidateField(ctx context.Context, field, value string) bool
}

// RoutingMatch represents a routing query result.
type RoutingMatch struct {
	Target string
	Weight int
}

// NewLLMTransducer creates a new LLM-first transducer.
func NewLLMTransducer(client LLMClient, kernel RoutingKernel, prompt string) *LLMTransducer {
	return &LLMTransducer{
		client: client,
		kernel: kernel,
		prompt: prompt,
	}
}

// Understand uses the LLM to interpret user intent.
// This is the primary (and only) classification path.
func (t *LLMTransducer) Understand(ctx context.Context, input string, history []Turn) (*Understanding, error) {
	// 1. Build the prompt with conversation history
	fullPrompt := t.buildPrompt(input, history)

	// 2. Call LLM for classification
	response, err := t.client.CompleteWithSystem(ctx, t.prompt, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM classification failed: %w", err)
	}

	// 3. Parse the response
	understanding, err := t.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 4. Validate against routing vocabulary
	// Don't fail - the LLM's understanding is still valuable even if
	// it uses terms outside our vocabulary. The harness will use defaults.
	_ = t.validate(ctx, understanding)

	// 5. Derive routing from understanding
	t.deriveRouting(ctx, understanding)

	return understanding, nil
}

// buildPrompt constructs the user prompt with conversation history.
func (t *LLMTransducer) buildPrompt(input string, history []Turn) string {
	var sb strings.Builder

	// Include relevant history for context
	if len(history) > 0 {
		sb.WriteString("## Recent Conversation\n\n")
		// Only include last few turns to stay focused
		start := 0
		if len(history) > 5 {
			start = len(history) - 5
		}
		for _, turn := range history[start:] {
			sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", turn.Role, turn.Content))
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## Current Request\n\n")
	sb.WriteString(input)

	return sb.String()
}

// parseResponse extracts Understanding from LLM JSON response.
func (t *LLMTransducer) parseResponse(response string) (*Understanding, error) {
	// Try to extract JSON from response (may have markdown wrapper)
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse the envelope
	var envelope UnderstandingEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		// Try parsing as just Understanding (no envelope)
		var understanding Understanding
		if err2 := json.Unmarshal([]byte(jsonStr), &understanding); err2 != nil {
			return nil, fmt.Errorf("JSON parse failed: %w (also tried: %v)", err, err2)
		}
		return &understanding, nil
	}

	// Copy surface response into understanding
	envelope.Understanding.SurfaceResponse = envelope.SurfaceResponse
	return &envelope.Understanding, nil
}

// extractJSON finds JSON object in response (handles markdown wrappers).
func extractJSON(response string) string {
	// Try to find JSON object
	start := strings.Index(response, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(response); i++ {
		switch response[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return response[start : i+1]
			}
		}
	}

	return ""
}

// validate checks Understanding against the routing vocabulary.
func (t *LLMTransducer) validate(ctx context.Context, u *Understanding) error {
	if t.kernel == nil {
		return nil // No kernel, skip validation
	}

	var errors []string

	if !t.kernel.ValidateField(ctx, "semantic_type", u.SemanticType) {
		errors = append(errors, fmt.Sprintf("invalid semantic_type: %s", u.SemanticType))
	}

	if !t.kernel.ValidateField(ctx, "action_type", u.ActionType) {
		errors = append(errors, fmt.Sprintf("invalid action_type: %s", u.ActionType))
	}

	if !t.kernel.ValidateField(ctx, "domain", u.Domain) {
		errors = append(errors, fmt.Sprintf("invalid domain: %s", u.Domain))
	}

	if u.Scope.Level != "" && !t.kernel.ValidateField(ctx, "scope_level", u.Scope.Level) {
		errors = append(errors, fmt.Sprintf("invalid scope_level: %s", u.Scope.Level))
	}

	if u.SuggestedApproach.Mode != "" && !t.kernel.ValidateField(ctx, "mode", u.SuggestedApproach.Mode) {
		errors = append(errors, fmt.Sprintf("invalid mode: %s", u.SuggestedApproach.Mode))
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// deriveRouting uses Mangle rules to derive routing from understanding.
func (t *LLMTransducer) deriveRouting(ctx context.Context, u *Understanding) {
	if t.kernel == nil {
		// No kernel, use LLM's suggestions directly
		u.Routing = &Routing{
			Mode:             u.SuggestedApproach.Mode,
			PrimaryShard:     u.SuggestedApproach.PrimaryShard,
			SupportingShards: u.SuggestedApproach.SupportingShards,
		}
		return
	}

	routing := &Routing{
		ContextPriorities: make(map[string]int),
		ToolPriorities:    make(map[string]int),
	}

	// Derive mode from semantic type and action
	routing.Mode = t.deriveMode(ctx, u)

	// Derive shard from action and domain
	routing.PrimaryShard, routing.SupportingShards = t.deriveShards(ctx, u)

	// Derive context priorities
	routing.ContextPriorities = t.deriveContextPriorities(ctx, u)

	// Derive tool priorities
	routing.ToolPriorities = t.deriveToolPriorities(ctx, u)

	// Apply constraint-based blocks
	routing.BlockedTools = t.deriveBlockedTools(ctx, u)

	u.Routing = routing
}

// deriveMode determines the harness mode from understanding.
func (t *LLMTransducer) deriveMode(ctx context.Context, u *Understanding) string {
	// Check signal-based overrides first
	if u.Signals.IsHypothetical {
		return "dream"
	}

	// Query mode from semantic type
	matches, _ := t.kernel.QueryRouting(ctx, "mode_from_semantic", u.SemanticType)
	if len(matches) > 0 {
		return matches[0].Target
	}

	// Query mode from action type
	matches, _ = t.kernel.QueryRouting(ctx, "mode_from_action", u.ActionType)
	if len(matches) > 0 {
		return matches[0].Target
	}

	// Query mode from domain
	matches, _ = t.kernel.QueryRouting(ctx, "mode_from_domain", u.Domain)
	if len(matches) > 0 {
		return matches[0].Target
	}

	// Use LLM's suggestion or default
	if u.SuggestedApproach.Mode != "" {
		return u.SuggestedApproach.Mode
	}
	return "normal"
}

// deriveShards determines which shards to use.
func (t *LLMTransducer) deriveShards(ctx context.Context, u *Understanding) (string, []string) {
	shardScores := make(map[string]int)

	// Score from action type
	matches, _ := t.kernel.QueryRouting(ctx, "shard_affinity_action", u.ActionType)
	for _, m := range matches {
		shardScores[m.Target] += m.Weight
	}

	// Score from domain
	matches, _ = t.kernel.QueryRouting(ctx, "shard_affinity_domain", u.Domain)
	for _, m := range matches {
		shardScores[m.Target] += m.Weight
	}

	// Find best shard
	var primary string
	var primaryScore int
	var supporting []string

	for shard, score := range shardScores {
		if score > primaryScore {
			if primary != "" {
				supporting = append(supporting, primary)
			}
			primary = shard
			primaryScore = score
		} else if score > 50 { // Threshold for supporting
			supporting = append(supporting, shard)
		}
	}

	// Fallback to LLM suggestion
	if primary == "" {
		primary = u.SuggestedApproach.PrimaryShard
		supporting = u.SuggestedApproach.SupportingShards
	}

	return primary, supporting
}

// deriveContextPriorities determines what context to prioritize.
func (t *LLMTransducer) deriveContextPriorities(ctx context.Context, u *Understanding) map[string]int {
	priorities := make(map[string]int)

	// From semantic type
	matches, _ := t.kernel.QueryRouting(ctx, "context_affinity_semantic", u.SemanticType)
	for _, m := range matches {
		priorities[m.Target] = max(priorities[m.Target], m.Weight)
	}

	// From action type
	matches, _ = t.kernel.QueryRouting(ctx, "context_affinity_action", u.ActionType)
	for _, m := range matches {
		priorities[m.Target] = max(priorities[m.Target], m.Weight)
	}

	// From domain
	matches, _ = t.kernel.QueryRouting(ctx, "context_affinity_domain", u.Domain)
	for _, m := range matches {
		priorities[m.Target] = max(priorities[m.Target], m.Weight)
	}

	// Include LLM's suggestions with moderate priority
	for _, need := range u.SuggestedApproach.ContextNeeded {
		if priorities[need] == 0 {
			priorities[need] = 70 // Default priority for LLM suggestions
		}
	}

	return priorities
}

// deriveToolPriorities determines which tools to prioritize.
func (t *LLMTransducer) deriveToolPriorities(ctx context.Context, u *Understanding) map[string]int {
	priorities := make(map[string]int)

	// From action type
	matches, _ := t.kernel.QueryRouting(ctx, "tool_affinity_action", u.ActionType)
	for _, m := range matches {
		priorities[m.Target] = max(priorities[m.Target], m.Weight)
	}

	// From domain
	matches, _ = t.kernel.QueryRouting(ctx, "tool_affinity_domain", u.Domain)
	for _, m := range matches {
		priorities[m.Target] = max(priorities[m.Target], m.Weight)
	}

	// Include LLM's suggestions
	for _, tool := range u.SuggestedApproach.ToolsNeeded {
		if priorities[tool] == 0 {
			priorities[tool] = 70
		}
	}

	return priorities
}

// deriveBlockedTools determines which tools are blocked by constraints.
func (t *LLMTransducer) deriveBlockedTools(ctx context.Context, u *Understanding) []string {
	var blocked []string

	// Check each constraint
	for _, constraint := range u.UserConstraints {
		matches, _ := t.kernel.QueryRouting(ctx, "constraint_blocks_tool", constraint)
		for _, m := range matches {
			blocked = append(blocked, m.Target)
		}
	}

	// Read-only mode blocks write tools
	if u.IsReadOnly() {
		blocked = append(blocked, "write_file", "edit_file", "git_commit", "git_push")
	}

	return blocked
}

// Turn represents a conversation turn for context.
type Turn struct {
	Role    string // "user" or "assistant"
	Content string
}

// MangleRoutingKernel implements RoutingKernel using the Mangle engine.
type MangleRoutingKernel struct {
	engine *mangle.Engine
}

// NewMangleRoutingKernel creates a routing kernel backed by Mangle.
func NewMangleRoutingKernel(engine *mangle.Engine) *MangleRoutingKernel {
	return &MangleRoutingKernel{engine: engine}
}

// QueryRouting queries the routing schema.
func (k *MangleRoutingKernel) QueryRouting(ctx context.Context, predicate string, arg string) ([]RoutingMatch, error) {
	// Build query based on predicate type
	var query string
	switch {
	case strings.HasPrefix(predicate, "mode_from_"):
		query = fmt.Sprintf("%s(/%s, Mode, Priority)", predicate, arg)
	case strings.HasPrefix(predicate, "context_affinity_"):
		query = fmt.Sprintf("%s(/%s, Context, Weight)", predicate, arg)
	case strings.HasPrefix(predicate, "shard_affinity_"):
		query = fmt.Sprintf("%s(/%s, Shard, Weight)", predicate, arg)
	case strings.HasPrefix(predicate, "tool_affinity_"):
		query = fmt.Sprintf("%s(/%s, Tool, Weight)", predicate, arg)
	case predicate == "constraint_blocks_tool":
		query = fmt.Sprintf("constraint_blocks_tool(/%s, Tool)", arg)
	default:
		return nil, fmt.Errorf("unknown predicate: %s", predicate)
	}

	result, err := k.engine.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var matches []RoutingMatch
	for _, binding := range result.Bindings {
		match := RoutingMatch{}

		// Extract target (Mode, Context, Shard, Tool)
		for key, val := range binding {
			if key == "Mode" || key == "Context" || key == "Shard" || key == "Tool" {
				if s, ok := val.(string); ok {
					match.Target = strings.TrimPrefix(s, "/")
				}
			}
			if key == "Priority" || key == "Weight" {
				switch v := val.(type) {
				case int:
					match.Weight = v
				case int64:
					match.Weight = int(v)
				case float64:
					match.Weight = int(v)
				}
			}
		}

		if match.Target != "" {
			matches = append(matches, match)
		}
	}

	// Sort by weight descending
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Weight > matches[i].Weight {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches, nil
}

// ValidateField checks if a value is valid for a field.
func (k *MangleRoutingKernel) ValidateField(ctx context.Context, field, value string) bool {
	var query string
	switch field {
	case "semantic_type":
		query = fmt.Sprintf("valid_semantic_type(/%s, _)", value)
	case "action_type":
		query = fmt.Sprintf("valid_action_type(/%s, _)", value)
	case "domain":
		query = fmt.Sprintf("valid_domain(/%s, _)", value)
	case "scope_level":
		query = fmt.Sprintf("valid_scope_level(/%s, _)", value)
	case "mode":
		query = fmt.Sprintf("valid_mode(/%s, _)", value)
	default:
		return true // Unknown field, assume valid
	}

	result, err := k.engine.Query(ctx, query)
	if err != nil {
		return false
	}

	return len(result.Bindings) > 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =============================================================================
// RealKernelRouter - Adapts core.RealKernel to RoutingKernel interface
// =============================================================================

// RealKernelRouter adapts a core.RealKernel to implement the RoutingKernel interface.
// This enables the UnderstandingTransducer to use the existing kernel for routing queries.
type RealKernelRouter struct {
	kernel *core.RealKernel
}

// NewRealKernelRouter creates a RoutingKernel backed by a RealKernel.
func NewRealKernelRouter(kernel *core.RealKernel) *RealKernelRouter {
	return &RealKernelRouter{kernel: kernel}
}

// QueryRouting queries the routing schema using the RealKernel's Query method.
func (k *RealKernelRouter) QueryRouting(ctx context.Context, predicate string, arg string) ([]RoutingMatch, error) {
	if k.kernel == nil {
		return nil, nil
	}

	// Build the query predicate
	fullPredicate := fmt.Sprintf("%s(/%s", predicate, arg)

	// Query the kernel for facts matching this predicate
	facts, err := k.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}

	var matches []RoutingMatch
	for _, fact := range facts {
		// Filter facts that match the argument
		if len(fact.Args) < 2 {
			continue
		}

		// First arg should be the query arg (semantic type, action, domain)
		firstArg := fmt.Sprintf("%v", fact.Args[0])
		if firstArg != "/"+arg && firstArg != arg {
			continue
		}

		// Second arg is the target (mode, context, shard, tool)
		match := RoutingMatch{
			Target: strings.TrimPrefix(fmt.Sprintf("%v", fact.Args[1]), "/"),
		}

		// Third arg (if present) is the weight
		if len(fact.Args) >= 3 {
			switch v := fact.Args[2].(type) {
			case int:
				match.Weight = v
			case int64:
				match.Weight = int(v)
			case float64:
				match.Weight = int(v)
			}
		}

		if match.Target != "" {
			matches = append(matches, match)
		}
	}

	// Sort by weight descending
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Weight > matches[i].Weight {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	_ = fullPredicate // Avoid unused variable warning
	return matches, nil
}

// ValidateField checks if a value is valid for a field.
func (k *RealKernelRouter) ValidateField(ctx context.Context, field, value string) bool {
	if k.kernel == nil {
		return true // No kernel = assume valid
	}

	// Build the validation predicate
	var predicate string
	switch field {
	case "semantic_type":
		predicate = "valid_semantic_type"
	case "action_type":
		predicate = "valid_action_type"
	case "domain":
		predicate = "valid_domain"
	case "scope_level":
		predicate = "valid_scope_level"
	case "mode":
		predicate = "valid_mode"
	default:
		return true // Unknown field, assume valid
	}

	// Query for validation facts
	facts, err := k.kernel.Query(predicate)
	if err != nil {
		return false
	}

	// Check if any fact matches the value
	for _, fact := range facts {
		if len(fact.Args) >= 1 {
			factValue := strings.TrimPrefix(fmt.Sprintf("%v", fact.Args[0]), "/")
			if factValue == value {
				return true
			}
		}
	}

	return false
}
