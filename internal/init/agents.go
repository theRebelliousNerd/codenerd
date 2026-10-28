// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"

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

// generateAgentPromptsYAMLWithContext generates prompts.yaml with methodology
// and domain content written by the planner client (the main client when no
// planner is configured), falling back to the static template on any failure.
// The atoms are the specialist's standing instructions for every later task,
// which is why they are planner work. Each request is bounded by withJITPrompt's
// per-call deadline, and generation is per-agent so one bad agent cannot poison
// the others.
func (i *Initializer) generateAgentPromptsYAMLWithContext(ctx context.Context, agent RecommendedAgent) error {
	agentDir := filepath.Join(i.config.Workspace, ".nerd", "agents", strings.ToLower(agent.Name))
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}
	promptsPath := filepath.Join(agentDir, "prompts.yaml")
	// A missing research DB does not authorize replacing curated prompt atoms.
	if existing, err := os.ReadFile(promptsPath); err == nil {
		if err := validatePromptsYAML(existing, strings.ToLower(agent.Name), i.atomMaxChars()); err != nil {
			// A prompts.yaml written before the atom cap holds oversize
			// atoms the embedding model refuses and selection can only
			// take whole. Re-atomizing preserves every word and field —
			// it only cuts content at heading boundaries into capped
			// sub-atoms — so it does not replace curation, it makes the
			// curated text usable. Anything else wrong stays an error.
			fixed, changed, ferr := resplitPromptsYAML(existing, i.atomMaxChars())
			if ferr == nil && changed && validatePromptsYAML(fixed, strings.ToLower(agent.Name), i.atomMaxChars()) == nil {
				if werr := os.WriteFile(promptsPath, fixed, 0644); werr != nil {
					return fmt.Errorf("existing agent prompts: %w (re-atomized form could not be written: %v)", err, werr)
				}
				logging.Boot("Re-atomized oversize prompt atoms in %s to the %d-character cap", promptsPath, i.atomMaxChars())
				return nil
			}
			return fmt.Errorf("existing agent prompts: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// A winning imported subagent or mode carries its own prompt. That text
	// is the methodology and the domain; a generated template would replace
	// the definition the corpus already stated.
	if imported := ecosystemImportedPrompt(i.config.Workspace, agent.Name); imported != "" {
		topicsStr := strings.Join(agent.Topics, ", ")
		domainExpertise := formatDomainExpertise(agent.Topics)
		agentNameLower := strings.ToLower(agent.Name)
		yamlStr := buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, imported, imported, i.atomMaxChars())
		if err := validatePromptsYAML([]byte(yamlStr), agentNameLower, i.atomMaxChars()); err != nil {
			logging.BootWarn("Imported prompt for %s failed validation: %v", agent.Name, err)
		} else {
			if err := os.WriteFile(promptsPath, []byte(yamlStr), 0644); err != nil {
				return fmt.Errorf("failed to write prompts.yaml: %w", err)
			}
			logging.Boot("Generated prompts.yaml for %s from the imported definition", agent.Name)
			return nil
		}
	}
	topicsStr := strings.Join(agent.Topics, ", ")
	domainExpertise := formatDomainExpertise(agent.Topics)
	agentNameLower := strings.ToLower(agent.Name)

	// Start with static content; replace with LLM content on success.
	methodologyContent := staticMethodologyContent()
	domainContent := staticDomainContent(topicsStr)
	usingLLM := false

	if i.route(slotPlanner).client == nil {
		logging.BootWarn("No LLM client for %s, using static prompts template", agent.Name)
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

	yamlStr := buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, methodologyContent, domainContent, i.atomMaxChars())
	if err := validatePromptsYAML([]byte(yamlStr), agentNameLower, i.atomMaxChars()); err != nil {
		logging.BootWarn("Generated YAML failed validation for %s: %v, falling back to static template", agent.Name, err)
		yamlStr = buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, staticMethodologyContent(), staticDomainContent(topicsStr), i.atomMaxChars())
		if err2 := validatePromptsYAML([]byte(yamlStr), agentNameLower, i.atomMaxChars()); err2 != nil {
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

// validatePromptsYAML parses the assembled YAML and confirms it holds a
// usable atom set: the base identity/methodology/domain atoms exist (each
// possibly followed by capped sub-atoms under the same prefix), ids are
// unique, contents are non-empty, and no atom exceeds
// atomCap (jit.atom_max_chars). Any failure is treated as corruption.
func validatePromptsYAML(data []byte, agentNameLower string, atomCap int) error {
	var atoms []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	if err := yaml.Unmarshal(data, &atoms); err != nil {
		return fmt.Errorf("yaml parse failed: %w", err)
	}
	if len(atoms) == 0 {
		return fmt.Errorf("no prompt atoms found")
	}
	found := make(map[string]bool, len(atoms))
	for _, a := range atoms {
		if strings.TrimSpace(a.ID) == "" {
			return fmt.Errorf("prompt atom with an empty id")
		}
		if found[a.ID] {
			return fmt.Errorf("duplicate prompt atom id %q", a.ID)
		}
		found[a.ID] = true
		if strings.TrimSpace(a.Content) == "" {
			return fmt.Errorf("prompt atom %q has empty content", a.ID)
		}
		if n := utf8.RuneCountInString(a.Content); n > atomCap {
			return fmt.Errorf("prompt atom %q content is %d characters, over the %d-character atom cap (jit.atom_max_chars): split it into smaller atoms under the same prefix", a.ID, n, atomCap)
		}
	}
	for _, base := range []string{"/identity", "/methodology", "/domain"} {
		if !found[agentNameLower+base] {
			return fmt.Errorf("missing expected atom %q", agentNameLower+base)
		}
	}
	return nil
}

// buildPromptsYAML assembles the full prompts.yaml document from its pieces.
// methodologyContent and domainContent are raw markdown; each body is
// atomized at its heading boundaries into capped atoms (atomizeContent), so
// no atom exceeds atomCap (jit.atom_max_chars) however long the generated
// markdown is. The first piece of each body keeps the base id
// (<agent>/methodology, <agent>/domain), so depends_on references and the
// atom counts callers expect are unchanged for short content.
func buildPromptsYAML(agentNameLower, displayName, description, domainExpertise, topicsStr, methodologyContent, domainContent string, atomCap int) string {
	// domainExpertise arrives pre-indented for the old block-scalar template
	// (formatDomainExpertise); the identity body is plain markdown here.
	expertiseLines := strings.Split(domainExpertise, "\n")
	for i, line := range expertiseLines {
		expertiseLines[i] = strings.TrimPrefix(line, "    ")
	}
	identityContent := fmt.Sprintf(`You are %s, a specialist agent in the codeNERD ecosystem.

## Role
%s

## Domain Expertise
%s

## Research Topics
%s

## Core Responsibilities
- Provide expert guidance in your domain
- Follow best practices and established patterns
- Maintain high code quality standards
- Integrate seamlessly with the codeNERD architecture

## Execution Mode
You operate under the control of the codeNERD kernel. You receive structured tasks
with clear objectives, focus patterns, and success criteria. Execute precisely.`,
		displayName, description, strings.Join(expertiseLines, "\n"), topicsStr)

	var atoms []agentPromptAtom
	atoms = append(atoms, atomizeContent(
		agentNameLower+"/identity", "identity", agentNameLower, true, nil,
		"Identity and mission for "+displayName,
		fmt.Sprintf("You are %s, a specialist agent in the codeNERD ecosystem.\nRole: %s\nTopics: %s", displayName, description, topicsStr),
		fmt.Sprintf("You are %s (%s). Operate under the codeNERD kernel.", displayName, description),
		identityContent, atomCap)...)
	// Identity sub-atoms stay mandatory (atomizeContent propagates the
	// flag): the identity was one mandatory atom before the cap existed,
	// and splitting it must not make pieces of who the agent is optional.
	atoms = append(atoms, atomizeContent(
		agentNameLower+"/methodology", "methodology", agentNameLower, false,
		[]string{agentNameLower + "/identity"},
		"Methodology and quality bar for "+displayName,
		"- Understand context before acting\n- Consider edge cases and failure modes\n- Write clear, maintainable code\n- Verify with tests when feasible",
		"Be precise, verify assumptions, and preserve correctness.",
		methodologyContent, atomCap)...)
	atoms = append(atoms, atomizeContent(
		agentNameLower+"/domain", "domain", agentNameLower, false,
		[]string{agentNameLower + "/identity", agentNameLower + "/methodology"},
		"Domain knowledge, pitfalls, and references for "+displayName,
		fmt.Sprintf("Domain focus: %s\nTopics: %s", description, topicsStr),
		fmt.Sprintf("Apply domain best practices for: %s", topicsStr),
		domainContent, atomCap)...)

	var b strings.Builder
	fmt.Fprintf(&b, "# Prompt atoms for %s\n", displayName)
	b.WriteString("# These are loaded into the JIT prompt compiler when the agent is spawned.\n")
	b.WriteString("# Edit this file to customize the agent's identity, methodology, and domain knowledge.\n\n")
	for i, atom := range atoms {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderAgentPromptAtom(atom))
	}
	return b.String()
}

// generateAgentAtomContent runs one per-atom generation on the planner route
// through withJITPrompt, which bounds the request by the configured per-call
// deadline and accounts for every provider attempt. Timeout and cancellation
// are wrapped distinctly so callers fall back to the generic (static)
// template with a clear reason while preserving
// errors.Is(err, context.DeadlineExceeded) and errors.Is(err, context.Canceled).
func (i *Initializer) generateAgentAtomContent(ctx context.Context, agent RecommendedAgent, kind, phase, task string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%s generation for %q skipped, parent context done, falling back to generic template: %w", kind, agent.Name, err)
	}
	res, err := i.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: phase, task: task},
		func(callCtx context.Context, client perception.LLMClient, systemPrompt string) (string, error) {
			return client.CompleteWithSystem(callCtx, systemPrompt, task)
		})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return "", fmt.Errorf("%s generation for %q timed out, falling back to generic template: %w", kind, agent.Name, err)
		case errors.Is(err, context.Canceled):
			return "", fmt.Errorf("%s generation for %q canceled, falling back to generic template: %w", kind, agent.Name, err)
		}
		return "", err
	}
	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("empty LLM response for %s generation for %q, falling back to generic template", kind, agent.Name)
	}
	return strings.TrimSpace(res), nil
}

