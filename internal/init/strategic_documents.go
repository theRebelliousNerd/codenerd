package init

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"codenerd/internal/atomicfile"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
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
	CampaignID  string                         `json:"campaign_id"`
	StartedAt   time.Time                      `json:"started_at"`
	LastUpdated time.Time                      `json:"last_updated"`
	Phase       string                         `json:"phase"` // "discovery", "analysis", "extraction", "synthesis"
	Documents   map[string]*DocProcessingEntry `json:"documents"`
	// TotalDiscovered, TotalProcessed, TotalStored and AtomsStored describe
	// the latest run: documents offered, documents orientation admitted,
	// documents extracted, and the atoms those extractions stored.
	TotalDiscovered int  `json:"total_discovered"`
	TotalProcessed  int  `json:"total_processed"`
	TotalStored     int  `json:"total_stored"`
	AtomsStored     int  `json:"atoms_stored"`
	SynthesisReady  bool `json:"synthesis_ready"`
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
func (i *Initializer) assertDocFact(kernel core.Kernel, status DocProcessingStatus, path, hash string) {
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

// extracted reports whether a document's atoms are in the knowledge base.
func extracted(status DocProcessingStatus) bool {
	return status == DocStatusStored || status == DocStatusSynthesized
}

// computeDocHash generates a SHA256 hash for document content deduplication.
func computeDocHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16] // First 16 chars sufficient
}

// loadIngestionState loads previous ingestion state for resumption. A missing
// file is a first run; an unreadable one starts over and says so.
func (i *Initializer) loadIngestionState() (*DocIngestionState, error) {
	statePath := filepath.Join(i.config.Workspace, ".nerd", "doc_ingestion_state.json")
	data, err := os.ReadFile(statePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read document ingestion state: %w", err)
	}
	var state DocIngestionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("document ingestion state is unreadable; every document is extracted again: %w", err)
	}
	if state.Documents == nil {
		state.Documents = make(map[string]*DocProcessingEntry)
	}
	return &state, nil
}

