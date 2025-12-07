// router.go implements the Tactile Router system shard.
//
// The Tactile Router is responsible for:
// - Mapping permitted actions to appropriate tools/virtual predicates
// - Checking tool allowlists and rate limits
// - Managing tool timeouts
// - Emitting exec_request facts for the VirtualStore
//
// This shard is ON-DEMAND (starts only when actions are pending) and
// LOGIC-PRIMARY (deterministic routing with LLM only for capability gaps).
package system

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolRoute defines how an action maps to a tool.
type ToolRoute struct {
	ActionPattern string        // Regex pattern for action matching
	ToolName      string        // Target tool
	Timeout       time.Duration // Tool-specific timeout
	RateLimit     int           // Max calls per minute (0 = unlimited)
	RequiresSafe  bool          // Must pass constitution gate first
}

// RouterConfig holds configuration for the tactile router.
type RouterConfig struct {
	// Routing tables
	DefaultRoutes []ToolRoute // Built-in routes
	CustomRoutes  []ToolRoute // User-defined routes

	// Performance
	TickInterval time.Duration // How often to check permitted actions (default: 100ms)
	IdleTimeout  time.Duration // Auto-stop after no pending actions (default: 30s)

	// Safety
	AllowUnmappedActions bool // Allow actions without explicit routes (default: false)
}

// DefaultRouterConfig returns sensible defaults.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		DefaultRoutes: []ToolRoute{
			// File operations
			{ActionPattern: "read_file", ToolName: "fs_read", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "write_file", ToolName: "fs_write", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "edit_file", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "delete_file", ToolName: "fs_delete", Timeout: 10 * time.Second, RequiresSafe: true},

			// Code operations
			{ActionPattern: "search_code", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "analyze_impact", ToolName: "impact_analyzer", Timeout: 60 * time.Second, RequiresSafe: false},

			// Execution
			{ActionPattern: "exec_cmd", ToolName: "shell_exec", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "run_tests", ToolName: "test_runner", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "build_project", ToolName: "build_tool", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 5},

			// Git operations
			{ActionPattern: "git_operation", ToolName: "git_tool", Timeout: 60 * time.Second, RequiresSafe: true, RateLimit: 20},

			// Network
			{ActionPattern: "fetch", ToolName: "http_fetch", Timeout: 30 * time.Second, RequiresSafe: true, RateLimit: 30},
			{ActionPattern: "browse", ToolName: "browser_tool", Timeout: 60 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "research", ToolName: "research_tool", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 5},

			// Delegation
			{ActionPattern: "delegate", ToolName: "shard_manager", Timeout: 600 * time.Second, RequiresSafe: true},

			// User interaction
			{ActionPattern: "ask_user", ToolName: "user_prompt", Timeout: 0, RequiresSafe: false}, // No timeout for user
			{ActionPattern: "escalate", ToolName: "escalation_handler", Timeout: 0, RequiresSafe: false},
		},
		TickInterval:         100 * time.Millisecond,
		IdleTimeout:          30 * time.Second,
		AllowUnmappedActions: false,
	}
}

// ToolCall represents a pending tool invocation.
type ToolCall struct {
	ID          string
	Tool        string
	Action      string
	Target      string
	Payload     map[string]interface{}
	Timeout     time.Duration
	QueuedAt    time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Status      string // pending, executing, completed, failed, timeout
	Result      string
	Error       string
}

// TactileRouterShard routes permitted actions to tools.
type TactileRouterShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config RouterConfig

	// Routing state
	routes       map[string]ToolRoute // action -> route
	rateLimiters map[string]*rateLimiter

	// Tracking
	pendingCalls   []ToolCall
	completedCalls []ToolCall
	lastActivity   time.Time

	// State
	running bool
}

// rateLimiter tracks call frequency per tool.
type rateLimiter struct {
	mu          sync.Mutex
	callsPerMin int
	lastReset   time.Time
	callCount   int
}

func newRateLimiter(callsPerMin int) *rateLimiter {
	return &rateLimiter{
		callsPerMin: callsPerMin,
		lastReset:   time.Now(),
	}
}

