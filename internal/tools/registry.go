package tools

import (
	"context"
	"fmt"
	"reflect"
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
	if tool == nil {
		return ErrToolNil
	}
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
	if tool == nil {
		panic("failed to register tool: tool cannot be nil")
	}
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

	// Sort by priority (highest first) using stable sort to prevent random tie-breaks
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Priority == tools[j].Priority {
			return tools[i].Name < tools[j].Name
		}
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
	// Defensive: a nil context can panic downstream when consumers call
	// ctx.Err() or ctx.Done(). Fall back to a background context.
	if ctx == nil {
		ctx = context.Background()
	}

	tool := r.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	return r.ExecuteTool(ctx, tool, args)
}

// ExecuteTool runs a specific tool with the given arguments.
func (r *Registry) ExecuteTool(ctx context.Context, tool *Tool, args map[string]any) (*ToolResult, error) {
	if tool == nil {
		return nil, ErrToolNil
	}
	if ctx == nil {
		ctx = context.Background()
	}
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

// validateArgs checks that all required arguments are present and (best-effort)
// that provided arguments match their declared JSON Schema type. Validation is
// intentionally lenient: properties with no declared Type are not checked, and
// numeric types accept both Go ints and JSON-unmarshaled float64.
func (r *Registry) validateArgs(tool *Tool, args map[string]any) error {
	if tool == nil {
		return ErrToolNil
	}
	for _, required := range tool.Schema.Required {
		if _, ok := args[required]; !ok {
			return fmt.Errorf("%w: %s", ErrMissingRequiredArg, required)
		}
	}

	if tool.Schema.Properties == nil {
		return nil
	}

	// Coarse type check for declared properties. Skip silently when no
	// Property is declared for a key (extra args are allowed by convention).
	for key, value := range args {
		prop, ok := tool.Schema.Properties[key]
		if !ok || prop.Type == "" || value == nil {
			continue
		}
		if !valueMatchesSchemaType(value, prop.Type) {
			return fmt.Errorf("%w: %s expected %s, got %T",
				ErrInvalidArgType, key, prop.Type, value)
		}
	}
	return nil
}

// valueMatchesSchemaType returns true if v satisfies the JSON-Schema type
// string. It accepts the type variations real callers produce (e.g. JSON
// unmarshaling yields float64 for both "integer" and "number").
func valueMatchesSchemaType(v any, schemaType string) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch schemaType {
	case "string":
		return rv.Kind() == reflect.String
	case "integer":
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return true
		case reflect.Float32, reflect.Float64:
			// Accept whole-number floats — JSON unmarshal always yields float64.
			return rv.Float() == float64(int64(rv.Float()))
		}
		return false
	case "number":
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return true
		}
		return false
	case "boolean":
		return rv.Kind() == reflect.Bool
	case "array":
		return rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array
	case "object":
		return rv.Kind() == reflect.Map || rv.Kind() == reflect.Struct
	default:
		// Unknown schema type: don't reject.
		return true
	}
}

// FilterByIntent returns tools that match the given intent.
// This maps intents to categories for tool selection. An empty or unknown
// intent returns all registered tools as a safe fallback so callers never get
// an empty toolbox just because an intent was missing or hallucinated.
func (r *Registry) FilterByIntent(intent string) []*Tool {
	if intent == "" {
		return r.All()
	}
	category := intentToCategory(intent)
	if category == "" {
		return r.All()
	}
	return r.GetByCategory(category)
}

// intentToCategory maps intent verbs to tool categories. Returns the empty
// string for unknown intents so FilterByIntent can fall back to All().
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
	case "/general":
		return CategoryGeneral
	default:
		// Unknown intent — let FilterByIntent fall back to All().
		return ""
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
