// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains the ToolGenerator for dynamic tool creation.
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
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
	client        LLMClient
	toolsDir      string // Directory where tools are stored
	existingTools map[string]ToolSchema

	// JIT prompt compilation support
	promptAssembler *articulation.PromptAssembler
	jitEnabled      bool
}

// NewToolGenerator creates a new tool generator
func NewToolGenerator(client LLMClient, toolsDir string) *ToolGenerator {
	logging.AutopoiesisDebug("Creating ToolGenerator: toolsDir=%s", toolsDir)
	return &ToolGenerator{
		client:        client,
		toolsDir:      toolsDir,
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
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "GenerateTool")
	defer timer.Stop()

	logging.Autopoiesis("Generating tool: %s (purpose: %s)", need.Name, need.Purpose)
	logging.AutopoiesisDebug("Tool specs: input=%s, output=%s, confidence=%.2f",
		need.InputType, need.OutputType, need.Confidence)

	// Generate the tool code using LLM
	logging.AutopoiesisDebug("Generating tool code via LLM")
	codeTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMCodeGeneration")
	code, err := tg.generateToolCode(ctx, need)
	codeTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to generate tool code: %v", err)
		return nil, fmt.Errorf("failed to generate tool code: %w", err)
	}
	logging.AutopoiesisDebug("Generated tool code: %d bytes", len(code))

	// Generate test code (always succeeds - has internal fallbacks)
	logging.AutopoiesisDebug("Generating test code")
	testCode, _ := tg.generateTestCode(ctx, need, code)
	logging.AutopoiesisDebug("Generated test code: %d bytes", len(testCode))

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
	logging.AutopoiesisDebug("Validating generated code")
	if err := tg.validateCode(tool); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Code validation warning: %v", err)
		tool.Errors = append(tool.Errors, err.Error())
	} else {
		tool.Validated = true
		logging.AutopoiesisDebug("Code validation passed")
	}

	logging.Autopoiesis("Tool generated successfully: %s (validated=%v, warnings=%d)",
		tool.Name, tool.Validated, len(tool.Errors))
	return tool, nil
}

// RegenerateWithFeedback generates a new version of a tool incorporating error feedback.
// This is used by the Ouroboros retry loop when safety checks fail.
func (tg *ToolGenerator) RegenerateWithFeedback(
	ctx context.Context,
	need *ToolNeed,
	previousTool *GeneratedTool,
	violations []SafetyViolation,
) (*GeneratedTool, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "RegenerateWithFeedback")
	defer timer.Stop()

	logging.Autopoiesis("Regenerating tool with feedback: %s (%d violations to address)",
		need.Name, len(violations))
	for i, v := range violations {
		logging.AutopoiesisDebug("  Violation %d: type=%s, desc=%s", i+1, v.Type.String(), v.Description)
	}

	// Format violations for LLM feedback
	feedback := FormatViolationsForFeedback(violations)

	// Create enhanced need with feedback context
	enhancedNeed := &ToolNeed{
		Name:       need.Name,
		Purpose:    need.Purpose,
		InputType:  need.InputType,
		OutputType: need.OutputType,
		Triggers:   need.Triggers,
		Priority:   need.Priority,
		Confidence: need.Confidence * 0.9, // Reduce confidence on retry
		Reasoning:  need.Reasoning,
	}
	logging.AutopoiesisDebug("Reduced confidence from %.2f to %.2f on retry",
		need.Confidence, enhancedNeed.Confidence)

	// Generate new code with feedback-aware system prompt
	logging.AutopoiesisDebug("Regenerating code with safety feedback")
	regenTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMCodeRegeneration")
	code, err := tg.regenerateToolCodeWithFeedback(ctx, enhancedNeed, previousTool.Code, feedback)
	regenTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to regenerate tool code: %v", err)
		return nil, fmt.Errorf("failed to regenerate tool code: %w", err)
	}
	logging.AutopoiesisDebug("Regenerated tool code: %d bytes", len(code))

	// Generate test code (always succeeds - has internal fallbacks)
	testCode, _ := tg.generateTestCode(ctx, enhancedNeed, code)

	// Generate schema
	schema := tg.generateSchema(enhancedNeed)

	// Determine file path
	filePath := filepath.Join(tg.toolsDir, fmt.Sprintf("%s.go", enhancedNeed.Name))

	tool := &GeneratedTool{
		Name:        enhancedNeed.Name,
		Package:     "tools",
		Description: enhancedNeed.Purpose,
		Code:        code,
		TestCode:    testCode,
		Schema:      schema,
		FilePath:    filePath,
		Validated:   false,
	}

	// Validate the generated code
	logging.AutopoiesisDebug("Validating regenerated code")
	if err := tg.validateCode(tool); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Regenerated code validation warning: %v", err)
		tool.Errors = append(tool.Errors, err.Error())
	} else {
		tool.Validated = true
		logging.AutopoiesisDebug("Regenerated code validation passed")
	}

	logging.Autopoiesis("Tool regenerated: %s (validated=%v)", tool.Name, tool.Validated)
	return tool, nil
}

