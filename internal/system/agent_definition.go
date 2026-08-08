package system

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
// It exists because `nerd define-agent` wrote none of it. That command called
// ShardManager.DefineProfile — an in-memory map — then exited, so the profile
// died with the process and `nerd spawn <name>` in the next process found
// nothing. Only the chat wizard ever wrote prompts.yaml, and it carried its own
// copy of this template.
//
// Returns the path of the written prompts.yaml. Writing is skipped (without
// error) when a definition already exists, so re-running define-agent never
// clobbers a hand-edited prompts.yaml.
func WriteAgentDefinition(workspace, name, role, topics string) (string, error) {
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

	if err := os.WriteFile(promptsPath, []byte(RenderAgentPromptsYAML(name, role, topics)), 0o644); err != nil {
		return "", fmt.Errorf("write prompts.yaml: %w", err)
	}

	// Best-effort: the agent is usable from prompts.yaml alone (discovery walks
	// the directory), agents.json is the cache the TUI and specialist injection
	// read.
	_ = SyncAgentRegistryFromDisk(workspace)

	return promptsPath, nil
}

// RenderAgentPromptsYAML builds the starter atom corpus for a user-defined
// agent: an identity atom (mandatory), a methodology atom, and a domain atom.
//
// The three tiers of content (content, content_concise, content_min) are what
// lets the JIT budget degrade an agent's prompt instead of dropping it.
func RenderAgentPromptsYAML(agentName, role, topics string) string {
	return fmt.Sprintf(`# Prompt atoms for %[1]s
# These are loaded into the JIT prompt compiler when the agent is spawned.
# Edit this file to customize the agent's identity, methodology, and domain knowledge.

- id: "%[1]s/identity"
  category: "identity"
  subcategory: "%[1]s"
  priority: 100
  is_mandatory: true
  description: "Identity and mission for %[1]s"
  content_concise: |
    You are %[1]s, a specialist agent in the codeNERD ecosystem.
    Domain: %[2]s
    Topics: %[3]s
  content_min: |
    You are %[1]s (%[2]s). Operate under the codeNERD kernel.
  content: |
    You are %[1]s, a specialist agent in the codeNERD ecosystem.

    ## Domain
    %[2]s

    ## Research Topics
    %[3]s

    ## Core Responsibilities
    - Provide expert guidance in your domain
    - Follow best practices and established patterns
    - Maintain high code quality standards
    - Integrate seamlessly with the codeNERD architecture

    ## Execution Mode
    You operate under the control of the codeNERD kernel. You receive structured tasks
    with clear objectives, focus patterns, and success criteria. Execute precisely.

- id: "%[1]s/methodology"
  category: "methodology"
  subcategory: "%[1]s"
  priority: 80
  is_mandatory: false
  depends_on: ["%[1]s/identity"]
  description: "Methodology and quality bar for %[1]s"
  content_concise: |
    - Understand context before acting
    - Consider edge cases and failure modes
    - Write clear, maintainable code
    - Verify with tests when feasible
  content_min: |
    Be precise, verify assumptions, and preserve correctness.
  content: |
    ## Methodology

    ### Analysis Approach
    - Understand the full context before acting
    - Consider edge cases and failure modes
    - Think through implications of changes

    ### Implementation Standards
    - Follow language idioms and conventions
    - Write clear, maintainable code
    - Include comprehensive error handling
    - Document non-obvious decisions

    ### Quality Assurance
    - Verify assumptions before proceeding
    - Test critical paths
    - Consider performance implications
    - Ensure backward compatibility when applicable

- id: "%[1]s/domain"
  category: "domain"
  subcategory: "%[1]s"
  priority: 70
  is_mandatory: false
  depends_on: ["%[1]s/identity", "%[1]s/methodology"]
  description: "Domain knowledge, pitfalls, and references for %[1]s"
  content_concise: |
    Domain: %[2]s
    Topics: %[3]s
  content_min: |
    Apply domain best practices for: %[3]s
  content: |
    ## Domain-Specific Knowledge

    ### Key Concepts
    [Add specific concepts, patterns, or frameworks relevant to this domain]

    ### Common Pitfalls
    [Add known issues, gotchas, or anti-patterns to avoid]

    ### Best Practices
    [Add domain-specific best practices and guidelines]

    ### Resources
    Research Topics: %[3]s

    [Add additional references, documentation links, or learning resources]
`,
		agentName, // 1: stable id prefix
		role,      // 2: domain/role
		topics,    // 3: topics
	)
}
