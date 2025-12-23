// Package init implements the "nerd init" cold-start initialization system.
// This file adds deep strategic knowledge generation using LLM analysis.
package init

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

// StrategicKnowledge represents deep philosophical and architectural understanding
// of a codebase - the "soul" of the project that the main agent uses for reasoning.
type StrategicKnowledge struct {
	// Identity - What is this project at its core?
	ProjectVision     string   `json:"project_vision"`      // The "why" - purpose and goals
	CorePhilosophy    string   `json:"core_philosophy"`     // Guiding principles
	DesignPrinciples  []string `json:"design_principles"`   // Key architectural decisions

	// Architecture - How is it built?
	ArchitectureStyle string            `json:"architecture_style"`  // e.g., "neuro-symbolic", "microservices"
	KeyComponents     []ComponentInfo   `json:"key_components"`      // Major subsystems
	DataFlowPattern   string            `json:"data_flow_pattern"`   // How data moves through the system

	// Patterns - What patterns does it use?
	CorePatterns      []PatternInfo     `json:"core_patterns"`       // Key design patterns
	CommunicationFlow string            `json:"communication_flow"`  // How components communicate

	// Capabilities - What can it do?
	CoreCapabilities  []string          `json:"core_capabilities"`   // Main features
	ExtensionPoints   []string          `json:"extension_points"`    // Where it can be extended

	// Constraints - What are its boundaries?
	SafetyConstraints []string          `json:"safety_constraints"`  // Safety invariants
	Limitations       []string          `json:"limitations"`         // Known limitations

	// Evolution - How does it grow?
	LearningMechanisms []string         `json:"learning_mechanisms"` // How it adapts
	FutureDirections   []string         `json:"future_directions"`   // Planned evolution
}

// ComponentInfo describes a major subsystem.
type ComponentInfo struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose"`
	Location    string `json:"location"`     // Directory or package
	Interfaces  string `json:"interfaces"`   // How it exposes functionality
	DependsOn   []string `json:"depends_on"` // What it needs
}

// PatternInfo describes a design pattern used in the codebase.
type PatternInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UsedIn      string `json:"used_in"`      // Where it's applied
	Why         string `json:"why"`          // Why this pattern was chosen
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

	response, err := i.config.LLMClient.Complete(ctx, prompt)
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

// DocumentInfo represents a discovered documentation file with metadata.
type DocumentInfo struct {
	Path        string // Relative path from workspace
	AbsPath     string // Absolute path
	Content     string // Full file content
	Title       string // Extracted title (from first # heading or filename)
	Size        int    // Content size in bytes
	Priority    int    // 0=highest priority (CLAUDE.md), 1=high (README), 2=docs folder, 3=other
	IsRelevant  bool   // Set by LLM analysis
	Reasoning   string // Why LLM marked it relevant/noise
	ContentHash string // SHA256 hash for deduplication and change detection
}

// DocProcessingStatus tracks the state of each document through the pipeline.
// Uses Mangle atoms for deterministic tracking (campaign pattern).
type DocProcessingStatus string

const (
	DocStatusDiscovered DocProcessingStatus = "/discovered" // Found during scan
	DocStatusAnalyzing  DocProcessingStatus = "/analyzing"  // LLM analyzing relevance
	DocStatusExtracting DocProcessingStatus = "/extracting" // Extracting knowledge atoms
	DocStatusStored     DocProcessingStatus = "/stored"     // Atoms persisted to DB
	DocStatusSynthesized DocProcessingStatus = "/synthesized" // Included in synthesis
	DocStatusSkipped    DocProcessingStatus = "/skipped"    // Not relevant
	DocStatusFailed     DocProcessingStatus = "/failed"     // Processing failed
)

