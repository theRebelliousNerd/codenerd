// Package researcher - Prompt atom generation for shard-specific knowledge.
// This file implements extraction of domain-specific prompt atoms from research results.
package researcher

import (
	"codenerd/internal/logging"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PromptAtomData represents a prompt atom for storage.
// This is a local type to avoid circular dependencies with the prompt package.
type PromptAtomData struct {
	ID               string
	Version          int
	Content          string
	TokenCount       int
	ContentHash      string
	Category         string
	Subcategory      string
	OperationalModes []string
	CampaignPhases   []string
	BuildLayers      []string
	InitPhases       []string
	NorthstarPhases  []string
	OuroborosStages  []string
	IntentVerbs      []string
	ShardTypes       []string
	Languages        []string
	Frameworks       []string
	WorldStates      []string
	Priority         int
	IsMandatory      bool
	IsExclusive      string
	DependsOn        []string
	ConflictsWith    []string
	EmbeddingTask    string
	CreatedAt        time.Time
}

// estimateTokens estimates the token count for content using chars/4 approximation.
func estimateTokens(content string) int {
	if content == "" {
		return 0
	}
	return (len(content) + 3) / 4
}

// hashContent computes a SHA256 hash of content for deduplication.
func hashContent(content string) string {
	if content == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// newPromptAtom creates a new PromptAtomData with computed fields.
func newPromptAtom(id string, category string, content string) *PromptAtomData {
	return &PromptAtomData{
		ID:          id,
		Version:     1,
		Category:    category,
		Content:     content,
		TokenCount:  estimateTokens(content),
		ContentHash: hashContent(content),
		CreatedAt:   time.Now(),
	}
}

// generateDomainAtoms extracts prompt atoms from research results.
// This method analyzes gathered knowledge to identify methodology patterns,
// best practices, and domain-specific guidance suitable for JIT prompt compilation.
//
// The generated atoms have:
// - Category: "domain" or "exemplar"
// - ShardTypes: Selector for the shard that owns them
// - Priority: 60-70 range (domain-specific atoms)
// - Appropriate selectors based on content analysis
func (r *ResearcherShard) generateDomainAtoms(ctx context.Context, result *ResearchResult) ([]*PromptAtomData, error) {
	if result == nil || len(result.Atoms) == 0 {
		return nil, nil
	}

	logging.Researcher("Generating prompt atoms from %d knowledge atoms", len(result.Atoms))

	var promptAtoms []*PromptAtomData

	// Group atoms by concept for pattern detection
	conceptGroups := make(map[string][]KnowledgeAtom)
	for _, atom := range result.Atoms {
		conceptGroups[atom.Concept] = append(conceptGroups[atom.Concept], atom)
	}

	// Extract methodology patterns
	methodologyAtoms := r.extractMethodologyAtoms(conceptGroups, result.Query)
	promptAtoms = append(promptAtoms, methodologyAtoms...)

	// Extract code pattern exemplars
	exemplarAtoms := r.extractExemplarAtoms(conceptGroups, result.Query)
	promptAtoms = append(promptAtoms, exemplarAtoms...)

	// Extract anti-pattern warnings
	antiPatternAtoms := r.extractAntiPatternAtoms(conceptGroups, result.Query)
	promptAtoms = append(promptAtoms, antiPatternAtoms...)

	// Extract best practices
	bestPracticeAtoms := r.extractBestPracticeAtoms(conceptGroups, result.Query)
	promptAtoms = append(promptAtoms, bestPracticeAtoms...)

	logging.Researcher("Generated %d prompt atoms from research", len(promptAtoms))
	return promptAtoms, nil
}

// extractMethodologyAtoms extracts methodology guidance atoms.
func (r *ResearcherShard) extractMethodologyAtoms(conceptGroups map[string][]KnowledgeAtom, query string) []*PromptAtomData {
	var atoms []*PromptAtomData

	// Look for methodology-related concepts
	methodologyConcepts := []string{
		"best_practice", "methodology", "approach", "pattern",
		"strategy", "technique", "process", "workflow",
	}

	for concept, knowledgeAtoms := range conceptGroups {
		isMethodology := false
		for _, mc := range methodologyConcepts {
			if strings.Contains(strings.ToLower(concept), mc) {
				isMethodology = true
				break
			}
		}

		if !isMethodology {
			continue
		}

		// Combine high-confidence atoms from this concept
		var contentBuilder strings.Builder
		count := 0

		for _, ka := range knowledgeAtoms {
			if ka.Confidence >= 0.7 {
				contentBuilder.WriteString(ka.Content)
				contentBuilder.WriteString("\n\n")
				count++
			}
		}

		if count == 0 {
			continue
		}

		content := strings.TrimSpace(contentBuilder.String())
		if len(content) < 50 {
			continue // Too short to be useful
		}

		// Create domain atom with methodology guidance
		atomID := fmt.Sprintf("domain/methodology/%s/%s",
			sanitizeForID(concept),
			sanitizeForID(query))

		atom := newPromptAtom(atomID, "domain", content)
		atom.Subcategory = "methodology"
		atom.Priority = 65
		atom.ShardTypes = r.inferShardTypes(content)
		atom.Languages = r.inferLanguages(content)
		atom.Frameworks = r.inferFrameworks(content)

		atoms = append(atoms, atom)
	}

	return atoms
}

// extractExemplarAtoms extracts code example atoms.
func (r *ResearcherShard) extractExemplarAtoms(conceptGroups map[string][]KnowledgeAtom, query string) []*PromptAtomData {
	var atoms []*PromptAtomData

	for concept, knowledgeAtoms := range conceptGroups {
		for _, ka := range knowledgeAtoms {
			// Only extract atoms that have code patterns
			if ka.CodePattern == "" {
				continue
			}

			// Build exemplar content with context
			var contentBuilder strings.Builder
			contentBuilder.WriteString(fmt.Sprintf("**%s**\n\n", ka.Title))
			if ka.Content != "" {
				contentBuilder.WriteString(ka.Content)
				contentBuilder.WriteString("\n\n")
			}
			contentBuilder.WriteString("```\n")
			contentBuilder.WriteString(ka.CodePattern)
			contentBuilder.WriteString("\n```\n")

			content := contentBuilder.String()

			// Create exemplar atom
			atomID := fmt.Sprintf("exemplar/%s/%s",
				sanitizeForID(concept),
				sanitizeForID(ka.Title))

			atom := newPromptAtom(atomID, "exemplar", content)
			atom.Priority = 60
			atom.ShardTypes = r.inferShardTypes(content)
			atom.Languages = r.inferLanguages(content)
			atom.Frameworks = r.inferFrameworks(content)

			// Mark high-confidence exemplars as mandatory
			if ka.Confidence >= 0.9 {
				atom.IsMandatory = true
			}

			atoms = append(atoms, atom)
		}
	}

	return atoms
}

// extractAntiPatternAtoms extracts anti-pattern warning atoms.
func (r *ResearcherShard) extractAntiPatternAtoms(conceptGroups map[string][]KnowledgeAtom, query string) []*PromptAtomData {
	var atoms []*PromptAtomData

	for concept, knowledgeAtoms := range conceptGroups {
		for _, ka := range knowledgeAtoms {
			// Only extract atoms that have anti-patterns
			if ka.AntiPattern == "" {
				continue
			}

			// Build anti-pattern content
			var contentBuilder strings.Builder
			contentBuilder.WriteString(fmt.Sprintf("**⚠️ Anti-Pattern: %s**\n\n", ka.Title))
			if ka.Content != "" {
				contentBuilder.WriteString(ka.Content)
				contentBuilder.WriteString("\n\n")
			}
			contentBuilder.WriteString("**Avoid:**\n```\n")
			contentBuilder.WriteString(ka.AntiPattern)
			contentBuilder.WriteString("\n```\n")

			content := contentBuilder.String()

			// Create domain atom with safety subcategory
			atomID := fmt.Sprintf("domain/anti-pattern/%s/%s",
				sanitizeForID(concept),
				sanitizeForID(ka.Title))

			atom := newPromptAtom(atomID, "domain", content)
			atom.Subcategory = "anti-pattern"
			atom.Priority = 70 // High priority for safety
			atom.ShardTypes = r.inferShardTypes(content)
			atom.Languages = r.inferLanguages(content)
			atom.Frameworks = r.inferFrameworks(content)

			// Anti-patterns are important warnings
			atom.IsMandatory = true

			atoms = append(atoms, atom)
		}
	}

	return atoms
}

// extractBestPracticeAtoms extracts best practice atoms.
func (r *ResearcherShard) extractBestPracticeAtoms(conceptGroups map[string][]KnowledgeAtom, query string) []*PromptAtomData {
	var atoms []*PromptAtomData

	// Look for best practice indicators
	bestPracticeConcepts := []string{
		"best_practice", "guideline", "recommendation",
		"standard", "convention", "idiom",
	}

	for concept, knowledgeAtoms := range conceptGroups {
		isBestPractice := false
		for _, bpc := range bestPracticeConcepts {
			if strings.Contains(strings.ToLower(concept), bpc) {
				isBestPractice = true
				break
			}
		}

		if !isBestPractice {
			// Also check if content mentions best practices
			for _, ka := range knowledgeAtoms {
				lower := strings.ToLower(ka.Content)
				if strings.Contains(lower, "best practice") ||
					strings.Contains(lower, "recommended approach") ||
					strings.Contains(lower, "should always") {
					isBestPractice = true
					break
				}
			}
		}

		if !isBestPractice {
			continue
		}

		// Combine high-confidence atoms
		var contentBuilder strings.Builder
		count := 0

		for _, ka := range knowledgeAtoms {
			if ka.Confidence >= 0.7 {
				contentBuilder.WriteString(fmt.Sprintf("- **%s**: %s\n", ka.Title, ka.Content))
				count++
			}
		}

		if count == 0 {
			continue
		}

		content := strings.TrimSpace(contentBuilder.String())

		// Create domain atom for best practices
		atomID := fmt.Sprintf("domain/best-practice/%s/%s",
			sanitizeForID(concept),
			sanitizeForID(query))

		atom := newPromptAtom(atomID, "domain", content)
		atom.Subcategory = "best-practice"
		atom.Priority = 68
		atom.ShardTypes = r.inferShardTypes(content)
		atom.Languages = r.inferLanguages(content)
		atom.Frameworks = r.inferFrameworks(content)

		atoms = append(atoms, atom)
	}

	return atoms
}

// inferShardTypes infers which shard types should use this atom.
func (r *ResearcherShard) inferShardTypes(content string) []string {
	lower := strings.ToLower(content)
	var types []string

	// Check for shard-specific keywords
	if strings.Contains(lower, "test") || strings.Contains(lower, "testing") {
		types = append(types, "/tester")
	}
	if strings.Contains(lower, "review") || strings.Contains(lower, "quality") ||
		strings.Contains(lower, "security") {
		types = append(types, "/reviewer")
	}
	if strings.Contains(lower, "code") || strings.Contains(lower, "implement") ||
		strings.Contains(lower, "function") || strings.Contains(lower, "class") {
		types = append(types, "/coder")
	}
	if strings.Contains(lower, "research") || strings.Contains(lower, "documentation") {
		types = append(types, "/researcher")
	}

	// If no specific shard detected, make it available to all
	if len(types) == 0 {
		types = []string{"/coder", "/tester", "/reviewer"}
	}

	return types
}

// inferLanguages infers which programming languages this atom applies to.
func (r *ResearcherShard) inferLanguages(content string) []string {
	lower := strings.ToLower(content)
	var languages []string

	langPatterns := map[string][]string{
		"/go":         {"golang", "go ", " go", "package ", "func ", "import "},
		"/python":     {"python", "def ", "import ", "class ", "pip "},
		"/javascript": {"javascript", "js", "const ", "let ", "function(", "npm "},
		"/typescript": {"typescript", "ts", "interface ", "type "},
		"/rust":       {"rust", "fn ", "use ", "cargo"},
		"/java":       {"java", "class ", "public ", "import java"},
		"/mangle":     {"mangle", "datalog", "predicate", "rule"},
	}

	for lang, patterns := range langPatterns {
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				languages = append(languages, lang)
				break
			}
		}
	}

	return languages
}

