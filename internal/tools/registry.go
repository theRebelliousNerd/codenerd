package tools

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
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

	// writeGuard, when set, runs before every tool execution and can refuse
	// it. See SetWriteGuard.
	writeGuard WriteGuard

	// workspaceRoot is the containment boundary handed to every tool this
	// registry executes. See SetWorkspaceRoot.
	workspaceRoot string

	// allowlist is the capability envelope. See SetAllowlist.
	allowlist *Allowlist

	// factSink, when set, receives one record per completed execution. See
	// SetFactSink.
	factSink FactSink

	// metrics accumulates per-tool success/failure/duration counters.
	metrics map[string]*ToolMetrics
}

// Allowlist is the runtime capability envelope for a registry: the set of tool
// names execution is permitted to reach.
//
// Enforced is deliberately separate from a nil/empty Names slice, because the
// two must not be confused. "No allowlist configured" (Enforced=false) is the
// unconstrained developer/CLI case. "An allowlist is configured and it is
// empty" (Enforced=true, len(Names)==0) means the agent was granted no
// capability at all, and it must deny everything — an absent capability
// envelope is not a grant of all capabilities.
//
// This mirrors the contract session.Executor.isToolAllowed already implements
// for the JIT config layer, and closes the hole underneath it: the registry is
// reachable process-globally through tools.Execute, so any caller that skips
// the session gate previously got the whole catalog.
type Allowlist struct {
	// Enforced turns the envelope on. When false the registry does not gate.
	Enforced bool

	// Names are the permitted tool names. Empty with Enforced=true denies all.
	Names []string
}

// FactSink receives one record per completed tool execution so the kernel can
// learn from it (schemas_tools.mg: tool_execution(ToolName, Success, Timestamp)).
//
// It is injected as a function for the same reason WriteGuard is: internal
// /tools must not import internal/core, and the kernel lives there.
type FactSink func(ctx context.Context, toolName string, success bool, durationMs int64, unixSeconds int64)

// ToolMetrics accumulates outcome counters for one tool.
type ToolMetrics struct {
	Calls        int64
	Successes    int64
	Failures     int64
	TotalMs      int64
	MaxMs        int64
	LastDuration int64
}

// SuccessRate returns successes/calls in [0,1]; zero when never called.
func (m ToolMetrics) SuccessRate() float64 {
	if m.Calls == 0 {
		return 0
	}
	return float64(m.Successes) / float64(m.Calls)
}

// AvgMs returns the mean duration in milliseconds; zero when never called.
func (m ToolMetrics) AvgMs() float64 {
	if m.Calls == 0 {
		return 0
	}
	return float64(m.TotalMs) / float64(m.Calls)
}

// WriteGuard vets a tool invocation before it runs. A non-nil error refuses
// the call and the tool never executes.
//
// This exists because nerd.md's forbidden-path enforcement lived only in
// callers — session.Executor.executeToolCall and VirtualStore.executeAction —
// while the tools themselves enforced nothing and the registry is reachable
// process-globally via tools.Execute. Any code path calling
// tools.Global().Execute(ctx, "write_file", ...) directly bypassed both gates,
// which is the exact failure the codebase already suffered once and documents
// at virtual_store_routing.go:317 ("a shard could write .nerd/config.json").
// Raised again by codeNERD's own security review of internal/tools/core/
// file_ops.go.
//
// The guard is injected as a function so internal/tools keeps its single
// dependency on internal/logging — it must not import projectdoc or core.
type WriteGuard func(ctx context.Context, toolName string, args map[string]any) error

