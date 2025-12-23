package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolRenderer renders a compiled tool set into LLM-consumable format.
type ToolRenderer struct {
	includeSchemas bool
	maxSchemaLen   int
}

// NewToolRenderer creates a new tool renderer.
func NewToolRenderer() *ToolRenderer {
	return &ToolRenderer{
		includeSchemas: true,
		maxSchemaLen:   500,
	}
}

// SetIncludeSchemas sets whether to include JSON schemas in full tool output.
func (r *ToolRenderer) SetIncludeSchemas(include bool) {
	r.includeSchemas = include
}

// SetMaxSchemaLen sets the maximum length for JSON schemas.
func (r *ToolRenderer) SetMaxSchemaLen(maxLen int) {
	r.maxSchemaLen = maxLen
}

// Render renders a compiled tool set into markdown format for LLM context.
func (r *ToolRenderer) Render(tools *CompiledToolSet) string {
	var sb strings.Builder

	totalSelected := len(tools.FullTools) + len(tools.CondensedTools) + len(tools.MinimalTools)

	sb.WriteString(fmt.Sprintf("## Available MCP Tools (%d of %d)\n\n", totalSelected, tools.Stats.TotalTools))

	// Primary tools with full schema
	if len(tools.FullTools) > 0 {
		sb.WriteString("### Primary Tools\n\n")
		for _, t := range tools.FullTools {
			r.renderFullTool(&sb, &t)
		}
	}

	// Secondary tools with condensed info
	if len(tools.CondensedTools) > 0 {
		sb.WriteString("### Secondary Tools\n\n")
		for _, t := range tools.CondensedTools {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name, t.Condensed))
		}
		sb.WriteString("\n")
	}

	// Mention additional available tools
	if len(tools.MinimalTools) > 0 {
		sb.WriteString(fmt.Sprintf("### Additional Tools (%d more)\n\n", len(tools.MinimalTools)))
		sb.WriteString("Available on request: ")
		sb.WriteString(strings.Join(tools.MinimalTools, ", "))
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// renderFullTool renders a full tool description.
func (r *ToolRenderer) renderFullTool(sb *strings.Builder, tool *MCPTool) {
	sb.WriteString(fmt.Sprintf("#### %s\n", tool.Name))

	if tool.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))
	}

	// Show capabilities if available
	if len(tool.Capabilities) > 0 {
		sb.WriteString(fmt.Sprintf("**Capabilities:** %s\n", strings.Join(tool.Capabilities, ", ")))
	}

	// Show categories if available
	if len(tool.Categories) > 0 {
		sb.WriteString(fmt.Sprintf("**Categories:** %s\n", strings.Join(tool.Categories, ", ")))
	}

	// Include input schema if enabled
	if r.includeSchemas && len(tool.InputSchema) > 0 {
		schema := r.formatSchema(tool.InputSchema)
		if schema != "" {
			sb.WriteString(fmt.Sprintf("\n**Parameters:**\n```json\n%s\n```\n", schema))
		}
	}

	sb.WriteString("\n")
}

// formatSchema formats a JSON schema for display.
func (r *ToolRenderer) formatSchema(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try to parse and pretty print
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}

	// Pretty print with indentation
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}

	result := string(formatted)

	// Truncate if too long
	if r.maxSchemaLen > 0 && len(result) > r.maxSchemaLen {
		result = result[:r.maxSchemaLen] + "\n  ...(truncated)"
	}

	return result
}

// RenderCompact renders a compact single-line summary.
func (r *ToolRenderer) RenderCompact(tools *CompiledToolSet) string {
	var parts []string

	if len(tools.FullTools) > 0 {
		names := make([]string, len(tools.FullTools))
		for i, t := range tools.FullTools {
			names[i] = t.Name
		}
		parts = append(parts, fmt.Sprintf("primary: %s", strings.Join(names, ", ")))
	}

	if len(tools.CondensedTools) > 0 {
		names := make([]string, len(tools.CondensedTools))
		for i, t := range tools.CondensedTools {
			names[i] = t.Name
		}
		parts = append(parts, fmt.Sprintf("secondary: %s", strings.Join(names, ", ")))
	}

	if len(tools.MinimalTools) > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", len(tools.MinimalTools)))
	}

	return fmt.Sprintf("MCP Tools [%s]", strings.Join(parts, " | "))
}

// RenderJSON renders the tool set as JSON for structured output.
func (r *ToolRenderer) RenderJSON(tools *CompiledToolSet) (string, error) {
	output := struct {
		PrimaryTools   []ToolJSONEntry   `json:"primary_tools"`
		SecondaryTools []ToolJSONEntry   `json:"secondary_tools"`
		AdditionalTools []string         `json:"additional_tools"`
		Stats          ToolCompilationStats `json:"stats"`
	}{
		PrimaryTools:    make([]ToolJSONEntry, 0, len(tools.FullTools)),
		SecondaryTools:  make([]ToolJSONEntry, 0, len(tools.CondensedTools)),
		AdditionalTools: tools.MinimalTools,
		Stats:           tools.Stats,
	}

	for _, t := range tools.FullTools {
		output.PrimaryTools = append(output.PrimaryTools, ToolJSONEntry{
			Name:         t.Name,
			Description:  t.Description,
			Capabilities: t.Capabilities,
			Categories:   t.Categories,
			InputSchema:  t.InputSchema,
		})
	}

	for _, t := range tools.CondensedTools {
		output.SecondaryTools = append(output.SecondaryTools, ToolJSONEntry{
			Name:        t.Name,
			Description: t.Condensed,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ToolJSONEntry represents a tool in JSON output.
type ToolJSONEntry struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Categories   []string        `json:"categories,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
}

// RenderForInvocation renders tool information for invocation context.
// This is a minimal format showing just what's needed to call the tools.
func (r *ToolRenderer) RenderForInvocation(tools *CompiledToolSet) string {
	var sb strings.Builder

	sb.WriteString("## Available Tool Calls\n\n")

	for _, t := range tools.FullTools {
		sb.WriteString(fmt.Sprintf("### %s\n", t.Name))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n", t.Description))
		}
		if len(t.InputSchema) > 0 {
			sb.WriteString(fmt.Sprintf("Input: `%s`\n", string(t.InputSchema)))
		}
		sb.WriteString("\n")
	}

	if len(tools.CondensedTools) > 0 {
		sb.WriteString("### Also Available\n")
		for _, t := range tools.CondensedTools {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Condensed))
		}
	}

	return sb.String()
}
