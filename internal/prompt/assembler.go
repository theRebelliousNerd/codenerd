package prompt

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

// FinalAssembler concatenates atoms into the final prompt string.
// It handles:
// 1. Category ordering (defined sequence)
// 2. Section headers and separators
// 3. Token counting verification
// 4. Dynamic context injection
type FinalAssembler struct {
	// categoryOrder defines the sequence of categories in the final prompt
	categoryOrder []AtomCategory

	// addSectionHeaders adds markdown headers between categories
	addSectionHeaders bool

	// sectionSeparator is inserted between sections
	sectionSeparator string

	// atomSeparator is inserted between atoms within a section
	atomSeparator string

	// templateEngine for dynamic content injection
	templateEngine *TemplateEngine
}

// NewFinalAssembler creates a new assembler with default settings.
func NewFinalAssembler() *FinalAssembler {
	return &FinalAssembler{
		categoryOrder:     defaultCategoryOrder(),
		addSectionHeaders: false, // Default: no headers for cleaner prompts
		sectionSeparator:  "\n\n",
		atomSeparator:     "\n\n",
		templateEngine:    NewTemplateEngine(),
	}
}

// defaultCategoryOrder returns the standard category sequence.
// Order is important: identity first, context last.
func defaultCategoryOrder() []AtomCategory {
	return []AtomCategory{
		CategoryIdentity,      // Who the agent is
		CategorySafety,        // Constitutional constraints
		CategoryProtocol,      // Operational protocols
		CategoryMethodology,   // Problem-solving approach
		CategoryHallucination, // Anti-hallucination guardrails
		CategoryLanguage,      // Language-specific guidance
		CategoryFramework,     // Framework-specific guidance
		CategoryDomain,        // Project/domain context
		CategoryCampaign,      // Active campaign context
		CategoryInit,          // Init phase specifics
		CategoryNorthstar,     // Planning specifics
		CategoryOuroboros,     // Self-improvement specifics
		CategoryContext,       // Dynamic context (files, symbols)
		CategoryExemplar,      // Few-shot examples (last)
	}
}

// SetCategoryOrder sets a custom category ordering.
func (a *FinalAssembler) SetCategoryOrder(order []AtomCategory) {
	a.categoryOrder = order
}

// SetSectionHeaders controls whether section headers are added.
func (a *FinalAssembler) SetSectionHeaders(enabled bool) {
	a.addSectionHeaders = enabled
}

// SetSeparators configures the separators between sections and atoms.
func (a *FinalAssembler) SetSeparators(section, atom string) {
	a.sectionSeparator = section
	a.atomSeparator = atom
}

// Assemble combines ordered atoms into a single prompt string.
func (a *FinalAssembler) Assemble(atoms []*OrderedAtom, cc *CompilationContext) (string, error) {
	timer := logging.StartTimer(logging.CategoryContext, "FinalAssembler.Assemble")
	defer timer.Stop()

	if len(atoms) == 0 {
		return "", nil
	}

	// Group atoms by category
	byCategory := make(map[AtomCategory][]*OrderedAtom)
	for _, oa := range atoms {
		cat := oa.Atom.Category
		byCategory[cat] = append(byCategory[cat], oa)
	}

	// Sort atoms within each category by order index (preserved from resolver)
	for cat := range byCategory {
		sort.Slice(byCategory[cat], func(i, j int) bool {
			return byCategory[cat][i].Order < byCategory[cat][j].Order
		})
	}

	// Build the prompt in category order
	var sections []string
	for _, cat := range a.categoryOrder {
		atomsInCat, exists := byCategory[cat]
		if !exists || len(atomsInCat) == 0 {
			continue
		}

		section, err := a.assembleSection(cat, atomsInCat, cc)
		if err != nil {
			return "", fmt.Errorf("failed to assemble section %s: %w", cat, err)
		}

		if section != "" {
			sections = append(sections, section)
		}
	}

	// Handle any categories not in the standard order
	for cat, atomsInCat := range byCategory {
		if a.categoryInOrder(cat) {
			continue // Already processed
		}

		section, err := a.assembleSection(cat, atomsInCat, cc)
		if err != nil {
			return "", fmt.Errorf("failed to assemble section %s: %w", cat, err)
		}

		if section != "" {
			sections = append(sections, section)
		}
	}

	// Join sections
	prompt := strings.Join(sections, a.sectionSeparator)

	// Apply final template processing
	if cc != nil {
		prompt = a.templateEngine.Process(prompt, cc)
	}

	logging.Get(logging.CategoryContext).Debug(
		"Assembled prompt: %d sections, %d chars, ~%d tokens",
		len(sections), len(prompt), EstimateTokens(prompt),
	)

	return prompt, nil
}

