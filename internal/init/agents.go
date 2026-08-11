// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// LOCAL TYPE STUBS (previously in deleted shard packages)
// =============================================================================
// Research functionality has been removed from /init.
// The JIT clean loop now handles research via prompt atoms and ConfigFactory.

// initKnowledgeAtom is a stub type for knowledge atoms.
type initKnowledgeAtom struct {
	Concept    string
	Content    string
	Title      string
	Confidence float64
	SourceURL  string
}

// initQualityMetrics holds research quality metrics.
type initQualityMetrics struct {
	Score  float64
	Rating string
}

// initResearchResult holds the result of research.
type initResearchResult struct {
	Atoms           []initKnowledgeAtom
	FallbackUsed    int
	FallbackReason  string
	AttemptsMade    int
	EffectiveTopics []string
}

const initFallbackNone = 0

// generateAgentPromptsYAML generates a prompts.yaml template for a Type B (persistent) agent.
// Creates .nerd/agents/{name}/prompts.yaml with identity, methodology, and domain knowledge atoms.
// generateAgentPromptsYAML generates a prompts.yaml template for a Type B (persistent) agent.
// Creates .nerd/agents/{name}/prompts.yaml with identity, methodology, and domain knowledge atoms.
// This is a convenience wrapper that uses a background context; prefer generateAgentPromptsYAMLWithContext
// when a caller context is available so LLM generation can be bounded and cancelled.
func (i *Initializer) generateAgentPromptsYAML(agent RecommendedAgent) error {
	return i.generateAgentPromptsYAMLWithContext(context.Background(), agent)
}

// generateAgentPromptsYAMLWithContext generates prompts.yaml using LLM-generated methodology
// and domain content when available, falling back to the static template on any failure.
// LLM generation is bounded by a timeout derived from ctx so a slow model cannot stall init,
// and generation is per-agent so one bad agent cannot poison the others.
func (i *Initializer) generateAgentPromptsYAMLWithContext(ctx context.Context, agent RecommendedAgent) error {
	agentDir := filepath.Join(i.config.Workspace, ".nerd", "agents", strings.ToLower(agent.Name))
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	topicsStr := strings.Join(agent.Topics, ", ")
	domainExpertise := formatDomainExpertise(agent.Topics)
	agentNameLower := strings.ToLower(agent.Name)

	// Start with static content; replace with LLM content on success.
	methodologyContent := staticMethodologyContent()
	domainContent := staticDomainContent(topicsStr)
	usingLLM := false

	if i.config.LLMClient == nil {
		logging.BootWarn("LLM client is nil for %s, using static prompts template", agent.Name)
	} else {
		// Methodology: per-atom bounded generation; failure of one atom does not poison the other.
		if meth, err := i.generateMethodologyContent(ctx, agent); err != nil {
			logging.BootWarn("Falling back to static methodology for %s: %v", agent.Name, err)
		} else if strings.TrimSpace(meth) == "" {
			logging.BootWarn("Falling back to static methodology for %s: empty LLM response", agent.Name)
		} else {
			methodologyContent = strings.TrimSpace(meth)
			usingLLM = true
			logging.Boot("Generated LLM methodology content for %s", agent.Name)
		}
		if dom, err := i.generateDomainContent(ctx, agent); err != nil {
			logging.BootWarn("Falling back to static domain for %s: %v", agent.Name, err)
		} else if strings.TrimSpace(dom) == "" {
			logging.BootWarn("Falling back to static domain for %s: empty LLM response", agent.Name)
		} else {
			domainContent = strings.TrimSpace(dom)
			usingLLM = true
			logging.Boot("Generated LLM domain content for %s", agent.Name)
		}
		if !usingLLM {
			logging.BootWarn("LLM generation produced no usable content for %s, using static template", agent.Name)
		}
	}

	yamlStr := buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, methodologyContent, domainContent)
	if err := validatePromptsYAML([]byte(yamlStr), agentNameLower); err != nil {
		logging.BootWarn("Generated YAML failed validation for %s: %v, falling back to static template", agent.Name, err)
		yamlStr = buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, staticMethodologyContent(), staticDomainContent(topicsStr))
		if err2 := validatePromptsYAML([]byte(yamlStr), agentNameLower); err2 != nil {
			logging.BootWarn("Static template validation failed for %s: %v", agent.Name, err2)
		}
		usingLLM = false
	} else if usingLLM {
		logging.Boot("Generated LLM-driven prompts.yaml for %s at %s", agent.Name, promptsPath)
	}

	if err := os.WriteFile(promptsPath, []byte(yamlStr), 0644); err != nil {
		return fmt.Errorf("failed to write prompts.yaml: %w", err)
	}
	if !usingLLM {
		logging.Boot("Generated prompts.yaml for %s at %s", agent.Name, promptsPath)
	}
	return nil
}

