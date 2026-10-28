package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	nerdinit "codenerd/internal/init"
)

// agentNamePattern is the accepted shape for a user-defined agent name. It has
// to survive being used as a directory name, a SQLite filename stem, and an
// intent verb segment ("/consult/<name>"), so it stays conservative.
var agentNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateAgentName reports whether name is usable as a user-defined agent id.
func ValidateAgentName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("agent name is empty")
	}
	if !agentNamePattern.MatchString(name) {
		return fmt.Errorf("invalid agent name %q: must be alphanumeric (dash/underscore allowed)", name)
	}
	return nil
}

// AgentPromptsPath returns the canonical prompts.yaml location for an agent.
func AgentPromptsPath(workspace, name string) string {
	return filepath.Join(workspace, ".nerd", "agents", name, "prompts.yaml")
}

// WriteAgentDefinition materializes a user-defined agent on disk and registers
// it in .nerd/agents.json.
//
// This is the single writer for the layout the rest of the system reads:
//
//	.nerd/agents/<name>/prompts.yaml   parsed by prompt sync into
//	                                   .nerd/shards/<lower(name)>_knowledge.db
//	                                   (internal/prompt/sync/synchronizer.go),
//	                                   which boot registers with the JIT
//	                                   compiler (factory.go initExecutionLayer)
//	.nerd/agents.json                  discovery + specialist injection
//	                                   (internal/prompt/compiler_specialists.go)
//
// The atoms are init's (nerdinit.AgentDefinitionYAML): the template identity,
// and the model-written methodology (an autopoiesis agent's system prompt)
// and knowledge (what research produced for the agent's domain) split one
// atom per heading and admitted by the generated-atom check. An empty body
// contributes no atom: the template's generic methodology and its "[Add
// specific concepts]" placeholders rode into every prompt of the agent.
//
// Returns the path of the written prompts.yaml. Writing is skipped (without
// error) when a definition already exists, so re-running define-agent never
// clobbers a hand-edited prompts.yaml; callers that have fresh knowledge to
// write check AgentPromptsPath first and tell the user when it was kept.
func WriteAgentDefinition(ctx context.Context, workspace, name, role, topics, methodology, knowledge string) (string, error) {
	if err := ValidateAgentName(name); err != nil {
		return "", err
	}
	if strings.TrimSpace(workspace) == "" {
		return "", fmt.Errorf("workspace is empty")
	}

	name = strings.TrimSpace(name)
	agentDir := filepath.Join(workspace, ".nerd", "agents", name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return "", fmt.Errorf("create agent directory: %w", err)
	}

	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	if _, err := os.Stat(promptsPath); err == nil {
		// Already defined. Still re-sync the registry so a directory created
		// out of band shows up in agents.json.
		_ = SyncAgentRegistryFromDisk(workspace)
		return promptsPath, nil
	}

	yml, err := nerdinit.AgentDefinitionYAML(ctx, workspace, name, role, topics, methodology, knowledge)
	if err != nil {
		return "", fmt.Errorf("render prompts.yaml: %w", err)
	}
	if err := os.WriteFile(promptsPath, []byte(yml), 0o644); err != nil {
		return "", fmt.Errorf("write prompts.yaml: %w", err)
	}

	// Best-effort: the agent is usable from prompts.yaml alone (discovery walks
	// the directory), agents.json is the cache the TUI and specialist injection
	// read.
	_ = SyncAgentRegistryFromDisk(workspace)

	return promptsPath, nil
}
