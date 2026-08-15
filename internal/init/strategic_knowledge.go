// Package init implements the "nerd init" cold-start initialization system.
// This file adds deep strategic knowledge generation using LLM analysis.
package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tools/research"
	"codenerd/internal/world"
)

// StrategicKnowledge represents deep philosophical and architectural understanding
// of a codebase - the "soul" of the project that the main agent uses for reasoning.
type StrategicKnowledge struct {
	// Identity - What is this project at its core?
	ProjectVision    string   `json:"project_vision"`    // The "why" - purpose and goals
	CorePhilosophy   string   `json:"core_philosophy"`   // Guiding principles
	DesignPrinciples []string `json:"design_principles"` // Key architectural decisions

	// Architecture - How is it built?
	ArchitectureStyle string          `json:"architecture_style"` // e.g., "neuro-symbolic", "microservices"
	KeyComponents     []ComponentInfo `json:"key_components"`     // Major subsystems
	DataFlowPattern   string          `json:"data_flow_pattern"`  // How data moves through the system

	// Patterns - What patterns does it use?
	CorePatterns      []PatternInfo `json:"core_patterns"`      // Key design patterns
	CommunicationFlow string        `json:"communication_flow"` // How components communicate

	// Capabilities - What can it do?
	CoreCapabilities []string `json:"core_capabilities"` // Main features
	ExtensionPoints  []string `json:"extension_points"`  // Where it can be extended

	// Constraints - What are its boundaries?
	SafetyConstraints []string `json:"safety_constraints"` // Safety invariants
	Limitations       []string `json:"limitations"`        // Known limitations

	// Evolution - How does it grow?
	LearningMechanisms []string `json:"learning_mechanisms"` // How it adapts
	FutureDirections   []string `json:"future_directions"`   // Planned evolution
}

// ComponentInfo describes a major subsystem.
type ComponentInfo struct {
	Name       string   `json:"name"`
	Purpose    string   `json:"purpose"`
	Location   string   `json:"location"`   // Directory or package
	Interfaces string   `json:"interfaces"` // How it exposes functionality
	DependsOn  []string `json:"depends_on"` // What it needs
}

// PatternInfo describes a design pattern used in the codebase.
type PatternInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UsedIn      string `json:"used_in"` // Where it's applied
	Why         string `json:"why"`     // Why this pattern was chosen
}

