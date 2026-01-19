package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// =============================================================================
// AGENT CREATION AND MANAGEMENT
// =============================================================================

// writeAgentSpec writes an agent specification to disk
func (o *Orchestrator) writeAgentSpec(spec *AgentSpec) error {
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
			Preferences: make(map[string]interface{}),
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

	writeFile = func(path string, data []byte) error {
		return os.WriteFile(path, data, 0644)
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
func (o *Orchestrator) executeAgentCreation(_ context.Context, action AutopoiesisAction) error {
	spec, ok := action.Payload.(*AgentSpec)
	if !ok {
		return fmt.Errorf("invalid payload for agent creation")
	}

	// Write the agent spec to disk
	if err := o.writeAgentSpec(spec); err != nil {
		return fmt.Errorf("failed to write agent spec: %w", err)
	}

	// Assert agent creation to kernel
	o.assertAgentCreated(spec)

	return nil
}
