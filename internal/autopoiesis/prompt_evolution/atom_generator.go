package prompt_evolution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// AtomGenerator generates new prompt atoms from failure patterns.
// This is the core of the prompt evolution system - learning from mistakes.
type AtomGenerator struct {
	llmClient LLMClient
	// rolePrompts compiles the generator's system prompts from the
	// /prompt_atom_generator and /strategy_refiner role atoms
	// (internal/prompt/atoms/autopoiesis/meta_atom_generator.yaml).
	rolePrompts   types.RolePromptCompiler
	strategyStore *StrategyStore

	// pinScope is the granularity at which generated atoms are bound to the
	// provider/model whose failures produced them.
	pinScope PinScope
}

// NewAtomGeneratorWithPinScope creates a new atom generator with an explicit
// pin scope. An unrecognized scope falls back to PinScopeModel (the default)
// rather than silently generating atoms pinned more loosely than asked.
func NewAtomGeneratorWithPinScope(llmClient LLMClient, rolePrompts types.RolePromptCompiler, strategyStore *StrategyStore, scope PinScope) *AtomGenerator {
	if !scope.Valid() {
		logging.Get(logging.CategoryAutopoiesis).Warn(
			"Unknown atom pin scope %q; falling back to %q", scope, DefaultPinScope)
		scope = DefaultPinScope
	}
	return &AtomGenerator{
		llmClient:     llmClient,
		rolePrompts:   rolePrompts,
		strategyStore: strategyStore,
		pinScope:      scope,
	}
}

// ServingPin identifies the LLM a set of failures was observed on. It travels
// with a failure group so generated atoms can be pinned to their origin.
type ServingPin struct {
	// Provider and Model are raw vendor spellings, exactly as recorded on the
	// ExecutionRecord ("anthropic", "claude-opus-4-20260501").
	Provider string
	Model    string
}

// IsZero reports whether the pin carries no serving information at all.
func (p ServingPin) IsZero() bool {
	return strings.TrimSpace(p.Provider) == "" && strings.TrimSpace(p.Model) == ""
}

// String renders the pin for logging and meta-prompt context.
func (p ServingPin) String() string {
	switch {
	case p.IsZero():
		return "unknown"
	case p.Model == "":
		return p.Provider
	case p.Provider == "":
		return p.Model
	default:
		return p.Provider + "/" + p.Model
	}
}

// GenerateFromFailures creates new atoms from the exact bounded cohort observed on
// one provider/model. The pin is stamped onto every generated atom according to
// the generator's PinScope, so an atom learned from one model's failures is not
// served to a different one.
func (ag *AtomGenerator) GenerateFromFailures(
	ctx context.Context,
	cohort *evidenceCohort,
	shardType, problemType string,
	pin ServingPin,
) ([]*GeneratedAtom, error) {
	if cohort == nil {
		return nil, nil
	}
	failures := cohort.verdicts
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "AtomGenerator.GenerateFromFailures")
	defer timer.Stop()

	if len(failures) == 0 {
		return nil, nil
	}

	// No pin, no atom: refused before the model call is bought.
	if err := requirePin(ag.pinScope, shardType, pin); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Atom generation refused: %v", err)
		return nil, err
	}

	logging.Autopoiesis("Generating atoms from %d failures for shard=%s, problem=%s, serving=%s, pin_scope=%s",
		len(failures), shardType, problemType, pin, ag.pinScope)

	userPrompt := ag.buildMetaPrompt(cohort, shardType, problemType, pin)
	systemPrompt, err := ag.systemPrompt(ctx, "prompt_atom_generator")
	if err != nil {
		return nil, err
	}

	// Call LLM
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMAtomGeneration")
	response, err := ag.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Atom generation LLM call failed: %v", err)
		return nil, fmt.Errorf("atom generation failed: %w", err)
	}

	// Parse the generated atoms
	atoms, err := ag.parseGeneratedAtoms(response, shardType, problemType, failures, pin)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to parse generated atoms: %v", err)
		return nil, fmt.Errorf("failed to parse atoms: %w", err)
	}

	logging.Autopoiesis("Generated %d atoms from failures", len(atoms))
	return atoms, nil
}