// generateStrategicKnowledge uses LLM to analyze the codebase deeply.
func (i *Initializer) generateStrategicKnowledge(ctx context.Context, profile ProjectProfile, scanResult *world.ScanResult) (*StrategicKnowledge, error) {
	if i.config.LLMClient == nil {
		return nil, fmt.Errorf("LLM client required for strategic knowledge generation")
	}

	// Gather context about the project
	codebaseContext := i.buildCodebaseContext(profile, scanResult)

	// Gather ALL documentation, then use LLM to filter for relevance
	allDocs := i.GatherProjectDocumentation()
	relevantDocs := i.filterDocumentsByRelevance(ctx, allDocs)
	docContent := i.buildRelevantDocContent(relevantDocs)
	if docContent != "" {
		codebaseContext += "\n\n## Project Documentation (LLM-filtered for strategic relevance):\n" + docContent
	}
	logging.Get(logging.CategoryBoot).Debug("Strategic knowledge: %d total docs → %d relevant", len(allDocs), len(relevantDocs))

	prompt := fmt.Sprintf(`You are analyzing a software project to generate deep strategic knowledge.
This knowledge will be used by an AI coding agent to understand the project at a philosophical and architectural level.

## Project Context:
%s

## Task:
Generate a comprehensive strategic analysis of this codebase. Focus on:
1. The project's PURPOSE and PHILOSOPHY - why does it exist? what problem does it solve?
2. The ARCHITECTURE - how are the major components organized? what patterns are used?
3. The DATA FLOW - how does information move through the system?
4. The EXTENSION POINTS - where can the system be extended?
5. The SAFETY CONSTRAINTS - what invariants must be maintained?

Respond with a JSON object matching this structure:
{
  "project_vision": "string - the core purpose and goal of this project",
  "core_philosophy": "string - the guiding principles (e.g., 'Logic determines Reality; the Model merely describes it')",
  "design_principles": ["principle 1", "principle 2", ...],
  "architecture_style": "string - e.g., 'neuro-symbolic', 'microservices', 'monolith'",
  "key_components": [
    {"name": "Component", "purpose": "what it does", "location": "path", "interfaces": "how to use it", "depends_on": ["dep1"]}
  ],
  "data_flow_pattern": "string - how data flows through the system",
  "core_patterns": [
    {"name": "Pattern", "description": "what it is", "used_in": "where", "why": "why chosen"}
  ],
  "communication_flow": "string - how components communicate",
  "core_capabilities": ["capability 1", "capability 2", ...],
  "extension_points": ["extension 1", "extension 2", ...],
  "safety_constraints": ["constraint 1", "constraint 2", ...],
  "limitations": ["limitation 1", ...],
  "learning_mechanisms": ["mechanism 1", ...],
  "future_directions": ["direction 1", ...]
}

IMPORTANT: Be specific to THIS project, not generic. Extract real insights from the codebase structure.
`, codebaseContext)

	// Use grounded completion if Gemini grounding is available
	var response string
	var err error
	if i.grounding != nil && i.grounding.IsGroundingAvailable() {
		// Get documentation URLs for the project's tech stack
		var docURLs []string
		if profile.Language != "" {
			docURLs = append(docURLs, research.GetDocURLsForTech(profile.Language)...)
		}
		if profile.Framework != "" {
			docURLs = append(docURLs, research.GetDocURLsForTech(profile.Framework)...)
		}

		// Enable URL context if we have relevant doc URLs
		if len(docURLs) > 0 {
			i.grounding.EnableURLContext(docURLs)
		}

		response, err = i.withJITPrompt(ctx, "analysis", prompt, &profile, func(ctx context.Context, p string) (string, error) {
			resp, _, err := i.grounding.CompleteWithGrounding(ctx, p)
			return resp, err
		})

		// Capture grounding sources
		sources := i.grounding.CaptureGroundingSources()
		if len(sources) > 0 {
			i.mu.Lock()
			i.groundingSources = append(i.groundingSources, sources...)
			i.mu.Unlock()
			logging.Boot("Strategic knowledge grounded with %d sources", len(sources))
		}
	} else {
		response, err = i.withJITPrompt(ctx, "analysis", prompt, &profile, func(ctx context.Context, p string) (string, error) {
			return i.config.LLMClient.Complete(ctx, p)
		})
	}
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Parse JSON from response
	knowledge := &StrategicKnowledge{}

	// Extract JSON from response (may be wrapped in markdown code blocks)
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), knowledge); err != nil {
		// If parsing fails, create minimal knowledge from profile
		logging.Get(logging.CategoryBoot).Warn("Failed to parse strategic knowledge JSON, using fallback: %v", err)
		knowledge = i.createFallbackStrategicKnowledge(profile)
	}

	return knowledge, nil
}

// buildCodebaseContext creates a rich context string for LLM analysis.
func (i *Initializer) buildCodebaseContext(profile ProjectProfile, scanResult *world.ScanResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Project: %s\n", profile.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n", profile.Description))
	sb.WriteString(fmt.Sprintf("Language: %s\n", profile.Language))
	if profile.Framework != "" {
		sb.WriteString(fmt.Sprintf("Framework: %s\n", profile.Framework))
	}
	if profile.Architecture != "" {
		sb.WriteString(fmt.Sprintf("Architecture: %s\n", profile.Architecture))
	}

	// Add directory structure (extract from facts)
	sb.WriteString("\n## Directory Structure:\n")
	if scanResult != nil && len(scanResult.Facts) > 0 {
		dirs := extractDirectoriesFromFacts(scanResult.Facts)
		for _, dir := range dirs[:min(30, len(dirs))] {
			sb.WriteString(fmt.Sprintf("- %s\n", dir))
		}
	}

	// Add entry points
	if len(profile.EntryPoints) > 0 {
		sb.WriteString("\n## Entry Points:\n")
		for _, ep := range profile.EntryPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", ep))
		}
	}

	// Add dependencies
	if len(profile.Dependencies) > 0 {
		sb.WriteString("\n## Key Dependencies:\n")
		for _, dep := range profile.Dependencies[:min(20, len(profile.Dependencies))] {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", dep.Name, dep.Type))
		}
	}

	// Add patterns if detected
	if len(profile.Patterns) > 0 {
		sb.WriteString("\n## Detected Patterns:\n")
		for _, pattern := range profile.Patterns {
			sb.WriteString(fmt.Sprintf("- %s\n", pattern))
		}
	}

	return sb.String()
}