// NewRegistry creates a new empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:      make(map[string]*Tool),
		byCategory: make(map[ToolCategory][]*Tool),
		metrics:    make(map[string]*ToolMetrics),
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
	// Secondary categories share the same index, so GetByCategory/FilterByIntent
	// see a tool under every intent family it genuinely serves. Without them
	// /review and /audit resolved to a category with zero registered tools and
	// the reviewer got an empty toolbox.
	for _, alt := range tool.AltCategories {
		if alt == "" || alt == tool.Category {
			continue
		}
		r.byCategory[alt] = append(r.byCategory[alt], tool)
	}

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
		rejected := time.Since(start)
		logging.Audit().ToolExec(tool.Name, "validate_args", rejected.Milliseconds(), false, err.Error())
		return &ToolResult{
			ToolName:   tool.Name,
			Error:      err,
			DurationMs: rejected.Milliseconds(),
		}, err
	}

	r.mu.RLock()
	guard := r.writeGuard
	allowlist := r.allowlist
	wsRoot := r.workspaceRoot
	sink := r.factSink
	r.mu.RUnlock()

	// Capability envelope first: a tool outside it must not even reach the
	// write guard, and an enforced-but-empty envelope denies everything.
	if err := allowlist.check(tool.Name); err != nil {
		refused := time.Since(start)
		logging.Audit().ToolExec(tool.Name, "allowlist", refused.Milliseconds(), false, err.Error())
		return &ToolResult{
			ToolName:   tool.Name,
			Error:      err,
			DurationMs: refused.Milliseconds(),
		}, err
	}

	// Consult the write guard before the tool can touch anything. This is the
	// chokepoint every execution path reaches, including the process-global
	// tools.Execute that bypassed the caller-side nerd.md gates entirely.
	if guard != nil {
		if err := guard(ctx, tool.Name, args); err != nil {
			refused := time.Since(start)
			logging.Audit().ToolExec(tool.Name, "write_guard", refused.Milliseconds(), false, err.Error())
			return &ToolResult{
				ToolName:   tool.Name,
				Error:      err,
				DurationMs: refused.Milliseconds(),
			}, err
		}
	}

	// Hand the tool its containment boundary. Tools receive only (ctx, args),
	// so the context is the only channel; an explicit value already on the
	// context wins so a caller can narrow the root for a single call (codedom
	// apply_edits does exactly that to stage edits under a temp root).
	if wsRoot != "" {
		if existing, ok := ctx.Value(CtxKeyWorkspaceRoot).(string); !ok || existing == "" {
			ctx = context.WithValue(ctx, CtxKeyWorkspaceRoot, wsRoot)
		}
	}

	// Execute the tool
	logging.ToolsDebug("Executing tool: %s", tool.Name)
	result, err := tool.Execute(ctx, args)

	duration := time.Since(start)
	logging.ToolsDebug("Tool %s completed in %v (success=%v)", tool.Name, duration, err == nil)

	// Record the invocation in the durable audit trail.
	//
	// The tool_invoke/tool_complete/tool_error families were declared in the
	// audit taxonomy with no producer anywhere in the repo, so a run that
	// executed hundreds of tools left no record of a single one. That is
	// survivable when a human is watching the TUI and fatal for an unattended
	// run, where the audit log is the only account of what happened.
	//
	// This is the chokepoint: ExecuteTool is reached by both the direct and the
	// by-name paths, and both outcomes pass through it.
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	logging.Audit().ToolExec(tool.Name, "execute", duration.Milliseconds(), err == nil, errMsg)

	r.recordMetrics(tool.Name, err == nil, duration.Milliseconds())

	// Feed the kernel's learning loop. tool_execution/3 has been declared in
	// schemas_tools.mg since Section 40.5 with no producer anywhere in the
	// repo, so every tool_usage_stats and tool_success_relevance derivation
	// downstream of it evaluated over an empty relation.
	if sink != nil {
		sink(ctx, tool.Name, err == nil, duration.Milliseconds(), start.Unix())
	}

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
//
// "All registered tools" is bounded by the capability envelope when one is
// enforced: the fallback exists to survive a missing intent, not to hand a
// caller capabilities the envelope withheld.
func (r *Registry) FilterByIntent(intent string) []*Tool {
	var candidates []*Tool
	switch {
	case intent == "":
		candidates = r.All()
	default:
		category := intentToCategory(intent)
		if category == "" {
			candidates = r.All()
		} else {
			candidates = r.GetByCategory(category)
		}
	}

	r.mu.RLock()
	allowlist := r.allowlist
	r.mu.RUnlock()
	if allowlist == nil || !allowlist.Enforced {
		return candidates
	}

	permitted := make([]*Tool, 0, len(candidates))
	for _, t := range candidates {
		if allowlist.check(t.Name) == nil {
			permitted = append(permitted, t)
		}
	}
	return permitted
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

// SetWriteGuard installs a guard consulted before every tool execution on this
// registry. Passing nil removes it.
//
// Defense in depth, not a replacement: the caller-side gates stay where they
// are. This one exists so that a call site which forgets them — or is added
// later by someone who does not know they exist — still cannot write a
// protected path.
func (r *Registry) SetWriteGuard(g WriteGuard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeGuard = g
}

// SetGlobalWriteGuard installs a write guard on the global registry, which is
// the one reachable via tools.Execute.
func SetGlobalWriteGuard(g WriteGuard) {
	globalRegistry.SetWriteGuard(g)
}

// check returns nil when name may execute under this envelope.
//
// A nil *Allowlist means "no envelope configured" and permits everything; that
// is the receiver-on-nil case and is why this is a method rather than a free
// function. An envelope that IS configured and lists nothing denies
// everything, which is the whole point of the type.
func (a *Allowlist) check(name string) error {
	if a == nil || !a.Enforced {
		return nil
	}
	if slices.Contains(a.Names, name) {
		return nil
	}
	return fmt.Errorf("%w: %s (allowlist has %d entries)", ErrToolNotAllowed, name, len(a.Names))
}

// SetAllowlist installs the capability envelope for this registry. Passing nil
// removes it (unconstrained).
//
// Contract with internal/session: session.Executor.isToolAllowed already fails
// closed on an empty EffectiveAgentRuntimeConfig.AllowedTools. This is the same
// rule enforced one layer down, at the registry every execution path funnels
// through, so a caller that reaches tools.Global().Execute without the session
// gate does not silently get the full catalog. The session should call
// SetAllowlist(&Allowlist{Enforced: cfg.EnableSafetyGate, Names: cfg.AllowedTools})
// whenever the effective config changes.
func (r *Registry) SetAllowlist(a *Allowlist) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a == nil {
		r.allowlist = nil
		return
	}
	clone := &Allowlist{Enforced: a.Enforced, Names: slices.Clone(a.Names)}
	r.allowlist = clone
}

