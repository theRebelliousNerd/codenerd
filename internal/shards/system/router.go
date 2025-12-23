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
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
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
			// System lifecycle actions (internal kernel events - no external tool)
			{ActionPattern: "system_start", ToolName: "kernel_internal", Timeout: 1 * time.Second, RequiresSafe: false},
			{ActionPattern: "initialize", ToolName: "kernel_internal", Timeout: 1 * time.Second, RequiresSafe: false},
			{ActionPattern: "shutdown", ToolName: "kernel_internal", Timeout: 1 * time.Second, RequiresSafe: false},
			{ActionPattern: "heartbeat", ToolName: "kernel_internal", Timeout: 1 * time.Second, RequiresSafe: false},

			// File operations
			{ActionPattern: "read_file", ToolName: "fs_read", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "fs_read", ToolName: "fs_read", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "write_file", ToolName: "fs_write", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "fs_write", ToolName: "fs_write", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "edit_file", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "delete_file", ToolName: "fs_delete", Timeout: 10 * time.Second, RequiresSafe: true},

			// Code operations
			{ActionPattern: "search_code", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "analyze_impact", ToolName: "impact_analyzer", Timeout: 60 * time.Second, RequiresSafe: false},

			// Execution
			{ActionPattern: "exec_cmd", ToolName: "shell_exec", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "run_tests", ToolName: "test_runner", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "build_project", ToolName: "build_tool", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 5},

			// Python environment + SWE-bench orchestration
			{ActionPattern: "python_env_setup", ToolName: "python_env", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 3},
			{ActionPattern: "python_env_exec", ToolName: "python_env", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "python_run_pytest", ToolName: "python_env", Timeout: 600 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "python_apply_patch", ToolName: "python_env", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "python_snapshot", ToolName: "python_env", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "python_restore", ToolName: "python_env", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "python_teardown", ToolName: "python_env", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "swebench_setup", ToolName: "swebench", Timeout: 600 * time.Second, RequiresSafe: true, RateLimit: 3},
			{ActionPattern: "swebench_apply_patch", ToolName: "swebench", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "swebench_run_tests", ToolName: "swebench", Timeout: 900 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "swebench_snapshot", ToolName: "swebench", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "swebench_restore", ToolName: "swebench", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "swebench_evaluate", ToolName: "swebench", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "swebench_teardown", ToolName: "swebench", Timeout: 180 * time.Second, RequiresSafe: true, RateLimit: 5},

			// Git operations
			{ActionPattern: "git_operation", ToolName: "git_tool", Timeout: 60 * time.Second, RequiresSafe: true, RateLimit: 20},
			{ActionPattern: "show_diff", ToolName: "git_tool", Timeout: 30 * time.Second, RequiresSafe: false, RateLimit: 20},

			// Network
			{ActionPattern: "fetch", ToolName: "http_fetch", Timeout: 30 * time.Second, RequiresSafe: true, RateLimit: 30},
			{ActionPattern: "browse", ToolName: "browser_tool", Timeout: 60 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "research", ToolName: "research_tool", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 5},

			// Delegation
			{ActionPattern: "delegate", ToolName: "shard_manager", Timeout: 600 * time.Second, RequiresSafe: true},

			// User interaction
			{ActionPattern: "ask_user", ToolName: "user_prompt", Timeout: 0, RequiresSafe: false}, // No timeout for user
			{ActionPattern: "escalate", ToolName: "escalation_handler", Timeout: 0, RequiresSafe: false},

			// Campaign operations (prefix matches all campaign_* actions)
			{ActionPattern: "campaign_", ToolName: "campaign_tool", Timeout: 120 * time.Second, RequiresSafe: true},
			{ActionPattern: "archive_campaign", ToolName: "campaign_tool", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "show_campaign", ToolName: "campaign_tool", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "run_phase_checkpoint", ToolName: "campaign_tool", Timeout: 60 * time.Second, RequiresSafe: true},
			{ActionPattern: "investigate_systemic", ToolName: "analysis_tool", Timeout: 120 * time.Second, RequiresSafe: true},
			{ActionPattern: "pause_and_replan", ToolName: "campaign_tool", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "ask_campaign_interrupt", ToolName: "user_prompt", Timeout: 0, RequiresSafe: false},

			// TDD repair loop
			{ActionPattern: "read_error_log", ToolName: "tdd_tool", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "analyze_root_cause", ToolName: "tdd_tool", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "generate_patch", ToolName: "shard_manager", Timeout: 120 * time.Second, RequiresSafe: true},
			{ActionPattern: "complete", ToolName: "tdd_tool", Timeout: 1 * time.Second, RequiresSafe: false},

			// Autopoiesis/Ouroboros (prefix matches ouroboros_*)
			{ActionPattern: "generate_tool", ToolName: "ouroboros_tool", Timeout: 120 * time.Second, RequiresSafe: true},
			{ActionPattern: "refine_tool", ToolName: "ouroboros_tool", Timeout: 60 * time.Second, RequiresSafe: true},
			{ActionPattern: "ouroboros_", ToolName: "ouroboros_tool", Timeout: 120 * time.Second, RequiresSafe: true},

			// Strategic/control
			{ActionPattern: "resume_task", ToolName: "session_control", Timeout: 1 * time.Second, RequiresSafe: false},
			{ActionPattern: "escalate_to_user", ToolName: "user_prompt", Timeout: 0, RequiresSafe: false},
			{ActionPattern: "interrogative_mode", ToolName: "session_control", Timeout: 1 * time.Second, RequiresSafe: false},
			{ActionPattern: "refresh_shard_context", ToolName: "session_control", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "update_world_model", ToolName: "world_model", Timeout: 30 * time.Second, RequiresSafe: false},

			// Context management
			{ActionPattern: "compress_context", ToolName: "context_manager", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "emergency_compress", ToolName: "context_manager", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "create_checkpoint", ToolName: "context_manager", Timeout: 10 * time.Second, RequiresSafe: false},

			// Code DOM operations
			{ActionPattern: "edit_element", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "open_file", ToolName: "fs_read", Timeout: 10 * time.Second, RequiresSafe: false},
			{ActionPattern: "query_elements", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "refresh_scope", ToolName: "code_scope", Timeout: 10 * time.Second, RequiresSafe: false},

			// Corrective operations (prefix matches corrective_*)
			{ActionPattern: "corrective_", ToolName: "shard_manager", Timeout: 120 * time.Second, RequiresSafe: true},

			// =====================================================================
			// PASS 2 ROUTES: Full coverage for all derived actions
			// =====================================================================

			// Investigation operations
			{ActionPattern: "investigate_anomaly", ToolName: "analysis_tool", Timeout: 120 * time.Second, RequiresSafe: false},

			// Review operations (delegate to specialized shards)
			{ActionPattern: "review", ToolName: "shard_manager", Timeout: 300 * time.Second, RequiresSafe: false},
			{ActionPattern: "lint", ToolName: "shard_manager", Timeout: 120 * time.Second, RequiresSafe: false},
			{ActionPattern: "check_security", ToolName: "shard_manager", Timeout: 180 * time.Second, RequiresSafe: false},

			// Code analysis operations
			{ActionPattern: "analyze_code", ToolName: "code_search", Timeout: 60 * time.Second, RequiresSafe: false},
			{ActionPattern: "parse_ast", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "query_symbols", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "check_syntax", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "code_graph", ToolName: "code_search", Timeout: 60 * time.Second, RequiresSafe: false},

			// Additional test operations
			{ActionPattern: "coverage", ToolName: "test_runner", Timeout: 300 * time.Second, RequiresSafe: true, RateLimit: 5},
			{ActionPattern: "test_single", ToolName: "test_runner", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 10},

			// Knowledge/vector operations (kernel internal - query only)
			{ActionPattern: "vector_search", ToolName: "kernel_internal", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "knowledge_query", ToolName: "kernel_internal", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "embed_text", ToolName: "kernel_internal", Timeout: 10 * time.Second, RequiresSafe: false},

			// Browser operations
			{ActionPattern: "browser_navigate", ToolName: "browser_tool", Timeout: 60 * time.Second, RequiresSafe: true, RateLimit: 10},
			{ActionPattern: "browser_screenshot", ToolName: "browser_tool", Timeout: 30 * time.Second, RequiresSafe: false, RateLimit: 20},
			{ActionPattern: "browser_read_dom", ToolName: "browser_tool", Timeout: 30 * time.Second, RequiresSafe: false, RateLimit: 30},

			// File search operations
			{ActionPattern: "glob_files", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "search_files", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},

			// Extended Code DOM operations
			{ActionPattern: "close_scope", ToolName: "code_scope", Timeout: 5 * time.Second, RequiresSafe: false},
			{ActionPattern: "edit_lines", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "insert_lines", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "delete_lines", ToolName: "fs_edit", Timeout: 30 * time.Second, RequiresSafe: true},
			{ActionPattern: "get_elements", ToolName: "code_search", Timeout: 30 * time.Second, RequiresSafe: false},
			{ActionPattern: "get_element", ToolName: "code_search", Timeout: 10 * time.Second, RequiresSafe: false},

			// Autopoiesis tool execution
			{ActionPattern: "exec_tool", ToolName: "shell_exec", Timeout: 120 * time.Second, RequiresSafe: true, RateLimit: 10},

			// Delegate routing (explicit shard delegation)
			{ActionPattern: "delegate_reviewer", ToolName: "shard_manager", Timeout: 300 * time.Second, RequiresSafe: false},
			{ActionPattern: "delegate_coder", ToolName: "shard_manager", Timeout: 600 * time.Second, RequiresSafe: true},
			{ActionPattern: "delegate_researcher", ToolName: "shard_manager", Timeout: 300 * time.Second, RequiresSafe: false},
			{ActionPattern: "delegate_tool_generator", ToolName: "shard_manager", Timeout: 180 * time.Second, RequiresSafe: true},
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

	// Dependencies
	BrowserManager *browser.SessionManager

	// Tracking
	pendingCalls   []ToolCall
	completedCalls []ToolCall
	lastActivity   time.Time
	lastLogPrune   time.Time

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
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionExecCmd,
		types.PermissionNetwork,
		types.PermissionBrowser,
	}
	base.Config.Model = types.ModelConfig{} // No LLM by default

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

