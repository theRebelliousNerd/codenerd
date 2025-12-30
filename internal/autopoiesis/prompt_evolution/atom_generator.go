package prompt_evolution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// AtomGenerator generates new prompt atoms from failure patterns.
// This is the core of the prompt evolution system - learning from mistakes.
type AtomGenerator struct {
	llmClient     LLMClient
	strategyStore *StrategyStore
}

// NewAtomGenerator creates a new atom generator.
func NewAtomGenerator(llmClient LLMClient, strategyStore *StrategyStore) *AtomGenerator {
	return &AtomGenerator{
		llmClient:     llmClient,
		strategyStore: strategyStore,
	}
}

// GenerateFromFailures creates new atoms based on failure patterns.
func (ag *AtomGenerator) GenerateFromFailures(
	ctx context.Context,
	failures []*JudgeVerdict,
	shardType string,
	problemType string,
) ([]*GeneratedAtom, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "AtomGenerator.GenerateFromFailures")
	defer timer.Stop()

	if len(failures) == 0 {
		return nil, nil
	}

	logging.Autopoiesis("Generating atoms from %d failures for shard=%s, problem=%s",
		len(failures), shardType, problemType)

	// Build the meta-prompt
	userPrompt := ag.buildMetaPrompt(failures, shardType, problemType)

	// Call LLM
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMAtomGeneration")
	response, err := ag.llmClient.CompleteWithSystem(ctx, atomGenerationSystemPrompt, userPrompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Atom generation LLM call failed: %v", err)
		return nil, fmt.Errorf("atom generation failed: %w", err)
	}

	// Parse the generated atoms
	atoms, err := ag.parseGeneratedAtoms(response, shardType, problemType, failures)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to parse generated atoms: %v", err)
		return nil, fmt.Errorf("failed to parse atoms: %w", err)
	}

	logging.Autopoiesis("Generated %d atoms from failures", len(atoms))
	return atoms, nil
}

// buildMetaPrompt constructs the prompt for atom generation.
func (ag *AtomGenerator) buildMetaPrompt(failures []*JudgeVerdict, shardType, problemType string) string {
	var sb strings.Builder

	sb.WriteString("## Current Context\n")
	sb.WriteString(fmt.Sprintf("- **Shard Type**: %s\n", shardType))
	sb.WriteString(fmt.Sprintf("- **Problem Type**: %s\n", problemType))
	sb.WriteString(fmt.Sprintf("- **Description**: %s\n\n", GetProblemTypeDescription(ProblemType(problemType))))

	sb.WriteString("## Failure Analysis\n\n")

	// Group failures by category
	byCategory := make(map[ErrorCategory][]*JudgeVerdict)
	for _, f := range failures {
		byCategory[f.Category] = append(byCategory[f.Category], f)
	}

	for category, categoryFailures := range byCategory {
		sb.WriteString(fmt.Sprintf("### %s Failures (%d)\n\n", category, len(categoryFailures)))

		for i, f := range categoryFailures {
			if i >= 3 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(categoryFailures)-3))
				break
			}
			sb.WriteString(fmt.Sprintf("**Failure %d:**\n", i+1))
			sb.WriteString(fmt.Sprintf("- Explanation: %s\n", f.Explanation))
			if f.ImprovementRule != "" {
				sb.WriteString(fmt.Sprintf("- Suggested Rule: %s\n", f.ImprovementRule))
			}
			sb.WriteString("\n")
		}
	}

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
		atom := ag.convertToPromptAtom(def, shardType)
		if atom == nil {
			continue
		}

		generated := &GeneratedAtom{
			Atom:        atom,
			Source:      "failure_analysis",
			SourceIDs:   sourceIDs,
			ShardType:   shardType,
			ProblemType: problemType,
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
	Priority    int      `yaml:"priority"`
	IsMandatory bool     `yaml:"is_mandatory"`
	ShardTypes  []string `yaml:"shard_types"`
	Languages   []string `yaml:"languages"`
	Content     string   `yaml:"content"`
}

// convertToPromptAtom converts a definition to a PromptAtom.
func (ag *AtomGenerator) convertToPromptAtom(def atomDefinition, shardType string) *prompt.PromptAtom {
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

	// Set defaults
	priority := def.Priority
	if priority == 0 {
		priority = 70 // Medium-high priority for evolved atoms
	}

	// Ensure shard types includes the source shard
	shardTypes := def.ShardTypes
	if len(shardTypes) == 0 {
		shardTypes = []string{shardType}
	}

	atom := &prompt.PromptAtom{
		ID:          id,
		Category:    category,
		Priority:    priority,
		IsMandatory: def.IsMandatory,
		Content:     def.Content,
		ShardTypes:  shardTypes,
		Languages:   def.Languages,
		TokenCount:  estimateTokens(def.Content),
	}

	// Compute content hash
	atom.ContentHash = computeHash(def.Content)

	return atom
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

	// Call LLM
	response, err := ag.llmClient.CompleteWithSystem(ctx, strategyRefinementSystemPrompt, userPrompt)
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
		if idx := strings.Index(s, marker); idx != -1 {
			content := s[idx+len(marker):]
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

// System prompts

var atomGenerationSystemPrompt = `You are an expert at creating prompt atoms for an AI coding agent.

Your task is to analyze failure patterns and create new prompt atoms that would prevent similar failures.

## Atom Structure

Each atom MUST follow this exact YAML structure:
- id: "category/subcategory/descriptive_name"
  category: "methodology|language|framework|domain|exemplar"
  priority: 50-90
  is_mandatory: false
  shard_types: ["/coder", "/tester", etc.]
  languages: ["/go", "/python", etc.]  # optional, if language-specific
  content: |
    Clear, actionable instructions...

## Guidelines

1. **Be Specific**: Focus on the exact failure patterns observed
2. **Be Actionable**: Use imperative mood ("Always check...", "Never assume...")
3. **Be Concise**: Each atom should address one concern
4. **Include Examples**: When helpful, show what to do and what to avoid
5. **Use Appropriate Categories**:
   - methodology: Problem-solving approaches
   - language: Language-specific guidance
   - framework: Framework-specific patterns
   - domain: Project/domain context
   - exemplar: Few-shot examples

Output ONLY valid YAML, no other text.`

var strategyRefinementSystemPrompt = `You are an expert at improving problem-solving strategies for an AI coding agent.

Your task is to refine a strategy based on recent successes and failures.

## Guidelines

1. **Preserve What Works**: Keep elements that contributed to successes
2. **Address Failures**: Add guidance to prevent observed failure patterns
3. **Be Specific**: Include concrete steps and examples
4. **Be Practical**: Focus on actionable guidance
5. **Maintain Structure**: Keep a clear, numbered format

Output the refined strategy content in a markdown code block.`