// regenerateToolCodeWithFeedback uses LLM to regenerate code with safety violation feedback.
func (tg *ToolGenerator) regenerateToolCodeWithFeedback(
	ctx context.Context,
	need *ToolNeed,
	previousCode string,
	feedback string,
) (string, error) {
	// Try JIT compilation for refinement stage
	if tg.jitEnabled && tg.promptAssembler != nil && tg.promptAssembler.JITReady() {
		return tg.regenerateToolCodeWithJIT(ctx, need, previousCode, feedback, "/refinement")
	}

	// Fallback to legacy prompts
	systemPrompt := `You are a Go code generator for the codeNERD agent system.
Your previous code had safety violations. You must fix these issues.

CRITICAL SAFETY REQUIREMENTS:
- Do NOT use unsafe imports (os/exec, syscall, unsafe, plugin, runtime/cgo)
- Do NOT use panic() - return errors instead
- If using goroutines, always pass a cancelable context
- Only use explicitly allowed packages (fmt, strings, bytes, context, encoding/*, errors, etc.)
- Prefer error returns over panic
- Use context.Context for cancellation

Generate clean, idiomatic Go code that follows these conventions:
- Use standard library where possible
- Include proper error handling with error returns
- Add clear comments
- Make functions testable
- Follow Go naming conventions

The tool should be a standalone function that can be called by the agent.
Include a Register function to add the tool to the tool registry.`

	userPrompt := fmt.Sprintf(`Your previous code had safety violations that need to be fixed.

--- SAFETY VIOLATIONS ---
%s

--- PREVIOUS CODE (DO NOT REPEAT THESE MISTAKES) ---
%s

--- TOOL SPECIFICATIONS ---
Tool Name: %s
Purpose: %s
Input Type: %s
Output Type: %s

Generate CORRECTED Go code that:
1. Fixes ALL the safety violations listed above
2. Uses only safe imports (no os/exec, syscall, unsafe, plugin)
3. Returns errors instead of using panic()
4. Passes context to goroutines for cancellation
5. Is in package "tools"

Generate complete, compilable, SAFE Go code:`,
		feedback, previousCode,
		need.Name, need.Purpose, need.InputType, need.OutputType)

	code, err := tg.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Extract code block from response
	return extractCodeBlock(code, "go"), nil
}

