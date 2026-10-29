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
// planner is configured), each atom admitted by the generated-atom check
// (agents_admission.go); a failed generation writes no atom for its body.
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
	// A winning imported subagent or mode carries its own prompt: the
	// user's own definition, written once as the methodology body (it was
	// written as both bodies, so every piece reached the prompt twice) and
	// not judged by the generated-atom admission check.
	if imported := ecosystemImportedPrompt(i.config.Workspace, agent.Name); imported != "" {
		topicsStr := strings.Join(agent.Topics, ", ")
		domainExpertise := formatDomainExpertise(agent.Topics)
		agentNameLower := strings.ToLower(agent.Name)
		yamlStr := buildPromptsYAML(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, imported, "", i.atomMaxChars())
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

	// Methodology and domain are model-written for this specialist. A failed
	// or empty generation contributes no atom: a generic template would only
	// ride into every prompt as advice the model already follows.
	var methodologyContent, domainContent string
	if i.route(slotPlanner).client == nil {
		logging.BootWarn("No LLM client for %s: prompts.yaml holds its identity only", agent.Name)
	} else {
		if meth, err := i.generateMethodologyContent(ctx, agent); err != nil {
			logging.BootWarn("No methodology atoms for %s: %v", agent.Name, err)
		} else {
			methodologyContent = strings.TrimSpace(meth)
		}
		if dom, err := i.generateDomainContent(ctx, agent); err != nil {
			logging.BootWarn("No domain atoms for %s: %v", agent.Name, err)
		} else {
			domainContent = strings.TrimSpace(dom)
		}
	}

	atoms := agentPromptAtoms(agentNameLower, agent.Name, agent.Description, domainExpertise, topicsStr, methodologyContent, domainContent, i.atomMaxChars())
	atoms = admitAgentAtoms(ctx, i.atomAdmission(), agentNameLower, atoms)
	yamlStr := renderPromptsYAML(agent.Name, atoms)
	if err := validatePromptsYAML([]byte(yamlStr), agentNameLower, i.atomMaxChars()); err != nil {
		return fmt.Errorf("generated prompts.yaml for %s: %w", agent.Name, err)
	}
	if err := os.WriteFile(promptsPath, []byte(yamlStr), 0644); err != nil {
		return fmt.Errorf("failed to write prompts.yaml: %w", err)
	}
	logging.Boot("Generated prompts.yaml for %s at %s (%d atoms)", agent.Name, promptsPath, len(atoms))
	return nil
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
// usable atom set: the identity atom exists, a methodology or domain body's
// base atom exists whenever a capped sub-atom continues it, ids are
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
	if !found[agentNameLower+"/identity"] {
		return fmt.Errorf("missing expected atom %q", agentNameLower+"/identity")
	}
	for _, body := range []string{"/methodology", "/domain"} {
		base := agentNameLower + body
		for id := range found {
			if strings.HasPrefix(id, base+"/") && !found[base] {
				return fmt.Errorf("atom %q continues %q, which is missing", id, base)
			}
		}
	}
	return nil
}

// RepairExistingPromptAtoms re-atomizes every existing
// .nerd/agents/*/prompts.yaml in place so oversized atoms (the 3-atom,
// up-to-80k-character files written before the atom cap) become capped,
// selectable atoms without rerunning nerd init. It preserves every word and
// field and only cuts content at heading boundaries into capped sub-atoms;
// files already within the cap are left untouched. It returns how many files
// were rewritten.
func RepairExistingPromptAtoms(workspace string) (int, error) {
	agentsDir := filepath.Join(workspace, ".nerd", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("list agent prompts: %w", err)
	}
	atomCap := WorkspaceAtomMaxChars(workspace)
	fixed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		agentNameLower := e.Name()
		promptsPath := filepath.Join(agentsDir, agentNameLower, "prompts.yaml")
		data, err := os.ReadFile(promptsPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fixed, fmt.Errorf("read %s: %w", promptsPath, err)
		}
		if err := validatePromptsYAML(data, agentNameLower, atomCap); err == nil {
			continue
		}
		refixed, changed, ferr := resplitPromptsYAML(data, atomCap)
		if ferr != nil {
			return fixed, fmt.Errorf("re-atomize %s: %w", promptsPath, ferr)
		}
		if !changed {
			return fixed, fmt.Errorf("re-atomize %s: still oversized and could not be split", promptsPath)
		}
		if err := validatePromptsYAML(refixed, agentNameLower, atomCap); err != nil {
			return fixed, fmt.Errorf("re-atomize %s: %w", promptsPath, err)
		}
		if err := os.WriteFile(promptsPath, refixed, 0644); err != nil {
			return fixed, fmt.Errorf("write %s: %w", promptsPath, err)
		}
		logging.Boot("Re-atomized oversize prompt atoms in %s to the %d-character cap", promptsPath, atomCap)
		fixed++
	}
	return fixed, nil
}

// buildPromptsYAML assembles the full prompts.yaml document from its pieces
// (agentPromptAtoms, renderPromptsYAML).
func buildPromptsYAML(agentNameLower, displayName, description, domainExpertise, topicsStr, methodologyContent, domainContent string, atomCap int) string {
	return renderPromptsYAML(displayName, agentPromptAtoms(agentNameLower, displayName, description, domainExpertise, topicsStr, methodologyContent, domainContent, atomCap))
}

