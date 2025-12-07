package coder

import (
	"encoding/json"
	"regexp"
	"strings"
)

// =============================================================================
// RESPONSE PARSING
// =============================================================================

// parseCodeResponse extracts code edits from LLM response.
func (c *CoderShard) parseCodeResponse(response string, task CoderTask) []CodeEdit {
	edits := make([]CodeEdit, 0)

	// Try to parse as JSON first
	var jsonResp struct {
		File      string `json:"file"`
		Content   string `json:"content"`
		Rationale string `json:"rationale"`
	}

	// Find JSON in response (may be wrapped in markdown code blocks)
	jsonStr := response
	if idx := strings.Index(response, "{"); idx != -1 {
		endIdx := strings.LastIndex(response, "}")
		if endIdx > idx {
			jsonStr = response[idx : endIdx+1]
		}
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
		edits = append(edits, edit)
		return edits
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
		edits = append(edits, edit)
		return edits
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
		edits = append(edits, edit)
	}

	return edits
}