// inferFrameworks infers which frameworks this atom applies to.
func (r *ResearcherShard) inferFrameworks(content string) []string {
	lower := strings.ToLower(content)
	var frameworks []string

	frameworkKeywords := map[string][]string{
		"/bubbletea": {"bubbletea", "bubble tea", "tea."},
		"/lipgloss":  {"lipgloss", "lip gloss"},
		"/rod":       {"rod", "chromedp"},
		"/gin":       {"gin", "gin-gonic"},
		"/echo":      {"echo", "labstack"},
		"/react":     {"react", "jsx", "usestate"},
		"/django":    {"django", "django."},
		"/flask":     {"flask", "flask."},
	}

	for fw, keywords := range frameworkKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				frameworks = append(frameworks, fw)
				break
			}
		}
	}

	return frameworks
}

// sanitizeForID converts a string to a valid atom ID component.
func sanitizeForID(s string) string {
	// Replace spaces and special chars with hyphens
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.Replace(s, "--", "-", -1)
	}

	// Trim hyphens
	s = strings.Trim(s, "-")

	// Truncate to reasonable length
	if len(s) > 50 {
		s = s[:50]
	}

	return s
}

// storePromptAtoms persists generated prompt atoms to the shard's knowledge database.
func (r *ResearcherShard) storePromptAtoms(atoms []*PromptAtomData) error {
	if r.localDB == nil {
		logging.Researcher("No local DB configured, skipping prompt atom storage")
		return nil
	}

	if len(atoms) == 0 {
		return nil
	}

	logging.Researcher("Storing %d prompt atoms to knowledge database", len(atoms))

	stored := 0
	failed := 0

	for _, atom := range atoms {
		// Store atom as JSON in cold storage
		// We use JSON serialization to avoid circular dependency with store package
		atomJSON, err := json.Marshal(atom)
		if err != nil {
			logging.Get(logging.CategoryResearcher).Warn("Failed to serialize prompt atom %s: %v", atom.ID, err)
			failed++
			continue
		}

		// Store as a fact in cold storage with predicate "prompt_atom_json"
		// This allows retrieval and reconstitution later
		if err := r.localDB.StoreFact("prompt_atom_json", []interface{}{
			atom.ID,
			string(atomJSON),
		}, "prompt", atom.Priority); err != nil {
			logging.Get(logging.CategoryResearcher).Warn("Failed to store prompt atom %s: %v", atom.ID, err)
			failed++
			continue
		}

		stored++
	}

	logging.Researcher("Stored %d/%d prompt atoms (failed: %d)", stored, len(atoms), failed)
	return nil
}

