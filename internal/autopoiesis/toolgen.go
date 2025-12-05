// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains the ToolGenerator for dynamic tool creation.
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// =============================================================================
// TOOL GENERATOR
// =============================================================================
// Detects when existing tools are insufficient and generates new ones.
// This is core autopoiesis - the system modifying itself to gain new capabilities.

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

// GeneratedTool contains the generated tool code and metadata
type GeneratedTool struct {
	Name         string            // Tool name (e.g., "json_validator")
	Package      string            // Go package name
	Description  string            // Human-readable description
	Code         string            // Generated Go code
	TestCode     string            // Generated test code
	Schema       ToolSchema        // JSON schema for inputs
	Dependencies []string          // Required imports
	FilePath     string            // Where to write the tool
	Validated    bool              // Whether code has been validated
	Errors       []string          // Any validation errors
}

// ToolSchema defines the input/output schema for a tool
type ToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]ParamSchema `json:"parameters"`
	Required    []string               `json:"required"`
	Returns     string                 `json:"returns"`
}

// ParamSchema defines a single parameter
type ParamSchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolGenerator detects tool needs and generates new tools
type ToolGenerator struct {
	client       LLMClient
	toolsDir     string // Directory where tools are stored
	existingTools map[string]ToolSchema
}

// NewToolGenerator creates a new tool generator
func NewToolGenerator(client LLMClient, toolsDir string) *ToolGenerator {
	return &ToolGenerator{
		client:       client,
		toolsDir:     toolsDir,
		existingTools: make(map[string]ToolSchema),
	}
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
	}

	if !needsNewTool {
		return nil, nil // No tool need detected
	}

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

	// Use LLM to refine the tool need
	need, err := tg.refineToolNeedWithLLM(ctx, input, failedAttempt, toolType, triggers)
	if err != nil {
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

// GenerateTool creates a new tool based on the detected need
func (tg *ToolGenerator) GenerateTool(ctx context.Context, need *ToolNeed) (*GeneratedTool, error) {
	// Generate the tool code using LLM
	code, err := tg.generateToolCode(ctx, need)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool code: %w", err)
	}

	// Generate test code
	testCode, err := tg.generateTestCode(ctx, need, code)
	if err != nil {
		// Non-fatal - we can proceed without tests
		testCode = fmt.Sprintf("// TODO: Generate tests for %s\npackage tools\n", need.Name)
	}

	// Generate schema
	schema := tg.generateSchema(need)

	// Determine file path
	filePath := filepath.Join(tg.toolsDir, fmt.Sprintf("%s.go", need.Name))

	tool := &GeneratedTool{
		Name:        need.Name,
		Package:     "tools",
		Description: need.Purpose,
		Code:        code,
		TestCode:    testCode,
		Schema:      schema,
		FilePath:    filePath,
		Validated:   false,
	}

	// Validate the generated code
	if err := tg.validateCode(tool); err != nil {
		tool.Errors = append(tool.Errors, err.Error())
	} else {
		tool.Validated = true
	}

	return tool, nil
}

// generateToolCode uses LLM to generate the actual Go code
func (tg *ToolGenerator) generateToolCode(ctx context.Context, need *ToolNeed) (string, error) {
	systemPrompt := `You are a Go code generator for the codeNERD agent system.
Generate clean, idiomatic Go code that follows these conventions:
- Use standard library where possible
- Include proper error handling
- Add clear comments
- Make functions testable
- Follow Go naming conventions

The tool should be a standalone function that can be called by the agent.
Include a Register function to add the tool to the tool registry.`

	userPrompt := fmt.Sprintf(`Generate a Go tool with these specifications:

Tool Name: %s
Purpose: %s
Input Type: %s
Output Type: %s

The tool should be in package "tools" and include:
1. Main tool function: %s(ctx context.Context, input %s) (%s, error)
2. Registration function: Register%s(registry ToolRegistry) error
3. Tool description constant

Generate complete, compilable Go code:`,
		need.Name, need.Purpose, need.InputType, need.OutputType,
		toCamelCase(need.Name), need.InputType, need.OutputType,
		toPascalCase(need.Name))

	code, err := tg.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Extract code block from response
	return extractCodeBlock(code, "go"), nil
}

// generateTestCode generates tests for the tool
func (tg *ToolGenerator) generateTestCode(ctx context.Context, need *ToolNeed, code string) (string, error) {
	prompt := fmt.Sprintf(`Generate Go test code for this tool:

Tool Code:
%s

Generate comprehensive tests including:
1. Happy path tests
2. Error case tests
3. Edge case tests

Generate complete test file in package "tools":`, code)

	testCode, err := tg.client.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return extractCodeBlock(testCode, "go"), nil
}

// generateSchema creates a JSON schema for the tool
func (tg *ToolGenerator) generateSchema(need *ToolNeed) ToolSchema {
	return ToolSchema{
		Name:        need.Name,
		Description: need.Purpose,
		Parameters: map[string]ParamSchema{
			"input": {
				Type:        need.InputType,
				Description: "Input for " + need.Name,
			},
		},
		Required: []string{"input"},
		Returns:  need.OutputType,
	}
}