// SetBrowserManager injects the browser session manager.
func (r *TactileRouterShard) SetBrowserManager(mgr *browser.SessionManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.BrowserManager = mgr
}

// Execute runs the Tactile Router's action routing loop.
// This shard is ON-DEMAND and auto-stops after IdleTimeout.
func (r *TactileRouterShard) Execute(ctx context.Context, task string) (string, error) {
	r.SetState(types.ShardStateRunning)
	r.mu.Lock()
	r.running = true
	r.StartTime = time.Now()
	r.lastActivity = time.Now()
	r.mu.Unlock()

	defer func() {
		r.SetState(types.ShardStateCompleted)
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	// Initialize kernel if not set
	if r.Kernel == nil {
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		r.Kernel = kernel
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
				_ = r.Kernel.Assert(types.Fact{
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

	// Query permitted_action facts (constitution-cleared actions)
	permitted, err := r.Kernel.Query("permitted_action")
	if err != nil {
		return fmt.Errorf("failed to query permitted_action: %w", err)
	}

	didWork := false
	for _, fact := range permitted {
		if len(fact.Args) < 3 {
			continue
		}
		didWork = true

		// permitted_action(ActionID, ActionType, Target, Payload, Timestamp)
		actionID := fmt.Sprintf("%v", fact.Args[0])
		actionType := fmt.Sprintf("%v", fact.Args[1])
		target := fmt.Sprintf("%v", fact.Args[2])
		payload := map[string]interface{}{}
		if len(fact.Args) > 3 {
			if pm, ok := fact.Args[3].(map[string]interface{}); ok {
				payload = pm
			}
		}

		// Update activity tracking
		r.mu.Lock()
		r.lastActivity = time.Now()
		r.mu.Unlock()

		// Find route for action
		route, found := r.findRoute(actionType)

		if !found {
			logging.Routing("No route found for action: %s (target=%s)", actionType, target)
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
			_ = r.Kernel.Assert(types.Fact{
				Predicate: "routing_error",
				Args:      []interface{}{actionType, "no_handler", time.Now().Unix()},
			})
			continue
		}
		logging.Routing("Route found: action=%s -> tool=%s (timeout=%v)", actionType, route.ToolName, route.Timeout)

		// Check rate limit
		if limiter, exists := r.rateLimiters[route.ToolName]; exists {
			if !limiter.allow() {
				logging.Routing("Rate limit exceeded for tool: %s (action=%s)", route.ToolName, actionType)
				_ = r.Kernel.Assert(types.Fact{
					Predicate: "routing_error",
					Args:      []interface{}{actionType, "rate_limit_exceeded", time.Now().Unix()},
				})
				continue
			}
		}

		// Handle kernel internal actions (system lifecycle events)
		if route.ToolName == "kernel_internal" {
			logging.Routing("Internal kernel action acknowledged: %s", actionType)
			_ = r.Kernel.Assert(types.Fact{
				Predicate: "system_event_handled",
				Args:      []interface{}{actionType, target, time.Now().Unix()},
			})
			// Clear the permitted action to avoid repeated processing
			if r.Kernel != nil {
				_ = r.Kernel.RetractExactFact(fact)
				_ = r.Kernel.RetractFact(types.Fact{
					Predicate: "action_permitted",
					Args:      []interface{}{actionID},
				})
			}
			continue
		}

		// Create tool call
		call := ToolCall{
			ID:       actionID,
			Tool:     route.ToolName,
			Action:   actionType,
			Target:   target,
			Payload:  payload,
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
			logging.Tools("Executing tool: %s (action=%s, target=%s, call_id=%s)", route.ToolName, actionType, target, call.ID)

			// Create action fact for VirtualStore (preserve payload)
			actionFact := types.Fact{
				Predicate: "next_action",
				Args:      []interface{}{call.ID, actionType, target, payload},
			}

			result, err := r.VirtualStore.RouteAction(ctx, actionFact)

			call.CompletedAt = time.Now()
			duration := call.CompletedAt.Sub(call.StartedAt)
			if err != nil {
				call.Status = "failed"
				call.Error = err.Error()
				logging.Get(logging.CategoryTools).Error("Tool execution failed: %s (call_id=%s, duration=%v, error=%s)", route.ToolName, call.ID, duration, err.Error())
				_ = r.Kernel.Assert(types.Fact{
					Predicate: "routing_result",
					Args:      []interface{}{call.ID, types.MangleAtom("/failure"), err.Error(), call.CompletedAt.Unix()},
				})
			} else {
				call.Status = "completed"
				call.Result = result
				logging.Tools("Tool execution completed: %s (call_id=%s, duration=%v, result_len=%d)", route.ToolName, call.ID, duration, len(result))
				_ = r.Kernel.Assert(types.Fact{
					Predicate: "routing_result",
					Args:      []interface{}{call.ID, types.MangleAtom("/success"), result, call.CompletedAt.Unix()},
				})
			}

			// Emit tool event for always-visible tool execution in chat
			if r.ToolEventBus != nil {
				result := call.Result
				if len(result) > 500 {
					result = result[:500] + "..."
				}
				success := true
				if call.Error != "" {
					result = call.Error
					success = false
				}
				r.ToolEventBus.Emit(transparency.ToolEvent{
					ToolName:  route.ToolName,
					Result:    result,
					Success:   success,
					Duration:  duration,
					Timestamp: time.Now(),
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
			_ = r.Kernel.Assert(types.Fact{
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

		// Clear the permitted action (exact match)
		if r.Kernel != nil {
			_ = r.Kernel.RetractExactFact(fact)
		}
		// Also clear unary marker for this action type
		_ = r.Kernel.RetractFact(types.Fact{
			Predicate: "action_permitted",
			Args:      []interface{}{actionID},
		})
	}

	if didWork {
		r.pruneRoutingResults()
	}

	return nil
}

func (r *TactileRouterShard) pruneRoutingResults() {
	if r.Kernel == nil {
		return
	}

	now := time.Now()
	r.mu.Lock()
	if !r.lastLogPrune.IsZero() && now.Sub(r.lastLogPrune) < 10*time.Second {
		r.mu.Unlock()
		return
	}
	r.lastLogPrune = now
	r.mu.Unlock()

	const retention = 15 * time.Minute
	cutoff := now.Add(-retention).Unix()

	results, err := r.Kernel.Query("routing_result")
	if err != nil || len(results) == 0 {
		return
	}

	toRemove := make([]types.Fact, 0, len(results)/4)
	for _, f := range results {
		ts, ok := unixSecondsArg(f, 3)
		if !ok {
			continue
		}
		if ts < cutoff {
			toRemove = append(toRemove, f)
		}
	}

	if len(toRemove) == 0 {
		return
	}
	_ = r.Kernel.RetractExactFactsBatch(toRemove)
}

func unixSecondsArg(f core.Fact, idx int) (int64, bool) {
	if idx < 0 || len(f.Args) <= idx {
		return 0, false
	}
	switch v := f.Args[idx].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// findRoute finds a route for the given action type.
func (r *TactileRouterShard) findRoute(actionType string) (ToolRoute, bool) {
	normalizedAction := strings.TrimPrefix(actionType, "/")

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact match (prefer normalized form, but allow raw key too).
	if route, exists := r.routes[normalizedAction]; exists {
		return route, true
	}
	if route, exists := r.routes[actionType]; exists {
		return route, true
	}

	// Deterministic best-match routing:
	// - Prefer prefix matches over contains matches.
	// - Prefer longer (more specific) patterns.
	// - Tie-break lexicographically on normalized pattern.
	const (
		matchNone     = 0
		matchContains = 1
		matchPrefix   = 2
		matchExact    = 3
	)

	bestScore := matchNone
	bestLen := -1
	bestPattern := ""
	bestRoute := ToolRoute{}

	for pattern, route := range r.routes {
		normalizedPattern := strings.TrimPrefix(pattern, "/")
		if normalizedPattern == "" {
			continue
		}

		score := matchNone
		switch {
		case normalizedAction == normalizedPattern:
			score = matchExact
		case strings.HasPrefix(normalizedAction, normalizedPattern):
			score = matchPrefix
		case strings.Contains(normalizedAction, normalizedPattern):
			score = matchContains
		default:
			continue
		}

		if score > bestScore ||
			(score == bestScore && len(normalizedPattern) > bestLen) ||
			(score == bestScore && len(normalizedPattern) == bestLen && normalizedPattern < bestPattern) {
			bestScore = score
			bestLen = len(normalizedPattern)
			bestPattern = normalizedPattern
			bestRoute = route
		}
	}

	if bestScore == matchNone {
		return ToolRoute{}, false
	}
	return bestRoute, true
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

	userPrompt := r.buildRouteProposalPrompt(cases)

	// Try JIT prompt compilation first, fall back to legacy constant
	systemPrompt, jitUsed := r.TryJITPrompt(ctx, "router_autopoiesis")
	if !jitUsed {
		systemPrompt = routerAutopoiesisPrompt
		logging.SystemShards("[TactileRouter] [FALLBACK] Using legacy autopoiesis prompt")
	} else {
		logging.SystemShards("[TactileRouter] [JIT] Using JIT-compiled autopoiesis prompt")
	}

	result, err := r.GuardedLLMCall(ctx, systemPrompt, userPrompt)
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
		_ = r.Kernel.Assert(types.Fact{
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

// DEPRECATED: routerAutopoiesisPrompt is the legacy system prompt for proposing new routes.
// Prefer JIT prompt compilation via TryJITPrompt() when available.
// This constant is retained as a fallback for when JIT is unavailable.
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
