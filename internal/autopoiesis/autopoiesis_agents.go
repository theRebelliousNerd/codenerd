package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
)

// =============================================================================
// AGENT CREATION AND MANAGEMENT
// =============================================================================

// writeAgentSpec writes an agent specification to disk
func (o *Orchestrator) writeAgentSpec(ctx context.Context, spec *AgentSpec) error {
	// Ensure agents directory exists
	agentsDir := o.config.AgentsDir
	if err := ensureDir(agentsDir); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	// Create agent-specific directory
	agentDir := filepath.Join(agentsDir, spec.Name)
	if err := ensureDir(agentDir); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}

	// Write the agent spec as JSON
	specData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent spec: %w", err)
	}

	specPath := filepath.Join(agentDir, "agent.json")
	if err := writeFile(specPath, specData); err != nil {
		return fmt.Errorf("failed to write agent spec: %w", err)
	}

	// Write the system prompt to a separate file for easy editing
	promptPath := filepath.Join(agentDir, "system_prompt.md")
	promptContent := fmt.Sprintf("# System Prompt for %s\n\n%s\n", spec.Name, spec.SystemPrompt)
	if err := writeFile(promptPath, []byte(promptContent)); err != nil {
		return fmt.Errorf("failed to write system prompt: %w", err)
	}

	// Initialize memory storage if enabled
	if spec.Memory.Enabled {
		memoryDir := filepath.Join(agentDir, "memory")
		if err := ensureDir(memoryDir); err != nil {
			return fmt.Errorf("failed to create memory directory: %w", err)
		}

		// Create initial memory structure
		initialMemory := AgentMemory{
			AgentName:   spec.Name,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Learnings:   []Learning{},
			Preferences: make(map[string]any),
			Patterns:    []LearnedPattern{},
		}

		memoryData, err := json.MarshalIndent(initialMemory, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal initial memory: %w", err)
		}

		memoryPath := filepath.Join(memoryDir, "memory.json")
		if err := writeFile(memoryPath, memoryData); err != nil {
			return fmt.Errorf("failed to write initial memory: %w", err)
		}
	}

	// Emit the artifact the shard runtime actually discovers.
	//
	// OWNERSHIP DECISION (TODO P2 "Agent spec → runtime scheduler ownership"):
	// autopoiesis AUTHORS persistent agents; the shard manager OWNS running
	// them. Chat boot calls system.SyncAgentRegistryFromDisk →
	// DiscoverAgentsOnDisk → shardMgr.DefineProfile, so a shard profile exists
	// for every agent directory found on disk. Autopoiesis does not need — and
	// should not grow — a scheduler of its own.
	//
	// The gap that decision exposed: DiscoverAgentsOnDisk keys off
	// prompts.yaml, and this writer only ever produced agent.json /
	// system_prompt.md / triggers.json. Every agent autopoiesis created was
	// therefore invisible to the runtime that is supposed to schedule it —
	// written to disk, never spawnable. internal/system owns the canonical
	// writer (WriteAgentDefinition: init's atoms, admitted) but importing it
	// here would close an import cycle (system/factory.go imports
	// autopoiesis), so boot installs it through SetAgentDefinitionWriter.
	if err := o.writeAgentPromptsYAML(ctx, spec, agentDir); err != nil {
		return err
	}

	// Write trigger configuration
	if len(spec.Triggers) > 0 {
		triggersPath := filepath.Join(agentDir, "triggers.json")
		triggersData, err := json.MarshalIndent(spec.Triggers, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal triggers: %w", err)
		}
		if err := writeFile(triggersPath, triggersData); err != nil {
			return fmt.Errorf("failed to write triggers: %w", err)
		}
	}

	return nil
}

// ListAgents returns all registered agents
func (o *Orchestrator) ListAgents() ([]*AgentSpec, error) {
	agents := []*AgentSpec{}

	agentsDir := o.config.AgentsDir
	entries, err := readDir(agentsDir)
	if err != nil {
		// No agents directory yet
		return agents, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		specPath := filepath.Join(agentsDir, entry.Name(), "agent.json")
		data, err := readFile(specPath)
		if err != nil {
			continue
		}

		var spec AgentSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			continue
		}

		agents = append(agents, &spec)
	}

	return agents, nil
}

// GetAgent retrieves an agent by name
func (o *Orchestrator) GetAgent(name string) (*AgentSpec, error) {
	specPath := filepath.Join(o.config.AgentsDir, name, "agent.json")
	data, err := readFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	var spec AgentSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse agent spec: %w", err)
	}

	return &spec, nil
}

// DeleteAgent removes an agent
func (o *Orchestrator) DeleteAgent(name string) error {
	agentDir := filepath.Join(o.config.AgentsDir, name)
	return removeDir(agentDir)
}

