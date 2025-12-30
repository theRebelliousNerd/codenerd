package core

import (
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// TOOL REGISTRY - Integration with Kernel and Shards
// =============================================================================
// This registry bridges the gap between generated tools (Ouroboros),
// the Mangle kernel (facts), and shards (execution).

// Tool represents a registered tool with metadata
type Tool struct {
	Name          string    `json:"name"`
	Command       string    `json:"command"`        // Path to binary or command to execute
	ShardAffinity string    `json:"shard_affinity"` // /coder, /tester, /reviewer, /researcher, /generalist, /all
	Description   string    `json:"description"`
	Capabilities  []string  `json:"capabilities"`
	Hash          string    `json:"hash"` // Binary hash for change detection
	RegisteredAt  time.Time `json:"registered_at"`
	ExecuteCount  int64     `json:"execute_count"`
}

// ToolRegistry manages registered tools and their integration with the kernel
type ToolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]*Tool
	kernel  Kernel
	workDir string
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(workDir string) *ToolRegistry {
	return &ToolRegistry{
		tools:   make(map[string]*Tool),
		workDir: workDir,
	}
}

// SetKernel sets the kernel for fact injection
func (tr *ToolRegistry) SetKernel(k Kernel) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.kernel = k
}

// RegisterTool registers a tool and injects facts into the kernel
func (tr *ToolRegistry) RegisterTool(name, command, shardAffinity string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	logging.ToolsDebug("RegisterTool: registering tool name=%s command=%s affinity=%s", name, command, shardAffinity)

	if name == "" {
		logging.ToolsError("RegisterTool: tool name cannot be empty")
		return fmt.Errorf("tool name cannot be empty")
	}
	if command == "" {
		logging.ToolsError("RegisterTool: tool command cannot be empty for %s", name)
		return fmt.Errorf("tool command cannot be empty")
	}

	// Verify command exists if it's a file path
	if !isCommandName(command) {
		absPath := command
		if !filepath.IsAbs(command) {
			absPath = filepath.Join(tr.workDir, command)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			logging.ToolsError("RegisterTool: tool binary not found: %s", absPath)
			return fmt.Errorf("tool binary not found: %s", absPath)
		}
		command = absPath
	}

	// Create tool
	tool := &Tool{
		Name:          name,
		Command:       command,
		ShardAffinity: shardAffinity,
		RegisteredAt:  time.Now(),
	}

	tr.tools[name] = tool

	// Inject facts into kernel
	if err := tr.injectToolFacts(tool); err != nil {
		logging.ToolsError("RegisterTool: failed to inject facts for %s: %v", name, err)
		return err
	}

	logging.Tools("RegisterTool: successfully registered tool %s (affinity=%s)", name, shardAffinity)
	return nil
}

// RegisterToolWithInfo registers a tool with full metadata
func (tr *ToolRegistry) RegisterToolWithInfo(tool *Tool) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if tool.RegisteredAt.IsZero() {
		tool.RegisteredAt = time.Now()
	}

	tr.tools[tool.Name] = tool

	// Inject facts into kernel
	return tr.injectToolFacts(tool)
}

// GetTool retrieves a registered tool by name
func (tr *ToolRegistry) GetTool(name string) (*Tool, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	tool, exists := tr.tools[name]
	return tool, exists
}

// GetToolsForShard returns all tools that can be used by a specific shard type
func (tr *ToolRegistry) GetToolsForShard(shardType string) []*Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := make([]*Tool, 0)
	for _, tool := range tr.tools {
		if tool.ShardAffinity == "/all" || tool.ShardAffinity == shardType {
			tools = append(tools, tool)
		}
	}
	return tools
}

// ListTools returns all registered tools
func (tr *ToolRegistry) ListTools() []*Tool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	tools := make([]*Tool, 0, len(tr.tools))
	for _, tool := range tr.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool runs a registered tool with raw input (ToolExecutor interface).
func (tr *ToolRegistry) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	args := parseToolInput(input)
	return tr.ExecuteRegisteredTool(ctx, toolName, args)
}

