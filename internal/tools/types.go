// Package tools provides modular tool definitions for the JIT Clean Loop.
//
// This package replaces the embedded tool implementations that were deleted
// with the domain shards. Each tool is now standalone and can be used by
// any agent based on intent-driven JIT selection.
//
// Architecture:
//
//	Intent → ConfigFactory → AllowedTools[] → Registry.Get() → Tool.Execute()
package tools

import (
	"context"
)

// ToolCategory classifies tools for intent-based filtering.
// Categories align with intent routing in internal/mangle/intent_routing.mg.
type ToolCategory string

const (
	// CategoryResearch covers Context7, web search, browser automation.
	CategoryResearch ToolCategory = "/research"

	// CategoryCode covers file operations, build, AST, refactoring.
	CategoryCode ToolCategory = "/code"

	// CategoryTest covers test execution, coverage, TDD loop.
	CategoryTest ToolCategory = "/test"

	// CategoryReview covers static analysis, hypotheses, metrics.
	CategoryReview ToolCategory = "/review"

	// CategoryAttack covers adversarial testing, Nemesis operations.
	CategoryAttack ToolCategory = "/attack"

	// CategoryGeneral is for tools usable by any intent.
	CategoryGeneral ToolCategory = "/general"
)

// Property describes a single parameter property for JSON schema.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Default     any    `json:"default,omitempty"`
	Enum        []any  `json:"enum,omitempty"`
	// Items describes array element schema (required for type="array")
	Items *PropertyItems `json:"items,omitempty"`
}

// PropertyItems describes the schema for array elements.
type PropertyItems struct {
	Type string `json:"type"`
}

// ToolSchema defines the JSON schema for tool arguments.
// This enables LLM tool calling with proper validation.
type ToolSchema struct {
	// Required lists parameters that must be provided.
	Required []string `json:"required"`

	// Properties describes each parameter.
	Properties map[string]Property `json:"properties"`
}

// ExecuteFunc is the signature for tool execution.
// Returns the result string and any error.
type ExecuteFunc func(ctx context.Context, args map[string]any) (string, error)

// Tool defines a modular tool that any agent can use.
// Tools are registered in the Registry and selected by ConfigFactory
// based on the user's intent.
type Tool struct {
	// Name is the unique identifier for the tool.
	// Must match the AllowedTools entries in ConfigAtoms.
	Name string

	// Description explains what the tool does.
	// Used for LLM tool calling and documentation.
	Description string

	// Category classifies the tool for intent filtering.
	Category ToolCategory

	// Execute runs the tool with the given arguments.
	Execute ExecuteFunc

	// Schema defines the expected arguments.
	Schema ToolSchema

	// Priority is used when multiple tools match.
	// Higher priority tools are preferred (default 50).
	Priority int

	// RequiresContext indicates if the tool needs session context.
	RequiresContext bool
}

// Validate checks if the tool definition is valid.
func (t *Tool) Validate() error {
	if t.Name == "" {
		return ErrToolNameEmpty
	}
	if t.Execute == nil {
		return ErrToolExecuteNil
	}
	return nil
}

// WithPriority returns a copy of the tool with the given priority.
func (t *Tool) WithPriority(priority int) *Tool {
	copy := *t
	copy.Priority = priority
	return &copy
}

// ToolResult wraps the result of tool execution with metadata.
type ToolResult struct {
	// ToolName identifies which tool was executed.
	ToolName string

	// Result is the string output from the tool.
	Result string

	// Error is set if the tool failed.
	Error error

	// DurationMs is how long execution took.
	DurationMs int64
}

// IsSuccess returns true if the tool executed without error.
func (r *ToolResult) IsSuccess() bool {
	return r.Error == nil
}
