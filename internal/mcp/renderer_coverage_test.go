package mcp

import (
	"encoding/json"
	"testing"
)

// --- NewToolRenderer ---

func TestNewToolRenderer_ShouldReturnDefaults(t *testing.T) {
	r := NewToolRenderer()
	if r == nil {
		t.Fatal("NewToolRenderer returned nil")
	}
	if !r.includeSchemas {
		t.Error("expected includeSchemas=true by default")
	}
	if r.maxSchemaLen != 500 {
		t.Errorf("expected maxSchemaLen=500, got %d", r.maxSchemaLen)
	}
}

func TestSetIncludeSchemas_ShouldToggle(t *testing.T) {
	r := NewToolRenderer()
	r.SetIncludeSchemas(false)
	if r.includeSchemas {
		t.Error("expected includeSchemas=false after SetIncludeSchemas(false)")
	}
}

func TestSetMaxSchemaLen_ShouldUpdate(t *testing.T) {
	r := NewToolRenderer()
	r.SetMaxSchemaLen(1000)
	if r.maxSchemaLen != 1000 {
		t.Errorf("expected maxSchemaLen=1000, got %d", r.maxSchemaLen)
	}
}

// --- Render ---

func TestRender_WhenEmptyToolSet_ShouldRenderHeader(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		Stats: ToolCompilationStats{TotalTools: 0},
	}

	result := r.Render(tools)
	if result == "" {
		t.Error("expected non-empty render output")
	}
	if !contains(result, "Available MCP Tools (0 of 0)") {
		t.Errorf("expected header with tool counts, got:\n%s", result)
	}
}

func TestRender_WhenFullToolsOnly_ShouldRenderPrimarySection(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{Name: "calc", Description: "Calculator"},
		},
		Stats: ToolCompilationStats{TotalTools: 1},
	}

	result := r.Render(tools)
	if !contains(result, "Primary Tools") {
		t.Error("expected 'Primary Tools' section")
	}
	if !contains(result, "calc") {
		t.Error("expected tool name 'calc' in output")
	}
}

func TestRender_WhenCondensedToolsOnly_ShouldRenderSecondarySection(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		CondensedTools: []ToolSummary{
			{Name: "helper", Condensed: "Helps with things"},
		},
		Stats: ToolCompilationStats{TotalTools: 1},
	}

	result := r.Render(tools)
	if !contains(result, "Secondary Tools") {
		t.Error("expected 'Secondary Tools' section")
	}
	if !contains(result, "helper") {
		t.Error("expected tool name 'helper' in output")
	}
}

func TestRender_WhenMinimalToolsOnly_ShouldRenderAdditionalSection(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		MinimalTools: []string{"a", "b", "c"},
		Stats:        ToolCompilationStats{TotalTools: 3},
	}

	result := r.Render(tools)
	if !contains(result, "Additional Tools (3 more)") {
		t.Error("expected 'Additional Tools' section")
	}
}

func TestRender_WhenAllTiers_ShouldRenderAllSections(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools:      []MCPTool{{Name: "full1", Description: "Full tool"}},
		CondensedTools: []ToolSummary{{Name: "cond1", Condensed: "Condensed tool"}},
		MinimalTools:   []string{"min1"},
		Stats:          ToolCompilationStats{TotalTools: 10},
	}

	result := r.Render(tools)
	if !contains(result, "Primary Tools") {
		t.Error("expected Primary section")
	}
	if !contains(result, "Secondary Tools") {
		t.Error("expected Secondary section")
	}
	if !contains(result, "Additional Tools") {
		t.Error("expected Additional section")
	}
	if !contains(result, "3 of 10") {
		t.Error("expected correct count in header")
	}
}

// --- renderFullTool ---

func TestRenderFullTool_WhenNoSchemas_ShouldRenderBasicInfo(t *testing.T) {
	r := NewToolRenderer()
	r.SetIncludeSchemas(false)
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{
				Name:         "mytool",
				Description:  "Does things",
				Capabilities: []string{"read", "write"},
				Categories:   []string{"filesystem"},
			},
		},
		Stats: ToolCompilationStats{TotalTools: 1},
	}

	result := r.Render(tools)
	if !contains(result, "mytool") {
		t.Error("expected tool name")
	}
	if !contains(result, "Does things") {
		t.Error("expected description")
	}
	if !contains(result, "read, write") {
		t.Error("expected capabilities")
	}
	if !contains(result, "filesystem") {
		t.Error("expected categories")
	}
}