// generateMethodologyContent asks the LLM for agent-specific methodology
// markdown. The instruction is the init/researcher/agent_methodology atom; the
// request carries the agent and the section limit.
func (i *Initializer) generateMethodologyContent(ctx context.Context, agent RecommendedAgent) (string, error) {
	return i.generateAgentAtomContent(ctx, agent, "methodology", "agent_methodology", i.agentPromptRequest(agent))
}

// generateDomainContent asks the LLM for agent-specific domain markdown. The
// instruction is the init/researcher/agent_domain atom; the request carries
// the agent and the section limit.
func (i *Initializer) generateDomainContent(ctx context.Context, agent RecommendedAgent) (string, error) {
	return i.generateAgentAtomContent(ctx, agent, "domain", "agent_domain", i.agentPromptRequest(agent))
}

// agentPromptRequest is the data of a specialist prompt-atom request.
func (i *Initializer) agentPromptRequest(agent RecommendedAgent) string {
	return fmt.Sprintf("Specialist agent: %q\nRole: %q\nResearch topics: %q\nSection limit: %d characters",
		agent.Name, agent.Description, strings.Join(agent.Topics, ", "), i.atomMaxChars())
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
	// SkippedTopics is how many of the agent's topics an upgrade did not
	// research again because its KB already covers them.
	SkippedTopics int
	// Notes are research outcomes the init result reports: questions that
	// could not be answered with a source, stale knowledge not refreshed.
	Notes []string
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
	// PromptErr is a prompts.yaml failure. The knowledge base still stands,
	// so it is a warning, but it must reach the init result: an invalid
	// curated prompts.yaml is the user's to fix.
	PromptErr error
}