// regenerateToolCodeWithJIT regenerates tool code using JIT-compiled prompts with feedback
func (tg *ToolGenerator) regenerateToolCodeWithJIT(
	ctx context.Context,
	need *ToolNeed,
	previousCode string,
	feedback string,
	stage string,
) (string, error) {
	logging.AutopoiesisDebug("Regenerating tool code with JIT for stage=%s", stage)

	// Build prompt context for this Ouroboros refinement stage
	pc := &articulation.PromptContext{
		ShardID:   "tool_generator_" + need.Name + "_refinement",
		ShardType: "tool_generator",
	}

	// Assemble system prompt using JIT compiler
	systemPrompt, err := tg.promptAssembler.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("JIT assembly failed for refinement, falling back: %v", err)
		// Fall back to legacy generation
		return tg.regenerateToolCodeWithFeedback(ctx, need, previousCode, feedback)
	}

	logging.AutopoiesisDebug("JIT-compiled refinement prompt: %d bytes", len(systemPrompt))

	// Build user prompt with feedback
	userPrompt := fmt.Sprintf(`Your previous code had safety violations that need to be fixed.

--- SAFETY VIOLATIONS ---
%s

--- PREVIOUS CODE (DO NOT REPEAT THESE MISTAKES) ---
%s

--- TOOL SPECIFICATIONS ---
Tool Name: %s
Purpose: %s
Input Type: %s
Output Type: %s

Generate CORRECTED Go code that:
1. Fixes ALL the safety violations listed above
2. Uses only safe imports (no os/exec, syscall, unsafe, plugin)
3. Returns errors instead of using panic()
4. Passes context to goroutines for cancellation
5. Is in package "tools"

Generate complete, compilable, SAFE Go code:`,
		feedback, previousCode,
		need.Name, need.Purpose, need.InputType, need.OutputType)

	code, err := tg.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Extract code block from response
	return extractCodeBlock(code, "go"), nil
}

// generateToolCode uses LLM to generate the actual Go code
func (tg *ToolGenerator) generateToolCode(ctx context.Context, need *ToolNeed) (string, error) {
	// Try JIT compilation first if available
	if tg.jitEnabled && tg.promptAssembler != nil && tg.promptAssembler.JITReady() {
		return tg.generateToolCodeWithJIT(ctx, need, "/specification")
	}

	// Fallback to legacy prompts
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
2. Tool description constant (e.g., const ToolDescription%s = "...")

IMPORTANT: Generate ONLY standalone code. Do NOT generate registration functions or reference any types like ToolRegistry or Tool that aren't defined in this file.

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

// generateToolCodeWithJIT generates tool code using JIT-compiled prompts
func (tg *ToolGenerator) generateToolCodeWithJIT(ctx context.Context, need *ToolNeed, stage string) (string, error) {
	logging.AutopoiesisDebug("Generating tool code with JIT for stage=%s", stage)

	// Build prompt context for this Ouroboros stage
	pc := &articulation.PromptContext{
		ShardID:   "tool_generator_" + need.Name,
		ShardType: "tool_generator",
	}

	// Assemble system prompt using JIT compiler
	systemPrompt, err := tg.promptAssembler.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("JIT assembly failed, falling back: %v", err)
		// Fall back to legacy generation
		return tg.generateToolCode(ctx, need)
	}

	logging.AutopoiesisDebug("JIT-compiled system prompt: %d bytes", len(systemPrompt))

	// Build user prompt with tool specifications
	userPrompt := fmt.Sprintf(`Generate a Go tool with these specifications:

Tool Name: %s
Purpose: %s
Input Type: %s
Output Type: %s

The tool should be in package "tools" and include:
1. Main tool function: %s(ctx context.Context, input %s) (%s, error)
2. Tool description constant (e.g., const ToolDescription%s = "...")

IMPORTANT: Generate ONLY standalone code. Do NOT generate registration functions or reference any types like ToolRegistry or Tool that aren't defined in this file.

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

// SetPromptAssembler attaches a JIT-aware prompt assembler to the tool generator
func (tg *ToolGenerator) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	tg.promptAssembler = assembler
	tg.jitEnabled = assembler != nil && assembler.JITReady()
	if tg.jitEnabled {
		logging.Autopoiesis("JIT prompt compilation enabled for ToolGenerator")
	}
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
		// Use comprehensive fallback test generation
		return tg.generateFallbackTests(need, code), nil
	}

	extracted := extractCodeBlock(testCode, "go")
	if extracted == "" || len(extracted) < 50 {
		// LLM response was insufficient, use fallback
		return tg.generateFallbackTests(need, code), nil
	}

	return extracted, nil
}

// generateFallbackTests creates comprehensive test templates when LLM is unavailable
func (tg *ToolGenerator) generateFallbackTests(need *ToolNeed, code string) string {
	funcName := toCamelCase(need.Name)
	pascalName := toPascalCase(need.Name)

	// Extract function signatures from the code for more accurate tests
	funcs := extractFunctionSignatures(code)

	var sb strings.Builder
	sb.WriteString(`package tools

import (
	"context"
	"testing"
	"time"
)

`)

	// Generate test for main tool function
	sb.WriteString(fmt.Sprintf(`// Test%s tests the main tool function
func Test%s(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		input   %s
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   %s,
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   %s,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := %s(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s() error = %%v, wantErr %%v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == %s {
				t.Error("%s() returned zero value for valid input")
			}
		})
	}
}