func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Since(r.lastReset) >= time.Minute {
		r.callCount = 0
		r.lastReset = time.Now()
	}

	if r.callsPerMin > 0 && r.callCount >= r.callsPerMin {
		return false
	}

	r.callCount++
	return true
}

// NewTactileRouterShard creates a new Tactile Router shard.
func NewTactileRouterShard() *TactileRouterShard {
	return NewTactileRouterShardWithConfig(DefaultRouterConfig())
}

// NewTactileRouterShardWithConfig creates a router with custom config.
func NewTactileRouterShardWithConfig(cfg RouterConfig) *TactileRouterShard {
	base := NewBaseSystemShard("tactile_router", StartupOnDemand)

	// Configure permissions
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionExecCmd,
		core.PermissionNetwork,
		core.PermissionBrowser,
	}
	base.Config.Model = core.ModelConfig{} // No LLM by default

	// Configure idle timeout
	base.CostGuard.IdleTimeout = cfg.IdleTimeout

	shard := &TactileRouterShard{
		BaseSystemShard: base,
		config:          cfg,
		routes:          make(map[string]ToolRoute),
		rateLimiters:    make(map[string]*rateLimiter),
		pendingCalls:    make([]ToolCall, 0),
		completedCalls:  make([]ToolCall, 0),
		lastActivity:    time.Now(),
	}

	// Build routing table
	for _, route := range cfg.DefaultRoutes {
		shard.routes[route.ActionPattern] = route
		if route.RateLimit > 0 {
			shard.rateLimiters[route.ToolName] = newRateLimiter(route.RateLimit)
		}
	}
	for _, route := range cfg.CustomRoutes {
		shard.routes[route.ActionPattern] = route
		if route.RateLimit > 0 {
			shard.rateLimiters[route.ToolName] = newRateLimiter(route.RateLimit)
		}
	}

	return shard
}

// Execute runs the Tactile Router's action routing loop.
// This shard is ON-DEMAND and auto-stops after IdleTimeout.
func (r *TactileRouterShard) Execute(ctx context.Context, task string) (string, error) {
	r.SetState(core.ShardStateRunning)
	r.mu.Lock()
	r.running = true
	r.StartTime = time.Now()
	r.lastActivity = time.Now()
	r.mu.Unlock()

	defer func() {
		r.SetState(core.ShardStateCompleted)
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	// Initialize kernel if not set
	if r.Kernel == nil {
		r.Kernel = core.NewRealKernel()
	}

	ticker := time.NewTicker(r.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return r.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-r.StopCh:
			return r.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Check idle timeout
			if r.CostGuard.IsIdle() {
				return r.generateShutdownSummary("idle timeout"), nil
			}

			// Process permitted actions
			if err := r.processPermittedActions(ctx); err != nil {
				// Log error but continue
				_ = r.Kernel.Assert(core.Fact{
					Predicate: "routing_error",
					Args:      []interface{}{"internal_error", err.Error(), time.Now().Unix()},
				})
			}

			// Emit heartbeat
			_ = r.EmitHeartbeat()

			// Check for autopoiesis (tool capability gaps)
			if r.Autopoiesis.ShouldPropose() {
				r.handleAutopoiesis(ctx)
			}
		}
	}
}