// staticMethodologyContent returns the static fallback methodology markdown.
func staticMethodologyContent() string {
	return "## Methodology\n\n### Analysis Approach\n- Understand the full context before acting\n- Consider edge cases and failure modes\n- Think through implications of changes\n\n### Implementation Standards\n- Follow language idioms and conventions\n- Write clear, maintainable code\n- Include comprehensive error handling\n- Document non-obvious decisions\n\n### Quality Assurance\n- Verify assumptions before proceeding\n- Test critical paths\n- Consider performance implications\n- Ensure backward compatibility when applicable"
}

// staticDomainContent returns the static fallback domain markdown with the agent's topics.
func staticDomainContent(topicsStr string) string {
	return fmt.Sprintf("## Domain-Specific Knowledge\n\n### Key Concepts\n[Add specific concepts, patterns, or frameworks relevant to this domain]\n\n### Common Pitfalls\n[Add known issues, gotchas, or anti-patterns to avoid]\n\n### Best Practices\n[Add domain-specific best practices and guidelines]\n\n### Resources\nResearch Topics: %s\n\n[Add additional references, documentation links, or learning resources]", topicsStr)
}

// indentForYAMLBlock indents every non-empty line of content with indent so the
// text remains inside the YAML literal block scalar. Empty lines are preserved
// as empty lines which are valid inside a literal block.
func indentForYAMLBlock(content, indent string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	for j, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[j] = ""
		} else {
			lines[j] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// validatePromptsYAML parses the assembled YAML and confirms it contains exactly
// the expected atom ids. Any parse failure or missing id is treated as corruption.
func validatePromptsYAML(data []byte, agentNameLower string) error {
	var atoms []struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &atoms); err != nil {
		return fmt.Errorf("yaml parse failed: %w", err)
	}
	expected := []string{
		agentNameLower + "/identity",
		agentNameLower + "/methodology",
		agentNameLower + "/domain",
	}
	if len(atoms) != len(expected) {
		return fmt.Errorf("expected %d atoms, got %d", len(expected), len(atoms))
	}
	found := make(map[string]bool, len(atoms))
	for _, a := range atoms {
		found[a.ID] = true
	}
	for _, exp := range expected {
		if !found[exp] {
			return fmt.Errorf("missing expected atom %q", exp)
		}
	}
	return nil
}