// AllowlistEnforced reports whether a capability envelope is currently active.
func (r *Registry) AllowlistEnforced() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.allowlist != nil && r.allowlist.Enforced
}

// IsAllowed reports whether name may execute under the current envelope.
func (r *Registry) IsAllowed(name string) bool {
	r.mu.RLock()
	a := r.allowlist
	r.mu.RUnlock()
	return a.check(name) == nil
}

// SetGlobalAllowlist installs the capability envelope on the global registry.
func SetGlobalAllowlist(a *Allowlist) {
	globalRegistry.SetAllowlist(a)
}

// SetWorkspaceRoot sets the containment boundary handed to every tool this
// registry executes, replacing reliance on the process-global
// CODENERD_WORKSPACE_ROOT environment variable. An empty string clears it and
// restores the env/cwd fallback in WorkspaceRoot.
func (r *Registry) SetWorkspaceRoot(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaceRoot = strings.TrimSpace(root)
}

// WorkspaceRoot returns the registry's configured containment boundary, or ""
// when none is set.
func (r *Registry) WorkspaceRoot() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.workspaceRoot
}

// SetGlobalWorkspaceRoot sets the containment boundary on the global registry.
func SetGlobalWorkspaceRoot(root string) {
	globalRegistry.SetWorkspaceRoot(root)
}

// SetFactSink installs a callback invoked once per completed execution so the
// kernel can assert tool_execution facts. Passing nil removes it.
func (r *Registry) SetFactSink(s FactSink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factSink = s
}

// SetGlobalFactSink installs the execution fact sink on the global registry.
func SetGlobalFactSink(s FactSink) {
	globalRegistry.SetFactSink(s)
}

func (r *Registry) recordMetrics(name string, success bool, durationMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.metrics == nil {
		r.metrics = make(map[string]*ToolMetrics)
	}
	m, ok := r.metrics[name]
	if !ok {
		m = &ToolMetrics{}
		r.metrics[name] = m
	}
	m.Calls++
	if success {
		m.Successes++
	} else {
		m.Failures++
	}
	m.TotalMs += durationMs
	m.LastDuration = durationMs
	if durationMs > m.MaxMs {
		m.MaxMs = durationMs
	}
}

// Metrics returns a snapshot of the counters for one tool.
func (r *Registry) Metrics(name string) ToolMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if m, ok := r.metrics[name]; ok {
		return *m
	}
	return ToolMetrics{}
}

// AllMetrics returns a snapshot of every tool's counters, keyed by tool name.
func (r *Registry) AllMetrics() map[string]ToolMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]ToolMetrics, len(r.metrics))
	for name, m := range r.metrics {
		out[name] = *m
	}
	return out
}

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