// buildMetaPrompt constructs the prompt for atom generation.
func (ag *AtomGenerator) buildMetaPrompt(cohort *evidenceCohort, shardType, problemType string, pin ServingPin) string {
	var sb strings.Builder

	sb.WriteString("## Current Context\n")
	sb.WriteString(fmt.Sprintf("- **Shard Type**: %s\n", shardType))
	sb.WriteString(fmt.Sprintf("- **Problem Type**: %s\n", problemType))
	sb.WriteString(fmt.Sprintf("- **Description**: %s\n", GetProblemTypeDescription(ProblemType(problemType))))

	// Naming the serving model matters for content, not just bookkeeping: these
	// atoms will only ever be served back to this model, so the generator is
	// free to write guidance specific to it instead of hedging for a general
	// audience.
	if !pin.IsZero() {
		sb.WriteString(fmt.Sprintf("- **Observed On**: %s\n", pin))
		sb.WriteString("\nAll failures below were produced by that model, and any atom you generate\n")
		sb.WriteString("will be served only to that model. Guidance specific to its known behavior\n")
		sb.WriteString("is therefore in scope and preferred over vendor-neutral advice.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Failure Analysis\n\n")

	sb.WriteString("Selected evidence cohort " + cohort.id + "\n")
	sb.WriteString(cohort.json + "\n\n")

	sb.WriteString("## Task\n\n")
	sb.WriteString("Generate 1-3 prompt atoms that would help prevent these failures.\n")
	sb.WriteString("Focus on the specific patterns observed in the failures above.\n")

	return sb.String()
}

// parseGeneratedAtoms extracts atoms from the LLM response.
func (ag *AtomGenerator) parseGeneratedAtoms(
	response string,
	shardType, problemType string,
	sourceFailures []*JudgeVerdict,
	pin ServingPin,
) ([]*GeneratedAtom, error) {
	// Extract YAML content
	yamlContent := extractYAMLBlock(response)
	if yamlContent == "" {
		// Try to find YAML without code fence
		yamlContent = extractYAMLContent(response)
	}

	if yamlContent == "" {
		return nil, fmt.Errorf("no YAML content found in response")
	}

	// Parse YAML into atom definitions
	var atomDefs []atomDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &atomDefs); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to GeneratedAtoms
	var result []*GeneratedAtom
	sourceIDs := make([]string, 0, len(sourceFailures))
	for _, f := range sourceFailures {
		sourceIDs = append(sourceIDs, f.TaskID)
	}

	for _, def := range atomDefs {
		atom := ag.convertToPromptAtom(def, shardType, pin)
		if atom == nil {
			continue
		}

		generated := &GeneratedAtom{
			Atom:        atom,
			Source:      "failure_analysis",
			SourceIDs:   sourceIDs,
			ShardType:   shardType,
			ProblemType: problemType,
			Provider:    pin.Provider,
			Model:       pin.Model,
			PinScope:    ag.pinScope,
			Confidence:  0.5, // Start with neutral confidence
			CreatedAt:   time.Now(),
		}

		result = append(result, generated)
	}

	return result, nil
}

// atomDefinition represents the YAML structure for generated atoms.
type atomDefinition struct {
	ID          string   `yaml:"id"`
	Category    string   `yaml:"category"`
	IsMandatory bool     `yaml:"is_mandatory"`
	Languages   []string `yaml:"languages"`
	Content     string   `yaml:"content"`
}

// convertToPromptAtom converts a definition to a PromptAtom, applying the
// serving pin.
func (ag *AtomGenerator) convertToPromptAtom(def atomDefinition, shardType string, pin ServingPin) *prompt.PromptAtom {
	if def.Content == "" {
		return nil
	}

	// Generate ID if missing
	id := def.ID
	if id == "" {
		id = fmt.Sprintf("evolved/%s/%s", def.Category, uuid.New().String()[:8])
	}

	// Validate and map category
	category := mapCategory(def.Category)

	atom := &prompt.PromptAtom{
		ID:          id,
		Category:    category,
		Importance:  prompt.ImportanceUnscored,
		IsMandatory: def.IsMandatory,
		Content:     def.Content,
		Languages:   def.Languages,
		TokenCount:  estimateTokens(def.Content),
	}

	// The role is the one whose failures produced the atom, whatever shard
	// types the generating model suggested; the model pin is the scope's.
	applyPin(atom, ag.pinScope, shardType, pin)

	// Compute content hash
	atom.ContentHash = computeHash(def.Content)

	return atom
}