// UpdateAgentMemory updates an agent's memory with new learnings
func (o *Orchestrator) UpdateAgentMemory(agentName string, learning Learning) error {
	memoryPath := filepath.Join(o.config.AgentsDir, agentName, "memory", "memory.json")

	data, err := readFile(memoryPath)
	if err != nil {
		return fmt.Errorf("failed to read agent memory: %w", err)
	}

	var memory AgentMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		return fmt.Errorf("failed to parse agent memory: %w", err)
	}

	// Add the new learning
	learning.LearnedAt = time.Now()
	learning.UseCount = 0
	memory.Learnings = append(memory.Learnings, learning)
	memory.UpdatedAt = time.Now()

	// Write back
	updatedData, err := json.MarshalIndent(memory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated memory: %w", err)
	}

	return writeFile(memoryPath, updatedData)
}

// Helper functions for file operations (to enable mocking in tests)
var (
	ensureDir = func(path string) error {
		return os.MkdirAll(path, 0755)
	}

	// Atomic, and this is the choke point every autopoiesis write goes
	// through: agent specs, prompts, triggers and memory.
	//
	// UpdateAgentMemory is the case that forces it. It is a read-modify-write
	// over memory.json, and the read treats a parse failure as a hard error --
	// correctly, since half a memory is not a memory. Together those mean a
	// single torn write does not lose one learning, it permanently wedges that
	// agent's memory: every later update fails on the file it cannot parse.
	// LoadAgent has the same shape over agent.json.
	//
	// Mode-preserving for uniformity rather than necessity: nothing under
	// .nerd/agents is executable today, and a rule that holds everywhere is
	// one fewer thing to get wrong when that changes.
	writeFile = func(path string, data []byte) error {
		return atomicfile.WriteFilePreservingMode(path, data, 0o644)
	}

	readFile = func(path string) ([]byte, error) {
		return os.ReadFile(path)
	}

	readDir = func(path string) ([]os.DirEntry, error) {
		return os.ReadDir(path)
	}

	removeDir = func(path string) error {
		return os.RemoveAll(path)
	}
)

// executeAgentCreation creates a new persistent agent
func (o *Orchestrator) executeAgentCreation(ctx context.Context, action AutopoiesisAction) error {
	spec, ok := action.Payload.(*AgentSpec)
	if !ok {
		return fmt.Errorf("invalid payload for agent creation")
	}

	// Write the agent spec to disk
	if err := o.writeAgentSpec(ctx, spec); err != nil {
		return fmt.Errorf("failed to write agent spec: %w", err)
	}

	// Assert agent creation to kernel
	o.assertAgentCreated(spec)

	return nil
}

// =============================================================================
// AGENT DEFINITION HANDOFF (autopoiesis authors, shards schedule)
// =============================================================================

// AgentDefinitionWriter writes the prompts.yaml that makes an agent
// discoverable by the shard runtime, returning the path it wrote.
// systemPrompt is the model-written prompt of the agent: the writer stores it
// as the agent's methodology atoms, each admitted by the generated-atom check.
//
// internal/system.WriteAgentDefinition is the writer. It cannot be referenced
// from here without an import cycle, so boot installs it via
// SetAgentDefinitionWriter.
type AgentDefinitionWriter func(ctx context.Context, workspace, name, role, topics, systemPrompt string) (string, error)

// SetAgentDefinitionWriter installs the canonical prompts.yaml writer.
func (o *Orchestrator) SetAgentDefinitionWriter(w AgentDefinitionWriter) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.agentDefWriter = w
}

// writeAgentPromptsYAML writes the agent's prompts.yaml through the installed
// writer. There is no second template here: the one this package carried
// wrote an unbounded identity with no content field and a priority key, so
// a created agent's atoms were neither capped nor admitted. Without a writer
// the creation fails and says why.
func (o *Orchestrator) writeAgentPromptsYAML(ctx context.Context, spec *AgentSpec, agentDir string) error {
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	if _, err := os.Stat(promptsPath); err == nil {
		// Never clobber an operator-edited definition.
		return nil
	}

	role := spec.Type
	if strings.TrimSpace(role) == "" {
		role = "specialist"
	}
	topics := strings.TrimSpace(spec.Purpose)

	o.mu.RLock()
	writer := o.agentDefWriter
	o.mu.RUnlock()

	if writer == nil {
		return fmt.Errorf("no agent definition writer installed for %s: boot installs internal/system.WriteAgentDefinition", spec.Name)
	}
	if _, err := writer(ctx, o.config.WorkspaceRoot, spec.Name, role, topics, spec.SystemPrompt); err != nil {
		return fmt.Errorf("failed to write agent definition for %s: %w", spec.Name, err)
	}
	return nil
}