// maxAgentWorkers bounds concurrent agent knowledge-base builds, and with them
// concurrent Context7 fetches and specialist prompt requests.
const maxAgentWorkers = 3

// createType3Agents creates or upgrades the knowledge base, prompts.yaml and
// registry entry of every recommended agent. An existing knowledge base is
// upgraded: new knowledge is appended, covered topics are not researched again.
func (i *Initializer) createType3Agents(ctx context.Context, nerdDir string, agents []RecommendedAgent, result *InitResult) ([]CreatedAgent, map[string]int) {
	// The seed is read by the parallel KB workers. Clear it when they have
	// all returned, not when the deriving phase returns.
	defer clearEcosystemSeed(i.config.Workspace)
	created := make([]CreatedAgent, 0, len(agents))
	kbSizes := make(map[string]int, len(agents))

	existingAgents, err := i.loadExistingAgentRegistry(nerdDir)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to load existing agent registry: %v", err))
		existingAgents = nil
	}

	for _, res := range i.createAgentsParallel(ctx, filepath.Join(nerdDir, "shards"), agents, existingAgents, maxAgentWorkers) {
		result.Warnings = append(result.Warnings, res.Stats.Notes...)
		if res.PromptErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("prompts.yaml for %s: %v", res.Agent.Name, res.PromptErr))
		}
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
		if res.UpgradeMode {
			fmt.Fprintf(i.output(), "     + %s upgraded (added %d new, skipped %d existing, total %d atoms)\n",
				res.Agent.Name, res.Stats.NewAtoms, res.Stats.SkippedAtoms, res.Stats.TotalAtoms)
		} else {
			fmt.Fprintf(i.output(), "     + %s ready (%d knowledge atoms)\n", res.Agent.Name, res.KBSize)
		}
	}
	return created, kbSizes
}