// validateCode performs basic validation on generated code
func (tg *ToolGenerator) validateCode(tool *GeneratedTool) error {
	// Check for required elements
	if !strings.Contains(tool.Code, "package ") {
		return fmt.Errorf("missing package declaration")
	}

	if !strings.Contains(tool.Code, "func ") {
		return fmt.Errorf("missing function declaration")
	}

	// Check for common issues
	if strings.Contains(tool.Code, "panic(") && !strings.Contains(tool.Code, "recover") {
		tool.Errors = append(tool.Errors, "warning: code contains panic without recover")
	}

	// Check imports are balanced
	importCount := strings.Count(tool.Code, "import")
	if importCount > 0 && !strings.Contains(tool.Code, ")") {
		return fmt.Errorf("malformed import block")
	}

	return nil
}

// WriteTool writes the generated tool to disk
func (tg *ToolGenerator) WriteTool(tool *GeneratedTool) error {
	// Ensure directory exists
	dir := filepath.Dir(tool.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write main code
	if err := os.WriteFile(tool.FilePath, []byte(tool.Code), 0644); err != nil {
		return fmt.Errorf("failed to write tool code: %w", err)
	}

	// Write test code
	testPath := strings.TrimSuffix(tool.FilePath, ".go") + "_test.go"
	if err := os.WriteFile(testPath, []byte(tool.TestCode), 0644); err != nil {
		// Non-fatal
		tool.Errors = append(tool.Errors, fmt.Sprintf("failed to write test code: %v", err))
	}

	return nil
}

// RegisterTool adds the tool to the registry (in-memory for hot reload)
func (tg *ToolGenerator) RegisterTool(tool *GeneratedTool) error {
	tg.existingTools[tool.Name] = tool.Schema
	return nil
}

// listExistingTools returns names of existing tools
func (tg *ToolGenerator) listExistingTools() []string {
	tools := make([]string, 0, len(tg.existingTools))
	for name := range tg.existingTools {
		tools = append(tools, name)
	}
	return tools
}

// LoadExistingTools loads tool schemas from the tools directory
func (tg *ToolGenerator) LoadExistingTools() error {
	pattern := filepath.Join(tg.toolsDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		// Extract tool name from filename
		base := filepath.Base(file)
		name := strings.TrimSuffix(base, ".go")

		// Read file and extract description (basic parsing)
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Look for description constant or comment
		desc := extractDescription(string(content))

		tg.existingTools[name] = ToolSchema{
			Name:        name,
			Description: desc,
		}
	}

	return nil
}

// =============================================================================
// TOOL TEMPLATES
// =============================================================================

// ToolTemplate is a template for generating common tool types
type ToolTemplate struct {
	Name     string
	Template *template.Template
}

var toolTemplates = map[string]string{
	"validator": `package tools

import (
	"context"
	"fmt"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// {{.FuncName}} validates {{.InputType}} input
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) (bool, error) {
	// TODO: Implement validation logic
	if input == {{.ZeroValue}} {
		return false, fmt.Errorf("empty input")
	}

	// Validation logic here
	return true, nil
}

// Register{{.PascalName}} adds this tool to the registry
func Register{{.PascalName}}(registry ToolRegistry) error {
	return registry.Register("{{.Name}}", {{.Name}}Description, {{.FuncName}})
}
`,
	"converter": `package tools

import (
	"context"
	"fmt"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// {{.FuncName}} converts input from one format to another
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) ({{.OutputType}}, error) {
	var result {{.OutputType}}

	if input == {{.ZeroValue}} {
		return result, fmt.Errorf("empty input")
	}

	// Conversion logic here
	return result, nil
}

// Register{{.PascalName}} adds this tool to the registry
func Register{{.PascalName}}(registry ToolRegistry) error {
	return registry.Register("{{.Name}}", {{.Name}}Description, {{.FuncName}})
}
`,
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// extractJSON extracts a JSON object from text
func extractJSON(text string) string {
	// Find first { and last }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		return "{}"
	}

	return text[start : end+1]
}

// extractCodeBlock extracts a code block from markdown-style response
func extractCodeBlock(text, lang string) string {
	// Look for ```go or ``` blocks
	patterns := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(text, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(text[start:], "```")
			if end != -1 {
				return strings.TrimSpace(text[start : start+end])
			}
		}
	}

	// If no code block found, return the whole text (might be raw code)
	return strings.TrimSpace(text)
}

// extractDescription extracts tool description from Go source
func extractDescription(code string) string {
	// Look for Description constant
	descPattern := regexp.MustCompile(`(?m)const\s+\w*Description\s*=\s*"([^"]+)"`)
	if matches := descPattern.FindStringSubmatch(code); len(matches) > 1 {
		return matches[1]
	}

	// Look for package comment
	if strings.HasPrefix(code, "//") {
		lines := strings.Split(code, "\n")
		if len(lines) > 0 {
			return strings.TrimPrefix(lines[0], "// ")
		}
	}

	return "No description available"
}

// toCamelCase converts snake_case to camelCase
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 0; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
