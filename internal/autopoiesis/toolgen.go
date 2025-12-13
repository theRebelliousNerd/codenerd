// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains the ToolGenerator for dynamic tool creation.
package autopoiesis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
)

// =============================================================================
// TOOL GENERATOR
// =============================================================================
// Detects when existing tools are insufficient and generates new ones.
// This is core autopoiesis - the system modifying itself to gain new capabilities.

// ToolGenerator detects tool needs and generates new tools
type ToolGenerator struct {
	client        LLMClient
	toolsDir      string // Directory where tools are stored
	existingTools map[string]ToolSchema

	// JIT prompt compilation support
	promptAssembler *articulation.PromptAssembler
	jitEnabled      bool

	// Learnings injection - aggregated patterns from past tool generation
	learningsContext string
}

// NewToolGenerator creates a new tool generator
func NewToolGenerator(client LLMClient, toolsDir string) *ToolGenerator {
	logging.AutopoiesisDebug("Creating ToolGenerator: toolsDir=%s", toolsDir)
	return &ToolGenerator{
		client:        client,
		toolsDir:      toolsDir,
		existingTools: make(map[string]ToolSchema),
	}
}

// SetLearningsContext injects aggregated learnings from past tool generation.
// These learnings are added to generation prompts to improve future tools.
func (tg *ToolGenerator) SetLearningsContext(ctx string) {
	tg.learningsContext = ctx
	if ctx != "" {
		logging.Autopoiesis("ToolGenerator: Injected learnings context (%d bytes)", len(ctx))
	}
}

// SetPromptAssembler attaches a JIT-aware prompt assembler to the tool generator
func (tg *ToolGenerator) SetPromptAssembler(assembler *articulation.PromptAssembler) {
	tg.promptAssembler = assembler
	tg.jitEnabled = assembler != nil && assembler.JITReady()
	if tg.jitEnabled {
		logging.Autopoiesis("JIT prompt compilation enabled for ToolGenerator")
	}
}

// WriteTool writes the generated tool to disk
func (tg *ToolGenerator) WriteTool(tool *GeneratedTool) error {
	// Ensure directory exists
	dir := filepath.Dir(tool.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write main code
	if err := os.WriteFile(tool.FilePath, []byte(tool.Code), 0644); err != nil {
		return fmt.Errorf("failed to write tool code: %w", err)
	}

	// Write test code
	testPath := strings.TrimSuffix(tool.FilePath, ".go") + "_test.go"
	if err := os.WriteFile(testPath, []byte(tool.TestCode), 0644); err != nil {
		// Non-fatal
		tool.Errors = append(tool.Errors, fmt.Sprintf("failed to write test code: %v", err))
	}

	return nil
}

// RegisterTool adds the tool to the registry (in-memory for hot reload)
func (tg *ToolGenerator) RegisterTool(tool *GeneratedTool) error {
	tg.existingTools[tool.Name] = tool.Schema
	return nil
}

// listExistingTools returns names of existing tools
func (tg *ToolGenerator) listExistingTools() []string {
	tools := make([]string, 0, len(tg.existingTools))
	for name := range tg.existingTools {
		tools = append(tools, name)
	}
	return tools
}

// HasTool returns true if a tool with the given name is already registered.
func (tg *ToolGenerator) HasTool(name string) bool {
	_, ok := tg.existingTools[name]
	return ok
}

// LoadExistingTools loads tool schemas from the tools directory
func (tg *ToolGenerator) LoadExistingTools() error {
	pattern := filepath.Join(tg.toolsDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		// Extract tool name from filename
		base := filepath.Base(file)
		name := strings.TrimSuffix(base, ".go")

		// Read file and extract description (basic parsing)
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Look for description constant or comment
		desc := extractDescription(string(content))

		tg.existingTools[name] = ToolSchema{
			Name:        name,
			Description: desc,
		}
	}

	return nil
}