// processPermittedActions routes all permitted actions to tools.
func (r *TactileRouterShard) processPermittedActions(ctx context.Context) error {
	if r.Kernel == nil {
		return nil
	}

	// Query action_permitted facts
	permitted, err := r.Kernel.Query("action_permitted")
	if err != nil {
		return fmt.Errorf("failed to query action_permitted: %w", err)
	}

	for _, fact := range permitted {
		if len(fact.Args) < 1 {
			continue
		}

		actionType, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		var target string
		if len(fact.Args) > 1 {
			target, _ = fact.Args[1].(string)
		}

		// Update activity tracking
		r.mu.Lock()
		r.lastActivity = time.Now()
		r.mu.Unlock()

		// Find route for action
		route, found := r.findRoute(actionType)

		if !found {
			if r.config.AllowUnmappedActions {
				// Record as unhandled for autopoiesis
				r.Autopoiesis.RecordUnhandled(
					fmt.Sprintf("route(%s)", actionType),
					map[string]string{"action": actionType, "target": target},
					nil,
				)
				continue
			}
			// Emit routing error
			_ = r.Kernel.Assert(core.Fact{
				Predicate: "routing_error",
				Args:      []interface{}{actionType, "no_handler", time.Now().Unix()},
			})
			continue
		}

		// Check rate limit
		if limiter, exists := r.rateLimiters[route.ToolName]; exists {
			if !limiter.allow() {
				_ = r.Kernel.Assert(core.Fact{
					Predicate: "routing_error",
					Args:      []interface{}{actionType, "rate_limit_exceeded", time.Now().Unix()},
				})
				continue
			}
		}

		// Create tool call
		call := ToolCall{
			ID:       fmt.Sprintf("call-%d", time.Now().UnixNano()),
			Tool:     route.ToolName,
			Action:   actionType,
			Target:   target,
			Timeout:  route.Timeout,
			QueuedAt: time.Now(),
			Status:   "pending",
		}

		r.mu.Lock()
		r.pendingCalls = append(r.pendingCalls, call)
		r.mu.Unlock()

		// Execute via VirtualStore if available (synchronous execution)
		if r.VirtualStore != nil {
			call.Status = "executing"
			call.StartedAt = time.Now()

			// Create action fact for VirtualStore
			actionFact := core.Fact{
				Predicate: "next_action",
				Args:      []interface{}{actionType, target},
			}

			result, err := r.VirtualStore.RouteAction(ctx, actionFact)

			call.CompletedAt = time.Now()
			if err != nil {
				call.Status = "failed"
				call.Error = err.Error()
				_ = r.Kernel.Assert(core.Fact{
					Predicate: "routing_result",
					Args:      []interface{}{call.ID, "failure", err.Error()},
				})
			} else {
				call.Status = "completed"
				call.Result = result
				_ = r.Kernel.Assert(core.Fact{
					Predicate: "routing_result",
					Args:      []interface{}{call.ID, "success", result},
				})
			}

			// Update call in pendingCalls
			r.mu.Lock()
			for i := range r.pendingCalls {
				if r.pendingCalls[i].ID == call.ID {
					r.pendingCalls[i] = call
					break
				}
			}
			r.mu.Unlock()
		} else {
			// Emit exec_request fact for async processing by VirtualStore
			_ = r.Kernel.Assert(core.Fact{
				Predicate: "exec_request",
				Args: []interface{}{
					route.ToolName,
					target,
					route.Timeout.Seconds(),
					call.ID,
					time.Now().Unix(),
				},
			})
		}

		// Clear the permitted action
		_ = r.Kernel.RetractFact(fact)
	}

	return nil
}

// findRoute finds a route for the given action type.
func (r *TactileRouterShard) findRoute(actionType string) (ToolRoute, bool) {
	// Exact match first
	if route, exists := r.routes[actionType]; exists {
		return route, true
	}

	// Prefix matching
	for pattern, route := range r.routes {
		if strings.HasPrefix(actionType, pattern) || strings.Contains(actionType, pattern) {
			return route, true
		}
	}

	return ToolRoute{}, false
}