// buildPromptsYAML assembles the full prompts.yaml document from its pieces.
// methodologyContent and domainContent are raw markdown; they are indented inside
// the YAML literal block scalars before interpolation. The remaining atom fields
// (ids, categories, priorities, is_mandatory, content_concise/min) are kept
// exactly as in the static template.
func buildPromptsYAML(agentNameLower, displayName, description, domainExpertise, topicsStr, methodologyContent, domainContent string) string {
	methIndented := indentForYAMLBlock(methodologyContent, "    ")
	domainIndented := indentForYAMLBlock(domainContent, "    ")
	return fmt.Sprintf(`# Prompt atoms for %[2]s
# These are loaded into the JIT prompt compiler when the agent is spawned.
# Edit this file to customize the agent's identity, methodology, and domain knowledge.

- id: "%[1]s/identity"
  category: "identity"
  subcategory: "%[1]s"
  priority: 100
  is_mandatory: true
  description: "Identity and mission for %[2]s"
  content_concise: |
    You are %[2]s, a specialist agent in the codeNERD ecosystem.
    Role: %[3]s
    Topics: %[5]s
  content_min: |
    You are %[2]s (%[3]s). Operate under the codeNERD kernel.
  content: |
    You are %[2]s, a specialist agent in the codeNERD ecosystem.

    ## Role
    %[3]s

    ## Domain Expertise
%[4]s

    ## Research Topics
    %[5]s

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
  description: "Methodology and quality bar for %[2]s"
  content_concise: |
    - Understand context before acting
    - Consider edge cases and failure modes
    - Write clear, maintainable code
    - Verify with tests when feasible
  content_min: |
    Be precise, verify assumptions, and preserve correctness.
  content: |
%[6]s

- id: "%[1]s/domain"
  category: "domain"
  subcategory: "%[1]s"
  priority: 70
  is_mandatory: false
  depends_on: ["%[1]s/identity", "%[1]s/methodology"]
  description: "Domain knowledge, pitfalls, and references for %[2]s"
  content_concise: |
    Domain focus: %[3]s
    Topics: %[5]s
  content_min: |
    Apply domain best practices for: %[5]s
  content: |
%[7]s
`,
		agentNameLower,
		displayName,
		description,
		domainExpertise,
		topicsStr,
		methIndented,
		domainIndented,
	)
}

// generateMethodologyContent asks the LLM for agent-specific methodology markdown.
// The prompt is deliberately specific to this agent's domain and forbids generic advice.
func (i *Initializer) generateMethodologyContent(ctx context.Context, agent RecommendedAgent) (string, error) {
	if i.config.LLMClient == nil {
		return "", fmt.Errorf("nil LLM client")
	}
	topicsStr := strings.Join(agent.Topics, ", ")
	prompt := fmt.Sprintf("You are generating the methodology prompt atom for the specialist agent %q whose role is %q and whose research topics are %q.\n\nWrite the markdown content for the methodology atom. Explain how THIS specialist approaches problems in its domain: its analysis approach, implementation standards, and quality assurance practices, tailored specifically to %s.\n\nRequirements:\n- Be specific to this agent's domain; do NOT give generic software-engineering advice.\n- The answer must be specific enough that it would be incorrect for a different specialist (for example, a Go concurrency expert vs a Cobra CLI expert vs a Mangle/Datalog logic expert).\n- Do NOT include YAML front matter or atom headers; only output the markdown body that will be placed inside the YAML 'content: |' block.\n- Keep it concise but thorough, using markdown headings and bullet points.\n- Generic software-engineering advice is not acceptable.", agent.Name, agent.Description, topicsStr, topicsStr)
	genCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := i.config.LLMClient.Complete(genCtx, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("empty LLM response")
	}
	return strings.TrimSpace(res), nil
}

// generateDomainContent asks the LLM for agent-specific domain markdown.
// The prompt demands concrete concepts, real pitfalls and practices for this specialist.
func (i *Initializer) generateDomainContent(ctx context.Context, agent RecommendedAgent) (string, error) {
	if i.config.LLMClient == nil {
		return "", fmt.Errorf("nil LLM client")
	}
	topicsStr := strings.Join(agent.Topics, ", ")
	prompt := fmt.Sprintf("You are generating the domain prompt atom for the specialist agent %q whose role is %q and whose research topics are %q.\n\nWrite the markdown content for the domain atom. Describe the concrete concepts, real pitfalls, and best practices that matter for those specific topics: %s.\n\nRequirements:\n- Cover Key Concepts (specific patterns, frameworks, or language features for this domain), Common Pitfalls (real gotchas and anti-patterns for these topics), Best Practices (domain-specific guidelines), and Resources.\n- Be specific to this agent's domain; do NOT give generic software-engineering advice.\n- The answer must be specific enough that it would be incorrect for a different specialist.\n- Do NOT include YAML front matter or atom headers; only output the markdown body for the YAML 'content: |' block.\n- Use markdown headings and bullet points.\n- Generic software-engineering advice is not acceptable.", agent.Name, agent.Description, topicsStr, topicsStr)
	genCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := i.config.LLMClient.Complete(genCtx, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("empty LLM response")
	}
	return strings.TrimSpace(res), nil
}