// assembleSection builds the content for a single category section.
func (a *FinalAssembler) assembleSection(
	category AtomCategory,
	atoms []*OrderedAtom,
	cc *CompilationContext,
) (string, error) {
	if len(atoms) == 0 {
		return "", nil
	}

	var parts []string

	// Add section header if enabled
	if a.addSectionHeaders {
		header := categoryHeader(category)
		if header != "" {
			parts = append(parts, header)
		}
	}

	// Add each atom's content
	for _, oa := range atoms {
		content := oa.Atom.Content

		// Apply template processing to individual atoms
		if cc != nil && strings.Contains(content, "{{") {
			content = a.templateEngine.Process(content, cc)
		}

		parts = append(parts, content)
	}

	return strings.Join(parts, a.atomSeparator), nil
}

// categoryInOrder checks if a category is in the standard order.
func (a *FinalAssembler) categoryInOrder(cat AtomCategory) bool {
	for _, c := range a.categoryOrder {
		if c == cat {
			return true
		}
	}
	return false
}

// categoryHeader returns a markdown header for a category.
func categoryHeader(cat AtomCategory) string {
	names := map[AtomCategory]string{
		CategoryIdentity:      "## Identity",
		CategorySafety:        "## Safety & Constraints",
		CategoryProtocol:      "## Protocols",
		CategoryMethodology:   "## Methodology",
		CategoryHallucination: "## Guardrails",
		CategoryLanguage:      "## Language Guidelines",
		CategoryFramework:     "## Framework Guidelines",
		CategoryDomain:        "## Domain Context",
		CategoryCampaign:      "## Campaign Context",
		CategoryInit:          "## Initialization",
		CategoryNorthstar:     "## Planning",
		CategoryOuroboros:     "## Self-Improvement",
		CategoryContext:       "## Current Context",
		CategoryExemplar:      "## Examples",
	}

	if name, ok := names[cat]; ok {
		return name
	}
	return fmt.Sprintf("## %s", cat)
}

// TemplateEngine handles dynamic content injection in prompts.
// Supports simple {{variable}} substitution from CompilationContext.
type TemplateEngine struct {
	// Custom functions for template processing
	functions map[string]TemplateFunc
}

// TemplateFunc is a function that can be called in templates.
type TemplateFunc func(cc *CompilationContext, args ...string) string

// NewTemplateEngine creates a new template engine with default functions.
func NewTemplateEngine() *TemplateEngine {
	te := &TemplateEngine{
		functions: make(map[string]TemplateFunc),
	}

	// Register default functions
	te.registerDefaults()

	return te
}

// registerDefaults adds the standard template functions.
func (te *TemplateEngine) registerDefaults() {
	// {{language}} - current language
	te.functions["language"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || cc.Language == "" {
			return "unknown"
		}
		return strings.TrimPrefix(cc.Language, "/")
	}

	// {{shard_type}} - current shard type
	te.functions["shard_type"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || cc.ShardType == "" {
			return "agent"
		}
		return strings.TrimPrefix(cc.ShardType, "/")
	}

	// {{operational_mode}} - current mode
	te.functions["operational_mode"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || cc.OperationalMode == "" {
			return "active"
		}
		return strings.TrimPrefix(cc.OperationalMode, "/")
	}

	// {{campaign_phase}} - current campaign phase
	te.functions["campaign_phase"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || cc.CampaignPhase == "" {
			return ""
		}
		return strings.TrimPrefix(cc.CampaignPhase, "/")
	}

	// {{intent_verb}} - current intent
	te.functions["intent_verb"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || cc.IntentVerb == "" {
			return ""
		}
		return strings.TrimPrefix(cc.IntentVerb, "/")
	}

	// {{frameworks}} - comma-separated frameworks
	te.functions["frameworks"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil || len(cc.Frameworks) == 0 {
			return ""
		}
		var clean []string
		for _, fw := range cc.Frameworks {
			clean = append(clean, strings.TrimPrefix(fw, "/"))
		}
		return strings.Join(clean, ", ")
	}

	// {{token_budget}} - available tokens
	te.functions["token_budget"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil {
			return "unknown"
		}
		return fmt.Sprintf("%d", cc.AvailableTokens())
	}

	// {{world_states}} - current world state indicators
	te.functions["world_states"] = func(cc *CompilationContext, args ...string) string {
		if cc == nil {
			return ""
		}
		states := cc.WorldStates()
		if len(states) == 0 {
			return "normal"
		}
		return strings.Join(states, ", ")
	}
}

