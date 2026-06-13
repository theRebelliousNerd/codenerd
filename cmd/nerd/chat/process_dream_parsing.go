package chat

import (
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func parseExecutionResults(facts []core.Fact) []systemExecutionResult {
	results := make([]systemExecutionResult, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) < 5 {
			continue
		}
		result := systemExecutionResult{
			ActionID:   types.ExtractString(fact.Args[0]),
			ActionType: types.ExtractString(fact.Args[1]),
			Target:     types.ExtractString(fact.Args[2]),
			Success:    parseBool(fact.Args[3]),
			Output:     types.ExtractString(fact.Args[4]),
		}
		if len(fact.Args) >= 6 {
			if ts, ok := fact.Args[5].(int64); ok {
				result.Timestamp = ts
			} else if tsVal, ok := fact.Args[5].(float64); ok {
				result.Timestamp = int64(tsVal)
			}
		}
		results = append(results, result)
	}
	return results
}

func parseBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func nextActionName(action core.Fact) string {
	if len(action.Args) > 0 {
		value := strings.TrimSpace(types.ExtractString(action.Args[0]))
		if value != "" {
			if !strings.HasPrefix(value, "/") {
				value = "/" + value
			}
			return value
		}
	}
	return strings.TrimSpace(action.Predicate)
}

func normalizeActionType(actionType string) string {
	actionType = strings.TrimSpace(strings.TrimPrefix(actionType, "/"))
	if actionType == "" {
		return ""
	}
	return strings.ToLower(actionType)
}

func actionTypeToShardType(actionType, target string) string {
	switch normalizeActionType(actionType) {
	case "delegate_reviewer":
		return "reviewer"
	case "delegate_tester":
		return "tester"
	case "delegate_coder":
		return "coder"
	case "delegate_researcher":
		return "researcher"
	case "delegate_tool_generator":
		return "tool_generator"
	case "delegate":
		return normalizeShardType(target)
	default:
		return ""
	}
}

func parseDelegateFact(fact core.Fact) (string, string, bool) {
	if len(fact.Args) < 3 {
		return "", "", false
	}
	shardType := normalizeShardType(types.ExtractString(fact.Args[0]))
	task := types.ExtractString(fact.Args[1])
	status := strings.ToLower(types.ExtractString(fact.Args[2]))
	pending := status == "/pending" || status == "pending"
	return shardType, strings.TrimSpace(task), pending
}

func normalizeShardType(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/")
	return strings.ToLower(raw)
}
