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
	"codenerd/internal/types"
)

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
	DocStatusDiscovered  DocProcessingStatus = "/discovered"  // Found during scan
	DocStatusAnalyzing   DocProcessingStatus = "/analyzing"   // LLM analyzing relevance
	DocStatusExtracting  DocProcessingStatus = "/extracting"  // Extracting knowledge atoms
	DocStatusStored      DocProcessingStatus = "/stored"      // Atoms persisted to DB
	DocStatusSynthesized DocProcessingStatus = "/synthesized" // Included in synthesis
	DocStatusSkipped     DocProcessingStatus = "/skipped"     // Not relevant
	DocStatusFailed      DocProcessingStatus = "/failed"      // Processing failed
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
	ProcessedAt  *time.Time          `json:"processed_at,omitzero"`
	ErrorMessage string              `json:"error_message,omitzero"`
}

// assertDocFact asserts a document tracking fact to the kernel.
// Pattern: doc_<status>(path, hash, timestamp)
func (i *Initializer) assertDocFact(kernel *core.RealKernel, status DocProcessingStatus, path, hash string) {
	if kernel == nil {
		return
	}
	fact := core.Fact{
		Predicate: "doc_ingestion",
		Args:      []any{path, string(status), hash, time.Now().Unix()},
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

	// For large docs or PDFs, use Files API + Context Caching if available
	ext := strings.ToLower(filepath.Ext(doc.Path))
	if ext == ".pdf" || doc.Size > 100*1024 { // >100KB
		if fileProvider, ok := i.config.LLMClient.(types.FileProvider); ok {
			// Check if we also support caching
			if cacheProvider, ok := i.config.LLMClient.(types.CacheProvider); ok {
				return i.extractFromLargeDoc(ctx, doc, db, fileProvider, cacheProvider)
			}
		}
		// Fallback to chunking if provider doesn't support files (though PDF chunking might fail if raw content is binary)
		if ext == ".pdf" {
			logging.Get(logging.CategoryBoot).Warn("Skipping PDF %s: Files API not supported by client", doc.Path)
			return 0, nil
		}
	}

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

		// Use grounded completion if available
		var response string
		var err error
		if i.grounding != nil && i.grounding.IsGroundingAvailable() {
			response, err = i.withJITPrompt(ctx, "kb_agent", prompt, nil, func(ctx context.Context, p string) (string, error) {
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
			response, err = i.withJITPrompt(ctx, "kb_agent", prompt, nil, func(ctx context.Context, p string) (string, error) {
				return i.config.LLMClient.Complete(ctx, p)
			})
		}
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

// extractFromLargeDoc handles large files by uploading to Files API and using Context Caching.
func (i *Initializer) extractFromLargeDoc(
	ctx context.Context,
	doc DocumentInfo,
	db *store.LocalStore,
	fileProvider types.FileProvider,
	cacheProvider types.CacheProvider,
) (int, error) {
	// 1. Upload File
	logging.Get(logging.CategoryBoot).Debug("Uploading large doc %s to Files API...", doc.Path)
	fileURI, err := fileProvider.UploadFile(ctx, doc.AbsPath, "")
	if err != nil {
		return 0, fmt.Errorf("upload failed: %w", err)
	}
	defer func() {
		// Cleanup file (best effort)
		_ = fileProvider.DeleteFile(ctx, fileURI)
	}()

	// 2. Create Context Cache
	// Cache just this file. TTL 5 mins is enough for extraction.
	logging.Get(logging.CategoryBoot).Debug("Creating context cache for %s...", doc.Path)
	cacheName, err := cacheProvider.CreateCachedContent(ctx, []string{fileURI}, 300)
	if err != nil {
		return 0, fmt.Errorf("create cache failed: %w", err)
	}
	defer func() {
		_ = cacheProvider.DeleteCachedContent(ctx, cacheName)
	}()

	// 3. Set Client to use Cache
	// CacheProvider now includes SetCachedContent
	cacheProvider.SetCachedContent(cacheName)
	defer cacheProvider.SetCachedContent("") // Clear after use

	// 4. Extract Knowledge
	// With the whole document in context, we can ask for specific categories in one go.
	// Or maybe two passes: High-level and Details.

	prompt := fmt.Sprintf(`Analyze the document in the context cache.
Title: %s

Extract key strategic knowledge atoms.
Return a JSON array of atoms:
[
  {"concept": "category/specific_topic", "content": "key insight", "confidence": 0.0-1.0}
]

Categories: architecture, philosophy, pattern, capability, constraint, integration.
Focus on high-level architectural decisions, core philosophy, and system boundaries.
`, doc.Title)

	response, err := i.withJITPrompt(ctx, "kb_agent", prompt, nil, func(ctx context.Context, p string) (string, error) {
		return i.config.LLMClient.Complete(ctx, p)
	})
	if err != nil {
		return 0, fmt.Errorf("extraction query failed: %w", err)
	}

	// Parse and store
	type ExtractedAtom struct {
		Concept    string  `json:"concept"`
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
	}
	var atoms []ExtractedAtom

	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), &atoms); err != nil {
		return 0, fmt.Errorf("failed to parse atoms: %w", err)
	}

	atomCount := 0
	for _, atom := range atoms {
		concept := fmt.Sprintf("doc/%s/%s", doc.Path, atom.Concept)
		if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, atom.Content, atom.Confidence); err == nil {
			atomCount++
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

	// Use grounded completion if available
	var response string
	if i.grounding != nil && i.grounding.IsGroundingAvailable() {
		var grErr error
		response, grErr = i.withJITPrompt(ctx, "kb_complete", prompt, nil, func(ctx context.Context, p string) (string, error) {
			resp, _, err := i.grounding.CompleteWithGrounding(ctx, p)
			return resp, err
		})
		if grErr != nil {
			return nil, fmt.Errorf("synthesis LLM call failed: %w", grErr)
		}
		// Capture grounding sources
		sources := i.grounding.CaptureGroundingSources()
		if len(sources) > 0 {
			i.mu.Lock()
			i.groundingSources = append(i.groundingSources, sources...)
			i.mu.Unlock()
		}
	} else {
		var llmErr error
		response, llmErr = i.withJITPrompt(ctx, "kb_complete", prompt, nil, func(ctx context.Context, p string) (string, error) {
			return i.config.LLMClient.Complete(ctx, p)
		})
		if llmErr != nil {
			return nil, fmt.Errorf("synthesis LLM call failed: %w", llmErr)
		}
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