// pinSelectors derives the atom's provider/model selectors from the serving pin
// under scope.
//
// The selectors are canonical tokens (prompt.NormalizeProviderToken /
// NormalizeModelToken), because that is the vocabulary the JIT compiler matches
// on. Emitting the raw vendor spelling here would produce an atom that looks
// pinned but never matches -- the failure mode this whole mechanism exists to
// prevent, and a silent one.
//
// A pin that cannot satisfy the scope never reaches here: requirePin refuses
// it first, so a missing model is an error, not a silently broader selector.
func pinSelectors(scope PinScope, pin ServingPin) (providers, models []string) {
	if token := prompt.NormalizeProviderToken(pin.Provider); token != "" {
		providers = []string{token}
	}

	if scope != PinScopeProvider {
		if exact := prompt.NormalizeModelToken(pin.Model); exact != "" {
			token := exact
			if scope == PinScopeModelFamily {
				if family := prompt.ModelFamilyToken(exact); family != "" {
					token = family
				}
			}
			models = []string{token}
		}
	}

	return providers, models
}

// RefineStrategy generates an improved version of a strategy.
func (ag *AtomGenerator) RefineStrategy(
	ctx context.Context,
	strategy *Strategy,
	recentOutcomes []*JudgeVerdict,
) (*Strategy, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "AtomGenerator.RefineStrategy")
	defer timer.Stop()

	logging.Autopoiesis("Refining strategy: id=%s, problem=%s, shard=%s",
		strategy.ID, strategy.ProblemType, strategy.ShardType)

	// Build refinement prompt
	userPrompt := ag.buildRefinementPrompt(strategy, recentOutcomes)

	systemPrompt, err := ag.systemPrompt(ctx, "strategy_refiner")
	if err != nil {
		return nil, err
	}

	// Call LLM
	response, err := ag.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("strategy refinement failed: %w", err)
	}

	// Extract refined content
	refinedContent := extractRefinedStrategy(response)
	if refinedContent == "" {
		return nil, fmt.Errorf("no refined strategy found in response")
	}

	// Create refined strategy
	refined := &Strategy{
		ID:          strategy.ID,
		ProblemType: strategy.ProblemType,
		ShardType:   strategy.ShardType,
		Content:     refinedContent,
		Version:     strategy.Version + 1,
		Source:      "evolved",
		LastRefined: time.Now(),
		CreatedAt:   strategy.CreatedAt,
	}

	logging.Autopoiesis("Strategy refined: id=%s, version=%d", refined.ID, refined.Version)
	return refined, nil
}

// buildRefinementPrompt constructs the prompt for strategy refinement.
func (ag *AtomGenerator) buildRefinementPrompt(strategy *Strategy, outcomes []*JudgeVerdict) string {
	var sb strings.Builder

	sb.WriteString("## Current Strategy\n\n")
	sb.WriteString(fmt.Sprintf("**Problem Type**: %s\n", strategy.ProblemType))
	sb.WriteString(fmt.Sprintf("**Shard Type**: %s\n", strategy.ShardType))
	sb.WriteString(fmt.Sprintf("**Success Rate**: %.1f%% (%d/%d)\n\n",
		strategy.SuccessRate*100,
		strategy.SuccessCount,
		strategy.SuccessCount+strategy.FailureCount))

	sb.WriteString("**Current Content:**\n```\n")
	sb.WriteString(strategy.Content)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Recent Outcomes\n\n")

	// Separate successes and failures
	var successes, failures []*JudgeVerdict
	for _, o := range outcomes {
		if o.IsPass() {
			successes = append(successes, o)
		} else {
			failures = append(failures, o)
		}
	}

	if len(failures) > 0 {
		sb.WriteString("### Failures:\n")
		for i, f := range failures {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", f.Category, f.Explanation))
			if f.ImprovementRule != "" {
				sb.WriteString(fmt.Sprintf("  → %s\n", f.ImprovementRule))
			}
		}
		sb.WriteString("\n")
	}

	if len(successes) > 0 {
		sb.WriteString("### Successes:\n")
		for i, s := range successes {
			if i >= 3 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", s.Explanation))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Task\n\n")
	sb.WriteString("Refine this strategy to reduce failures while maintaining what works.\n")
	sb.WriteString("Focus on addressing the specific failure patterns observed.\n")

	return sb.String()
}

// Helper functions

func extractYAMLBlock(s string) string {
	start := strings.Index(s, "```yaml")
	if start == -1 {
		start = strings.Index(s, "```yml")
	}
	if start == -1 {
		return ""
	}

	start = strings.Index(s[start:], "\n")
	if start == -1 {
		return ""
	}
	start += strings.Index(s, "```") + 1

	end := strings.LastIndex(s, "```")
	if end == -1 || end <= start {
		return ""
	}

	return strings.TrimSpace(s[start:end])
}

func extractYAMLContent(s string) string {
	// Look for YAML-like content (starts with -)
	lines := strings.Split(s, "\n")
	var yamlLines []string
	inYAML := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- id:") || strings.HasPrefix(trimmed, "-  id:") {
			inYAML = true
		}
		if inYAML {
			yamlLines = append(yamlLines, line)
		}
	}

	if len(yamlLines) == 0 {
		return ""
	}

	return strings.Join(yamlLines, "\n")
}