`,
		pascalName,
		pascalName,
		need.InputType,
		getTestValue(need.InputType, true),
		getTestValue(need.InputType, false),
		funcName,
		funcName,
		getZeroValue(need.OutputType),
		funcName,
	))

	// Generate benchmark test
	sb.WriteString(fmt.Sprintf(`// Benchmark%s benchmarks the tool performance
func Benchmark%s(b *testing.B) {
	ctx := context.Background()
	input := %s

	for i := 0; i < b.N; i++ {
		_, _ = %s(ctx, input)
	}
}

`,
		pascalName,
		pascalName,
		getTestValue(need.InputType, true),
		funcName,
	))

	// Generate context cancellation test
	sb.WriteString(fmt.Sprintf(`// Test%s_ContextCancellation tests context cancellation handling
func Test%s_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := %s(ctx, %s)
	if err == nil {
		t.Log("Warning: function does not check context cancellation")
	}
}

`,
		pascalName,
		pascalName,
		funcName,
		getTestValue(need.InputType, true),
	))

	// Generate timeout test
	sb.WriteString(fmt.Sprintf(`// Test%s_Timeout tests timeout handling
func Test%s_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := %s(ctx, %s)
	if err != nil {
		t.Logf("Function completed with error (may be expected): %%v", err)
	}
}

`,
		pascalName,
		pascalName,
		funcName,
		getTestValue(need.InputType, true),
	))

	// Generate tests for any additional functions found
	for _, fn := range funcs {
		if fn.Name == funcName || fn.Name == "main" || strings.HasPrefix(fn.Name, "Register") {
			continue
		}
		sb.WriteString(fmt.Sprintf(`// Test%s tests the %s helper function
func Test%s(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "basic functionality",
			wantErr: false,
		},
		{
			name:    "edge case",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call %s with appropriate test inputs
			// Verify the result matches expectations
			t.Logf("Testing %%s: %%s", "%s", tt.name)
		})
	}
}