// formatDomainExpertise formats the topics as a bulleted list for the identity atom.
func formatDomainExpertise(topics []string) string {
	if len(topics) == 0 {
		return "    - General expertise"
	}

	var lines []string
	for _, topic := range topics {
		lines = append(lines, fmt.Sprintf("    - %s", topic))
	}
	return strings.Join(lines, "\n")
}

// AgentRegistry represents the persisted agent registry structure.
type AgentRegistry struct {
	Version   string         `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	Agents    []CreatedAgent `json:"agents"`
}

// KnowledgeBaseStats tracks statistics for KB upgrade operations.
type KnowledgeBaseStats struct {
	NewAtoms      int
	ExistingAtoms int
	SkippedAtoms  int
	TotalAtoms    int

	// Legacy names retained for registry compatibility. These are atom-count
	// population metrics, not semantic quality measurements.
	QualityScore  float64
	QualityRating string
}

// determineRequiredAgents analyzes the project and recommends Type 3 agents.
func (i *Initializer) determineRequiredAgents(profile ProjectProfile) []RecommendedAgent {
	agents := make([]RecommendedAgent, 0)

	// Language-specific agents
	switch strings.ToLower(profile.Language) {
	case "go", "golang":
		agents = append(agents, RecommendedAgent{
			Name:        "GoExpert",
			Type:        "persistent",
			Description: "Expert in Go idioms, concurrency patterns, and standard library",
			Topics:      []string{"go concurrency", "go error handling", "go interfaces", "go testing"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Go project detected - expert knowledge improves code quality",
		})

	case "python":
		agents = append(agents, RecommendedAgent{
			Name:        "PythonExpert",
			Type:        "persistent",
			Description: "Expert in Python best practices, type hints, and async patterns",
			Topics:      []string{"python typing", "python async", "python testing", "python packaging"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Python project detected - expert knowledge improves code quality",
		})

	case "typescript", "javascript":
		agents = append(agents, RecommendedAgent{
			Name:        "TSExpert",
			Type:        "persistent",
			Description: "Expert in TypeScript/JavaScript patterns and modern ES features",
			Topics:      []string{"typescript types", "javascript async", "react patterns", "node.js"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "TypeScript/JavaScript project detected",
		})

	case "rust":
		agents = append(agents, RecommendedAgent{
			Name:        "RustExpert",
			Type:        "persistent",
			Description: "Expert in Rust ownership, lifetimes, and async patterns",
			Topics:      []string{"rust ownership", "rust lifetimes", "rust async", "rust error handling"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Rust project detected - ownership expertise critical",
		})

	case "kotlin":
		// FIX(BUG-006): Add Kotlin/Android language support
		agents = append(agents, RecommendedAgent{
			Name:        "AndroidExpert",
			Type:        "persistent",
			Description: "Expert in Kotlin Android development, Jetpack Compose, and mobile patterns",
			Topics:      []string{"kotlin android", "jetpack compose", "android architecture", "coroutines", "room database", "hilt dependency injection"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Kotlin/Android project detected - mobile expertise critical",
		})
	}

	// Framework-specific agents
	switch strings.ToLower(profile.Framework) {
	case "gin", "echo", "fiber":
		agents = append(agents, RecommendedAgent{
			Name:        "WebAPIExpert",
			Type:        "persistent",
			Description: "Expert in REST API design and HTTP middleware patterns",
			Topics:      []string{"REST API design", "HTTP middleware", "API authentication", "OpenAPI"},
			Permissions: []string{"read_file", "network"},
			Priority:    80,
			Reason:      fmt.Sprintf("%s framework detected - API expertise beneficial", profile.Framework),
		})

	case "react", "nextjs", "vue":
		agents = append(agents, RecommendedAgent{
			Name:        "FrontendExpert",
			Type:        "persistent",
			Description: "Expert in modern frontend patterns and state management",
			Topics:      []string{"react hooks", "state management", "component patterns", "CSS-in-JS"},
			Permissions: []string{"read_file", "browser"},
			Priority:    80,
			Reason:      fmt.Sprintf("%s framework detected - frontend expertise beneficial", profile.Framework),
		})
	}

	// Dependency-specific agents
	depNames := make(map[string]bool)
	for _, dep := range profile.Dependencies {
		depNames[dep.Name] = true
	}

	// Browser automation experts
	if depNames["rod"] {
		agents = append(agents, RecommendedAgent{
			Name:        "RodExpert",
			Type:        "persistent",
			Description: "Expert in Rod browser automation, selectors, and CDP protocol",
			Topics:      []string{"rod browser automation", "CDP protocol", "web scraping", "headless chrome", "page selectors"},
			Permissions: []string{"read_file", "browser", "exec_cmd"},
			Priority:    95,
			Reason:      "Rod browser automation detected - specialized expertise beneficial",
		})
	}
	if depNames["chromedp"] || depNames["puppeteer"] || depNames["playwright"] {
		agents = append(agents, RecommendedAgent{
			Name:        "BrowserAutomationExpert",
			Type:        "persistent",
			Description: "Expert in browser automation patterns and CDP",
			Topics:      []string{"browser automation", "CDP protocol", "page navigation", "element interaction"},
			Permissions: []string{"read_file", "browser"},
			Priority:    90,
			Reason:      "Browser automation library detected",
		})
	}

	// Logic/Datalog experts
	if depNames["mangle"] {
		agents = append(agents, RecommendedAgent{
			Name:        "MangleExpert",
			Type:        "persistent",
			Description: "Expert in Google Mangle/Datalog, logic programming, and rule systems",
			Topics:      []string{"datalog", "mangle syntax", "logic programming", "horn clauses", "fact derivation", "negation as failure"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    95,
			Reason:      "Mangle/Datalog detected - logic programming expertise critical",
		})
	}

	// LLM integration experts
	if depNames["openai"] || depNames["anthropic"] {
		agents = append(agents, RecommendedAgent{
			Name:        "LLMIntegrationExpert",
			Type:        "persistent",
			Description: "Expert in LLM API integration, prompt engineering, and token optimization",
			Topics:      []string{"LLM APIs", "prompt engineering", "token optimization", "streaming responses", "function calling"},
			Permissions: []string{"read_file", "network"},
			Priority:    90,
			Reason:      "LLM API integration detected - expertise improves reliability",
		})
	}

	// CLI/TUI experts
	if depNames["bubbletea"] {
		agents = append(agents, RecommendedAgent{
			Name:        "BubbleTeaExpert",
			Type:        "persistent",
			Description: "Expert in Bubbletea TUI framework, Elm architecture, and terminal rendering",
			Topics:      []string{"bubbletea", "elm architecture", "terminal UI", "lipgloss styling", "bubbles components"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    85,
			Reason:      "Bubbletea TUI framework detected",
		})
	}
	if depNames["cobra"] {
		agents = append(agents, RecommendedAgent{
			Name:        "CobraExpert",
			Type:        "persistent",
			Description: "Expert in Cobra CLI framework, command structure, and flag handling",
			Topics:      []string{"cobra CLI", "command patterns", "flag handling", "CLI best practices"},
			Permissions: []string{"read_file"},
			Priority:    75,
			Reason:      "Cobra CLI framework detected",
		})
	}

	// Database experts
	if depNames["gorm"] || depNames["sqlx"] || depNames["sql"] || depNames["prisma"] || depNames["typeorm"] {
		agents = append(agents, RecommendedAgent{
			Name:        "DatabaseExpert",
			Type:        "persistent",
			Description: "Expert in database patterns, ORM usage, and query optimization",
			Topics:      []string{"database design", "ORM patterns", "SQL optimization", "migrations", "connection pooling"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    80,
			Reason:      "Database ORM/driver detected",
		})
	}

	// FIX(BUG-006): ArangoDB graph database expert
	if depNames["arangodb"] {
		agents = append(agents, RecommendedAgent{
			Name:        "ArangoExpert",
			Type:        "persistent",
			Description: "Expert in ArangoDB graph database, AQL queries, and document/graph modeling",
			Topics:      []string{"arangodb", "AQL queries", "graph traversal", "document modeling", "multi-model database", "graph database patterns"},
			Permissions: []string{"read_file", "code_graph", "network"},
			Priority:    85,
			Reason:      "ArangoDB detected - graph database expertise beneficial",
		})
	}

	// FIX(BUG-006): Google ADK agent orchestration expert
	if depNames["adk"] {
		agents = append(agents, RecommendedAgent{
			Name:        "ADKExpert",
			Type:        "persistent",
			Description: "Expert in Google ADK for LLM agent orchestration and tool use",
			Topics:      []string{"google adk", "agent orchestration", "llm tool use", "multi-agent systems", "agent workflows"},
			Permissions: []string{"read_file", "code_graph", "network"},
			Priority:    90,
			Reason:      "Google ADK detected - agent orchestration expertise beneficial",
		})
	}

	// FIX(BUG-006): A2A card/manifest agent expert
	if depNames["a2a"] {
		agents = append(agents, RecommendedAgent{
			Name:        "A2AExpert",
			Type:        "persistent",
			Description: "Expert in A2A card-driven and manifest-based agent patterns",
			Topics:      []string{"a2a protocol", "agent cards", "manifest agents", "agent interoperability", "card-driven workflows"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    85,
			Reason:      "A2A protocol detected - agent interop expertise beneficial",
		})
	}

	// FIX: Only add core agents if no language-specific or dependency-specific agents were found
	// This prevents the system from adding only generic agents when it should be detecting specialists
	if len(agents) == 0 {
		// Fallback: Only add generic agents if we couldn't detect anything specific
		agents = append(agents,
			RecommendedAgent{
				Name:        "SecurityAuditor",
				Type:        "persistent",
				Description: "Security vulnerability detection and best practices",
				Topics:      []string{"OWASP top 10", "secure coding", "vulnerability patterns", "code injection"},
				Permissions: []string{"read_file", "code_graph"},
				Priority:    90,
				Reason:      "Security analysis is critical for all projects",
			},
			RecommendedAgent{
				Name:        "TestArchitect",
				Type:        "persistent",
				Description: "Test strategy, coverage analysis, and TDD patterns",
				Topics:      []string{"unit testing", "integration testing", "test coverage", "mocking patterns"},
				Permissions: []string{"read_file", "exec_cmd"},
				Priority:    85,
				Reason:      "Test quality directly impacts code reliability",
			},
		)
	}

	// Assign tools to all agents based on their type and project language
	for idx := range agents {
		tools, prefs := GetToolsForAgentType(agents[idx].Name, profile.Language)
		agents[idx].Tools = tools
		agents[idx].ToolPreferences = prefs
	}

	return agents
}

// loadExistingAgentRegistry loads the agent registry from .nerd/agents.json if it exists.
// Returns nil map if the file doesn't exist (new installation).
func (i *Initializer) loadExistingAgentRegistry(nerdDir string) (map[string]CreatedAgent, error) {
	registryPath := filepath.Join(nerdDir, "agents.json")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Boot("No existing agent registry found at %s (new installation)", registryPath)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent registry: %w", err)
	}

	var registry AgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse agent registry: %w", err)
	}

	// Convert to map for easy lookup
	agentMap := make(map[string]CreatedAgent)
	for _, agent := range registry.Agents {
		agentMap[agent.Name] = agent
	}

	logging.Boot("Loaded existing agent registry with %d agents", len(agentMap))
	return agentMap, nil
}

// agentCreationResult holds the result of creating a single agent KB.
type agentCreationResult struct {
	Agent       CreatedAgent
	KBSize      int
	Stats       KnowledgeBaseStats
	KBPath      string
	UpgradeMode bool
	Error       error
}

// createType3Agents creates the knowledge bases and registers Type 3 agents.
// In upgrade mode (--force with existing KB), it appends new knowledge rather than overwriting.
// Uses parallel creation with a worker pool for improved performance.
func (i *Initializer) createType3Agents(ctx context.Context, nerdDir string, agents []RecommendedAgent, result *InitResult) ([]CreatedAgent, map[string]int) {
	created := make([]CreatedAgent, 0)
	kbSizes := make(map[string]int)

	shardsDir := filepath.Join(nerdDir, "shards")

	// Load existing agent registry for upgrade detection
	existingAgents, err := i.loadExistingAgentRegistry(nerdDir)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to load existing agent registry: %v", err))
		existingAgents = nil
	}

	// Use parallel creation for better performance (3 concurrent workers)
	const maxWorkers = 3
	if len(agents) > 1 {
		results := i.createAgentsParallel(ctx, shardsDir, agents, existingAgents, maxWorkers)

		for _, res := range results {
			if res.Error != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create KB for %s: %v", res.Agent.Name, res.Error))
				i.sendAgentProgress(res.Agent.Name, res.Agent.Type, "failed", 0)
				continue
			}

			kbSizes[res.Agent.Name] = res.KBSize
			created = append(created, res.Agent)

			if !res.UpgradeMode {
				result.FilesCreated = append(result.FilesCreated, res.KBPath)
			}

			// Log result
			if res.UpgradeMode {
				fmt.Printf("     + %s upgraded (added %d new, skipped %d existing, total %d atoms)\n",
					res.Agent.Name, res.Stats.NewAtoms, res.Stats.SkippedAtoms, res.Stats.TotalAtoms)
			} else {
				fmt.Printf("     + %s ready (%d knowledge atoms)\n", res.Agent.Name, res.KBSize)
			}
		}

		return created, kbSizes
	}

	// Sequential fallback for single agent
	for idx, agent := range agents {
		progress := 0.55 + (float64(idx)/float64(len(agents)))*0.25
		i.sendProgress("kb_creation", fmt.Sprintf("Creating %s...", agent.Name), progress)
		i.sendAgentProgress(agent.Name, agent.Type, "creating", 0)

		kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

		upgradeMode := false
		var existingAtomCount int
		if _, statErr := os.Stat(kbPath); statErr == nil {
			upgradeMode = true
			existingAtomCount = i.getExistingAtomCount(kbPath)
			logging.Boot("Upgrading %s (existing KB: %d atoms)", agent.Name, existingAtomCount)
			fmt.Printf("   Upgrading %s knowledge base (existing: %d atoms)...\n", agent.Name, existingAtomCount)
		} else {
			logging.Boot("Creating fresh %s knowledge base", agent.Name)
			fmt.Printf("   Creating %s knowledge base...\n", agent.Name)
		}

		stats, err := i.createAgentKnowledgeBase(ctx, kbPath, agent, upgradeMode)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create KB for %s: %v", agent.Name, err))
			i.sendAgentProgress(agent.Name, agent.Type, "failed", 0)
			continue
		}

		// Generate prompts.yaml for the agent (only for new agents, not upgrades)
		if !upgradeMode {
			if promptErr := i.generateAgentPromptsYAMLWithContext(ctx, agent); promptErr != nil {
				logging.Boot("Warning: failed to generate prompts.yaml for %s: %v", agent.Name, promptErr)
			}
		}

		totalKBSize := stats.TotalAtoms
		kbSizes[agent.Name] = totalKBSize
		i.sendAgentProgress(agent.Name, agent.Type, "ready", totalKBSize)

		creationTime := time.Now()
		if existingAgent, exists := existingAgents[agent.Name]; exists && upgradeMode {
			creationTime = existingAgent.CreatedAt
		}

		createdAgent := CreatedAgent{
			Name:            agent.Name,
			Type:            agent.Type,
			KnowledgePath:   kbPath,
			KBSize:          totalKBSize,
			CreatedAt:       creationTime,
			Status:          "ready",
			Tools:           agent.Tools,
			ToolPreferences: agent.ToolPreferences,
			QualityScore:    stats.QualityScore,
			QualityRating:   stats.QualityRating,
		}
		created = append(created, createdAgent)

		if !upgradeMode {
			result.FilesCreated = append(result.FilesCreated, kbPath)
		}

		if upgradeMode {
			fmt.Printf("     + %s upgraded (added %d new, skipped %d existing, total %d atoms)\n",
				agent.Name, stats.NewAtoms, stats.SkippedAtoms, stats.TotalAtoms)
		} else if stats.QualityScore > 0 {
			fmt.Printf("     + %s ready (%d atoms, KB population: %.0f%% - %s)\n",
				agent.Name, totalKBSize, stats.QualityScore, stats.QualityRating)
		} else {
			fmt.Printf("     + %s ready (%d knowledge atoms)\n", agent.Name, totalKBSize)
		}
	}

	return created, kbSizes
}

// createAgentsParallel creates agent knowledge bases concurrently using a worker pool.
func (i *Initializer) createAgentsParallel(ctx context.Context, shardsDir string, agents []RecommendedAgent, existingAgents map[string]CreatedAgent, maxWorkers int) []agentCreationResult {
	results := make([]agentCreationResult, len(agents))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	fmt.Printf("   Creating %d agent KBs in parallel (max %d workers)...\n", len(agents), maxWorkers)

	for idx, agent := range agents {
		idx, agent := idx, agent
		wg.Go(func() {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				results[idx] = agentCreationResult{
					Agent: CreatedAgent{Name: agent.Name, Type: agent.Type},
					Error: ctx.Err(),
				}
				return
			default:
			}

			kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

			// Check upgrade mode
			upgradeMode := false
			if _, statErr := os.Stat(kbPath); statErr == nil {
				upgradeMode = true
				existingCount := i.getExistingAtomCount(kbPath)
				logging.Boot("Parallel: Upgrading %s (existing KB: %d atoms)", agent.Name, existingCount)
			} else {
				logging.Boot("Parallel: Creating fresh %s knowledge base", agent.Name)
			}

			// Create/upgrade knowledge base
			stats, err := i.createAgentKnowledgeBase(ctx, kbPath, agent, upgradeMode)
			if err != nil {
				results[idx] = agentCreationResult{
					Agent: CreatedAgent{Name: agent.Name, Type: agent.Type},
					Error: err,
				}
				return
			}

			// Generate prompts.yaml for the agent (only for new agents, not upgrades)
			if !upgradeMode {
				if promptErr := i.generateAgentPromptsYAMLWithContext(ctx, agent); promptErr != nil {
					logging.Boot("Warning: failed to generate prompts.yaml for %s: %v", agent.Name, promptErr)
				}
			}

			// Determine creation time
			creationTime := time.Now()
			if existingAgent, exists := existingAgents[agent.Name]; exists && upgradeMode {
				creationTime = existingAgent.CreatedAt
			}

			results[idx] = agentCreationResult{
				Agent: CreatedAgent{
					Name:            agent.Name,
					Type:            agent.Type,
					KnowledgePath:   kbPath,
					KBSize:          stats.TotalAtoms,
					CreatedAt:       creationTime,
					Status:          "ready",
					Tools:           agent.Tools,
					ToolPreferences: agent.ToolPreferences,
					QualityScore:    stats.QualityScore,
					QualityRating:   stats.QualityRating,
				},
				KBSize:      stats.TotalAtoms,
				Stats:       stats,
				KBPath:      kbPath,
				UpgradeMode: upgradeMode,
				Error:       nil,
			}

			i.sendAgentProgress(agent.Name, agent.Type, "ready", stats.TotalAtoms)
		})
	}

	wg.Wait()
	return results
}