// DocIngestionState tracks the entire ingestion campaign state.
// Persisted to .nerd/doc_ingestion_state.json for resumption.
type DocIngestionState struct {
	CampaignID      string                         `json:"campaign_id"`
	StartedAt       time.Time                      `json:"started_at"`
	LastUpdated     time.Time                      `json:"last_updated"`
	Phase           string                         `json:"phase"` // "discovery", "analysis", "extraction", "synthesis"
	Documents       map[string]*DocProcessingEntry `json:"documents"`
	TotalDiscovered int                            `json:"total_discovered"`
	TotalProcessed  int                            `json:"total_processed"`
	TotalStored     int                            `json:"total_stored"`
	SynthesisReady  bool                           `json:"synthesis_ready"`
}

// DocProcessingEntry tracks individual document processing state.
type DocProcessingEntry struct {
	Path         string              `json:"path"`
	Title        string              `json:"title"`
	ContentHash  string              `json:"content_hash"`
	Status       DocProcessingStatus `json:"status"`
	Priority     int                 `json:"priority"`
	IsRelevant   bool                `json:"is_relevant"`
	Reasoning    string              `json:"reasoning"`
	AtomsStored  int                 `json:"atoms_stored"`
	ProcessedAt  *time.Time          `json:"processed_at,omitempty"`
	ErrorMessage string              `json:"error_message,omitempty"`
}

// assertDocFact asserts a document tracking fact to the kernel.
// Pattern: doc_<status>(path, hash, timestamp)
func (i *Initializer) assertDocFact(kernel *core.RealKernel, status DocProcessingStatus, path, hash string) {
	if kernel == nil {
		return
	}
	fact := core.Fact{
		Predicate: "doc_ingestion",
		Args:      []interface{}{path, string(status), hash, time.Now().Unix()},
	}
	if err := kernel.Assert(fact); err != nil {
		logging.Get(logging.CategoryBoot).Debug("Failed to assert doc fact: %v", err)
	}
}

// computeDocHash generates a SHA256 hash for document content deduplication.
func computeDocHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16] // First 16 chars sufficient
}