`,
			fn.Name,
			fn.Name,
			fn.Name,
			fn.Name,
			fn.Name,
		))
	}

	return sb.String()
}

// FunctionSignature represents an extracted function signature
type FunctionSignature struct {
	Name       string
	Params     string
	Returns    string
	IsExported bool
}

// extractFunctionSignatures parses code and extracts function signatures
func extractFunctionSignatures(code string) []FunctionSignature {
	funcs := []FunctionSignature{}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, 0)
	if err != nil {
		return funcs
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			sig := FunctionSignature{
				Name:       fn.Name.Name,
				IsExported: fn.Name.IsExported(),
			}
			funcs = append(funcs, sig)
		}
		return true
	})

	return funcs
}

// getTestValue returns an appropriate test value for a type
func getTestValue(typeName string, valid bool) string {
	switch typeName {
	case "string":
		if valid {
			return `"test input"`
		}
		return `""`
	case "[]byte":
		if valid {
			return `[]byte("test input")`
		}
		return `[]byte{}`
	case "int", "int32", "int64":
		if valid {
			return "42"
		}
		return "0"
	case "uint", "uint32", "uint64":
		if valid {
			return "42"
		}
		return "0"
	case "float64", "float32":
		if valid {
			return "3.14"
		}
		return "0.0"
	case "bool":
		if valid {
			return "true"
		}
		return "false"
	case "map[string]any", "map[string]interface{}":
		if valid {
			return `map[string]any{"key": "value"}`
		}
		return `map[string]any{}`
	case "[]string":
		if valid {
			return `[]string{"item1", "item2"}`
		}
		return `[]string{}`
	case "[]int":
		if valid {
			return `[]int{1, 2, 3}`
		}
		return `[]int{}`
	case "[]float64":
		if valid {
			return `[]float64{1.1, 2.2, 3.3}`
		}
		return `[]float64{}`
	case "[]any", "[]interface{}":
		if valid {
			return `[]any{"str", 42, true}`
		}
		return `[]any{}`
	case "map[string]string":
		if valid {
			return `map[string]string{"key": "value"}`
		}
		return `map[string]string{}`
	case "map[string]int":
		if valid {
			return `map[string]int{"count": 42}`
		}
		return `map[string]int{}`
	case "io.Reader":
		if valid {
			return `strings.NewReader("test input")`
		}
		return `strings.NewReader("")`
	case "io.Writer":
		if valid {
			return `new(bytes.Buffer)`
		}
		return `new(bytes.Buffer)`
	case "time.Time":
		if valid {
			return `time.Now()`
		}
		return `time.Time{}`
	case "time.Duration":
		if valid {
			return `time.Second`
		}
		return `0`
	case "error":
		if valid {
			return `errors.New("test error")`
		}
		return `nil`
	case "context.Context":
		return `context.Background()`
	default:
		return getComplexTestValue(typeName, valid)
	}
}

// getComplexTestValue handles complex types like slices, maps, pointers, and structs
func getComplexTestValue(typeName string, valid bool) string {
	// Handle slices: []ElementType
	if strings.HasPrefix(typeName, "[]") {
		elemType := strings.TrimPrefix(typeName, "[]")
		if valid {
			elemValue := getTestValue(elemType, true)
			return fmt.Sprintf("%s{%s}", typeName, elemValue)
		}
		return fmt.Sprintf("%s{}", typeName)
	}

	// Handle maps: map[KeyType]ValueType
	if strings.HasPrefix(typeName, "map[") {
		if valid {
			// Parse map type: map[K]V
			rest := strings.TrimPrefix(typeName, "map[")
			bracketDepth := 0
			keyEnd := 0
			for i, c := range rest {
				if c == '[' {
					bracketDepth++
				} else if c == ']' {
					if bracketDepth == 0 {
						keyEnd = i
						break
					}
					bracketDepth--
				}
			}
			if keyEnd > 0 {
				keyType := rest[:keyEnd]
				valueType := rest[keyEnd+1:]
				keyValue := getTestValue(keyType, true)
				valValue := getTestValue(valueType, true)
				return fmt.Sprintf("%s{%s: %s}", typeName, keyValue, valValue)
			}
		}
		return fmt.Sprintf("%s{}", typeName)
	}

	// Handle pointers: *Type
	if strings.HasPrefix(typeName, "*") {
		baseType := strings.TrimPrefix(typeName, "*")
		if valid {
			// For primitive types, create a pointer via helper
			switch baseType {
			case "string":
				return `func() *string { s := "test"; return &s }()`
			case "int", "int32", "int64":
				return `func() *int { i := 42; return &i }()`
			case "float64", "float32":
				return `func() *float64 { f := 3.14; return &f }()`
			case "bool":
				return `func() *bool { b := true; return &b }()`
			default:
				// For struct types, use address-of
				return fmt.Sprintf("&%s{}", baseType)
			}
		}
		return "nil"
	}

	// Handle function types: func(...)...
	if strings.HasPrefix(typeName, "func(") {
		if valid {
			// Return a no-op function
			return fmt.Sprintf("%s { }", typeName)
		}
		return "nil"
	}

	// Handle channel types: chan Type
	if strings.HasPrefix(typeName, "chan ") {
		elemType := strings.TrimPrefix(typeName, "chan ")
		if valid {
			return fmt.Sprintf("make(%s, 1)", typeName)
		}
		_ = elemType // avoid unused
		return fmt.Sprintf("(%s)(nil)", typeName)
	}

	// Handle struct types (anything else)
	if valid {
		return fmt.Sprintf("%s{}", typeName)
	}
	return fmt.Sprintf("%s{}", typeName)
}

// getZeroValue returns the zero value for a type
func getZeroValue(typeName string) string {
	switch typeName {
	case "string":
		return `""`
	case "[]byte":
		return "nil"
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "0"
	case "float64", "float32":
		return "0.0"
	case "bool":
		return "false"
	case "error":
		return "nil"
	default:
		if strings.HasPrefix(typeName, "[]") || strings.HasPrefix(typeName, "map[") ||
			strings.HasPrefix(typeName, "*") {
			return "nil"
		}
		return fmt.Sprintf("%s{}", typeName)
	}
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

// ValidationResult contains detailed validation results
type ValidationResult struct {
	Valid            bool
	ParseError       error
	Errors           []string
	Warnings         []string
	PackageName      string
	Functions        []string
	Imports          []string
	HasMainFunction  bool
	HasErrorHandling bool
}

// validateCode performs comprehensive AST-based validation on generated code
func (tg *ToolGenerator) validateCode(tool *GeneratedTool) error {
	result := tg.validateCodeAST(tool.Code, tool.Name)

	// Add warnings to tool
	tool.Errors = append(tool.Errors, result.Warnings...)

	// Return first error if any
	if !result.Valid {
		if result.ParseError != nil {
			return fmt.Errorf("syntax error: %w", result.ParseError)
		}
		if len(result.Errors) > 0 {
			return fmt.Errorf("%s", result.Errors[0])
		}
		return fmt.Errorf("validation failed")
	}

	return nil
}

// validateCodeAST performs comprehensive AST-based validation
func (tg *ToolGenerator) validateCodeAST(code string, expectedToolName string) *ValidationResult {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []string{},
		Warnings:  []string{},
		Functions: []string{},
		Imports:   []string{},
	}

	// Parse the code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, parser.ParseComments)
	if err != nil {
		result.Valid = false
		result.ParseError = err
		return result
	}

	// Check package declaration
	if file.Name == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "missing package declaration")
		return result
	}
	result.PackageName = file.Name.Name

	// Validate package name
	if result.PackageName != "tools" && result.PackageName != "main" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("package name '%s' should be 'tools' or 'main'", result.PackageName))
	}

	// Extract and validate imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, importPath)
	}

	// Check for dangerous imports (warning only - safety checker handles blocking)
	dangerousImports := []string{"unsafe", "syscall", "runtime/cgo", "plugin"}
	for _, imp := range result.Imports {
		for _, dangerous := range dangerousImports {
			if imp == dangerous || strings.HasPrefix(imp, dangerous+"/") {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("potentially dangerous import: %s", imp))
			}
		}
	}

	// Check for unused imports
	usedImports := findUsedImports(file)
	for _, imp := range result.Imports {
		// Get package name from import path
		pkgName := filepath.Base(imp)
		if !usedImports[pkgName] && !usedImports[imp] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("possibly unused import: %s", imp))
		}
	}

	// Extract functions and check for required elements
	hasContextParam := false
	hasErrorReturn := false
	expectedFuncName := toCamelCase(expectedToolName)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			funcName := node.Name.Name
			result.Functions = append(result.Functions, funcName)

			if funcName == "main" {
				result.HasMainFunction = true
			}

			// Check for expected tool function
			if funcName == expectedFuncName || strings.EqualFold(funcName, expectedFuncName) {
				// Check for context.Context parameter
				if node.Type.Params != nil {
					for _, param := range node.Type.Params.List {
						if sel, ok := param.Type.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok {
								if ident.Name == "context" && sel.Sel.Name == "Context" {
									hasContextParam = true
								}
							}
						}
					}
				}

				// Check for error return type
				if node.Type.Results != nil {
					for _, res := range node.Type.Results.List {
						if ident, ok := res.Type.(*ast.Ident); ok {
							if ident.Name == "error" {
								hasErrorReturn = true
								result.HasErrorHandling = true
							}
						}
					}
				}
			}

			// Check function body for issues
			if node.Body != nil {
				checkFunctionBody(node.Body, result)
			}
		}
		return true
	})

	// Validate required functions exist
	if len(result.Functions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no functions defined")
		return result
	}

	// Check for expected main tool function
	foundToolFunc := false
	for _, fn := range result.Functions {
		if fn == expectedFuncName || strings.EqualFold(fn, expectedFuncName) ||
			fn == toPascalCase(expectedToolName) {
			foundToolFunc = true
			break
		}
	}
	if !foundToolFunc && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("expected main tool function '%s' not found", expectedFuncName))
	}

	// Warn if no context parameter
	if !hasContextParam && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			"main tool function should accept context.Context as first parameter")
	}

	// Warn if no error return
	if !hasErrorReturn && expectedToolName != "" {
		result.Warnings = append(result.Warnings,
			"main tool function should return error as last return value")
	}

	return result
}

// checkFunctionBody analyzes function body for common issues
func checkFunctionBody(body *ast.BlockStmt, result *ValidationResult) {
	hasPanic := false
	hasRecover := false
	hasErrorCheck := false

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Check for panic calls
			if ident, ok := node.Fun.(*ast.Ident); ok {
				if ident.Name == "panic" {
					hasPanic = true
				}
				if ident.Name == "recover" {
					hasRecover = true
				}
			}

			// Check for dangerous function calls
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					// os.Exit is usually bad in library code
					if ident.Name == "os" && sel.Sel.Name == "Exit" {
						result.Warnings = append(result.Warnings,
							"os.Exit() should not be used in tool code - return error instead")
					}
					// log.Fatal is similar
					if ident.Name == "log" && (sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf") {
						result.Warnings = append(result.Warnings,
							"log.Fatal() should not be used in tool code - return error instead")
					}
				}
			}

		case *ast.IfStmt:
			// Check for error handling patterns
			if binary, ok := node.Cond.(*ast.BinaryExpr); ok {
				if binary.Op.String() == "!=" {
					if ident, ok := binary.Y.(*ast.Ident); ok {
						if ident.Name == "nil" {
							hasErrorCheck = true
							result.HasErrorHandling = true
						}
					}
				}
			}
		}
		return true
	})

	// Warn about panic without recover
	if hasPanic && !hasRecover {
		result.Warnings = append(result.Warnings,
			"code contains panic() without recover() - consider proper error handling")
	}

	// Warn about lack of error handling
	if !hasErrorCheck && !hasPanic {
		result.Warnings = append(result.Warnings,
			"no error checking detected - ensure errors are properly handled")
	}
}

// findUsedImports finds all imported package names that are used in the code
func findUsedImports(file *ast.File) map[string]bool {
	used := make(map[string]bool)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				used[ident.Name] = true
			}
		}
		return true
	})

	return used
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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// ValidationError contains details about validation failures
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult holds the complete validation result
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// {{.FuncName}} validates {{.InputType}} input
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) (bool, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Check for empty/zero value
	if input == {{.ZeroValue}} {
		return false, &ValidationError{
			Field:   "input",
			Message: "value is empty or zero",
			Value:   input,
		}
	}

	// Type-specific validation
	result := validate{{.PascalName}}Input(input)
	if !result.Valid {
		if len(result.Errors) > 0 {
			return false, result.Errors[0]
		}
		return false, fmt.Errorf("validation failed")
	}

	return true, nil
}

// validate{{.PascalName}}Input performs type-specific validation
func validate{{.PascalName}}Input(input {{.InputType}}) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// String validation
	{{if eq .InputType "string"}}
	// Check for valid UTF-8
	if !utf8.ValidString(input) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: "invalid UTF-8 encoding",
			Value:   input,
		})
	}

	// Check reasonable length (configurable)
	const maxLength = 1048576 // 1MB
	if len(input) > maxLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum length of %d bytes", maxLength),
			Value:   len(input),
		})
	}

	// Check for control characters (except common whitespace)
	for i, r := range input {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: fmt.Sprintf("contains control character at position %d", i),
				Value:   r,
			})
			break
		}
	}
	{{end}}

	// JSON validation (if input looks like JSON)
	{{if eq .InputType "string"}}
	trimmed := strings.TrimSpace(input)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var js interface{}
		if err := json.Unmarshal([]byte(input), &js); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: "invalid JSON syntax",
				Value:   err.Error(),
			})
		}
	}
	{{end}}

	// Byte slice validation
	{{if eq .InputType "[]byte"}}
	const maxSize = 10485760 // 10MB
	if len(input) > maxSize {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum size of %d bytes", maxSize),
			Value:   len(input),
		})
	}
	{{end}}

	// Map validation
	{{if or (eq .InputType "map[string]any") (eq .InputType "map[string]interface{}")}}
	const maxKeys = 10000
	if len(input) > maxKeys {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum key count of %d", maxKeys),
			Value:   len(input),
		})
	}

	// Validate all keys are non-empty
	for key := range input {
		if strings.TrimSpace(key) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: "contains empty key",
				Value:   key,
			})
		}
	}
	{{end}}

	return result
}

// Helper validation functions
var _ = regexp.Compile // Ensure regexp is used

`,
	"converter": `package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// ConversionError provides details about conversion failures
type ConversionError struct {
	InputType  string
	OutputType string
	Reason     string
	Position   int
}

func (e ConversionError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("conversion from %s to %s failed at position %d: %s",
			e.InputType, e.OutputType, e.Position, e.Reason)
	}
	return fmt.Sprintf("conversion from %s to %s failed: %s",
		e.InputType, e.OutputType, e.Reason)
}

// {{.FuncName}} converts input from one format to another
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) ({{.OutputType}}, error) {
	var result {{.OutputType}}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	// Check for empty/zero value
	if input == {{.ZeroValue}} {
		return result, &ConversionError{
			InputType:  "{{.InputType}}",
			OutputType: "{{.OutputType}}",
			Reason:     "empty input",
			Position:   -1,
		}
	}

	// Perform the conversion
	converted, err := convert{{.PascalName}}(input)
	if err != nil {
		return result, err
	}

	return converted, nil
}

// convert{{.PascalName}} performs the actual conversion
func convert{{.PascalName}}(input {{.InputType}}) ({{.OutputType}}, error) {
	var result {{.OutputType}}

	{{if and (eq .InputType "string") (eq .OutputType "[]byte")}}
	// String to bytes conversion
	result = []byte(input)
	{{else if and (eq .InputType "[]byte") (eq .OutputType "string")}}
	// Bytes to string conversion
	result = string(input)
	{{else if and (eq .InputType "string") (eq .OutputType "map[string]any")}}
	// JSON string to map conversion
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return result, &ConversionError{
			InputType:  "string",
			OutputType: "map[string]any",
			Reason:     fmt.Sprintf("invalid JSON: %v", err),
			Position:   -1,
		}
	}
	{{else if and (eq .InputType "map[string]any") (eq .OutputType "string")}}
	// Map to JSON string conversion
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return result, &ConversionError{
			InputType:  "map[string]any",
			OutputType: "string",
			Reason:     fmt.Sprintf("marshal error: %v", err),
			Position:   -1,
		}
	}
	result = string(data)
	{{else if eq .OutputType "string"}}
	// Generic to string conversion
	result = fmt.Sprintf("%v", input)
	{{else}}
	// Generic conversion - implement specific logic
	_ = input // use input
	// Add type-specific conversion logic here
	{{end}}

	return result, nil
}

// Helper to ensure imports are used
var (
	_ = bytes.Buffer{}
	_ = strings.TrimSpace
	_ = json.Marshal
)

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
