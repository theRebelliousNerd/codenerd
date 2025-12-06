package core

import (
	"context"
	"fmt"
	"os"
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
	Command       string    `json:"command"`         // Path to binary or command to execute
	ShardAffinity string    `json:"shard_affinity"`  // /coder, /tester, /reviewer, /researcher, /generalist, /all
	Description   string    `json:"description"`
	Capabilities  []string  `json:"capabilities"`
	Hash          string    `json:"hash"`            // Binary hash for change detection
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

	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if command == "" {
		return fmt.Errorf("tool command cannot be empty")
	}

	// Verify command exists if it's a file path
	if !isCommandName(command) {
		absPath := command
		if !filepath.IsAbs(command) {
			absPath = filepath.Join(tr.workDir, command)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
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
	return tr.injectToolFacts(tool)
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

// ExecuteRegisteredTool executes a registered tool with the given arguments
func (tr *ToolRegistry) ExecuteRegisteredTool(ctx context.Context, toolName string, args []string) (string, error) {
	tool, exists := tr.GetTool(toolName)
	if !exists {
		return "", fmt.Errorf("tool not registered: %s", toolName)
	}

	// Update execution count
	tr.mu.Lock()
	tool.ExecuteCount++
	tr.mu.Unlock()

	// Execute via the tool executor interface if available
	// This is handled by VirtualStore.handleExecTool which uses the ToolExecutor
	return "", fmt.Errorf("direct execution not implemented - use VirtualStore.handleExecTool")
}

// UnregisterTool removes a tool from the registry and retracts its facts
func (tr *ToolRegistry) UnregisterTool(name string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if _, exists := tr.tools[name]; !exists {
		return fmt.Errorf("tool not registered: %s", name)
	}

	delete(tr.tools, name)

	// Retract facts from kernel
	if tr.kernel != nil {
		_ = tr.kernel.Retract("registered_tool")
		_ = tr.kernel.Retract("tool_registered")
		_ = tr.kernel.Retract("tool_hash")

		// Re-inject all remaining tools
		for _, tool := range tr.tools {
			_ = tr.injectToolFacts(tool)
		}
	}

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

// SyncFromOuroboros synchronizes tools from an Ouroboros registry
func (tr *ToolRegistry) SyncFromOuroboros(toolExecutor ToolExecutor) error {
	if toolExecutor == nil {
		return nil
	}

	tools := toolExecutor.ListTools()
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
			return fmt.Errorf("failed to sync tool %s: %w", tool.Name, err)
		}
	}

	return nil
}

// RestoreFromDisk restores the registry from a directory of compiled tools
func (tr *ToolRegistry) RestoreFromDisk(compiledDir string) error {
	entries, err := os.ReadDir(compiledDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist yet, that's okay
		}
		return fmt.Errorf("failed to read compiled tools directory: %w", err)
	}

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
			return fmt.Errorf("failed to restore tool %s: %w", name, err)
		}
	}

	return nil
}

// isCommandName checks if a string is a command name (not a path)
func isCommandName(s string) bool {
	return !filepath.IsAbs(s) && !strings.Contains(s, string(filepath.Separator))
}
