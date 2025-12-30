package autopoiesis

import (
	"fmt"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// KERNEL INTEGRATION - Bridge to Mangle Logic Core
// =============================================================================

// SetKernel attaches a Mangle kernel for fact assertion and query.
// This enables the full neuro-symbolic loop where autopoiesis
// events are reflected as Mangle facts for logic-driven orchestration.
// Also syncs any existing tools from the registry to the kernel.
func (o *Orchestrator) SetKernel(kernel KernelInterface) {
	o.mu.Lock()
	o.kernel = kernel
	o.mu.Unlock()

	// Sync existing tools from registry to kernel (for tools restored from disk)
	o.syncExistingToolsToKernel()
}

// syncExistingToolsToKernel asserts facts for all tools already in the registry.
// Called when kernel is first attached to ensure restored tools are discoverable.
// Uses batch assertion for performance (single evaluate() call instead of one per fact).
func (o *Orchestrator) syncExistingToolsToKernel() {
	if o.ouroboros == nil || o.kernel == nil {
		return
	}

	tools := o.ouroboros.registry.List()
	if len(tools) == 0 {
		return
	}

	logging.Autopoiesis("Syncing %d existing tools to kernel", len(tools))

	// Collect all facts for batch assertion (avoids O(n) evaluate() calls)
	var allFacts []KernelFact
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		timestamp := tool.RegisteredAt.Format("2006-01-02T15:04:05Z07:00")

		allFacts = append(allFacts, KernelFact{Predicate: "tool_registered", Args: []interface{}{tool.Name, timestamp}})
		allFacts = append(allFacts, KernelFact{Predicate: "tool_hash", Args: []interface{}{tool.Name, tool.Hash}})
		allFacts = append(allFacts, KernelFact{Predicate: "has_capability", Args: []interface{}{tool.Name}})
		if tool.Description != "" {
			allFacts = append(allFacts, KernelFact{Predicate: "tool_description", Args: []interface{}{tool.Name, tool.Description}})
		}
		if tool.BinaryPath != "" {
			allFacts = append(allFacts, KernelFact{Predicate: "tool_binary_path", Args: []interface{}{tool.Name, tool.BinaryPath}})
		}
	}

	if err := o.kernel.AssertFactBatch(allFacts); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to batch assert tool facts: %v", err)
	}
	logging.AutopoiesisDebug("Kernel sync complete: %d tools registered", len(tools))
}

// GetKernel returns the attached kernel (may be nil).
func (o *Orchestrator) GetKernel() KernelInterface {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.kernel
}

// =============================================================================
// KERNEL FACT ASSERTION - Wire Autopoiesis Events to Mangle
// =============================================================================

// assertToKernel safely asserts a fact to the kernel if attached.
func (o *Orchestrator) assertToKernel(predicate string, args ...interface{}) error {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return nil // No kernel attached, silently skip
	}

	return kernel.AssertFact(KernelFact{
		Predicate: predicate,
		Args:      args,
	})
}

// assertToolRegistered asserts tool_registered and related facts to kernel.
// Called when a tool is successfully generated and registered.
// Facts: tool_registered, tool_hash, has_capability, tool_description, tool_binary_path
func (o *Orchestrator) assertToolRegistered(tool *RuntimeTool) {
	if tool == nil {
		return
	}

	timestamp := tool.RegisteredAt.Format("2006-01-02T15:04:05Z07:00")

	// tool_registered(ToolName, RegisteredAt)
	_ = o.assertToKernel("tool_registered", tool.Name, timestamp)

	// tool_hash(ToolName, Hash)
	_ = o.assertToKernel("tool_hash", tool.Name, tool.Hash)

	// has_capability(ToolName)
	_ = o.assertToKernel("has_capability", tool.Name)

	// tool_description(ToolName, Description) - for LLM tool discovery
	if tool.Description != "" {
		_ = o.assertToKernel("tool_description", tool.Name, tool.Description)
	}

	// tool_binary_path(ToolName, BinaryPath) - for tool execution
	if tool.BinaryPath != "" {
		_ = o.assertToKernel("tool_binary_path", tool.Name, tool.BinaryPath)
	}
}

