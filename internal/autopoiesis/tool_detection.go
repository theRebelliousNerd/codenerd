package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/logging"
)

// ToolNeed represents a detected need for a new tool
type ToolNeed struct {
	Name        string   // Proposed tool name
	Purpose     string   // What the tool should do
	InputType   string   // Expected input type
	OutputType  string   // Expected output type
	Triggers    []string // What user inputs suggest this need
	Priority    float64  // How urgently this is needed (0.0 - 1.0)
	Confidence  float64  // How confident we are this is a real need
	Reasoning   string   // Why we think this tool is needed
}

// Tool need detection patterns
var (
	// Patterns suggesting a missing capability
	missingCapabilityPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)can('t| not|'t)\s+(you\s+)?(do|handle|process|parse|convert|validate|check|analyze)`),
		regexp.MustCompile(`(?i)is\s+there\s+(a\s+)?(way|tool|method)\s+to\s+`),
		regexp.MustCompile(`(?i)i\s+(need|want)\s+(a\s+)?(tool|way|method)\s+(to|for|that)`),
		regexp.MustCompile(`(?i)how\s+(do|can)\s+(i|you)\s+(do|make|create|convert|validate|parse)`),
		regexp.MustCompile(`(?i)(wish|wished)\s+(there\s+was|you\s+could|i\s+could)`),
		regexp.MustCompile(`(?i)would\s+be\s+(nice|great|helpful)\s+(if|to\s+have)`),
	}

	// Patterns suggesting specific tool types
	toolTypePatterns = map[string][]*regexp.Regexp{
		"validator": {
			regexp.MustCompile(`(?i)validate\s+(the\s+)?(json|yaml|xml|config|schema|format)`),
			regexp.MustCompile(`(?i)check\s+(if|whether)\s+.+\s+(is\s+)?(valid|correct|proper)`),
		},
		"converter": {
			regexp.MustCompile(`(?i)convert\s+(from\s+)?(\w+)\s+to\s+(\w+)`),
			regexp.MustCompile(`(?i)transform\s+.+\s+(into|to)\s+`),
		},
		"parser": {
			regexp.MustCompile(`(?i)parse\s+(the\s+)?(\w+)\s+(file|data|output|response)`),
			regexp.MustCompile(`(?i)extract\s+.+\s+from\s+`),
		},
		"analyzer": {
			regexp.MustCompile(`(?i)analyze\s+(the\s+)?(\w+)\s+(for|to\s+find)`),
			regexp.MustCompile(`(?i)find\s+(all|every)\s+.+\s+in\s+`),
		},
		"formatter": {
			regexp.MustCompile(`(?i)format\s+(the\s+)?(\w+)\s+(as|like|to)`),
			regexp.MustCompile(`(?i)pretty\s*print\s+`),
		},
	}
)

// DetectToolNeed analyzes input to determine if a new tool is needed
func (tg *ToolGenerator) DetectToolNeed(ctx context.Context, input string, failedAttempt string) (*ToolNeed, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "DetectToolNeed")
	defer timer.Stop()

	logging.AutopoiesisDebug("Detecting tool need from input (%d chars)", len(input))

	lower := strings.ToLower(input)

	// Check if input suggests missing capability
	needsNewTool := false
	var triggers []string

	for _, pattern := range missingCapabilityPatterns {
		if pattern.MatchString(lower) {
			needsNewTool = true
			triggers = append(triggers, pattern.String())
		}
	}

	// Check if there was a failed attempt that suggests tool gap
	if failedAttempt != "" {
		needsNewTool = true
		triggers = append(triggers, "Previous attempt failed")
		logging.AutopoiesisDebug("Tool need triggered by failed attempt")
	}

	if !needsNewTool {
		logging.AutopoiesisDebug("No tool need detected from input")
		return nil, nil // No tool need detected
	}

	logging.AutopoiesisDebug("Tool need detected with %d triggers", len(triggers))

	// Determine tool type from patterns
	toolType := "utility" // default
	for ttype, patterns := range toolTypePatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(lower) {
				toolType = ttype
				break
			}
		}
	}
	logging.AutopoiesisDebug("Detected tool type: %s", toolType)

	// Use LLM to refine the tool need
	logging.Autopoiesis("Refining tool need with LLM for type=%s", toolType)
	need, err := tg.refineToolNeedWithLLM(ctx, input, failedAttempt, toolType, triggers)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("LLM refinement failed: %v, using heuristic fallback", err)
		// Fall back to heuristic-based need
		return &ToolNeed{
			Name:       fmt.Sprintf("%s_tool", toolType),
			Purpose:    input,
			InputType:  "string",
			OutputType: "string",
			Triggers:   triggers,
			Priority:   0.5,
			Confidence: 0.4,
			Reasoning:  "Detected via pattern matching, LLM refinement failed",
		}, nil
	}

	logging.Autopoiesis("Tool need refined: name=%s, confidence=%.2f", need.Name, need.Confidence)
	return need, nil
}

// refineToolNeedWithLLM uses LLM to better understand the tool need
func (tg *ToolGenerator) refineToolNeedWithLLM(ctx context.Context, input, failedAttempt, toolType string, triggers []string) (*ToolNeed, error) {
	prompt := fmt.Sprintf(`Analyze this user request and determine if a new tool is needed.

User Request: %q
Previous Attempt Failed: %q
Detected Tool Type: %s
Trigger Patterns: %v

Existing tools available: %v

Only recommend a new tool if there is a clear capability gap that cannot be met by existing tools. New tools are expensive and risky; default to reuse unless strongly justified.

Return JSON only:
{
  "needs_new_tool": true/false,
  "tool_name": "snake_case_name",
  "purpose": "clear description of what the tool should do",
  "input_type": "go type for input (string, []byte, map[string]any, etc)",
  "output_type": "go type for output",
  "priority": 0.0-1.0,
  "confidence": 0.0-1.0,
  "reasoning": "why this tool is needed"
}

JSON only:`, input, failedAttempt, toolType, triggers, tg.listExistingTools())

	resp, err := tg.client.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result struct {
		NeedsNewTool bool    `json:"needs_new_tool"`
		ToolName     string  `json:"tool_name"`
		Purpose      string  `json:"purpose"`
		InputType    string  `json:"input_type"`
		OutputType   string  `json:"output_type"`
		Priority     float64 `json:"priority"`
		Confidence   float64 `json:"confidence"`
		Reasoning    string  `json:"reasoning"`
	}

	// Extract JSON from response
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if !result.NeedsNewTool {
		return nil, nil
	}

	return &ToolNeed{
		Name:       result.ToolName,
		Purpose:    result.Purpose,
		InputType:  result.InputType,
		OutputType: result.OutputType,
		Triggers:   triggers,
		Priority:   result.Priority,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
	}, nil
}
