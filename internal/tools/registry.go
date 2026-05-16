package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Registry holds all available tools and provides lookup functionality.
// It is thread-safe and supports registration at runtime.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool

	// byCategory provides fast lookup by category.
	byCategory map[ToolCategory][]*Tool
}

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]*Tool),
		byCategory: make(map[ToolCategory][]*Tool),
	}
}

// Register adds a tool to the registry.
// Returns an error if a tool with the same name already exists.
func (r *Registry) Register(tool *Tool) error {
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("invalid tool: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("%w: %s", ErrToolAlreadyRegistered, tool.Name)
	}

	// Set default priority if not specified
	if tool.Priority == 0 {
		tool.Priority = 50
	}

	r.tools[tool.Name] = tool
	r.byCategory[tool.Category] = append(r.byCategory[tool.Category], tool)

	logging.ToolsDebug("Registered tool: %s (category=%s, priority=%d)", tool.Name, tool.Category, tool.Priority)
	return nil
}

// MustRegister registers a tool and panics on error.
// Use this for static tool registration at init time.
func (r *Registry) MustRegister(tool *Tool) {
	if err := r.Register(tool); err != nil {
		panic(fmt.Sprintf("failed to register tool %s: %v", tool.Name, err))
	}
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Has returns true if a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// GetByCategory returns all tools in a category, sorted by priority (descending).
func (r *Registry) GetByCategory(category ToolCategory) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, len(r.byCategory[category]))
	copy(tools, r.byCategory[category])

	// Sort by priority (highest first)
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Priority > tools[j].Priority
	})

	return tools
}

// GetMultiple returns tools matching the given names.
// Missing tools are silently skipped.
func (r *Registry) GetMultiple(names []string) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Tool, 0, len(names))
	for _, name := range names {
		if tool, ok := r.tools[name]; ok {
			result = append(result, tool)
		}
	}
	return result
}

// All returns all registered tools.
func (r *Registry) All() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// Names returns all registered tool names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Execute runs a tool by name with the given arguments.
// Returns ErrToolNotFound if the tool doesn't exist.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	tool := r.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	return r.ExecuteTool(ctx, tool, args)
}

// ExecuteTool runs a specific tool with the given arguments.
func (r *Registry) ExecuteTool(ctx context.Context, tool *Tool, args map[string]any) (*ToolResult, error) {
	start := time.Now()

	// Validate required arguments
	if err := r.validateArgs(tool, args); err != nil {
		return &ToolResult{
			ToolName:   tool.Name,
			Error:      err,
			DurationMs: time.Since(start).Milliseconds(),
		}, err
	}

	// Execute the tool
	logging.ToolsDebug("Executing tool: %s", tool.Name)
	result, err := tool.Execute(ctx, args)

	duration := time.Since(start)
	logging.ToolsDebug("Tool %s completed in %v (success=%v)", tool.Name, duration, err == nil)

	return &ToolResult{
		ToolName:   tool.Name,
		Result:     result,
		Error:      err,
		DurationMs: duration.Milliseconds(),
	}, err
}

// validateArgs checks that all required arguments are present.
func (r *Registry) validateArgs(tool *Tool, args map[string]any) error {
	for _, required := range tool.Schema.Required {
		if _, ok := args[required]; !ok {
			return fmt.Errorf("%w: %s", ErrMissingRequiredArg, required)
		}
	}
	return nil
}

// FilterByIntent returns tools that match the given intent.
// This maps intents to categories for tool selection.
func (r *Registry) FilterByIntent(intent string) []*Tool {
	category := intentToCategory(intent)
	if category == "" {
		return r.All()
	}
	return r.GetByCategory(category)
}

// intentToCategory maps intent verbs to tool categories.
func intentToCategory(intent string) ToolCategory {
	switch intent {
	case "/research", "/explore", "/learn", "/document":
		return CategoryResearch
	case "/fix", "/implement", "/refactor", "/create", "/edit":
		return CategoryCode
	case "/test", "/cover", "/verify":
		return CategoryTest
	case "/review", "/audit", "/check":
		return CategoryReview
	case "/attack", "/break", "/nemesis":
		return CategoryAttack
	default:
		return CategoryGeneral
	}
}

// Global registry instance for convenience.
var globalRegistry = NewRegistry()

// Global returns the global tool registry.
func Global() *Registry {
	return globalRegistry
}

// Register adds a tool to the global registry.
func Register(tool *Tool) error {
	return globalRegistry.Register(tool)
}

// MustRegisterGlobal registers a tool in the global registry, panicking on error.
func MustRegisterGlobal(tool *Tool) {
	globalRegistry.MustRegister(tool)
}

// Get retrieves a tool from the global registry.
func Get(name string) *Tool {
	return globalRegistry.Get(name)
}

// Execute runs a tool from the global registry.
func Execute(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	return globalRegistry.Execute(ctx, name, args)
}