// keysFromMap extracts keys from a map for display.
func keysFromMap(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GatherProjectDocumentation discovers ALL documentation files in the workspace.
// It does NOT apply arbitrary limits - the LLM will analyze and filter for relevance.
// Uses ResearcherShard patterns: signal keywords, heuristic sniffing, priority ordering.
func (i *Initializer) GatherProjectDocumentation() []DocumentInfo {
	var docs []DocumentInfo
	seen := make(map[string]bool)

	// Priority files (highest importance)
	priorityFiles := map[string]int{
		"CLAUDE.md":       0,
		"README.md":       1,
		"ARCHITECTURE.md": 1,
		"DESIGN.md":       1,
		"VISION.md":       1,
		"PHILOSOPHY.md":   1,
		"CONTRIBUTING.md": 2,
		"CHANGELOG.md":    2,
		"ROADMAP.md":      2,
		"GOALS.md":        2,
		"STRATEGY.md":     2,
		"API.md":          2,
	}

	// Target directories (ResearcherShard pattern)
	targetDirs := map[string]bool{
		"docs":          true,
		"doc":           true,
		"documentation": true,
		"spec":          true,
		"specs":         true,
		"planning":      true,
		"design":        true,
		"research":      true,
		"analysis":      true,
		"architecture":  true,
		".github":       true,
		".claude":       true,
	}

	// Signal keywords for heuristic content sniffing (ResearcherShard pattern)
	signalKeywords := []string{
		"Vision", "Philosophy", "Architecture", "Design", "Strategy", "Roadmap",
		"Goals", "Objectives", "Specification", "Overview", "Introduction",
		"Core Concept", "Principle", "Pattern", "Guideline", "Convention",
		"Integration", "Workflow", "How it works", "Getting started",
	}

	// Skip directories (noise)
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true,
		"dist": true, "build": true, "__pycache__": true,
		"target": true, ".next": true, "coverage": true,
		".vscode": true, ".idea": true,
	}

	// Walk entire workspace - no depth limit
	err := filepath.Walk(i.config.Workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			name := info.Name()
			// Skip noise directories
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// Skip hidden dirs except .github and .claude
			if strings.HasPrefix(name, ".") && name != "." && name != ".github" && name != ".claude" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process documentation files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" && ext != ".txt" && ext != ".rst" && ext != ".pdf" {
			return nil
		}

		absPath, _ := filepath.Abs(path)
		if seen[absPath] {
			return nil
		}
		seen[absPath] = true

		relPath, _ := filepath.Rel(i.config.Workspace, path)
		if relPath == "" {
			relPath = info.Name()
		}

		// Read content
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 {
			return nil
		}

		// Determine priority
		priority := 3 // Default: other

		// Check if it's a priority file
		for pFile, pVal := range priorityFiles {
			if strings.EqualFold(filepath.Base(path), pFile) {
				priority = pVal
				break
			}
		}

		// Check if in target directory
		if priority == 3 {
			parts := strings.SplitSeq(relPath, string(os.PathSeparator))
			for part := range parts {
				if targetDirs[strings.ToLower(part)] {
					priority = 2
					break
				}
			}
		}

		// Heuristic content sniffing for non-priority files
		if priority == 3 && ext == ".md" {
			header := string(content)
			if len(header) > 2000 {
				header = header[:2000]
			}
			for _, signal := range signalKeywords {
				if strings.Contains(header, "# "+signal) ||
					strings.Contains(header, "## "+signal) ||
					strings.Contains(header, signal+":") {
					priority = 2
					break
				}
			}
		}

		// Extract title from first heading
		title := filepath.Base(path)
		lines := strings.SplitSeq(string(content), "\n")
		for line := range lines {
			if after, ok := strings.CutPrefix(line, "# "); ok {
				title = strings.TrimSpace(after)
				break
			}
		}

		docs = append(docs, DocumentInfo{
			Path:     relPath,
			AbsPath:  absPath,
			Content:  string(content),
			Title:    title,
			Size:     len(content),
			Priority: priority,
		})

		return nil
	})

	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Error walking workspace for docs: %v", err)
	}

	// Sort by priority (lower = more important)
	for i := 0; i < len(docs)-1; i++ {
		for j := i + 1; j < len(docs); j++ {
			if docs[j].Priority < docs[i].Priority {
				docs[i], docs[j] = docs[j], docs[i]
			}
		}
	}

	logging.Get(logging.CategoryBoot).Debug("Discovered %d documentation files", len(docs))
	return docs
}