// generateAndStorePromptAtoms is the main entry point for prompt atom generation.
// It should be called after successful research completion.
func (r *ResearcherShard) generateAndStorePromptAtoms(ctx context.Context, result *ResearchResult) error {
	// Generate atoms from research results
	atoms, err := r.generateDomainAtoms(ctx, result)
	if err != nil {
		logging.Get(logging.CategoryResearcher).Warn("Failed to generate prompt atoms: %v", err)
		return err
	}

	if len(atoms) == 0 {
		logging.Researcher("No prompt atoms generated from research")
		return nil
	}

	// Store atoms in knowledge database
	if err := r.storePromptAtoms(atoms); err != nil {
		return fmt.Errorf("failed to store prompt atoms: %w", err)
	}

	// Also store metadata as facts for Mangle queries
	for _, atom := range atoms {
		// Create a fact about this atom being available
		r.localDB.StoreFact("prompt_atom_available", []interface{}{
			atom.ID,
			atom.Category,
			atom.Priority,
		}, "prompt", atom.Priority)
	}

	// Log metrics
	metrics := getPromptAtomMetrics(atoms)
	logPromptAtomMetrics(metrics)

	return nil
}

// PromptAtomMetrics tracks statistics about generated prompt atoms.
type PromptAtomMetrics struct {
	TotalGenerated   int
	ByCategory       map[string]int
	ByShardType      map[string]int
	AverageTokens    int
	MandatoryCount   int
	ExemplarCount    int
	AntiPatternCount int
}