func extractRefinedStrategy(s string) string {
	// Look for markdown code block
	start := strings.Index(s, "```")
	if start != -1 {
		start = strings.Index(s[start:], "\n")
		if start != -1 {
			start += strings.Index(s, "```") + 1
			end := strings.LastIndex(s, "```")
			if end > start {
				return strings.TrimSpace(s[start:end])
			}
		}
	}

	// Look for content after "Refined Strategy:" or similar
	markers := []string{
		"Refined Strategy:",
		"**Refined Strategy**",
		"## Refined Strategy",
		"### Refined",
	}

	for _, marker := range markers {
		if _, after, ok := strings.Cut(s, marker); ok {
			content := after
			// Take until next section or end
			if nextSection := strings.Index(content, "\n## "); nextSection != -1 {
				content = content[:nextSection]
			}
			return strings.TrimSpace(content)
		}
	}

	return ""
}

func mapCategory(s string) prompt.AtomCategory {
	normalized := strings.ToLower(strings.TrimSpace(s))
	switch normalized {
	case "identity":
		return prompt.CategoryIdentity
	case "protocol":
		return prompt.CategoryProtocol
	case "safety":
		return prompt.CategorySafety
	case "methodology":
		return prompt.CategoryMethodology
	case "hallucination":
		return prompt.CategoryHallucination
	case "language":
		return prompt.CategoryLanguage
	case "framework":
		return prompt.CategoryFramework
	case "domain":
		return prompt.CategoryDomain
	case "campaign":
		return prompt.CategoryCampaign
	case "context":
		return prompt.CategoryContext
	case "exemplar":
		return prompt.CategoryExemplar
	case "reviewer":
		return prompt.CategoryReviewer
	case "knowledge":
		return prompt.CategoryKnowledge
	case "intent":
		return prompt.CategoryIntent
	case "world_state":
		return prompt.CategoryWorldState
	default:
		return prompt.CategoryMethodology // Default for evolved atoms
	}
}

func estimateTokens(s string) int {
	// Rough estimate: ~4 characters per token
	return len(s) / 4
}

func computeHash(s string) string {
	// Simple hash for content comparison
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	return fmt.Sprintf("%x", h)
}

// systemPrompt compiles role's system prompt with intent /judge, which keeps
// worker intent atoms out; the role's atom supersedes the Piggyback protocol,
// since Go parses the reply. No compiler, or a failed compile, is an error
// wrapping types.ErrRolePromptUnavailable.
func (ag *AtomGenerator) systemPrompt(ctx context.Context, role string) (string, error) {
	if ag.rolePrompts == nil {
		return "", fmt.Errorf("%w: no role prompt compiler for the %s", types.ErrRolePromptUnavailable, role)
	}
	return ag.rolePrompts.CompileRolePrompt(ctx, types.RolePromptRequest{Role: role, Intent: "judge"})
}