// handleAutopoiesis uses LLM to propose new tool routes.
func (r *TactileRouterShard) handleAutopoiesis(ctx context.Context) {
	cases := r.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	// If no LLM, just log
	if r.LLMClient == nil {
		return
	}

	// Check cost guard
	can, _ := r.CostGuard.CanCall()
	if !can {
		for _, cas := range cases {
			r.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	prompt := r.buildRouteProposalPrompt(cases)

	result, err := r.GuardedLLMCall(ctx, routerAutopoiesisPrompt, prompt)
	if err != nil {
		for _, cas := range cases {
			r.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Parse and add new route
	newRoute := r.parseProposedRoute(result)
	if newRoute.ActionPattern != "" && newRoute.ToolName != "" {
		r.mu.Lock()
		r.routes[newRoute.ActionPattern] = newRoute
		if newRoute.RateLimit > 0 {
			r.rateLimiters[newRoute.ToolName] = newRateLimiter(newRoute.RateLimit)
		}
		r.mu.Unlock()

		// Emit route_added fact
		_ = r.Kernel.Assert(core.Fact{
			Predicate: "route_added",
			Args:      []interface{}{newRoute.ActionPattern, newRoute.ToolName, time.Now().Unix()},
		})
	}
}

// buildRouteProposalPrompt creates a prompt for route proposals.
func (r *TactileRouterShard) buildRouteProposalPrompt(cases []UnhandledCase) string {
	var sb strings.Builder
	sb.WriteString("The following actions have no routing rules:\n\n")

	for i, cas := range cases {
		sb.WriteString(fmt.Sprintf("%d. Action: %s\n", i+1, cas.Query))
		if cas.Context != nil {
			for k, v := range cas.Context {
				sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
			}
		}
	}

	sb.WriteString("\nPropose a tool route for these actions.\n")
	sb.WriteString("Format:\n")
	sb.WriteString("ACTION: <action pattern>\n")
	sb.WriteString("TOOL: <tool name>\n")
	sb.WriteString("TIMEOUT: <seconds>\n")
	sb.WriteString("RATE_LIMIT: <calls per minute, 0 for unlimited>\n")
	sb.WriteString("REQUIRES_SAFE: <true/false>\n")

	return sb.String()
}

// parseProposedRoute extracts a route from LLM output.
func (r *TactileRouterShard) parseProposedRoute(output string) ToolRoute {
	route := ToolRoute{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ACTION:") {
			route.ActionPattern = strings.TrimSpace(strings.TrimPrefix(line, "ACTION:"))
		} else if strings.HasPrefix(line, "TOOL:") {
			route.ToolName = strings.TrimSpace(strings.TrimPrefix(line, "TOOL:"))
		} else if strings.HasPrefix(line, "TIMEOUT:") {
			var secs int
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "TIMEOUT:")), "%d", &secs)
			route.Timeout = time.Duration(secs) * time.Second
		} else if strings.HasPrefix(line, "RATE_LIMIT:") {
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "RATE_LIMIT:")), "%d", &route.RateLimit)
		} else if strings.HasPrefix(line, "REQUIRES_SAFE:") {
			route.RequiresSafe = strings.Contains(strings.ToLower(line), "true")
		}
	}

	return route
}

// generateShutdownSummary creates a summary of the shard's activity.
func (r *TactileRouterShard) generateShutdownSummary(reason string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return fmt.Sprintf(
		"Tactile Router shutdown (%s). Routed: %d, Pending: %d, Routes: %d, Runtime: %s",
		reason,
		len(r.completedCalls),
		len(r.pendingCalls),
		len(r.routes),
		time.Since(r.StartTime).String(),
	)
}

// AddRoute adds a custom route at runtime.
func (r *TactileRouterShard) AddRoute(route ToolRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[route.ActionPattern] = route
	if route.RateLimit > 0 {
		r.rateLimiters[route.ToolName] = newRateLimiter(route.RateLimit)
	}
}

// GetRoutes returns all registered routes.
func (r *TactileRouterShard) GetRoutes() map[string]ToolRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]ToolRoute)
	for k, v := range r.routes {
		result[k] = v
	}
	return result
}

// routerAutopoiesisPrompt is the system prompt for proposing new routes.
const routerAutopoiesisPrompt = `You are the Tactile Router's Autopoiesis system.
Your role is to propose tool routes for unmapped actions.

Available tools in the VirtualStore:
- fs_read, fs_write, fs_edit, fs_delete: File operations
- code_search, impact_analyzer: Code analysis
- shell_exec: Shell command execution
- test_runner, build_tool: Testing and building
- git_tool: Git operations
- http_fetch, browser_tool, research_tool: Network operations
- shard_manager: Delegation to specialized shards
- user_prompt, escalation_handler: User interaction

When proposing routes:
1. Match actions to appropriate tools
2. Set conservative timeouts
3. Apply rate limits to expensive operations
4. Require safety checks (RequiresSafe=true) for mutations

DO NOT propose routes that:
- Bypass safety checks for dangerous operations
- Have no timeout for automated tools
- Allow unlimited rate for expensive operations`