// getPromptAtomMetrics returns statistics about generated atoms for a research result.
func getPromptAtomMetrics(atoms []*PromptAtomData) PromptAtomMetrics {
	metrics := PromptAtomMetrics{
		TotalGenerated: len(atoms),
		ByCategory:     make(map[string]int),
		ByShardType:    make(map[string]int),
	}

	totalTokens := 0

	for _, atom := range atoms {
		// Category stats
		metrics.ByCategory[atom.Category]++

		// Shard type stats
		for _, st := range atom.ShardTypes {
			metrics.ByShardType[st]++
		}

		// Token stats
		totalTokens += atom.TokenCount

		// Special counts
		if atom.IsMandatory {
			metrics.MandatoryCount++
		}
		if atom.Category == "exemplar" {
			metrics.ExemplarCount++
		}
		if atom.Subcategory == "anti-pattern" {
			metrics.AntiPatternCount++
		}
	}

	if len(atoms) > 0 {
		metrics.AverageTokens = totalTokens / len(atoms)
	}

	return metrics
}

// logPromptAtomMetrics logs statistics about generated atoms.
func logPromptAtomMetrics(metrics PromptAtomMetrics) {
	if metrics.TotalGenerated == 0 {
		return
	}

	logging.Researcher("Prompt Atom Generation Summary:")
	logging.Researcher("  Total: %d atoms (avg %d tokens each)", metrics.TotalGenerated, metrics.AverageTokens)
	logging.Researcher("  Mandatory: %d, Exemplars: %d, Anti-Patterns: %d",
		metrics.MandatoryCount, metrics.ExemplarCount, metrics.AntiPatternCount)

	if len(metrics.ByCategory) > 0 {
		categoriesJSON, _ := json.MarshalIndent(metrics.ByCategory, "    ", "  ")
		logging.Researcher("  By Category: %s", string(categoriesJSON))
	}

	if len(metrics.ByShardType) > 0 {
		shardTypesJSON, _ := json.MarshalIndent(metrics.ByShardType, "    ", "  ")
		logging.Researcher("  By Shard Type: %s", string(shardTypesJSON))
	}
}