func TestRenderFullTool_WhenSchemaIncluded_ShouldRenderJSON(t *testing.T) {
	r := NewToolRenderer()
	r.SetIncludeSchemas(true)
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{
				Name:        "tool",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"}}}`),
			},
		},
		Stats: ToolCompilationStats{TotalTools: 1},
	}

	result := r.Render(tools)
	if !contains(result, "Parameters") {
		t.Error("expected Parameters section")
	}
	if !contains(result, "number") {
		t.Error("expected schema content")
	}
}

// --- formatSchema ---

func TestFormatSchema_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	r := NewToolRenderer()
	result := r.formatSchema(nil)
	if result != "" {
		t.Errorf("expected empty string for nil schema, got %q", result)
	}

	result = r.formatSchema(json.RawMessage{})
	if result != "" {
		t.Errorf("expected empty string for empty schema, got %q", result)
	}
}

func TestFormatSchema_WhenInvalidJSON_ShouldReturnRaw(t *testing.T) {
	r := NewToolRenderer()
	raw := json.RawMessage(`{bad json}`)
	result := r.formatSchema(raw)
	if result != `{bad json}` {
		t.Errorf("expected raw string for invalid JSON, got %q", result)
	}
}

func TestFormatSchema_WhenLongSchema_ShouldTruncate(t *testing.T) {
	r := NewToolRenderer()
	r.SetMaxSchemaLen(20)

	raw := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"number"}}}`)
	result := r.formatSchema(raw)
	if len(result) <= 20 {
		// The truncated result should have the suffix
	}
	if !contains(result, "truncated") {
		t.Error("expected truncation indicator")
	}
}

// --- RenderCompact ---

func TestRenderCompact_WhenEmpty_ShouldReturnMinimal(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{}

	result := r.RenderCompact(tools)
	if result != "MCP Tools []" {
		t.Errorf("expected 'MCP Tools []', got %q", result)
	}
}

func TestRenderCompact_WhenAllTiers_ShouldIncludeAll(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools:      []MCPTool{{Name: "a"}},
		CondensedTools: []ToolSummary{{Name: "b"}},
		MinimalTools:   []string{"c", "d"},
	}

	result := r.RenderCompact(tools)
	if !contains(result, "primary: a") {
		t.Error("expected primary section")
	}
	if !contains(result, "secondary: b") {
		t.Error("expected secondary section")
	}
	if !contains(result, "+2 more") {
		t.Error("expected +2 more")
	}
}

// --- RenderJSON ---

func TestRenderJSON_ShouldProduceValidJSON(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{Name: "tool1", Description: "desc1", Capabilities: []string{"cap1"}},
		},
		CondensedTools: []ToolSummary{
			{Name: "tool2", Condensed: "short desc"},
		},
		MinimalTools: []string{"tool3"},
		Stats:        ToolCompilationStats{TotalTools: 3},
	}

	jsonStr, err := r.RenderJSON(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	primary, ok := parsed["primary_tools"].([]interface{})
	if !ok || len(primary) != 1 {
		t.Error("expected 1 primary tool")
	}
}

func TestRenderJSON_WhenEmpty_ShouldProduceValidJSON(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{}

	jsonStr, err := r.RenderJSON(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jsonStr == "" {
		t.Error("expected non-empty JSON output")
	}
}

// --- RenderForInvocation ---

func TestRenderForInvocation_ShouldRenderToolCalls(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{Name: "calc", Description: "Calculator", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		CondensedTools: []ToolSummary{
			{Name: "helper", Condensed: "Helps"},
		},
	}

	result := r.RenderForInvocation(tools)
	if !contains(result, "Available Tool Calls") {
		t.Error("expected header")
	}
	if !contains(result, "calc") {
		t.Error("expected full tool name")
	}
	if !contains(result, "Also Available") {
		t.Error("expected 'Also Available' section for condensed tools")
	}
}

func TestRenderForInvocation_WhenNoCondensed_ShouldSkipSection(t *testing.T) {
	r := NewToolRenderer()
	tools := &CompiledToolSet{
		FullTools: []MCPTool{
			{Name: "calc", Description: "Calculator"},
		},
	}

	result := r.RenderForInvocation(tools)
	if contains(result, "Also Available") {
		t.Error("should not have 'Also Available' section when no condensed tools")
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