// createAgentsParallel builds agent knowledge bases on at most maxWorkers
// goroutines; every goroutine has returned when it returns.
func (i *Initializer) createAgentsParallel(ctx context.Context, shardsDir string, agents []RecommendedAgent, existingAgents map[string]CreatedAgent, maxWorkers int) []agentCreationResult {
	results := make([]agentCreationResult, len(agents))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	fmt.Fprintf(i.output(), "   Creating %d agent KBs (max %d at a time)...\n", len(agents), maxWorkers)
	total := len(agents)
	var done int
	var doneMu sync.Mutex

	for idx, agent := range agents {
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[idx] = agentCreationResult{Agent: CreatedAgent{Name: agent.Name, Type: agent.Type}, Error: ctx.Err()}
				return
			}
			defer func() { <-sem }()
			results[idx] = i.createAgent(ctx, shardsDir, agent, existingAgents)

			doneMu.Lock()
			done++
			finished := done
			doneMu.Unlock()
			i.sendProgress("kb_creation", fmt.Sprintf("Agent knowledge bases: %d/%d (%s)", finished, total, agent.Name))
		})
	}

	wg.Wait()
	return results
}

// createAgent builds or upgrades one agent's knowledge base and makes sure it
// has a prompts.yaml. prompts.yaml generation is attempted on every run: it
// is a no-op when the file exists, and an earlier run that could not write
// one is retried rather than skipped because the knowledge base exists.
func (i *Initializer) createAgent(ctx context.Context, shardsDir string, agent RecommendedAgent, existingAgents map[string]CreatedAgent) agentCreationResult {
	if err := ctx.Err(); err != nil {
		return agentCreationResult{Agent: CreatedAgent{Name: agent.Name, Type: agent.Type}, Error: err}
	}
	i.sendAgentProgress(agent.Name, agent.Type, "creating", 0)
	kbPath := types.AgentKnowledgeDBPath(filepath.Dir(shardsDir), agent.Name)

	upgradeMode := false
	if _, statErr := os.Stat(kbPath); statErr == nil {
		upgradeMode = true
		logging.Boot("Upgrading %s (existing KB: %d atoms)", agent.Name, i.getExistingAtomCount(kbPath))
	} else {
		logging.Boot("Creating fresh %s knowledge base", agent.Name)
	}

	stats, err := i.createAgentKnowledgeBase(ctx, kbPath, agent, upgradeMode)
	if err != nil {
		return agentCreationResult{Agent: CreatedAgent{Name: agent.Name, Type: agent.Type}, Error: err}
	}
	promptErr := i.generateAgentPromptsYAMLWithContext(ctx, agent)

	creationTime := time.Now()
	if existingAgent, exists := existingAgents[agent.Name]; exists && upgradeMode {
		creationTime = existingAgent.CreatedAt
	}
	i.sendAgentProgress(agent.Name, agent.Type, "ready", stats.TotalAtoms)
	return agentCreationResult{
		Agent: CreatedAgent{
			Name:            agent.Name,
			Type:            agent.Type,
			KnowledgePath:   kbPath,
			KBSize:          stats.TotalAtoms,
			CreatedAt:       creationTime,
			Status:          "ready",
			Tools:           agent.Tools,
			ToolPreferences: agent.ToolPreferences,
		},
		KBSize:      stats.TotalAtoms,
		Stats:       stats,
		KBPath:      kbPath,
		UpgradeMode: upgradeMode,
		PromptErr:   promptErr,
	}
}
