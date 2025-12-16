package core

import (
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

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

// queryToolsFromKernel queries the Mangle kernel for registered tools.
// Uses predicates: tool_registered, tool_description, tool_binary_path
// This enables dynamic, Mangle-governed tool discovery for agents.
func (sm *ShardManager) queryToolsFromKernel() []ToolInfo {
	if sm.kernel == nil {
		logging.ShardsDebug("queryToolsFromKernel: no kernel available")
		return nil
	}

	logging.ShardsDebug("queryToolsFromKernel: querying tool_registered predicate")

	// Query all registered tools
	registeredFacts, err := sm.kernel.Query("tool_registered")
	if err != nil || len(registeredFacts) == 0 {
		logging.ShardsDebug("queryToolsFromKernel: no registered tools found (err=%v, count=%d)", err, len(registeredFacts))
		return nil
	}
	logging.ShardsDebug("queryToolsFromKernel: found %d registered tools", len(registeredFacts))

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
	shardAtom := normalizeMangleAtom(query.ShardType)
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
	_ = sm.kernel.Retract("current_time")

	// Assert current shard type (with / prefix for Mangle atom)
	shardAtom := normalizeMangleAtom(query.ShardType)
	_ = sm.kernel.Assert(Fact{
		Predicate: "current_shard_type",
		Args:      []interface{}{shardAtom},
	})

	// Assert current intent if available
	if query.IntentVerb != "" {
		// Create a synthetic intent ID for routing purposes. Keep it distinct from
		// "/current_intent" so routing context cannot pollute the main policy.
		intentID := "/tool_routing_context"
		verbAtom := normalizeMangleAtom(query.IntentVerb)
		_ = sm.kernel.RetractFact(Fact{Predicate: "user_intent", Args: []interface{}{intentID}})
		_ = sm.kernel.Assert(Fact{
			Predicate: "current_intent",
			Args:      []interface{}{intentID},
		})
		// Ensure user_intent fact exists for derivation rules
		_ = sm.kernel.Assert(Fact{
			Predicate: "user_intent",
			// Use a non-/mutation category so campaign rules do not treat tool-routing
			// context as a user "mutation intent" during active campaigns.
			Args: []interface{}{intentID, "/routing", verbAtom, query.TargetFile, "_"},
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
	shardAtom := normalizeMangleAtom(shardType)
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

// normalizeMangleAtom ensures consistent atom formatting (leading /, no doubles).
func normalizeMangleAtom(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "/")
	for strings.Contains(value, "//") {
		value = strings.ReplaceAll(value, "//", "/")
	}
	if value == "" {
		return ""
	}
	return "/" + value
}

// normalizeShardTypeName removes leading slashes from shard type names.
func normalizeShardTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimLeft(typeName, "/")
	return typeName
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