// ExecuteRegisteredTool executes a registered tool with the given arguments
func (tr *ToolRegistry) ExecuteRegisteredTool(ctx context.Context, toolName string, args []string) (string, error) {
	tool, exists := tr.GetTool(toolName)
	if !exists {
		logging.ToolsError("ExecuteRegisteredTool: tool not registered: %s", toolName)
		return "", fmt.Errorf("tool not registered: %s", toolName)
	}

	// Update execution count
	tr.mu.Lock()
	tool.ExecuteCount++
	execCount := tool.ExecuteCount
	tr.mu.Unlock()

	logging.Tools("ExecuteRegisteredTool: executing tool=%s exec_count=%d args=%v", toolName, execCount, args)

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, tool.Command, args...)
	if tr.workDir != "" {
		cmd.Dir = tr.workDir
	}
	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime)
	output := string(out)

	if err != nil {
		logging.ToolsError("ExecuteRegisteredTool: tool=%s failed after %v: %v (output_len=%d)", toolName, duration, err, len(output))
		return output, fmt.Errorf("tool execution failed: %w", err)
	}

	logging.Tools("ExecuteRegisteredTool: tool=%s completed in %v (output_len=%d)", toolName, duration, len(output))
	return output, nil
}

// UnregisterTool removes a tool from the registry and retracts its facts
func (tr *ToolRegistry) UnregisterTool(name string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	logging.ToolsDebug("UnregisterTool: unregistering tool %s", name)

	if _, exists := tr.tools[name]; !exists {
		logging.ToolsError("UnregisterTool: tool not registered: %s", name)
		return fmt.Errorf("tool not registered: %s", name)
	}

	delete(tr.tools, name)

	// Retract only facts for this specific tool (not all tool facts)
	if tr.kernel != nil {
		var errs []error

		// Retract facts specific to this tool using the tool name as first argument
		if err := tr.kernel.RetractFact(Fact{Predicate: "registered_tool", Args: []interface{}{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract registered_tool: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_registered", Args: []interface{}{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_registered: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_hash", Args: []interface{}{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_hash: %w", err))
		}
		if err := tr.kernel.RetractFact(Fact{Predicate: "tool_capability", Args: []interface{}{name}}); err != nil {
			errs = append(errs, fmt.Errorf("failed to retract tool_capability: %w", err))
		}

		if len(errs) > 0 {
			logging.ToolsError("UnregisterTool: errors retracting facts for %s: %v", name, errs)
			return fmt.Errorf("errors retracting tool facts: %v", errs)
		}
	}

	logging.Tools("UnregisterTool: successfully unregistered tool %s", name)
	return nil
}

// injectToolFacts injects tool registration facts into the kernel
func (tr *ToolRegistry) injectToolFacts(tool *Tool) error {
	if tr.kernel == nil {
		return nil // No kernel configured, skip fact injection
	}

	// Inject registration facts
	facts := []Fact{
		{
			Predicate: "registered_tool",
			Args:      []interface{}{tool.Name, tool.Command, tool.ShardAffinity},
		},
		{
			Predicate: "tool_registered",
			Args:      []interface{}{tool.Name, tool.RegisteredAt.Format(time.RFC3339)},
		},
	}

	if tool.Hash != "" {
		facts = append(facts, Fact{
			Predicate: "tool_hash",
			Args:      []interface{}{tool.Name, tool.Hash},
		})
	}

	// Inject capability facts
	for _, cap := range tool.Capabilities {
		facts = append(facts, Fact{
			Predicate: "tool_capability",
			Args:      []interface{}{tool.Name, cap},
		})
	}

	// Assert all facts
	for _, fact := range facts {
		if err := tr.kernel.Assert(fact); err != nil {
			return fmt.Errorf("failed to inject fact %v: %w", fact, err)
		}
	}

	return nil
}

// SyncFromOuroboros synchronizes tools from an Ouroboros registry.
// Continues syncing even if individual tools fail, collecting all errors.
func (tr *ToolRegistry) SyncFromOuroboros(toolExecutor ToolExecutor) error {
	if toolExecutor == nil {
		logging.ToolsDebug("SyncFromOuroboros: no tool executor provided, skipping")
		return nil
	}

	tools := toolExecutor.ListTools()
	logging.ToolsDebug("SyncFromOuroboros: syncing %d tools from Ouroboros", len(tools))
	var errs []error
	syncedCount := 0

	for _, toolInfo := range tools {
		tool := &Tool{
			Name:          toolInfo.Name,
			Command:       toolInfo.BinaryPath,
			ShardAffinity: "/all", // Default affinity
			Description:   toolInfo.Description,
			Hash:          toolInfo.Hash,
			RegisteredAt:  toolInfo.RegisteredAt,
			ExecuteCount:  toolInfo.ExecuteCount,
		}

		if err := tr.RegisterToolWithInfo(tool); err != nil {
			logging.ToolsWarn("SyncFromOuroboros: failed to sync tool %s: %v", tool.Name, err)
			errs = append(errs, fmt.Errorf("tool %s: %w", tool.Name, err))
		} else {
			syncedCount++
		}
	}

	if len(errs) > 0 {
		logging.ToolsError("SyncFromOuroboros: synced %d tools, %d failed", syncedCount, len(errs))
		return fmt.Errorf("synced %d tools, %d failed: %v", syncedCount, len(errs), errs)
	}
	logging.Tools("SyncFromOuroboros: successfully synced %d tools from Ouroboros", syncedCount)
	return nil
}

// RestoreFromDisk restores the registry from a directory of compiled tools.
// Continues restoring even if individual tools fail, collecting all errors.
func (tr *ToolRegistry) RestoreFromDisk(compiledDir string) error {
	logging.ToolsDebug("RestoreFromDisk: restoring tools from %s", compiledDir)
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		if os.IsNotExist(err) {
			logging.ToolsDebug("RestoreFromDisk: directory %s does not exist, skipping", compiledDir)
			return nil // Directory doesn't exist yet, that's okay
		}
		logging.ToolsError("RestoreFromDisk: failed to read directory %s: %v", compiledDir, err)
		return fmt.Errorf("failed to read compiled tools directory: %w", err)
	}

	var errs []error
	restoredCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Strip extension
		if ext := filepath.Ext(name); ext == ".exe" {
			name = name[:len(name)-len(ext)]
		}

		binaryPath := filepath.Join(compiledDir, entry.Name())

		tool := &Tool{
			Name:          name,
			Command:       binaryPath,
			ShardAffinity: "/all", // Default affinity
			Description:   "Restored from disk",
			RegisteredAt:  time.Now(),
		}

		if err := tr.RegisterToolWithInfo(tool); err != nil {
			logging.ToolsWarn("RestoreFromDisk: failed to restore tool %s: %v", name, err)
			errs = append(errs, fmt.Errorf("tool %s: %w", name, err))
		} else {
			restoredCount++
		}
	}

	if len(errs) > 0 {
		logging.ToolsError("RestoreFromDisk: restored %d tools, %d failed from %s", restoredCount, len(errs), compiledDir)
		return fmt.Errorf("restored %d tools, %d failed: %v", restoredCount, len(errs), errs)
	}
	logging.Tools("RestoreFromDisk: successfully restored %d tools from %s", restoredCount, compiledDir)
	return nil
}

// StaticToolDef represents a tool definition loaded from available_tools.json.
// This mirrors init.ToolDefinition but avoids import cycles.
type StaticToolDef struct {
	Name          string
	Category      string
	Description   string
	Command       string
	ShardAffinity string
}

// RestoreFromStaticDefs loads tools from a slice of StaticToolDef into the registry.
// This is used to hydrate tools from available_tools.json at session boot.
// Continues restoring even if individual tools fail, collecting all errors.
func (tr *ToolRegistry) RestoreFromStaticDefs(defs []StaticToolDef) error {
	logging.ToolsDebug("RestoreFromStaticDefs: restoring %d static tool definitions", len(defs))
	var errs []error
	restoredCount := 0

	for _, def := range defs {
		// Normalize shard affinity to Mangle name constant format
		affinity := def.ShardAffinity
		if affinity == "" {
			affinity = "/all"
		} else if !strings.HasPrefix(affinity, "/") {
			// Convert "CoderShard" -> "/codershard"
			affinity = "/" + strings.ToLower(strings.TrimSuffix(affinity, "Shard"))
		}

		tool := &Tool{
			Name:          def.Name,
			Command:       def.Command,
			ShardAffinity: affinity,
			Description:   def.Description,
			Capabilities:  []string{def.Category}, // Category becomes capability
			RegisteredAt:  time.Now(),
		}

		if err := tr.RegisterToolWithInfo(tool); err != nil {
			logging.ToolsWarn("RestoreFromStaticDefs: failed to restore tool %s: %v", def.Name, err)
			errs = append(errs, fmt.Errorf("tool %s: %w", def.Name, err))
		} else {
			restoredCount++
		}
	}

	if len(errs) > 0 {
		logging.ToolsError("RestoreFromStaticDefs: restored %d tools, %d failed", restoredCount, len(errs))
		return fmt.Errorf("restored %d tools, %d failed: %v", restoredCount, len(errs), errs)
	}
	logging.Tools("RestoreFromStaticDefs: successfully restored %d tools from static definitions", restoredCount)
	return nil
}

// BuildToolCatalog creates a formatted tool catalog string for prompt injection.
// This enables Piggyback++ architecture where tools are requested via structured
// output (control_packet.tool_requests) instead of native LLM function calling.
//
// The catalog is organized by shard affinity and includes:
// - Tool name and description
// - Parameters with types and descriptions (from schema when available)
// - Usage examples
//
// Parameters:
//   - shardType: Filter tools by shard affinity ("/coder", "/tester", "/all", etc.)
//     Pass empty string to include all tools.
func (tr *ToolRegistry) BuildToolCatalog(shardType string) string {
	tools := tr.GetToolsForShard(shardType)
	if len(tools) == 0 {
		// Also check for "/all" if specific shard has no tools
		if shardType != "" && shardType != "/all" {
			tools = tr.GetToolsForShard("/all")
		}
	}

	if len(tools) == 0 {
		return ""
	}

	var catalog strings.Builder
	catalog.WriteString("\n## Available Tools\n\n")
	catalog.WriteString("Request tools via `tool_requests` in control_packet:\n")
	catalog.WriteString("```json\n")
	catalog.WriteString("\"tool_requests\": [{\n")
	catalog.WriteString("  \"id\": \"req_1\",\n")
	catalog.WriteString("  \"tool_name\": \"<tool_name>\",\n")
	catalog.WriteString("  \"tool_args\": { ... },\n")
	catalog.WriteString("  \"purpose\": \"why this tool is needed\"\n")
	catalog.WriteString("}]\n")
	catalog.WriteString("```\n\n")

	// Group tools by affinity for better organization
	byAffinity := make(map[string][]*Tool)
	for _, tool := range tools {
		affinity := tool.ShardAffinity
		if affinity == "" {
			affinity = "/all"
		}
		byAffinity[affinity] = append(byAffinity[affinity], tool)
	}

	// Output tools grouped by affinity
	for affinity, toolList := range byAffinity {
		catalog.WriteString(fmt.Sprintf("### %s Tools\n\n", strings.TrimPrefix(affinity, "/")))
		for _, tool := range toolList {
			catalog.WriteString(fmt.Sprintf("**%s**\n", tool.Name))
			if tool.Description != "" {
				catalog.WriteString(fmt.Sprintf("%s\n", tool.Description))
			}
			if len(tool.Capabilities) > 0 {
				catalog.WriteString(fmt.Sprintf("Capabilities: %s\n", strings.Join(tool.Capabilities, ", ")))
			}
			catalog.WriteString("\n")
		}
	}

	// Add tool generation encouragement
	catalog.WriteString("### Missing a Tool?\n\n")
	catalog.WriteString("If you need a capability not available above, request tool generation:\n")
	catalog.WriteString("1. Add a mangle_update: `missing_tool_for(\"<capability>\", \"<description>\")`\n")
	catalog.WriteString("2. The Ouroboros system will generate, compile, and register the tool\n")
	catalog.WriteString("3. The tool will be available in subsequent turns\n\n")

	logging.ToolsDebug("BuildToolCatalog: built catalog with %d tools for shard %s (length=%d)", len(tools), shardType, catalog.Len())
	return catalog.String()
}

// isCommandName checks if a string is a command name (not a path)
func isCommandName(s string) bool {
	return !filepath.IsAbs(s) && !strings.Contains(s, string(filepath.Separator))
}

func parseToolInput(input string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []string{trimmed}
	}
	return strings.Fields(trimmed)
}