// assertMissingTool asserts missing_tool_for fact to kernel.
// Called when a capability gap is detected.
func (o *Orchestrator) assertMissingTool(intentID, capability string) {
	// missing_tool_for(Intent, Capability)
	_ = o.assertToKernel("missing_tool_for", intentID, capability)
}

// assertToolHotReloaded asserts tool_hot_loaded and tool_version facts to kernel.
// GAP-019 FIX: Propagates hot-reload events from OuroborosLoop's internal engine
// to the parent kernel for spreading activation and JIT awareness.
func (o *Orchestrator) assertToolHotReloaded(toolName string) {
	timestamp := time.Now().Unix()

	// tool_hot_loaded(ToolName, Timestamp)
	_ = o.assertToKernel("tool_hot_loaded", toolName, timestamp)

	// tool_version(ToolName, Version) - start at 1 for newly hot-loaded tools
	// Note: Versioning is tracked in OuroborosLoop's internal engine, but we
	// assert version 1 to the parent kernel as a marker that the tool is current
	_ = o.assertToKernel("tool_version", toolName, 1)
}

// assertToolLearning asserts tool_learning fact to kernel.
// Called when execution feedback is recorded.
func (o *Orchestrator) assertToolLearning(toolName string, executions int, successRate, avgQuality float64) {
	// tool_learning(ToolName, Executions, SuccessRate, AvgQuality)
	_ = o.assertToKernel("tool_learning", toolName, executions, successRate, avgQuality)
}

// assertToolKnownIssue asserts tool_known_issue fact to kernel.
func (o *Orchestrator) assertToolKnownIssue(toolName string, issueType string) {
	// tool_known_issue(ToolName, IssueType) - use name constant format
	_ = o.assertToKernel("tool_known_issue", toolName, "/"+issueType)
}

// assertAgentCreated asserts facts about the created agent to the kernel
func (o *Orchestrator) assertAgentCreated(spec *AgentSpec) {
	if spec == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05Z07:00")

	// agent_created(AgentName, Type, CreatedAt)
	_ = o.assertToKernel("agent_created", spec.Name, spec.Type, timestamp)

	// agent_purpose(AgentName, Purpose)
	_ = o.assertToKernel("agent_purpose", spec.Name, spec.Purpose)

	// agent_schedule(AgentName, ScheduleType)
	_ = o.assertToKernel("agent_schedule", spec.Name, spec.Schedule.Type)

	// For each trigger, assert trigger facts
	for _, trigger := range spec.Triggers {
		_ = o.assertToKernel("agent_trigger", spec.Name, trigger.Type, trigger.Pattern)
	}

	// If memory is enabled, assert memory capability
	if spec.Memory.Enabled {
		_ = o.assertToKernel("agent_has_memory", spec.Name)
	}
}

// SyncLearningsToKernel pushes all current learnings to the kernel.
// Call this periodically or after significant learning updates.
func (o *Orchestrator) SyncLearningsToKernel() {
	learnings := o.learnings.GetAllLearnings()
	for _, learning := range learnings {
		// Prune old learnings for this tool (functional update)
		_ = o.kernel.RetractFact(KernelFact{
			Predicate: "tool_learning",
			Args:      []interface{}{learning.ToolName}, // Match by ToolName
		})

		// Assert new learning
		o.assertToolLearning(
			learning.ToolName,
			learning.TotalExecutions,
			learning.SuccessRate,
			learning.AverageQuality,
		)

		// Prune known issues
		_ = o.kernel.RetractFact(KernelFact{
			Predicate: "tool_known_issue",
			Args:      []interface{}{learning.ToolName},
		})

		for _, issue := range learning.KnownIssues {
			o.assertToolKnownIssue(learning.ToolName, string(issue))
		}
	}
}

// =============================================================================
// CODE DOM INTEGRATION
// =============================================================================
// Methods for querying code elements from the kernel.
// Code DOM facts are asserted to the kernel by VirtualStore when files are opened.

