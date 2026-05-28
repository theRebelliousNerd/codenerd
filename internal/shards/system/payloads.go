package system

import (
	"encoding/json"
	"fmt"
	"strings"
)

func encodeActionPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func decodeActionPayload(arg any) (map[string]any, string) {
	payload := map[string]any{}
	intentID := ""

	switch v := arg.(type) {
	case map[string]any:
		payload = v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || trimmed == "{}" {
			return payload, ""
		}
		if strings.HasPrefix(trimmed, "{") {
			if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
				break
			}
		}
		if strings.HasPrefix(trimmed, "map[") && strings.HasSuffix(trimmed, "]") {
			payload = parsePseudoMapPayload(trimmed)
		}
	}

	if id, ok := payload["intent_id"].(string); ok {
		intentID = id
	}
	return payload, intentID
}

func parsePseudoMapPayload(raw string) map[string]any {
	payload := map[string]any{}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), "map["), "]")
	if trimmed == "" {
		return payload
	}
	for field := range strings.FieldsSeq(trimmed) {
		parts := strings.SplitN(field, ":", 2)
		if len(parts) != 2 {
			continue
		}
		payload[parts[0]] = parts[1]
	}
	return payload
}

func escalationSubject(actionType, target string) string {
	if strings.TrimSpace(target) == "" {
		return actionType
	}
	return fmt.Sprintf("%s:%s", actionType, target)
}
