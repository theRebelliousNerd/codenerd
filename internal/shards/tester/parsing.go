package tester

import (
	"fmt"
	"strings"
)

// =============================================================================
// TASK PARSING
// =============================================================================

// parseTask extracts action and parameters from task string.
func (t *TesterShard) parseTask(task string) (*TesterTask, error) {
	parsed := &TesterTask{
		Action:  "run_tests",
		Options: make(map[string]string),
	}

	parts := strings.Fields(task)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty task")
	}

	// First token is the action
	action := strings.ToLower(parts[0])
	switch action {
	case "run_tests", "test", "run":
		parsed.Action = "run_tests"
	case "generate_tests", "generate", "gen":
		parsed.Action = "generate_tests"
	case "coverage", "cov":
		parsed.Action = "coverage"
	case "tdd", "tdd_loop", "repair":
		parsed.Action = "tdd"
	case "regenerate_mocks", "regen_mocks", "update_mocks":
		parsed.Action = "regenerate_mocks"
	case "detect_stale_mocks", "check_mocks", "stale_mocks":
		parsed.Action = "detect_stale_mocks"
	default:
		// Assume run_tests if action is a file path
		if strings.Contains(action, ".") || strings.Contains(action, "/") {
			parsed.Action = "run_tests"
			parsed.Target = action
		}
	}

	// Parse key:value pairs
	for _, part := range parts[1:] {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.ToLower(kv[0])
			value := kv[1]

			switch key {
			case "file":
				parsed.File = value
				if parsed.Target == "" {
					parsed.Target = value
				}
			case "function", "func":
				parsed.Function = value
			case "package", "pkg":
				parsed.Package = value
				if parsed.Target == "" {
					parsed.Target = value
				}
			case "in":
				parsed.File = value
			default:
				parsed.Options[key] = value
			}
		} else if !strings.HasPrefix(part, "-") {
			// Bare argument - treat as target
			if parsed.Target == "" {
				parsed.Target = part
			}
		}
	}

	// Default target
	if parsed.Target == "" && parsed.Package == "" {
		parsed.Target = "./..."
	}

	return parsed, nil
}
