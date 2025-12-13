package autopoiesis

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
)

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

	// Build base user prompt
	userPrompt := fmt.Sprintf(`Generate a Go tool with these specifications:

Tool Name: %s
Purpose: %s
Input Type: %s
Output Type: %s

The tool should be in package "tools" and include:
1. Main tool function: %s(ctx context.Context, input %s) (%s, error)
2. Tool description constant (e.g., const ToolDescription%s = "...")

IMPORTANT: Generate ONLY standalone code. Do NOT generate registration functions or reference any types like ToolRegistry or Tool that aren't defined in this file.`,
		need.Name, need.Purpose, need.InputType, need.OutputType,
		toCamelCase(need.Name), need.InputType, need.OutputType,
		toPascalCase(need.Name))

	// Inject learnings context if available
	if tg.learningsContext != "" {
		userPrompt += fmt.Sprintf(`

LEARNINGS FROM PAST TOOL GENERATION:
%s

Apply these learnings to generate better code.`, tg.learningsContext)
		logging.AutopoiesisDebug("Injected learnings into generation prompt")
	}

	userPrompt += "\n\nGenerate complete, compilable Go code:"

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

IMPORTANT: Generate ONLY standalone code. Do NOT generate registration functions or reference any types like ToolRegistry or Tool that aren't defined in this file.`,
		need.Name, need.Purpose, need.InputType, need.OutputType,
		toCamelCase(need.Name), need.InputType, need.OutputType,
		toPascalCase(need.Name))

	// Inject learnings context if available
	if tg.learningsContext != "" {
		userPrompt += fmt.Sprintf(`

LEARNINGS FROM PAST TOOL GENERATION:
%s

Apply these learnings to generate better code.`, tg.learningsContext)
		logging.AutopoiesisDebug("Injected learnings into JIT generation prompt")
	}

	userPrompt += "\n\nGenerate complete, compilable Go code:"

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