// QueryCodeElementCount queries the kernel for code_element facts.
// Returns the number of elements in scope.
func (o *Orchestrator) QueryCodeElementCount() int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("code_element")
	if err != nil {
		return 0
	}
	return len(facts)
}

// QueryElementsByType returns count of elements matching a type.
func (o *Orchestrator) QueryElementsByType(elemType string) int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("code_element")
	if err != nil {
		return 0
	}

	count := 0
	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			if t, ok := fact.Args[1].(string); ok {
				if t == "/"+elemType || t == elemType {
					count++
				}
			}
		}
	}
	return count
}

// QueryActiveFile returns the currently active file from the kernel.
func (o *Orchestrator) QueryActiveFile() string {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return ""
	}

	facts, err := kernel.QueryPredicate("active_file")
	if err != nil || len(facts) == 0 {
		return ""
	}

	if len(facts[0].Args) >= 1 {
		if path, ok := facts[0].Args[0].(string); ok {
			return path
		}
	}
	return ""
}

// QueryFilesInScope returns the count of files in the current scope.
func (o *Orchestrator) QueryFilesInScope() int {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return 0
	}

	facts, err := kernel.QueryPredicate("file_in_scope")
	if err != nil {
		return 0
	}
	return len(facts)
}

// RecordCodeEditOutcome records the outcome of a code edit for learning.
func (o *Orchestrator) RecordCodeEditOutcome(elementRef string, editType string, success bool) {
	successStr := "/false"
	if success {
		successStr = "/true"
	}

	timestamp := time.Now().Unix()

	// Prune old events if we exceed the limit
	// We use the 4-arity predicate code_edit_outcome(Ref, Type, Success, Timestamp)
	facts, err := o.kernel.QueryPredicate("code_edit_outcome")
	if err == nil && len(facts) >= o.config.MaxLearningFacts {
		// Find oldest fact to retract
		// Note: This assumes all facts are 4-arity and 4th arg is timestamp (int/int64/float64)
		var oldestFact *KernelFact
		var oldestTime int64 = -1

		for _, f := range facts {
			if len(f.Args) < 4 {
				continue // Ignore legacy 3-arity facts
			}

			// Extract timestamp
			var ts int64
			switch v := f.Args[3].(type) {
			case int:
				ts = int64(v)
			case int64:
				ts = v
			case float64:
				ts = int64(v)
			default:
				continue
			}

			if oldestTime == -1 || ts < oldestTime {
				oldestTime = ts
				// Copy loop variable
				factCopy := f
				oldestFact = &factCopy
			}
		}

		if oldestFact != nil {
			_ = o.kernel.RetractFact(*oldestFact)
		}
	}

	// Assert new fact with timestamp
	_ = o.assertToKernel("code_edit_outcome", elementRef, "/"+editType, successStr, timestamp)
}

// QueryNextAction queries the kernel for derived next_action facts.
// Returns the action type if the kernel derives one, empty string otherwise.
func (o *Orchestrator) QueryNextAction() string {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return ""
	}

	// Check for various autopoiesis-related next actions
	actions := []string{
		"next_action(/generate_tool)",
		"next_action(/refine_tool)",
	}

	for _, action := range actions {
		if kernel.QueryBool(action) {
			// Extract the action name from the query
			return action
		}
	}

	return ""
}

// ShouldGenerateTool queries the kernel to check if tool generation is needed.
// This provides logic-driven triggering instead of just heuristics.
func (o *Orchestrator) ShouldGenerateTool() bool {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return false
	}

	return kernel.QueryBool("next_action(/generate_tool)")
}

// ShouldRefineToolByKernel queries the kernel to check if a tool needs refinement.
func (o *Orchestrator) ShouldRefineToolByKernel(toolName string) bool {
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()

	if kernel == nil {
		return false
	}

	// Query for tool_needs_refinement(toolName)
	return kernel.QueryBool(fmt.Sprintf("tool_needs_refinement(%q)", toolName))
}
