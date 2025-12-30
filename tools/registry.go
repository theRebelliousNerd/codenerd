package tools

import "context"

// ToolFunc is the signature for a tool execution function.
type ToolFunc func(ctx context.Context, path string) (int, error)

// ToolRegistry is the interface for tool registration.
// This allows tools to be registered with the system for later execution.
type ToolRegistry interface {
	// Register adds a tool to the registry.
	Register(name, description string, fn ToolFunc) error
}