// agentPromptAtoms builds a specialist's atoms: the identity from the
// template, then the methodology and domain markdown, each atomized at its
// heading boundaries into capped atoms (atomizeContent), so no atom exceeds
// atomCap (jit.atom_max_chars). The first piece of each body keeps the base id
// (<agent>/methodology, <agent>/domain). An empty body contributes no atom.
func agentPromptAtoms(agentNameLower, displayName, description, domainExpertise, topicsStr, methodologyContent, domainContent string, atomCap int) []agentPromptAtom {
	// domainExpertise arrives pre-indented for the old block-scalar template
	// (formatDomainExpertise); the identity body is plain markdown here.
	expertiseLines := strings.Split(domainExpertise, "\n")
	for i, line := range expertiseLines {
		expertiseLines[i] = strings.TrimPrefix(line, "    ")
	}
	identityContent := fmt.Sprintf("You are %s, a specialist agent in the codeNERD ecosystem, carrying out the tasks the kernel assigns.\n\nRole: %s\n\nExpertise:\n%s\n\nResearch topics: %s",
		displayName, description, strings.Join(expertiseLines, "\n"), topicsStr)

	// Identity sub-atoms stay mandatory (atomizeContent propagates the
	// flag): splitting the identity must not make pieces of who the agent is
	// optional.
	atoms := atomizeContent(
		agentNameLower+"/identity", "identity", agentNameLower, true, nil,
		"Identity and mission for "+displayName,
		fmt.Sprintf("You are %s, a specialist agent in the codeNERD ecosystem.\nRole: %s\nTopics: %s", displayName, description, topicsStr),
		fmt.Sprintf("You are %s (%s).", displayName, description),
		identityContent, atomCap, false)
	domainDeps := []string{agentNameLower + "/identity"}
	if strings.TrimSpace(methodologyContent) != "" {
		atoms = append(atoms, atomizeContent(
			agentNameLower+"/methodology", "methodology", agentNameLower, false,
			[]string{agentNameLower + "/identity"},
			"Methodology for "+displayName, "", "",
			methodologyContent, atomCap, true)...)
		domainDeps = append(domainDeps, agentNameLower+"/methodology")
	}
	if strings.TrimSpace(domainContent) != "" {
		atoms = append(atoms, atomizeContent(
			agentNameLower+"/domain", "domain", agentNameLower, false, domainDeps,
			"Domain knowledge and pitfalls for "+displayName, "", "",
			domainContent, atomCap, true)...)
	}
	return atoms
}

// renderPromptsYAML renders a specialist's atoms as its prompts.yaml.
func renderPromptsYAML(displayName string, atoms []agentPromptAtom) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Prompt atoms for %s\n", displayName)
	b.WriteString("# These are loaded into the JIT prompt compiler when the agent is spawned.\n")
	b.WriteString("# One concern per atom: each is selected and embedded on its own.\n\n")
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
// are wrapped distinctly so callers report a clear reason for the missing
// atoms while preserving
// errors.Is(err, context.DeadlineExceeded) and errors.Is(err, context.Canceled).
func (i *Initializer) generateAgentAtomContent(ctx context.Context, agent RecommendedAgent, kind, phase, task string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%s generation for %q skipped, parent context done: %w", kind, agent.Name, err)
	}
	res, err := i.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: phase, task: task},
		func(callCtx context.Context, client perception.LLMClient, systemPrompt string) (string, error) {
			return client.CompleteWithSystem(callCtx, systemPrompt, task)
		})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return "", fmt.Errorf("%s generation for %q timed out: %w", kind, agent.Name, err)
		case errors.Is(err, context.Canceled):
			return "", fmt.Errorf("%s generation for %q canceled: %w", kind, agent.Name, err)
		}
		return "", err
	}
	if strings.TrimSpace(res) == "" {
		return "", fmt.Errorf("empty LLM response for %s generation for %q, no atoms written", kind, agent.Name)
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

// agentSectionLimit is the per-section target the request gives the model.
// One concern per atom means sections stay well under the atom cap
// (jit.atom_max_chars): a third of the cap clamped to 300..600 characters, so
// even a section that uses its full budget atomizes to one small selectable
// atom instead of one cap-sized blob.
func (i *Initializer) agentSectionLimit() int {
	limit := i.atomMaxChars() / 3
	if limit < 300 {
		limit = 300
	}
	if limit > 600 {
		limit = 600
	}
	if cap := i.atomMaxChars(); limit > cap {
		limit = cap
	}
	return limit
}

// agentPromptRequest is the data of a specialist prompt-atom request.
func (i *Initializer) agentPromptRequest(agent RecommendedAgent) string {
	return fmt.Sprintf("Specialist agent: %q\nRole: %q\nResearch topics: %q\nSection limit: %d characters",
		agent.Name, agent.Description, strings.Join(agent.Topics, ", "), i.agentSectionLimit())
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
func (i *Initializer) createType3Agents(ctx context.Context, nerdDir string, agents []RecommendedAgent, result *InitResult) ([]CreatedAgent, map[string]int) { // The seed is read by the parallel KB workers. Clear it when they have
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
	// Heal pre-existing agent DBs that this run did not create: empty KBs get
	// baseline atoms and unembedded atoms get vectors, so all agents converge
	// without requiring --force.
	EnsureAllAgentKnowledge(ctx, i.config.Workspace, i.embedEngine)
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
			Domains:         agent.Domains,
		},
		KBSize:      stats.TotalAtoms,
		Stats:       stats,
		KBPath:      kbPath,
		UpgradeMode: upgradeMode,
		PromptErr:   promptErr,
	}
}