// loadIngestionState loads previous ingestion state for resumption.
func (i *Initializer) loadIngestionState() *DocIngestionState {
	statePath := filepath.Join(i.config.Workspace, ".nerd", "doc_ingestion_state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil
	}
	var state DocIngestionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

// saveIngestionState persists ingestion state for resumption.
func (i *Initializer) saveIngestionState(state *DocIngestionState) error {
	state.LastUpdated = time.Now()
	statePath := filepath.Join(i.config.Workspace, ".nerd", "doc_ingestion_state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0644)
}

// ProcessDocumentsWithTracking processes documents with Mangle fact tracking
// and incremental knowledge persistence. Uses campaign patterns:
// 1. Assert doc_discovered for each found doc
// 2. Assert doc_analyzing when LLM analyzes
// 3. Store atoms incrementally and assert doc_stored
// 4. Track progress for resumption
func (i *Initializer) ProcessDocumentsWithTracking(
	ctx context.Context,
	docs []DocumentInfo,
	db *store.LocalStore,
	kernel *core.RealKernel,
) (*DocIngestionState, error) {
	// Load or create ingestion state
	state := i.loadIngestionState()
	if state == nil {
		state = &DocIngestionState{
			CampaignID: fmt.Sprintf("doc_init_%d", time.Now().Unix()),
			StartedAt:  time.Now(),
			Phase:      "discovery",
			Documents:  make(map[string]*DocProcessingEntry),
		}
	}

	// Phase 1: Discovery - assert all discovered docs
	for _, doc := range docs {
		hash := computeDocHash(doc.Content)
		doc.ContentHash = hash

		// Check if already processed (for resumption)
		if existing, ok := state.Documents[doc.Path]; ok {
			if existing.ContentHash == hash && existing.Status == DocStatusStored {
				logging.Get(logging.CategoryBoot).Debug("Skipping already processed: %s", doc.Path)
				continue
			}
		}

		// Assert discovery fact
		i.assertDocFact(kernel, DocStatusDiscovered, doc.Path, hash)

		state.Documents[doc.Path] = &DocProcessingEntry{
			Path:        doc.Path,
			Title:       doc.Title,
			ContentHash: hash,
			Status:      DocStatusDiscovered,
			Priority:    doc.Priority,
		}
		state.TotalDiscovered++
	}
	state.Phase = "analysis"
	i.saveIngestionState(state)

	// Phase 2: LLM Analysis - filter for relevance
	relevantDocs := i.filterDocumentsByRelevance(ctx, docs)
	for _, doc := range relevantDocs {
		if entry, ok := state.Documents[doc.Path]; ok {
			entry.IsRelevant = true
			entry.Reasoning = doc.Reasoning
			entry.Status = DocStatusAnalyzing
			i.assertDocFact(kernel, DocStatusAnalyzing, doc.Path, entry.ContentHash)
		}
	}
	// Mark non-relevant as skipped
	relevantPaths := make(map[string]bool)
	for _, doc := range relevantDocs {
		relevantPaths[doc.Path] = true
	}
	for path, entry := range state.Documents {
		if !relevantPaths[path] && entry.Status == DocStatusDiscovered {
			entry.Status = DocStatusSkipped
			entry.Reasoning = "Not relevant based on LLM analysis"
			i.assertDocFact(kernel, DocStatusSkipped, path, entry.ContentHash)
		}
	}
	state.Phase = "extraction"
	i.saveIngestionState(state)

	// Phase 3: Extraction - process each relevant doc and store atoms incrementally
	for _, doc := range relevantDocs {
		select {
		case <-ctx.Done():
			return state, ctx.Err()
		default:
		}

		entry := state.Documents[doc.Path]
		if entry.Status == DocStatusStored {
			continue // Already processed
		}

		entry.Status = DocStatusExtracting
		i.assertDocFact(kernel, DocStatusExtracting, doc.Path, entry.ContentHash)

		// Extract and store knowledge atoms incrementally
		atomCount, err := i.extractAndStoreDocKnowledge(ctx, doc, db)
		if err != nil {
			entry.Status = DocStatusFailed
			entry.ErrorMessage = err.Error()
			i.assertDocFact(kernel, DocStatusFailed, doc.Path, entry.ContentHash)
			logging.Get(logging.CategoryBoot).Warn("Failed to extract from %s: %v", doc.Path, err)
			continue
		}

		now := time.Now()
		entry.Status = DocStatusStored
		entry.AtomsStored = atomCount
		entry.ProcessedAt = &now
		state.TotalStored++
		i.assertDocFact(kernel, DocStatusStored, doc.Path, entry.ContentHash)

		// Save state after each doc for resumption
		i.saveIngestionState(state)
		logging.Get(logging.CategoryBoot).Debug("Stored %d atoms from %s", atomCount, doc.Path)
	}

	state.TotalProcessed = len(relevantDocs)
	state.SynthesisReady = true
	state.Phase = "synthesis"
	i.saveIngestionState(state)

	return state, nil
}

// extractAndStoreDocKnowledge uses LLM to extract knowledge atoms from a document
// and stores them incrementally to the database.
func (i *Initializer) extractAndStoreDocKnowledge(
	ctx context.Context,
	doc DocumentInfo,
	db *store.LocalStore,
) (int, error) {
	if i.config.LLMClient == nil || db == nil {
		return 0, fmt.Errorf("LLM client and database required")
	}

	// For large docs, chunk and process
	content := doc.Content
	chunks := chunkDocument(content, 8000) // ~2k tokens per chunk

	atomCount := 0
	for chunkIdx, chunk := range chunks {
		prompt := fmt.Sprintf(`Extract key knowledge atoms from this documentation chunk.

Document: %s (chunk %d/%d)
Title: %s

Content:
%s

Extract the following as JSON array:
[
  {"concept": "category/specific_topic", "content": "key insight or fact", "confidence": 0.0-1.0}
]

Categories to use:
- "architecture/..." for structural patterns
- "philosophy/..." for design principles
- "pattern/..." for recurring patterns
- "capability/..." for system capabilities
- "constraint/..." for limitations or invariants
- "integration/..." for how components connect

Be specific and extract only genuinely useful insights. Skip boilerplate.
`, doc.Path, chunkIdx+1, len(chunks), doc.Title, chunk)

		response, err := i.config.LLMClient.Complete(ctx, prompt)
		if err != nil {
			logging.Get(logging.CategoryBoot).Debug("LLM extraction failed for chunk %d: %v", chunkIdx, err)
			continue
		}

		// Parse atoms from response
		type ExtractedAtom struct {
			Concept    string  `json:"concept"`
			Content    string  `json:"content"`
			Confidence float64 `json:"confidence"`
		}
		var atoms []ExtractedAtom

		jsonStr := extractJSON(response)
		if err := json.Unmarshal([]byte(jsonStr), &atoms); err != nil {
			// Fallback: store the whole chunk as a single atom (with embedding for semantic search)
			if err := db.StoreKnowledgeAtomWithEmbedding(
				ctx,
				fmt.Sprintf("doc/%s", doc.Path),
				chunk,
				0.7,
			); err == nil {
				atomCount++
			}
			continue
		}

		// Store each extracted atom (with embedding for semantic search)
		for _, atom := range atoms {
			concept := fmt.Sprintf("doc/%s/%s", doc.Path, atom.Concept)
			if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, atom.Content, atom.Confidence); err == nil {
				atomCount++
			}
		}
	}

	return atomCount, nil
}

// chunkDocument splits a document into chunks for processing.
func chunkDocument(content string, maxChars int) []string {
	if len(content) <= maxChars {
		return []string{content}
	}

	var chunks []string
	lines := strings.Split(content, "\n")
	var currentChunk strings.Builder

	for _, line := range lines {
		if currentChunk.Len()+len(line)+1 > maxChars {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
			}
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// SynthesizeFromStoredAtoms performs a second pass over stored knowledge atoms
// to create the final strategic knowledge synthesis.
func (i *Initializer) SynthesizeFromStoredAtoms(
	ctx context.Context,
	db *store.LocalStore,
	state *DocIngestionState,
) (*StrategicKnowledge, error) {
	if i.config.LLMClient == nil || db == nil {
		return nil, fmt.Errorf("LLM client and database required for synthesis")
	}

	// Query all stored doc atoms from DB
	atoms, err := db.GetKnowledgeAtomsByPrefix("doc/")
	if err != nil {
		return nil, fmt.Errorf("failed to query stored atoms: %w", err)
	}

	if len(atoms) == 0 {
		return nil, fmt.Errorf("no atoms found for synthesis")
	}

	// Build synthesis context from stored atoms
	var atomsSummary strings.Builder
	atomsSummary.WriteString("## Extracted Knowledge Atoms\n\n")

	// Group atoms by concept category
	categories := make(map[string][]string)
	for _, atom := range atoms {
		parts := strings.SplitN(atom.Concept, "/", 3)
		category := "other"
		if len(parts) >= 2 {
			category = parts[1] // e.g., "architecture", "philosophy"
		}
		categories[category] = append(categories[category], atom.Content)
	}

	for category, contents := range categories {
		atomsSummary.WriteString(fmt.Sprintf("### %s\n", category))
		for _, content := range contents {
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			atomsSummary.WriteString(fmt.Sprintf("- %s\n", content))
		}
		atomsSummary.WriteString("\n")
	}

	// Synthesis prompt
	prompt := fmt.Sprintf(`You are synthesizing extracted knowledge into strategic understanding.

## Processing Stats
- Documents processed: %d
- Knowledge atoms extracted: %d
- Categories: %v

%s

## Task
Synthesize these atoms into a coherent strategic knowledge structure.
Focus on:
1. The overarching PROJECT VISION and PHILOSOPHY
2. Key ARCHITECTURE patterns and decisions
3. Core CAPABILITIES and how they interconnect
4. Important CONSTRAINTS and safety invariants
5. How components COMMUNICATE and integrate

Respond with JSON matching this structure:
{
  "project_vision": "synthesized vision statement",
  "core_philosophy": "guiding principles",
  "design_principles": ["principle 1", ...],
  "architecture_style": "style name",
  "key_components": [{"name": "...", "purpose": "...", "location": "...", "interfaces": "...", "depends_on": [...]}],
  "data_flow_pattern": "how data moves",
  "core_patterns": [{"name": "...", "description": "...", "used_in": "...", "why": "..."}],
  "communication_flow": "how components communicate",
  "core_capabilities": ["capability 1", ...],
  "extension_points": ["extension 1", ...],
  "safety_constraints": ["constraint 1", ...],
  "limitations": ["limitation 1", ...],
  "learning_mechanisms": ["mechanism 1", ...],
  "future_directions": ["direction 1", ...]
}
`, state.TotalProcessed, len(atoms), keysFromMap(categories), atomsSummary.String())

	response, err := i.config.LLMClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("synthesis LLM call failed: %w", err)
	}

	// Parse synthesis result
	knowledge := &StrategicKnowledge{}
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), knowledge); err != nil {
		return nil, fmt.Errorf("failed to parse synthesis: %w", err)
	}

	// Mark documents as synthesized
	for path, entry := range state.Documents {
		if entry.Status == DocStatusStored {
			entry.Status = DocStatusSynthesized
			i.assertDocFact(nil, DocStatusSynthesized, path, entry.ContentHash)
		}
	}
	state.Phase = "completed"
	i.saveIngestionState(state)

	return knowledge, nil
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
		"CLAUDE.md":             0,
		"README.md":             1,
		"ARCHITECTURE.md":       1,
		"DESIGN.md":             1,
		"VISION.md":             1,
		"PHILOSOPHY.md":         1,
		"CONTRIBUTING.md":       2,
		"CHANGELOG.md":          2,
		"ROADMAP.md":            2,
		"GOALS.md":              2,
		"STRATEGY.md":           2,
		"API.md":                2,
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
		if ext != ".md" && ext != ".mdx" && ext != ".txt" && ext != ".rst" {
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
			parts := strings.Split(relPath, string(os.PathSeparator))
			for _, part := range parts {
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
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
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

	// Process in batches to handle large doc counts
	const batchSize = 10
	var relevant []DocumentInfo

	for batchStart := 0; batchStart < len(docs); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(docs) {
			batchEnd = len(docs)
		}
		batch := docs[batchStart:batchEnd]

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

		response, err := i.config.LLMClient.Complete(ctx, prompt)
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("LLM relevance filtering failed for batch: %v", err)
			// On error, include priority docs
			for _, doc := range batch {
				if doc.Priority <= 2 {
					doc.IsRelevant = true
					doc.Reasoning = "Fallback: high priority (LLM error)"
					relevant = append(relevant, doc)
				}
			}
			continue
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
			// Fallback to priority filtering
			for _, doc := range batch {
				if doc.Priority <= 2 {
					doc.IsRelevant = true
					doc.Reasoning = "Fallback: high priority (parse error)"
					relevant = append(relevant, doc)
				}
			}
			continue
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

	logging.Get(logging.CategoryBoot).Debug("LLM filtered %d docs → %d relevant", len(docs), len(relevant))
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
		ProjectVision:    profile.Description,
		CorePhilosophy:   fmt.Sprintf("A %s project built with %s.", profile.Language, profile.Framework),
		DesignPrinciples: profile.Patterns,
		ArchitectureStyle: profile.Architecture,
		KeyComponents:    []ComponentInfo{},
		DataFlowPattern:  "Standard request-response flow",
		CorePatterns:     []PatternInfo{},
		CoreCapabilities: []string{},
		SafetyConstraints: []string{},
		Limitations:      []string{},
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

	// Try to find raw JSON object
	if start := strings.Index(s, "{"); start != -1 {
		depth := 0
		for i := start; i < len(s); i++ {
			switch s[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