// filterDocumentsByRelevance uses LLM to analyze which documents are relevant
// to understanding the codebase's strategic nature vs noise.
func (i *Initializer) filterDocumentsByRelevance(ctx context.Context, docs []DocumentInfo) []DocumentInfo {
	if i.config.LLMClient == nil || len(docs) == 0 {
		// No LLM available - return high priority docs only
		var filtered []DocumentInfo
		for _, doc := range docs {
			if doc.Priority <= 2 {
				doc.IsRelevant = true
				doc.Reasoning = "High priority file (no LLM filtering available)"
				filtered = append(filtered, doc)
			}
		}
		return filtered
	}

	// Process in batches to handle large doc counts.
	//
	// Batches run CONCURRENTLY. This loop used to be sequential, which made cold
	// start impossible on a large repo: ~1960 docs is 196 batches, and at the
	// ~16s per call an API model actually takes that is ~54 minutes against a
	// 25-minute operation timeout. Every batch is independent — the LLM only
	// labels its own ten documents — so there was never a reason to serialize.
	// Real API concurrency is still bounded by the APIScheduler
	// (core_limits.max_concurrent_api_calls); the pool below just avoids
	// spawning a goroutine per batch.
	const batchSize = 10
	const maxParallelBatches = 8

	type batchRange struct{ start, end int }
	var ranges []batchRange
	for batchStart := 0; batchStart < len(docs); batchStart += batchSize {
		ranges = append(ranges, batchRange{batchStart, min(batchStart+batchSize, len(docs))})
	}

	// Results are collected per batch and concatenated in batch order, so the
	// relevance-ordered output does not depend on completion order.
	perBatch := make([][]DocumentInfo, len(ranges))
	sem := make(chan struct{}, maxParallelBatches)
	var wg sync.WaitGroup

	for bi, r := range ranges {
		wg.Add(1)
		go func(bi int, r batchRange) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// Each goroutine owns a disjoint slice range of docs, so the
			// IsRelevant/Reasoning writes below do not race.
			perBatch[bi] = i.analyzeDocBatch(ctx, docs[r.start:r.end])
		}(bi, r)
	}
	wg.Wait()

	var relevant []DocumentInfo
	for _, b := range perBatch {
		relevant = append(relevant, b...)
	}

	logging.Get(logging.CategoryBoot).Debug("LLM filtered %d docs → %d relevant", len(docs), len(relevant))
	return relevant
}

// analyzeDocBatch asks the LLM which of one batch's documents are strategically
// relevant, and returns those. Any failure (LLM error, unparseable response)
// degrades to the priority heuristic for that batch alone.
func (i *Initializer) analyzeDocBatch(ctx context.Context, batch []DocumentInfo) []DocumentInfo {
	var relevant []DocumentInfo
	// priorityFallback keeps the batch's high-priority docs when the LLM cannot
	// be consulted or its answer cannot be read.
	priorityFallback := func(reason string) []DocumentInfo {
		var out []DocumentInfo
		for _, doc := range batch {
			if doc.Priority <= 2 {
				doc.IsRelevant = true
				doc.Reasoning = reason
				out = append(out, doc)
			}
		}
		return out
	}
	{
		// Build analysis prompt
		var docList strings.Builder
		for idx, doc := range batch {
			// Include path, title, and first 500 chars as preview
			preview := doc.Content
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			docList.WriteString(fmt.Sprintf("\n[%d] %s\nTitle: %s\nSize: %d bytes\nPreview:\n%s\n---\n",
				idx, doc.Path, doc.Title, doc.Size, preview))
		}

		prompt := fmt.Sprintf(`You are analyzing documentation files to determine which are strategically relevant for understanding a codebase.

STRATEGIC DOCUMENTATION includes:
- Project vision, philosophy, core principles
- Architecture decisions and patterns
- Design rationale and trade-offs
- Key component descriptions
- Integration patterns and workflows
- Safety constraints and invariants

NOISE includes:
- Auto-generated docs (API references, package listings)
- Changelog entries without architectural context
- Meeting notes without conclusions
- Duplicate or superseded documentation
- License files, boilerplate

## Documents to Analyze:
%s

## Task:
For each document [N], respond with a JSON array:
[
  {"index": 0, "relevant": true/false, "reason": "brief explanation"},
  ...
]

Be selective - only mark as relevant documents that provide genuine strategic insight.
Prefer fewer, high-quality documents over including everything.
`, docList.String())

		// Use grounded completion if available
		var response string
		var err error
		if i.grounding != nil && i.grounding.IsGroundingAvailable() {
			response, err = i.withJITPrompt(ctx, "analysis", prompt, nil, func(ctx context.Context, p string) (string, error) {
				resp, _, err := i.grounding.CompleteWithGrounding(ctx, p)
				return resp, err
			})
			// Capture grounding sources
			sources := i.grounding.CaptureGroundingSources()
			if len(sources) > 0 {
				i.mu.Lock()
				i.groundingSources = append(i.groundingSources, sources...)
				i.mu.Unlock()
			}
		} else {
			response, err = i.withJITPrompt(ctx, "analysis", prompt, nil, func(ctx context.Context, p string) (string, error) {
				return i.config.LLMClient.Complete(ctx, p)
			})
		}
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("LLM relevance filtering failed for batch: %v", err)
			return priorityFallback("Fallback: high priority (LLM error)")
		}

		// Parse response
		type RelevanceResult struct {
			Index    int    `json:"index"`
			Relevant bool   `json:"relevant"`
			Reason   string `json:"reason"`
		}
		var results []RelevanceResult

		// Extract JSON from response
		jsonStr := extractJSON(response)
		if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
			logging.Get(logging.CategoryBoot).Debug("Failed to parse relevance JSON: %v", err)
			return priorityFallback("Fallback: high priority (parse error)")
		}

		// Apply results
		for _, result := range results {
			if result.Index >= 0 && result.Index < len(batch) {
				batch[result.Index].IsRelevant = result.Relevant
				batch[result.Index].Reasoning = result.Reason
				if result.Relevant {
					relevant = append(relevant, batch[result.Index])
				}
			}
		}
	}

	return relevant
}