// saveIngestionState persists ingestion state for resumption, atomically: a
// torn write would make the next refresh extract every document again.
func (i *Initializer) saveIngestionState(state *DocIngestionState) error {
	state.LastUpdated = time.Now()
	statePath := filepath.Join(i.config.Workspace, ".nerd", "doc_ingestion_state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(statePath, data, 0o644)
}

// ProcessDocumentsWithTracking processes documents with Mangle fact tracking
// and incremental knowledge persistence. An unchanged, already extracted
// document is skipped unless force is set. Uses campaign patterns:
// 1. Assert doc_discovered for each found doc
// 2. Assert doc_analyzing when LLM analyzes
// 3. Store atoms incrementally and assert doc_stored
// 4. Track progress for resumption
func (i *Initializer) ProcessDocumentsWithTracking(
	ctx context.Context,
	docs []DocumentInfo,
	db *store.LocalStore,
	kernel core.Kernel,
	force bool,
) (*DocIngestionState, error) {
	// Load or create ingestion state
	state, loadErr := i.loadIngestionState()
	if loadErr != nil {
		logging.Get(logging.CategoryBoot).Warn("%v", loadErr)
	}
	if state == nil {
		state = &DocIngestionState{
			CampaignID: fmt.Sprintf("doc_init_%d", time.Now().Unix()),
			StartedAt:  time.Now(),
			Phase:      "discovery",
			Documents:  make(map[string]*DocProcessingEntry),
		}
	}

	// The counters describe this run, not every run the state file has seen.
	state.TotalDiscovered, state.TotalProcessed, state.TotalStored, state.AtomsStored = len(docs), 0, 0, 0

	// Phase 1: Discovery - assert all discovered docs
	for _, doc := range docs {
		hash := computeDocHash(doc.Content)

		// An unchanged document already extracted (and perhaps synthesized)
		// is not extracted again, unless the caller forces a full pass.
		if existing, ok := state.Documents[doc.Path]; ok && !force {
			if existing.ContentHash == hash && extracted(existing.Status) {
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
	}
	state.Phase = "analysis"
	if err := i.saveIngestionState(state); err != nil {
		return state, fmt.Errorf("save document ingestion state: %w", err)
	}

	// The orientation fixpoint owns admission; extraction cannot re-elect docs.
	if i.orientation == nil {
		return state, fmt.Errorf("document extraction requires orientation")
	}
	rows, err := i.orientation.Query("orient_read_candidate")
	if err != nil {
		return state, err
	}
	admitted := map[string]string{}
	for _, row := range rows {
		admitted[types.ExtractString(row.Args[0])] = types.ExtractString(row.Args[1])
	}
	var relevantDocs []DocumentInfo
	for _, doc := range docs {
		if reason, ok := admitted[filepath.ToSlash(doc.Path)]; ok {
			doc.IsRelevant, doc.Reasoning = true, reason
			relevantDocs = append(relevantDocs, doc)
		}
	}
	for _, doc := range relevantDocs {
		if entry, ok := state.Documents[doc.Path]; ok {
			entry.IsRelevant = true
			entry.Reasoning = doc.Reasoning
			// An unchanged document already extracted keeps its status: this
			// used to reset every admitted document to analyzing, so the
			// extraction loop below re-extracted all of them on every refresh.
			if extracted(entry.Status) && entry.ContentHash == computeDocHash(doc.Content) && !force {
				continue
			}
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
		if relevantPaths[path] {
			continue
		}
		switch entry.Status {
		case DocStatusDiscovered:
			entry.Status = DocStatusSkipped
			entry.Reasoning = "Not admitted by orientation policy"
			i.assertDocFact(kernel, DocStatusSkipped, path, entry.ContentHash)
		case DocStatusStored, DocStatusSynthesized:
			// A document orientation no longer admits (deleted, demoted,
			// superseded) takes its extracted atoms with it.
			if db != nil {
				if _, err := db.DeleteKnowledgeAtomsByPrefix("doc/" + path + "/"); err != nil {
					return state, fmt.Errorf("retire atoms of %s: %w", path, err)
				}
			}
			entry.Status = DocStatusSkipped
			entry.AtomsStored = 0
			entry.Reasoning = "No longer admitted by orientation policy; its atoms were retired"
			i.assertDocFact(kernel, DocStatusSkipped, path, entry.ContentHash)
		}
	}
	state.Phase = "extraction"
	if err := i.saveIngestionState(state); err != nil {
		return state, fmt.Errorf("save document ingestion state: %w", err)
	}

	// Phase 3: Extraction - process each relevant doc and store atoms incrementally
	for _, doc := range relevantDocs {
		select {
		case <-ctx.Done():
			return state, ctx.Err()
		default:
		}

		entry := state.Documents[doc.Path]
		if extracted(entry.Status) {
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
		state.AtomsStored += atomCount
		i.assertDocFact(kernel, DocStatusStored, doc.Path, entry.ContentHash)

		// Save state after each doc for resumption
		if err := i.saveIngestionState(state); err != nil {
			return state, fmt.Errorf("save document ingestion state: %w", err)
		}
		logging.Get(logging.CategoryBoot).Debug("Stored %d atoms from %s", atomCount, doc.Path)
	}

	state.TotalProcessed = len(relevantDocs)
	state.SynthesisReady = true
	state.Phase = "synthesis"
	if err := i.saveIngestionState(state); err != nil {
		return state, fmt.Errorf("save document ingestion state: %w", err)
	}
	return state, nil
}

// extractAndStoreDocKnowledge transduces one document into knowledge atoms on
// the main route and stores them under doc/<path>/. Per-chunk extraction is
// bulk curation, one request per chunk of every admitted document, so it runs
// on the main client; the synthesis that turns all atoms into strategic
// knowledge is planner work (SynthesizeFromStoredAtoms). Every document is
// paged in text chunks, as orientation pages them; a provider file upload or
// context cache would mutate state on a client the whole session shares.
// The document's atoms from an earlier extraction are retired first, so a
// changed document does not keep the claims of the text it replaced.
func (i *Initializer) extractAndStoreDocKnowledge(
	ctx context.Context,
	doc DocumentInfo,
	db *store.LocalStore,
) (int, error) {
	if i.route(slotMain).client == nil || db == nil {
		return 0, fmt.Errorf("LLM client and database required")
	}
	if !utf8.ValidString(doc.Content) {
		return 0, fmt.Errorf("%s is not UTF-8 text", doc.Path)
	}
	if _, err := db.DeleteKnowledgeAtomsByPrefix("doc/" + doc.Path + "/"); err != nil {
		return 0, fmt.Errorf("retire earlier atoms of %s: %w", doc.Path, err)
	}
	if _, err := db.DeleteKnowledgeAtomsByConcept("doc/" + doc.Path); err != nil {
		return 0, fmt.Errorf("retire earlier atoms of %s: %w", doc.Path, err)
	}

	chunks := chunkDocument(doc.Content, 8000) // ~2k tokens per chunk
	atomCount := 0
	var failures []error
	for chunkIdx, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return atomCount, err
		}
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

Every atom must state something this chunk says. Be specific and extract only genuinely useful insights. Skip boilerplate. An empty array is a valid answer.
`, doc.Path, chunkIdx+1, len(chunks), doc.Title, chunk)

		response, err := i.withJITPrompt(ctx, initCall{slot: slotMain, phase: "kb_agent", task: prompt},
			func(ctx context.Context, client perception.LLMClient, p string) (string, error) {
				return client.Complete(ctx, p)
			})
		if err != nil {
			failures = append(failures, fmt.Errorf("chunk %d/%d: %w", chunkIdx+1, len(chunks), err))
			continue
		}

		var atoms []struct {
			Concept    string  `json:"concept"`
			Content    string  `json:"content"`
			Confidence float64 `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(extractJSON(response)), &atoms); err != nil {
			// A reply that is not the requested array is a failed extraction,
			// never knowledge: storing the raw chunk in its place filed the
			// document's own text as an "extracted" atom.
			failures = append(failures, fmt.Errorf("chunk %d/%d: reply is not a JSON array of atoms: %w", chunkIdx+1, len(chunks), err))
			continue
		}
		for _, atom := range atoms {
			concept := strings.Trim(strings.TrimSpace(atom.Concept), "/")
			content := strings.TrimSpace(atom.Content)
			if concept == "" || content == "" {
				continue
			}
			confidence := atom.Confidence
			if confidence <= 0 || confidence > 1 {
				confidence = 0.7
			}
			if err := db.StoreKnowledgeAtomWithEmbedding(ctx, fmt.Sprintf("doc/%s/%s", doc.Path, concept), content, confidence); err != nil {
				failures = append(failures, fmt.Errorf("store atom %s: %w", concept, err))
				continue
			}
			atomCount++
		}
	}
	if len(failures) == len(chunks) && atomCount == 0 && len(chunks) > 0 {
		return 0, fmt.Errorf("no chunk of %s was extracted: %w", doc.Path, errors.Join(failures...))
	}
	if len(failures) > 0 {
		logging.Get(logging.CategoryBoot).Warn("Partial extraction of %s: %v", doc.Path, errors.Join(failures...))
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

// buildAtomsSummary groups stored atoms by concept category and formats them
// for the synthesis prompt. Atom content passes whole: a 200-char cut fed the
// synthesis model truncated context for every long insight.
func buildAtomsSummary(atoms []store.KnowledgeAtom) (string, map[string][]string) {
	categories := make(map[string][]string)
	for _, atom := range atoms {
		parts := strings.SplitN(atom.Concept, "/", 3)
		category := "other"
		if len(parts) >= 2 {
			category = parts[1] // e.g., "architecture", "philosophy"
		}
		categories[category] = append(categories[category], atom.Content)
	}

	var sb strings.Builder
	sb.WriteString("## Extracted Knowledge Atoms\n\n")
	for category, contents := range categories {
		sb.WriteString(fmt.Sprintf("### %s\n", category))
		for _, content := range contents {
			sb.WriteString(fmt.Sprintf("- %s\n", content))
		}
		sb.WriteString("\n")
	}
	return sb.String(), categories
}

// SynthesizeFromStoredAtoms performs a second pass over stored knowledge atoms
// to create the final strategic knowledge synthesis. It runs on the planner
// route: the strategic atoms it produces are read by every later session.
func (i *Initializer) SynthesizeFromStoredAtoms(
	ctx context.Context,
	db *store.LocalStore,
	state *DocIngestionState,
) (*StrategicKnowledge, error) {
	if i.route(slotPlanner).client == nil || db == nil {
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
	atomsSummary, categories := buildAtomsSummary(atoms)

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
`, state.TotalProcessed, len(atoms), keysFromMap(categories), atomsSummary)

	response, err := i.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: "kb_complete", task: prompt},
		func(ctx context.Context, client perception.LLMClient, p string) (string, error) {
			return client.Complete(ctx, p)
		})
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
	if err := i.saveIngestionState(state); err != nil {
		return knowledge, fmt.Errorf("save document ingestion state: %w", err)
	}
	return knowledge, nil
}
