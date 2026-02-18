package shards

import (
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// queryToolsFromKernel queries the Mangle kernel for registered tools.
func (sm *ShardManager) queryToolsFromKernel() []types.ToolInfo {
	if sm.kernel == nil {
		logging.ShardsDebug("queryToolsFromKernel: no kernel available")
		return nil
	}

	logging.ShardsDebug("queryToolsFromKernel: querying tool_registered predicate")

	// Query all registered tools
	registeredFacts, err := sm.kernel.Query("tool_registered")
	if err != nil || len(registeredFacts) == 0 {
		logging.ShardsDebug("queryToolsFromKernel: no registered tools found")
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
	tools := make([]types.ToolInfo, 0, len(toolNames))
	for _, name := range toolNames {
		tools = append(tools, types.ToolInfo{
			Name:        name,
			Description: descriptions[name],
			BinaryPath:  binaryPaths[name],
		})
	}

	return tools
}

// ToolRelevanceQuery holds parameters for intelligent tool discovery.
type ToolRelevanceQuery struct {
	ShardType   string
	IntentVerb  string
	TargetFile  string
	TokenBudget int
}

func (sm *ShardManager) queryRelevantTools(query ToolRelevanceQuery) []types.ToolInfo {
	if sm.kernel == nil {
		return nil
	}

	sm.assertToolRoutingContext(query)

	shardAtom := normalizeMangleAtom(query.ShardType)
	relevantFacts, err := sm.kernel.Query("relevant_tool")
	if err != nil || len(relevantFacts) == 0 {
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	relevantToolNames := make([]string, 0)
	for _, fact := range relevantFacts {
		if len(fact.Args) >= 2 {
			factShardType, _ := fact.Args[0].(string)
			toolName, _ := fact.Args[1].(string)
			if factShardType == shardAtom && toolName != "" {
				relevantToolNames = append(relevantToolNames, toolName)
			}
		}
	}

	if len(relevantToolNames) == 0 {
		allTools := sm.queryToolsFromKernel()
		return sm.trimToTokenBudget(allTools, query.TokenBudget)
	}

	allTools := sm.queryToolsFromKernel()
	if allTools == nil {
		return nil
	}

	relevantSet := make(map[string]bool)
	for _, name := range relevantToolNames {
		relevantSet[name] = true
	}

	tools := make([]types.ToolInfo, 0)
	for _, tool := range allTools {
		if relevantSet[tool.Name] {
			tools = append(tools, tool)
		}
	}

	sm.sortToolsByPriority(tools, query.ShardType)

	return sm.trimToTokenBudget(tools, query.TokenBudget)
}

func (sm *ShardManager) assertToolRoutingContext(query ToolRelevanceQuery) {
	if sm.kernel == nil {
		return
	}

	if err := sm.kernel.Retract("current_shard_type"); err != nil {
		logging.Get(logging.CategoryKernel).Warn("Failed to retract current_shard_type: %v", err)
	}
	if err := sm.kernel.Retract("current_intent"); err != nil {
		logging.Get(logging.CategoryKernel).Warn("Failed to retract current_intent: %v", err)
	}
	if err := sm.kernel.Retract("current_time"); err != nil {
		logging.Get(logging.CategoryKernel).Warn("Failed to retract current_time: %v", err)
	}

	shardAtom := normalizeMangleAtom(query.ShardType)
	if err := sm.kernel.Assert(types.Fact{
		Predicate: "current_shard_type",
		Args:      []interface{}{shardAtom},
	}); err != nil {
		logging.Get(logging.CategoryKernel).Warn("Failed to assert current_shard_type: %v", err)
	}

	if query.IntentVerb != "" {
		intentID := "/tool_routing_context"
		verbAtom := normalizeMangleAtom(query.IntentVerb)
		if err := sm.kernel.RetractFact(types.Fact{Predicate: "user_intent", Args: []interface{}{intentID}}); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to retract user_intent for tool routing: %v", err)
		}
		if err := sm.kernel.Assert(types.Fact{
			Predicate: "current_intent",
			Args:      []interface{}{intentID},
		}); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to assert current_intent: %v", err)
		}
		if err := sm.kernel.Assert(types.Fact{
			Predicate: "user_intent",
			Args:      []interface{}{intentID, "/routing", verbAtom, query.TargetFile, "_"},
		}); err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to assert user_intent for tool routing: %v", err)
		}
	}

	if err := sm.kernel.Assert(types.Fact{
		Predicate: "current_time",
		Args:      []interface{}{int64(time.Now().Unix())},
	}); err != nil {
		logging.Get(logging.CategoryKernel).Warn("Failed to assert current_time: %v", err)
	}
}

func (sm *ShardManager) sortToolsByPriority(tools []types.ToolInfo, shardType string) {
	if sm.kernel == nil || len(tools) == 0 {
		return
	}

	shardAtom := normalizeMangleAtom(shardType)
	baseRelevanceFacts, _ := sm.kernel.Query("tool_base_relevance")

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

	sort.Slice(tools, func(i, j int) bool {
		scoreI := scores[tools[i].Name]
		scoreJ := scores[tools[j].Name]
		return scoreI > scoreJ
	})
}

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

func (sm *ShardManager) trimToTokenBudget(tools []types.ToolInfo, budget int) []types.ToolInfo {
	if budget <= 0 {
		budget = 2000
	}

	result := make([]types.ToolInfo, 0)
	tokensUsed := 0

	for _, tool := range tools {
		toolTokens := estimateTokens(tool.Name) +
			estimateTokens(tool.Description) +
			estimateTokens(tool.BinaryPath) + 20

		if tokensUsed+toolTokens <= budget {
			result = append(result, tool)
			tokensUsed += toolTokens
		} else {
			break
		}
	}

	return result
}

func estimateTokens(s string) int {
	return len(s) / 4
}

// DisableExecutiveBootGuard prevents the executive policy shard from running at boot.
func (sm *ShardManager) DisableExecutiveBootGuard() {
	sm.DisableSystemShard("executive_policy")
}