// buildRelevantDocContent formats the relevant documents for strategic analysis.
func (i *Initializer) buildRelevantDocContent(docs []DocumentInfo) string {
	if len(docs) == 0 {
		return ""
	}

	var sb strings.Builder
	totalChars := 0
	const softLimit = 100000 // Soft limit for LLM context - but include all relevant

	for _, doc := range docs {
		sb.WriteString(fmt.Sprintf("\n### %s\n", doc.Path))
		if doc.Reasoning != "" {
			sb.WriteString(fmt.Sprintf("*Relevance: %s*\n\n", doc.Reasoning))
		}
		sb.WriteString(doc.Content)
		sb.WriteString("\n")
		totalChars += len(doc.Content)
	}

	if totalChars > softLimit {
		logging.Get(logging.CategoryBoot).Debug(
			"Relevant docs exceed soft limit (%d chars > %d) but including all %d docs",
			totalChars, softLimit, len(docs))
	}

	return sb.String()
}

// createFallbackStrategicKnowledge creates minimal knowledge when LLM fails.
func (i *Initializer) createFallbackStrategicKnowledge(profile ProjectProfile) *StrategicKnowledge {
	return &StrategicKnowledge{
		ProjectVision:     profile.Description,
		CorePhilosophy:    fmt.Sprintf("A %s project built with %s.", profile.Language, profile.Framework),
		DesignPrinciples:  profile.Patterns,
		ArchitectureStyle: profile.Architecture,
		KeyComponents:     []ComponentInfo{},
		DataFlowPattern:   "Standard request-response flow",
		CorePatterns:      []PatternInfo{},
		CoreCapabilities:  []string{},
		SafetyConstraints: []string{},
		Limitations:       []string{},
	}
}