// RegisterFunction adds a custom template function.
func (te *TemplateEngine) RegisterFunction(name string, fn TemplateFunc) {
	te.functions[name] = fn
}

// Process applies template substitutions to content.
func (te *TemplateEngine) Process(content string, cc *CompilationContext) string {
	if !strings.Contains(content, "{{") {
		return content // Fast path: no templates
	}

	result := content

	// Process each registered function
	for name, fn := range te.functions {
		placeholder := "{{" + name + "}}"
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fn(cc))
		}
	}

	return result
}

// AssemblyOptions configures the assembly process.
type AssemblyOptions struct {
	// IncludeSectionHeaders adds markdown headers
	IncludeSectionHeaders bool

	// MinifyWhitespace reduces unnecessary whitespace
	MinifyWhitespace bool

	// IncludeMetadata adds atom metadata as comments
	IncludeMetadata bool

	// MaxLength truncates at this character count (0 = no limit)
	MaxLength int
}

// DefaultAssemblyOptions returns sensible defaults.
func DefaultAssemblyOptions() AssemblyOptions {
	return AssemblyOptions{
		IncludeSectionHeaders: false,
		MinifyWhitespace:      false,
		IncludeMetadata:       false,
		MaxLength:             0,
	}
}

// AssembleWithOptions assembles with custom options.
func (a *FinalAssembler) AssembleWithOptions(
	atoms []*OrderedAtom,
	cc *CompilationContext,
	opts AssemblyOptions,
) (string, error) {
	// Temporarily modify assembler settings
	originalHeaders := a.addSectionHeaders
	a.addSectionHeaders = opts.IncludeSectionHeaders

	defer func() {
		a.addSectionHeaders = originalHeaders
	}()

	// Assemble
	prompt, err := a.Assemble(atoms, cc)
	if err != nil {
		return "", err
	}

	// Post-processing
	if opts.MinifyWhitespace {
		prompt = minifyWhitespace(prompt)
	}

	if opts.MaxLength > 0 && len(prompt) > opts.MaxLength {
		prompt = truncatePrompt(prompt, opts.MaxLength)
	}

	return prompt, nil
}

// minifyWhitespace reduces excessive whitespace while preserving structure.
func minifyWhitespace(content string) string {
	// Replace multiple newlines with double newlines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	// Trim trailing whitespace from lines
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}

// truncatePrompt truncates content at a sensible boundary.
func truncatePrompt(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	// Try to truncate at a paragraph boundary
	truncated := content[:maxLen]
	lastPara := strings.LastIndex(truncated, "\n\n")
	if lastPara > maxLen/2 {
		truncated = truncated[:lastPara]
	}

	return truncated + "\n\n[Content truncated due to length limits]"
}

// PromptStats returns statistics about an assembled prompt.
type PromptStats struct {
	CharCount       int
	TokenCount      int
	LineCount       int
	SectionCount    int
	AtomCount       int
	CategoryCounts  map[AtomCategory]int
	MandatoryCount  int
	LongestAtomLen  int
	ShortestAtomLen int
}

// AnalyzePrompt returns statistics about an assembled prompt.
func AnalyzePrompt(prompt string, atoms []*OrderedAtom) PromptStats {
	stats := PromptStats{
		CharCount:       len(prompt),
		TokenCount:      EstimateTokens(prompt),
		LineCount:       strings.Count(prompt, "\n") + 1,
		AtomCount:       len(atoms),
		CategoryCounts:  make(map[AtomCategory]int),
		ShortestAtomLen: -1,
	}

	for _, oa := range atoms {
		stats.CategoryCounts[oa.Atom.Category]++

		if oa.Atom.IsMandatory {
			stats.MandatoryCount++
		}

		atomLen := len(oa.Atom.Content)
		if atomLen > stats.LongestAtomLen {
			stats.LongestAtomLen = atomLen
		}
		if stats.ShortestAtomLen < 0 || atomLen < stats.ShortestAtomLen {
			stats.ShortestAtomLen = atomLen
		}
	}

	stats.SectionCount = len(stats.CategoryCounts)

	return stats
}
