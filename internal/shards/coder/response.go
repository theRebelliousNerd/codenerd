package coder

import (
	"encoding/json"
	"regexp"
	"strings"
)

// =============================================================================
// RESPONSE PARSING
// =============================================================================

// ArtifactType classifies what kind of code artifact is being created.
type ArtifactType string

const (
	// ArtifactTypeProjectCode is code for the user's project (default).
	ArtifactTypeProjectCode ArtifactType = "project_code"
	// ArtifactTypeSelfTool is a tool for codeNERD's own use.
	ArtifactTypeSelfTool ArtifactType = "self_tool"
	// ArtifactTypeDiagnostic is a one-time diagnostic/debugging script.
	ArtifactTypeDiagnostic ArtifactType = "diagnostic"
)

// CodeResponse represents the parsed LLM response for code generation.
type CodeResponse struct {
	File         string       `json:"file"`
	Content      string       `json:"content"`
	Rationale    string       `json:"rationale"`
	ArtifactType ArtifactType `json:"artifact_type"`
}

// IsSelfTool returns true if this artifact should be routed to Ouroboros.
func (r *CodeResponse) IsSelfTool() bool {
	return r.ArtifactType == ArtifactTypeSelfTool || r.ArtifactType == ArtifactTypeDiagnostic
}

// ParsedCodeResult contains both the edits and the artifact metadata.
type ParsedCodeResult struct {
	Edits        []CodeEdit
	ArtifactType ArtifactType
	ToolName     string // Extracted from file path for self-tools
}

// parseCodeResponse extracts code edits from LLM response.
// Returns ParsedCodeResult containing edits and artifact routing info.
func (c *CoderShard) parseCodeResponse(response string, task CoderTask) *ParsedCodeResult {
	result := &ParsedCodeResult{
		Edits:        make([]CodeEdit, 0),
		ArtifactType: ArtifactTypeProjectCode, // Default
	}

	// Try to parse as JSON first
	var jsonResp CodeResponse

	// Check for wrapped response (ReasoningTraceDirective)
	var wrapper struct {
		ReasoningTrace string          `json:"reasoning_trace"`
		Result         json.RawMessage `json:"result"`
	}

	// Find JSON in response (may be wrapped in markdown code blocks)
	jsonStr := response
	if idx := strings.Index(response, "{"); idx != -1 {
		endIdx := strings.LastIndex(response, "}")
		if endIdx > idx {
			jsonStr = response[idx : endIdx+1]
		}
	}

	// Try parsing as wrapper first
	if err := json.Unmarshal([]byte(jsonStr), &wrapper); err == nil && len(wrapper.Result) > 0 {
		// Check if Result is a direct string (code content) vs a JSON object
		var codeStr string
		if err := json.Unmarshal(wrapper.Result, &codeStr); err == nil && codeStr != "" {
			// Result is a direct string - use it as code content
			edit := CodeEdit{
				File:       task.Target,
				NewContent: codeStr,
				Type:       task.Action,
				Language:   detectLanguage(task.Target),
				Rationale:  wrapper.ReasoningTrace,
			}
			result.Edits = append(result.Edits, edit)
			return result
		}
		// Result is an object - continue to parse as CodeResponse
		jsonStr = string(wrapper.Result)
	}

	if err := json.Unmarshal([]byte(jsonStr), &jsonResp); err == nil && jsonResp.Content != "" {
		edit := CodeEdit{
			File:       jsonResp.File,
			NewContent: jsonResp.Content,
			Type:       task.Action,
			Language:   detectLanguage(jsonResp.File),
			Rationale:  jsonResp.Rationale,
		}
		if edit.File == "" {
			edit.File = task.Target
		}
		result.Edits = append(result.Edits, edit)

		// Capture artifact type for routing
		if jsonResp.ArtifactType != "" {
			result.ArtifactType = jsonResp.ArtifactType
		}

		// Extract tool name from file path for self-tools
		if result.ArtifactType == ArtifactTypeSelfTool || result.ArtifactType == ArtifactTypeDiagnostic {
			result.ToolName = extractToolName(edit.File)
		}

		return result
	}

	// Fallback: extract code from markdown code blocks
	codeBlockRegex := regexp.MustCompile("```(?:\\w+)?\\n([\\s\\S]*?)```")
	matches := codeBlockRegex.FindAllStringSubmatch(response, -1)
	if len(matches) > 0 {
		// Use the last code block (often the final answer)
		content := matches[len(matches)-1][1]
		edit := CodeEdit{
			File:       task.Target,
			NewContent: strings.TrimSpace(content),
			Type:       task.Action,
			Language:   detectLanguage(task.Target),
			Rationale:  "Generated from LLM response",
		}
		result.Edits = append(result.Edits, edit)
		return result
	}

	// Last resort: use raw response (for simple cases)
	if len(response) > 0 && strings.Contains(response, "\n") {
		edit := CodeEdit{
			File:       task.Target,
			NewContent: response,
			Type:       task.Action,
			Language:   detectLanguage(task.Target),
			Rationale:  "Raw LLM response",
		}
		result.Edits = append(result.Edits, edit)
	}

	return result
}

// extractToolName extracts a tool name from a file path.
// e.g., "check_knowledge_db.go" -> "check_knowledge_db"
func extractToolName(filePath string) string {
	// Get base name
	base := filePath
	if idx := strings.LastIndex(filePath, "/"); idx != -1 {
		base = filePath[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx != -1 {
		base = base[idx+1:]
	}
	// Remove extension
	if idx := strings.LastIndex(base, "."); idx != -1 {
		base = base[:idx]
	}
	return base
}