// PersistStrategicKnowledge saves the knowledge to the main knowledge.db.
// Uses embedding-enabled storage for semantic search capability.
func (i *Initializer) PersistStrategicKnowledge(ctx context.Context, knowledge *StrategicKnowledge, db *store.LocalStore) (int, error) {
	atomCount := 0

	// Helper to store with embedding for semantic search
	storeAtom := func(concept, content string, confidence float64) {
		if content == "" {
			return
		}
		if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, content, confidence); err == nil {
			atomCount++
		} else {
			logging.Get(logging.CategoryBoot).Debug("Failed to store atom %s: %v", concept, err)
		}
	}

	// Store core identity (highest confidence)
	storeAtom("strategic/vision", knowledge.ProjectVision, 1.0)
	storeAtom("strategic/philosophy", knowledge.CorePhilosophy, 1.0)
	storeAtom("strategic/architecture_style", knowledge.ArchitectureStyle, 0.95)
	storeAtom("strategic/data_flow", knowledge.DataFlowPattern, 0.95)
	storeAtom("strategic/communication", knowledge.CommunicationFlow, 0.95)

	// Store design principles
	for _, principle := range knowledge.DesignPrinciples {
		storeAtom("strategic/principle", principle, 0.9)
	}

	// Store components
	for _, comp := range knowledge.KeyComponents {
		content := fmt.Sprintf("%s: %s (location: %s, interfaces: %s)",
			comp.Name, comp.Purpose, comp.Location, comp.Interfaces)
		storeAtom("strategic/component", content, 0.9)
	}

	// Store patterns
	for _, pattern := range knowledge.CorePatterns {
		content := fmt.Sprintf("%s: %s. Used in: %s. Why: %s",
			pattern.Name, pattern.Description, pattern.UsedIn, pattern.Why)
		storeAtom("strategic/pattern", content, 0.9)
	}

	// Store capabilities
	for _, cap := range knowledge.CoreCapabilities {
		storeAtom("strategic/capability", cap, 0.85)
	}

	// Store extension points
	for _, ext := range knowledge.ExtensionPoints {
		storeAtom("strategic/extension_point", ext, 0.85)
	}

	// Store safety constraints (high confidence - these are critical)
	for _, constraint := range knowledge.SafetyConstraints {
		storeAtom("strategic/safety_constraint", constraint, 0.95)
	}

	// Store limitations
	for _, limit := range knowledge.Limitations {
		storeAtom("strategic/limitation", limit, 0.8)
	}

	// Store learning mechanisms
	for _, mech := range knowledge.LearningMechanisms {
		storeAtom("strategic/learning", mech, 0.85)
	}

	// Store future directions
	for _, dir := range knowledge.FutureDirections {
		storeAtom("strategic/future", dir, 0.7)
	}

	// Also persist as JSON for easy loading
	jsonBytes, _ := json.MarshalIndent(knowledge, "", "  ")
	storeAtom("strategic/full_knowledge", string(jsonBytes), 1.0)

	return atomCount, nil
}

// extractJSON extracts JSON from a string that might have markdown code blocks.
func extractJSON(s string) string {
	// Try to find JSON in code blocks first
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + 3
		// Skip optional language identifier
		if nlIdx := strings.Index(s[start:], "\n"); nlIdx != -1 {
			start += nlIdx + 1
		}
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}

	if value := extractBalancedJSON(s); value != "" {
		return value
	}
	return s
}

// extractBalancedJSON returns the first complete JSON object or array in s.
//
// The previous version only looked for '{' and counted braces without regard
// for string literals, which broke both callers of this function:
//
//   - analyzeDocBatch asks for a JSON *array* of relevance verdicts. An
//     unfenced `[{...},{...}]` reply made this return just the first object,
//     the unmarshal into []RelevanceResult failed, and the whole batch silently
//     fell back to the priority heuristic — so on any provider that answers
//     without a code fence, LLM document filtering never actually ran.
//   - generateStrategicKnowledge asks for an object whose values are prose. A
//     '}' inside any string value (a Mangle snippet, a brace in a description)
//     closed the object early and the parse failed, discarding the analysis in
//     favour of the profile-only fallback.
func extractBalancedJSON(s string) string {
	start := -1
	var openCh, closeCh byte
	for idx := 0; idx < len(s); idx++ {
		if s[idx] == '{' {
			start, openCh, closeCh = idx, '{', '}'
			break
		}
		if s[idx] == '[' {
			start, openCh, closeCh = idx, '[', ']'
			break
		}
	}
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for idx := start; idx < len(s); idx++ {
		c := s[idx]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case openCh:
			depth++
		case closeCh:
			depth--
			if depth == 0 {
				return s[start : idx+1]
			}
		}
	}
	return ""
}

// extractDirectoriesFromFacts extracts directory paths from file_topology facts.
func extractDirectoriesFromFacts(facts []core.Fact) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) >= 2 {
			// file_topology(path, type) where type is /directory
			if typeArg, ok := f.Args[1].(string); ok && typeArg == "/directory" {
				if path, ok := f.Args[0].(string); ok && !seen[path] {
					seen[path] = true
					dirs = append(dirs, path)
				}
			}
		}
	}
	return dirs
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
