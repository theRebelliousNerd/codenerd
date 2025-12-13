package campaign

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ShardLister provides shard discovery for campaign planning.
type ShardLister interface {
	ListAvailableShards() []coreshards.ShardInfo
}

// Decomposer creates campaign plans through LLM + Mangle collaboration.
// It parses messy specifications and user goals into structured, validated plans.
type Decomposer struct {
	kernel         *core.RealKernel
	llmClient      perception.LLMClient
	workspace      string
	promptProvider PromptProvider // Optional JIT prompt provider
	shardLister    ShardLister    // Optional shard discovery for shard-aware planning
}

// NewDecomposer creates a new decomposer.
func NewDecomposer(kernel *core.RealKernel, llmClient perception.LLMClient, workspace string) *Decomposer {
	logging.CampaignDebug("Creating new Decomposer for workspace: %s", workspace)
	return &Decomposer{
		kernel:         kernel,
		llmClient:      llmClient,
		workspace:      workspace,
		promptProvider: NewStaticPromptProvider(), // Default to static prompts
	}
}

// SetPromptProvider sets the PromptProvider for JIT-compiled prompts.
// This allows using JIT-compiled prompts from the articulation package.
// If not set, static prompts will be used.
func (d *Decomposer) SetPromptProvider(provider PromptProvider) {
	d.promptProvider = provider
	if provider != nil {
		logging.CampaignDebug("Decomposer configured with custom prompt provider")
	}
}

// SetShardLister sets the shard discovery interface for shard-aware planning.
// When set, the decomposer can inform the LLM about available shards.
func (d *Decomposer) SetShardLister(lister ShardLister) {
	d.shardLister = lister
	if lister != nil {
		logging.CampaignDebug("Decomposer configured with shard discovery")
	}
}

// DecomposeRequest represents a request to create a campaign.
type DecomposeRequest struct {
	Goal          string       // High-level goal description
	SourcePaths   []string     // Paths to spec docs, requirements, etc.
	CampaignType  CampaignType // Type of campaign
	UserHints     []string     // Optional user guidance
	MaxPhases     int          // Max phases (0 = unlimited)
	ContextBudget int          // Token budget (0 = default 100k)
}

// DecomposeResult represents the result of decomposition.
type DecomposeResult struct {
	Campaign     *Campaign
	ValidationOK bool
	Issues       []PlanValidationIssue
	SourceDocs   []SourceDocument
	Requirements []Requirement
}

// DocClassification holds the LLM's judgement of a file.
type DocClassification struct {
	Layer      string  `json:"layer"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// Decompose creates a campaign plan through LLM + Mangle collaboration.
func (d *Decomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	timer := logging.StartTimer(logging.CategoryCampaign, "Decompose")
	defer timer.StopWithInfo()

	logging.Campaign("=== Starting campaign decomposition ===")
	logging.Campaign("Goal: %s", req.Goal[:min(200, len(req.Goal))])
	logging.CampaignDebug("Campaign type: %s, source paths: %d, context budget: %d",
		req.CampaignType, len(req.SourcePaths), req.ContextBudget)

	// Generate campaign ID
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	safeCampaignID := sanitizeCampaignID(campaignID)
	logging.Campaign("Generated campaign ID: %s", campaignID)

	// Set defaults
	if req.ContextBudget == 0 {
		req.ContextBudget = 100000 // 100k tokens default
	}

	kbPath := filepath.Join(d.workspace, ".nerd", "campaigns", safeCampaignID, "knowledge.db")

	// Step 1: Ingest source documents
	logging.Campaign("Step 1: Ingesting source documents")
	ingestTimer := logging.StartTimer(logging.CategoryCampaign, "ingestSourceDocuments")
	sourceDocs, fileMeta, err := d.ingestSourceDocuments(ctx, campaignID, req.SourcePaths)
	ingestTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Source document ingestion failed: %v", err)
		return nil, fmt.Errorf("failed to ingest source documents: %w", err)
	}
	logging.Campaign("Ingested %d source documents, %d file metadata entries", len(sourceDocs), len(fileMeta))

	// Seed metadata + goal signals for Mangle-driven selection
	logging.CampaignDebug("Seeding document facts into kernel")
	d.seedDocFacts(campaignID, req.Goal, fileMeta)

	// Step 1b: Ingest into campaign knowledge store (vectors + graph) for retrieval
	logging.Campaign("Step 1b: Ingesting into knowledge store")
	if err := d.ingestIntoKnowledgeStore(ctx, campaignID, kbPath, fileMeta); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Knowledge ingestion failed (non-fatal): %v", err)
	}

	// Step 2: Extract requirements from source documents
	logging.Campaign("Step 2: Extracting requirements (RAG-based)")
	reqTimer := logging.StartTimer(logging.CategoryCampaign, "extractRequirementsSmart")
	requirements, err := d.extractRequirementsSmart(ctx, campaignID, req.Goal, kbPath, fileMeta)
	reqTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Requirement extraction failed: %v", err)
		return nil, fmt.Errorf("failed to extract requirements: %w", err)
	}
	logging.Campaign("Extracted %d requirements", len(requirements))

	// Step 3: LLM proposes phases and tasks
	logging.Campaign("Step 3: LLM proposing plan structure")
	planTimer := logging.StartTimer(logging.CategoryCampaign, "llmProposePlan")
	rawPlan, err := d.llmProposePlan(ctx, campaignID, req, kbPath, fileMeta, requirements)
	planTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM plan proposal failed: %v", err)
		return nil, fmt.Errorf("failed to propose plan: %w", err)
	}
	logging.Campaign("LLM proposed plan: %s (confidence=%.2f, phases=%d)",
		rawPlan.Title, rawPlan.Confidence, len(rawPlan.Phases))

	// Step 4: Convert to Campaign structure
	logging.Campaign("Step 4: Building campaign structure")
	campaign := d.buildCampaign(campaignID, req, rawPlan)
	campaign.SourceDocs = sourceDocs
	campaign.KnowledgeBase = kbPath
	logging.CampaignDebug("Campaign built: phases=%d, totalTasks=%d", len(campaign.Phases), campaign.TotalTasks)

	// Step 5: Load into Mangle for validation
	logging.Campaign("Step 5: Loading campaign facts into Mangle kernel")
	facts := campaign.ToFacts()
	logging.CampaignDebug("Loading %d facts", len(facts))
	if err := d.kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to load campaign facts: %v", err)
		return nil, fmt.Errorf("failed to load campaign facts: %w", err)
	}

	// Step 6: Mangle validates (circular deps, unreachable tasks, etc.)
	logging.Campaign("Step 6: Mangle validation")
	issues := d.validatePlan(campaignID)
	if len(issues) > 0 {
		logging.Get(logging.CategoryCampaign).Warn("Validation found %d issues", len(issues))
		for i, issue := range issues {
			logging.CampaignDebug("Issue %d: [%s] %s", i+1, issue.IssueType, issue.Description)
		}
	} else {
		logging.Campaign("Validation passed with no issues")
	}

	// Step 7: If issues, attempt LLM refinement
	if len(issues) > 0 {
		logging.Campaign("Step 7: Attempting LLM refinement to fix %d issues", len(issues))
		refineTimer := logging.StartTimer(logging.CategoryCampaign, "refinePlan")
		refinedPlan, err := d.refinePlan(ctx, rawPlan, issues)
		refineTimer.Stop()
		if err == nil && refinedPlan != nil {
			logging.Campaign("Refinement successful, rebuilding campaign")
			campaign = d.buildCampaign(campaignID, req, refinedPlan)
			// Reload and revalidate
			d.kernel.Retract("campaign")
			d.kernel.Retract("campaign_phase")
			d.kernel.Retract("campaign_task")
			d.kernel.LoadFacts(campaign.ToFacts())
			issues = d.validatePlan(campaignID)
			logging.Campaign("After refinement: %d issues remaining", len(issues))
		} else if err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Refinement failed: %v", err)
		}
	}

	// Step 8: Link requirements to tasks
	logging.Campaign("Step 8: Linking requirements to tasks")
	d.linkRequirementsToTasks(requirements, campaign)
	coveredCount := 0
	for _, req := range requirements {
		if len(req.CoveredBy) > 0 {
			coveredCount++
		}
	}
	logging.Campaign("Requirement coverage: %d/%d requirements linked to tasks", coveredCount, len(requirements))

	logging.Campaign("=== Decomposition complete: %s ===", campaign.Title)
	logging.Campaign("Final plan: phases=%d, tasks=%d, validation=%v",
		campaign.TotalPhases, campaign.TotalTasks, len(issues) == 0)

	return &DecomposeResult{
		Campaign:     campaign,
		ValidationOK: len(issues) == 0,
		Issues:       issues,
		SourceDocs:   sourceDocs,
		Requirements: requirements,
	}, nil
}

// ingestSourceDocuments reads and parses source documents (metadata only).
func (d *Decomposer) ingestSourceDocuments(ctx context.Context, campaignID string, paths []string) ([]SourceDocument, []FileMetadata, error) {
	logging.CampaignDebug("Ingesting source documents from %d paths", len(paths))

	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	for _, path := range paths {
		// Check for cancellation between file reads
		select {
		case <-ctx.Done():
			logging.CampaignDebug("Source ingestion cancelled")
			return docs, meta, ctx.Err()
		default:
		}
		// Resolve path
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(d.workspace, path)
		}

		logging.CampaignDebug("Processing path: %s", fullPath)

		stat, err := os.Stat(fullPath)
		if err != nil {
			// Try glob pattern
			matches, _ := filepath.Glob(fullPath)
			if len(matches) == 0 {
				logging.CampaignDebug("Skipping missing path: %s", fullPath)
				continue // Skip missing files
			}
			logging.CampaignDebug("Glob matched %d files", len(matches))
			for _, match := range matches {
				mds, mmeta := d.readDocumentsFromPath(match, campaignID)
				docs = append(docs, mds...)
				meta = append(meta, mmeta...)
			}
			continue
		}

		if stat.IsDir() {
			logging.CampaignDebug("Reading directory: %s", fullPath)
			mds, mmeta := d.readDocumentsFromDir(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		} else {
			mds, mmeta := d.readDocumentsFromPath(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		}
	}

	logging.CampaignDebug("Classifying %d documents by architectural layer", len(meta))
	meta = d.classifyDocuments(ctx, meta)

	logging.CampaignDebug("Ingestion complete: docs=%d, meta=%d", len(docs), len(meta))
	return docs, meta, nil
}

// classifyDocuments routes files through the Librarian to assign architectural layers.
func (d *Decomposer) classifyDocuments(ctx context.Context, files []FileMetadata) []FileMetadata {
	if len(files) == 0 {
		return files
	}

	if d.llmClient == nil {
		logging.CampaignDebug("No LLM client, using default layer classification")
		for i := range files {
			if files[i].Layer == "" {
				files[i].Layer = "/scaffold"
			}
			if files[i].LayerConfidence == 0 {
				files[i].LayerConfidence = 0.1
			}
		}
		return files
	}

	classifiedCount := 0
	for i := range files {
		select {
		case <-ctx.Done():
			logging.CampaignDebug("Document classification cancelled after %d files", classifiedCount)
			return files
		default:
		}

		// Sensible defaults if classification is unavailable
		files[i].Layer = "/scaffold"
		files[i].LayerConfidence = 0.1

		data, err := os.ReadFile(files[i].Path)
		if err != nil {
			logging.CampaignDebug("Cannot read file for classification: %s", files[i].Path)
			continue
		}

		class, err := d.classifyDocument(ctx, files[i].Path, string(data))
		if err != nil {
			logging.CampaignDebug("Classification failed for %s: %v", files[i].Path, err)
			continue
		}

		if class.Layer != "" {
			files[i].Layer = class.Layer
		}
		if class.Confidence > 0 {
			files[i].LayerConfidence = class.Confidence
		}
		if class.Reasoning != "" {
			files[i].LayerReason = class.Reasoning
		}
		classifiedCount++
		logging.CampaignDebug("Classified %s -> %s (confidence=%.2f)",
			filepath.Base(files[i].Path), files[i].Layer, files[i].LayerConfidence)
	}

	logging.CampaignDebug("Classified %d/%d documents", classifiedCount, len(files))
	return files
}

func (d *Decomposer) readDocumentsFromDir(dir string, campaignID string) ([]SourceDocument, []FileMetadata) {
	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !isSupportedDocExt(path) {
			return nil
		}
		mds, mmeta := d.readDocumentsFromPath(path, campaignID)
		docs = append(docs, mds...)
		meta = append(meta, mmeta...)
		return nil
	})

	return docs, meta
}

func (d *Decomposer) readDocumentsFromPath(path string, campaignID string) ([]SourceDocument, []FileMetadata) {
	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	docType := d.inferDocType(path)
	stat, err := os.Stat(path)
	if err != nil {
		return docs, meta
	}

	docs = append(docs, SourceDocument{
		CampaignID: campaignID,
		Path:       path,
		Type:       docType,
		ParsedAt:   time.Now(),
	})
	tags := deriveTagsFromPath(path)
	meta = append(meta, FileMetadata{
		Path:       path,
		Type:       docType,
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime(),
		Tags:       tags,
	})
	return docs, meta
}

// classifyDocument asks the LLM to bucket the document into an architectural layer.
func (d *Decomposer) classifyDocument(ctx context.Context, filename, content string) (DocClassification, error) {
	defaultClass := DocClassification{Layer: "/scaffold", Confidence: 0.1}

	if d.llmClient == nil {
		return defaultClass, nil
	}

	trimmed := strings.TrimSpace(content)
	lowerName := strings.ToLower(filename)

	// Optimization: Don't classify trivial files
	if len(trimmed) < 50 || strings.HasSuffix(lowerName, ".txt") {
		return DocClassification{Layer: "/scaffold", Confidence: 0.5, Reasoning: "defaulted (trivial content)"}, nil
	}

	// Get prompt (JIT or static)
	basePrompt, err := d.promptProvider.GetPrompt(ctx, RoleLibrarian, "")
	if err != nil {
		logging.CampaignDebug("Failed to get Librarian prompt, using fallback: %v", err)
		basePrompt = LibrarianLogic
	}

	prompt := fmt.Sprintf(`%s

FILE: %s
CONTENT START:
%s
CONTENT END

Return JSON only: {"layer": "/string", "confidence": 0.0-1.0, "reasoning": "brief"}`,
		basePrompt, filename, limitString(trimmed, 2000))

	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		return defaultClass, err
	}

	var result DocClassification
	if err := json.Unmarshal([]byte(cleanJSONResponse(resp)), &result); err != nil {
		return defaultClass, err
	}

	if result.Layer == "" {
		result.Layer = "/scaffold"
	}
	if result.Confidence == 0 {
		result.Confidence = defaultClass.Confidence
	}

	return result, nil
}

// ingestIntoKnowledgeStore persists all document chunks into the campaign knowledge DB (vectors + KG).
func (d *Decomposer) ingestIntoKnowledgeStore(ctx context.Context, campaignID, dbPath string, files []FileMetadata) error {
	if len(files) == 0 {
		logging.CampaignDebug("No files to ingest into knowledge store")
		return nil
	}

	logging.CampaignDebug("Initializing document ingestor: dbPath=%s", dbPath)
	ingestor, err := NewDocumentIngestor(dbPath, embedding.DefaultConfig())
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create document ingestor: %v", err)
		return err
	}
	defer ingestor.Close()

	ingestedCount := 0
	totalBytes := int64(0)
	for _, fm := range files {
		data, err := os.ReadFile(fm.Path)
		if err != nil {
			logging.CampaignDebug("Failed to read file for ingestion: %s - %v", fm.Path, err)
			continue
		}
		payload := map[string]string{fm.Path: string(data)}
		_, _ = ingestor.Ingest(ctx, campaignID, payload)
		ingestedCount++
		totalBytes += int64(len(data))
	}

	logging.Campaign("Knowledge store ingestion complete: files=%d, bytes=%d", ingestedCount, totalBytes)
	return nil
}

// seedDocFacts pushes lightweight document metadata into the kernel for logic-based selection.
func (d *Decomposer) seedDocFacts(campaignID, goal string, files []FileMetadata) {
	if d.kernel == nil {
		logging.CampaignDebug("No kernel available for seeding doc facts")
		return
	}

	logging.CampaignDebug("Seeding %d document facts for campaign %s", len(files), campaignID)
	facts := make([]core.Fact, 0, len(files)+1)
	// Campaign goal fact already loaded later; still record a preliminary goal signal for selection rules.
	facts = append(facts, core.Fact{
		Predicate: "campaign_goal",
		Args:      []interface{}{campaignID, goal},
	})

	topics := extractTopicsFromGoal(goal)
	for _, topic := range topics {
		facts = append(facts, core.Fact{
			Predicate: "goal_topic",
			Args:      []interface{}{campaignID, fmt.Sprintf("/%s", topic)},
		})
	}

	for _, fm := range files {
		facts = append(facts, core.Fact{
			Predicate: "doc_metadata",
			Args:      []interface{}{campaignID, fm.Path, fm.Type, fm.SizeBytes, fm.ModifiedAt.Unix()},
		})
		layer := fm.Layer
		if layer == "" {
			layer = "/scaffold"
		}
		confidence := fm.LayerConfidence
		if confidence == 0 {
			confidence = 0.1
		}
		facts = append(facts, core.Fact{
			Predicate: "doc_layer",
			Args:      []interface{}{fm.Path, layer, confidence},
		})
		for _, tag := range fm.Tags {
			facts = append(facts, core.Fact{
				Predicate: "doc_tag",
				Args:      []interface{}{fm.Path, fmt.Sprintf("/%s", tag)},
			})
		}
	}

	if err := d.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to assert doc facts batch: %v", err)
	} else {
		logging.CampaignDebug("Seeded %d facts into kernel", len(facts))
	}
}

// inferDocType infers the document type from filename.
func (d *Decomposer) inferDocType(path string) string {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(lower, "spec"):
		return "/spec"
	case strings.Contains(lower, "requirement"):
		return "/requirements"
	case strings.Contains(lower, "design"):
		return "/design"
	case strings.Contains(lower, "readme"):
		return "/readme"
	case strings.Contains(lower, "api"):
		return "/api_doc"
	case strings.Contains(lower, "tutorial"):
		return "/tutorial"
	default:
		return "/spec"
	}
}

// extractRequirements uses LLM to extract requirements from source content.
func (d *Decomposer) extractRequirements(ctx context.Context, campaignID string, content map[string]string) ([]Requirement, error) {
	if len(content) == 0 {
		return nil, nil
	}

	if d.llmClient == nil {
		return nil, nil
	}

	reqs := make([]Requirement, 0)
	seen := make(map[string]bool)
	reqCounter := 0

	for path, text := range content {
		chunks := chunkText(text, 6000)
		if len(chunks) == 0 {
			continue
		}

		for idx, chunk := range chunks {
			prompt := fmt.Sprintf(`%s

Document: %s
Chunk: %d of %d
Content:
%s
`, ExtractorLogic, path, idx+1, len(chunks), chunk)

			resp, err := d.llmClient.Complete(ctx, prompt)
			if err != nil {
				continue
			}

			resp = cleanJSONResponse(resp)
			var parsed struct {
				Requirements []struct {
					ID          string `json:"id"`
					Description string `json:"description"`
					Priority    string `json:"priority"`
					Source      string `json:"source"`
				} `json:"requirements"`
			}

			if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
				continue
			}

			for _, r := range parsed.Requirements {
				reqCounter++
				id := fmt.Sprintf("/req_%s_%04d", sanitizeCampaignID(campaignID), reqCounter)
				key := fmt.Sprintf("%s|%s", path, r.Description)
				if seen[key] {
					continue
				}
				seen[key] = true
				reqs = append(reqs, Requirement{
					ID:          id,
					CampaignID:  campaignID,
					Description: r.Description,
					Priority:    defaultPriority(r.Priority),
					Source:      path,
				})
			}
		}
	}

	return reqs, nil
}

// extractRequirementsSmart performs retrieval-augmented requirement extraction using the vector store.
func (d *Decomposer) extractRequirementsSmart(ctx context.Context, campaignID, goal, kbPath string, files []FileMetadata) ([]Requirement, error) {
	if d.llmClient == nil {
		logging.CampaignDebug("No LLM client, skipping requirement extraction")
		return nil, nil
	}

	questions := d.generateDiscoveryQuestions(goal)
	if len(questions) == 0 {
		logging.CampaignDebug("No discovery questions generated")
		return nil, nil
	}
	logging.CampaignDebug("Generated %d discovery questions", len(questions))

	kbStore, err := store.NewLocalStore(kbPath)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to open knowledge store: %v", err)
		return nil, fmt.Errorf("failed to open knowledge store: %w", err)
	}
	defer kbStore.Close()

	reqs := make([]Requirement, 0)
	seen := make(map[string]bool)
	reqCounter := 0
	allowedPaths := d.relevantPathsFromKernel()
	if len(allowedPaths) == 0 {
		allowedPaths = pathsForGoal(goal, files)
	}
	logging.CampaignDebug("Using %d allowed paths for vector recall", len(allowedPaths))

	for i, q := range questions {
		logging.CampaignDebug("Processing question %d/%d: %s", i+1, len(questions), q[:min(80, len(q))])

		var entries []store.VectorEntry
		var err error
		if len(allowedPaths) > 0 {
			entries, err = kbStore.VectorRecallSemanticByPaths(ctx, q, 6, allowedPaths)
		} else {
			entries, err = kbStore.VectorRecallSemanticFiltered(ctx, q, 6, "campaign_id", campaignID)
		}
		if err != nil {
			logging.CampaignDebug("Vector recall failed: %v", err)
			continue
		}
		if len(entries) == 0 {
			logging.CampaignDebug("No vector entries found for question")
			continue
		}
		logging.CampaignDebug("Retrieved %d vector entries", len(entries))

		var sb strings.Builder
		for _, e := range entries {
			path := ""
			if p, ok := e.Metadata["path"].(string); ok {
				path = p
			}
			sb.WriteString(fmt.Sprintf("PATH: %s\n", path))
			sb.WriteString(e.Content)
			sb.WriteString("\n---\n")
		}

		prompt := fmt.Sprintf(`Goal: %s
Question: %s
Given the retrieved snippets, extract discrete requirements as JSON:
{
  "requirements": [
    {"description": "...", "priority": "/critical|/high|/normal|/low", "source": "path"}
  ]
}

Snippets:
%s
Return JSON only.`, goal, q, sb.String())

		resp, err := d.llmClient.Complete(ctx, prompt)
		if err != nil {
			logging.CampaignDebug("LLM extraction failed: %v", err)
			continue
		}

		resp = cleanJSONResponse(resp)
		var parsed struct {
			Requirements []struct {
				Description string `json:"description"`
				Priority    string `json:"priority"`
				Source      string `json:"source"`
			} `json:"requirements"`
		}
		if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
			logging.CampaignDebug("Failed to parse requirements JSON: %v", err)
			continue
		}

		extractedCount := 0
		for _, r := range parsed.Requirements {
			key := fmt.Sprintf("%s|%s", r.Source, r.Description)
			if seen[key] {
				continue
			}
			reqCounter++
			id := fmt.Sprintf("/req_%s_%04d", sanitizeCampaignID(campaignID), reqCounter)
			seen[key] = true
			reqs = append(reqs, Requirement{
				ID:          id,
				CampaignID:  campaignID,
				Description: r.Description,
				Priority:    defaultPriority(r.Priority),
				Source:      r.Source,
			})
			extractedCount++
		}
		logging.CampaignDebug("Extracted %d new requirements from question %d", extractedCount, i+1)
	}

	logging.Campaign("Total requirements extracted: %d", len(reqs))
	return reqs, nil
}

// relevantPathsFromKernel reads Mangle-derived relevance decisions.
func (d *Decomposer) relevantPathsFromKernel() []string {
	if d.kernel == nil {
		return nil
	}

	facts, err := d.kernel.Query("is_relevant")
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	paths := make([]string, 0, len(facts))
	for _, fact := range facts {
		if len(fact.Args) == 0 {
			continue
		}
		path, ok := fact.Args[0].(string)
		if !ok {
			path = fmt.Sprintf("%v", fact.Args[0])
		}
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	return paths
}

// pathsForGoal derives candidate file paths whose tags align with the goal keywords.
func pathsForGoal(goal string, files []FileMetadata) []string {
	if len(files) == 0 {
		return nil
	}
	goal = strings.ToLower(goal)
	tokens := strings.FieldsFunc(goal, func(r rune) bool {
		return r == ' ' || r == '/' || r == '-' || r == '_'
	})
	tokenSet := make(map[string]struct{})
	for _, t := range tokens {
		if len(t) < 3 {
			continue
		}
		tokenSet[t] = struct{}{}
	}

	paths := make([]string, 0)
	for _, f := range files {
		match := false
		for _, tag := range f.Tags {
			if len(tag) < 3 {
				continue
			}
			if _, ok := tokenSet[tag]; ok {
				match = true
				break
			}
		}
		if match {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// topologyContextSummary builds a concise summary of the planner's doc-driven topology hints.
func (d *Decomposer) topologyContextSummary() string {
	if d.kernel == nil {
		return ""
	}

	var sb strings.Builder

	// Proposed phases (active layers)
	phaseSet := make(map[string]struct{})
	if facts, err := d.kernel.Query("proposed_phase"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) == 0 {
				continue
			}
			phase := fmt.Sprintf("%v", fact.Args[0])
			if phase != "" {
				phaseSet[phase] = struct{}{}
			}
		}
	}
	if len(phaseSet) > 0 {
		phases := make([]string, 0, len(phaseSet))
		for p := range phaseSet {
			phases = append(phases, p)
		}
		sort.Strings(phases)
		sb.WriteString("- Proposed phases: ")
		sb.WriteString(strings.Join(phases, ", "))
		sb.WriteString("\n")
	}

	// Dependencies between layers
	deps := make([]string, 0)
	if facts, err := d.kernel.Query("phase_dependency_generated"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) < 2 {
				continue
			}
			deps = append(deps, fmt.Sprintf("%v -> %v", fact.Args[0], fact.Args[1]))
		}
	}
	if len(deps) > 0 {
		sort.Strings(deps)
		sb.WriteString("- Generated ordering:\n")
		for i, dep := range deps {
			if i >= 6 {
				break
			}
			sb.WriteString("  * ")
			sb.WriteString(dep)
			sb.WriteString("\n")
		}
	}

	// Context scope per phase (sample)
	scope := make(map[string][]string)
	if facts, err := d.kernel.Query("phase_context_scope"); err == nil {
		for _, fact := range facts {
			if len(fact.Args) < 2 {
				continue
			}
			phase := fmt.Sprintf("%v", fact.Args[0])
			doc := fmt.Sprintf("%v", fact.Args[1])
			if phase == "" || doc == "" {
				continue
			}
			if len(scope[phase]) < 3 {
				scope[phase] = append(scope[phase], doc)
			}
		}
	}
	if len(scope) > 0 {
		keys := make([]string, 0, len(scope))
		for k := range scope {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 3 {
			keys = keys[:3]
		}
		sb.WriteString("- Context scope (sample):\n")
		for _, phase := range keys {
			sb.WriteString("  * ")
			sb.WriteString(phase)
			sb.WriteString(": ")
			sb.WriteString(strings.Join(scope[phase], ", "))
			sb.WriteString("\n")
		}
	}

	// Conflicts (if any)
	conflicts := make([]string, 0, 3)
	if facts, err := d.kernel.Query("doc_conflict"); err == nil {
		for _, fact := range facts {
			if len(conflicts) >= 3 {
				break
			}
			if len(fact.Args) < 3 {
				continue
			}
			conflicts = append(conflicts, fmt.Sprintf("%v crosses %v vs %v", fact.Args[0], fact.Args[1], fact.Args[2]))
		}
	}
	if len(conflicts) > 0 {
		sb.WriteString("- Potentially broad docs:\n")
		for _, c := range conflicts {
			sb.WriteString("  * ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String())
}

// generateDiscoveryQuestions creates targeted retrieval questions from the goal.
func (d *Decomposer) generateDiscoveryQuestions(goal string) []string {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil
	}

	base := []string{
		"What are the functional requirements?",
		"What are the security and compliance requirements?",
		"What integration or API contracts are required?",
		"What UI/UX or branding constraints exist?",
	}

	questions := make([]string, 0, len(base)+2)
	for _, q := range base {
		questions = append(questions, fmt.Sprintf("%s (Goal: %s)", q, goal))
	}

	// Add a targeted ask using the goal keyword directly.
	questions = append(questions,
		fmt.Sprintf("Key specifications related to: %s", goal),
		fmt.Sprintf("Edge cases and non-functional requirements for: %s", goal),
	)

	return questions
}

// extractTopicsFromGoal tokenizes a goal into lowercase topics for Mangle selection.
func extractTopicsFromGoal(goal string) []string {
	goal = strings.ToLower(goal)
	if goal == "" {
		return nil
	}

	re := regexp.MustCompile(`[a-z0-9]+`)
	matches := re.FindAllString(goal, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	topics := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		topics = append(topics, m)
	}

	return topics
}

// deriveTagsFromPath converts structured folder/file names into tag tokens.
func deriveTagsFromPath(path string) []string {
	clean := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(clean, "/")
	tags := make(map[string]struct{})
	rePrefix := regexp.MustCompile(`^\d+[-_]?`)

	for _, p := range parts {
		base := strings.ToLower(strings.TrimSuffix(p, filepath.Ext(p)))
		base = rePrefix.ReplaceAllString(base, "")
		if base == "" {
			continue
		}
		tags[base] = struct{}{}
		for _, seg := range strings.Split(base, "-") {
			if seg != "" {
				tags[seg] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(tags))
	for t := range tags {
		out = append(out, t)
	}
	return out
}

// RawPlan represents the LLM's proposed plan structure.
type RawPlan struct {
	Title      string     `json:"title"`
	Confidence float64    `json:"confidence"`
	Phases     []RawPhase `json:"phases"`
}

// RawPhase represents a proposed phase.
type RawPhase struct {
	Name               string    `json:"name"`
	Order              int       `json:"order"`
	Description        string    `json:"description"`
	Category           string    `json:"category"`
	ObjectiveType      string    `json:"objective_type"`
	VerificationMethod string    `json:"verification_method"`
	Complexity         string    `json:"complexity"`
	DependsOn          []int     `json:"depends_on"` // Indices of dependent phases
	Tasks              []RawTask `json:"tasks"`
	FocusPatterns      []string  `json:"focus_patterns"`
	RequiredTools      []string  `json:"required_tools"`
}

// RawTask represents a proposed task.
type RawTask struct {
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Order       int      `json:"order,omitempty"`
	DependsOn   []int    `json:"depends_on"` // Indices of dependent tasks in same phase
	Artifacts   []string `json:"artifacts"`

	// Shard routing (optional - enables explicit shard selection)
	Shard       string `json:"shard,omitempty"`        // Which shard to use (e.g., "coder", "researcher")
	ShardInput  string `json:"shard_input,omitempty"`  // Full input to pass to shard
	ContextFrom []int  `json:"context_from,omitempty"` // Task indices to pull results from for context
}

// llmProposePlan asks LLM to propose a plan structure using retrieved context.
func (d *Decomposer) llmProposePlan(ctx context.Context, campaignID string, req DecomposeRequest, kbPath string, files []FileMetadata, requirements []Requirement) (*RawPlan, error) {
	timer := logging.StartTimer(logging.CategoryCampaign, "llmProposePlan")
	defer timer.Stop()

	logging.Campaign("Requesting LLM plan proposal")
	logging.CampaignDebug("Context: files=%d, requirements=%d, hints=%d",
		len(files), len(requirements), len(req.UserHints))

	// Build context
	var contextBuilder strings.Builder

	// Add goal
	contextBuilder.WriteString(fmt.Sprintf("GOAL: %s\n\n", req.Goal))

	// Add campaign type context
	contextBuilder.WriteString(fmt.Sprintf("CAMPAIGN TYPE: %s\n\n", req.CampaignType))

	// Add user hints
	if len(req.UserHints) > 0 {
		contextBuilder.WriteString("USER HINTS:\n")
		for _, hint := range req.UserHints {
			contextBuilder.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		contextBuilder.WriteString("\n")
	}

	// Add requirements summary
	if len(requirements) > 0 {
		contextBuilder.WriteString("EXTRACTED REQUIREMENTS:\n")
		for _, r := range requirements {
			contextBuilder.WriteString(fmt.Sprintf("- [%s] %s (Priority: %s)\n", r.ID, r.Description, r.Priority))
		}
		contextBuilder.WriteString("\n")
	}

	// Add strict build taxonomy guidance
	contextBuilder.WriteString(TaxonomyLogic)
	contextBuilder.WriteString("\n\n")

	// Add source metadata
	if len(files) > 0 {
		contextBuilder.WriteString("SOURCE DOCUMENTS (metadata):\n")
		for _, f := range files {
			contextBuilder.WriteString(fmt.Sprintf("- %s (%s, %d bytes, modified %s)\n", f.Path, f.Type, f.SizeBytes, f.ModifiedAt.Format(time.RFC3339)))
		}
		contextBuilder.WriteString("\n")
	}

	// Add topology hints derived from document layers
	if topo := d.topologyContextSummary(); topo != "" {
		contextBuilder.WriteString("TOPOLOGY HINTS:\n")
		contextBuilder.WriteString(topo)
		contextBuilder.WriteString("\n\n")
	}

	// Add available shards for shard-aware planning
	if d.shardLister != nil {
		shards := d.shardLister.ListAvailableShards()
		if shardList := formatShardList(shards); shardList != "" {
			contextBuilder.WriteString(shardList)
			contextBuilder.WriteString("\n")
			logging.Campaign("Shard-aware planning: injected %d available shards into prompt", len(shards))
		}
	} else {
		logging.Campaign("Shard-aware planning: shardLister is nil, skipping shard injection")
	}

	// Retrieve goal-focused snippets for context
	if kbPath != "" {
		if ls, err := store.NewLocalStore(kbPath); err == nil {
			defer ls.Close()
			entries, _ := ls.VectorRecallSemanticFiltered(ctx, req.Goal, 6, "campaign_id", campaignID)
			if len(entries) == 0 {
				entries, _ = ls.VectorRecallSemantic(ctx, req.Goal, 6)
			}
			if len(entries) > 0 {
				contextBuilder.WriteString("RETRIEVED SNIPPETS:\n")
				for idx, e := range entries {
					path := ""
					if p, ok := e.Metadata["path"].(string); ok {
						path = p
					}
					contextBuilder.WriteString(fmt.Sprintf("--- Snippet %d (%s) ---\n%s\n", idx+1, path, e.Content))
				}
				contextBuilder.WriteString("\n")
			}
		}
	}

	// Get Planner prompt (JIT or static)
	plannerPrompt, err := d.promptProvider.GetPrompt(ctx, RolePlanner, campaignID)
	if err != nil {
		logging.CampaignDebug("Failed to get Planner prompt, using fallback: %v", err)
		plannerPrompt = PlannerLogic
	}

	prompt := fmt.Sprintf(`%s

%s

Output ONLY valid JSON:`, plannerPrompt, contextBuilder.String())

	logging.CampaignDebug("Sending plan proposal request to LLM (prompt length=%d)", len(prompt))
	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM plan proposal failed: %v", err)
		return nil, err
	}
	logging.CampaignDebug("LLM response received (length=%d)", len(resp))

	// Parse response
	resp = cleanJSONResponse(resp)
	var plan RawPlan
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to parse plan JSON: %v", err)
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	logging.Campaign("Plan proposed: %s (confidence=%.2f, phases=%d)", plan.Title, plan.Confidence, len(plan.Phases))
	for i, phase := range plan.Phases {
		logging.CampaignDebug("  Phase %d: %s (category=%s, tasks=%d)",
			i, phase.Name, phase.Category, len(phase.Tasks))
	}

	return &plan, nil
}

// buildCampaign converts a RawPlan to a Campaign.
func (d *Decomposer) buildCampaign(campaignID string, req DecomposeRequest, plan *RawPlan) *Campaign {
	logging.CampaignDebug("Building campaign structure from raw plan")
	logging.CampaignDebug("Raw plan: title=%s, confidence=%.2f, phases=%d", plan.Title, plan.Confidence, len(plan.Phases))

	now := time.Now()

	campaign := &Campaign{
		ID:              campaignID,
		Type:            req.CampaignType,
		Title:           plan.Title,
		Goal:            req.Goal,
		SourceMaterial:  req.SourcePaths,
		Status:          StatusValidating,
		CreatedAt:       now,
		UpdatedAt:       now,
		Confidence:      plan.Confidence,
		ContextBudget:   req.ContextBudget,
		Phases:          make([]Phase, 0),
		ContextProfiles: make([]ContextProfile, 0),
	}

	// Build phases
	phaseIDMap := make(map[int]string)      // Map phase order -> phaseID
	globalTaskIDMap := make(map[int]string) // Map global task index -> taskID (for cross-phase context_from)
	globalTaskIndex := 0                    // Running counter for global task indices
	for i, rawPhase := range plan.Phases {
		phaseID := fmt.Sprintf("/phase_%s_%d", campaignID[10:], i)
		phaseIDMap[i] = phaseID
		phaseOrder := rawPhase.Order
		if phaseOrder == 0 {
			phaseOrder = i
		}
		logging.CampaignDebug("Building phase %d: %s (category=%s, tasks=%d, deps=%v)",
			i, rawPhase.Name, rawPhase.Category, len(rawPhase.Tasks), rawPhase.DependsOn)

		// Create context profile
		profileID := fmt.Sprintf("/profile_%s_%d", campaignID[10:], i)
		contextProfile := ContextProfile{
			ID:              profileID,
			RequiredSchemas: []string{"file_topology", "symbol_graph", "diagnostic"},
			RequiredTools:   rawPhase.RequiredTools,
			FocusPatterns:   rawPhase.FocusPatterns,
		}

		// Load context profile
		d.kernel.LoadFacts(contextProfile.ToFacts())
		campaign.ContextProfiles = append(campaign.ContextProfiles, contextProfile)

		phase := Phase{
			ID:             phaseID,
			CampaignID:     campaignID,
			Name:           rawPhase.Name,
			Order:          phaseOrder,
			Category:       normalizeCategory(rawPhase.Category),
			Status:         PhasePending,
			ContextProfile: profileID,
			Objectives: []PhaseObjective{{
				Type:               ObjectiveType(rawPhase.ObjectiveType),
				Description:        rawPhase.Description,
				VerificationMethod: VerificationMethod(rawPhase.VerificationMethod),
			}},
			EstimatedTasks:      len(rawPhase.Tasks),
			EstimatedComplexity: rawPhase.Complexity,
			Tasks:               make([]Task, 0),
		}

		// Build dependencies
		for _, depIdx := range rawPhase.DependsOn {
			if depPhaseID, ok := phaseIDMap[depIdx]; ok {
				phase.Dependencies = append(phase.Dependencies, PhaseDependency{
					DependsOnPhaseID: depPhaseID,
					Type:             DepHard,
				})
				logging.CampaignDebug("Phase %s depends on %s (hard dependency)", phaseID, depPhaseID)
			} else {
				logging.Get(logging.CategoryCampaign).Warn("Phase %s references unknown dependency index %d", phaseID, depIdx)
			}
		}

		// Build tasks
		taskIDMap := make(map[int]string) // Phase-local map for depends_on
		logging.CampaignDebug("Building %d tasks for phase %s", len(rawPhase.Tasks), phaseID)
		for j, rawTask := range rawPhase.Tasks {
			taskID := fmt.Sprintf("/task_%s_%d_%d", campaignID[10:], i, j)
			taskIDMap[j] = taskID
			globalTaskIDMap[globalTaskIndex] = taskID // Track global index for cross-phase context_from
			globalTaskIndex++
			orderIndex := j
			if rawTask.Order > 0 {
				orderIndex = rawTask.Order
			}
			logging.CampaignDebug("Task %d: type=%s, priority=%s, artifacts=%d, deps=%v",
				j, rawTask.Type, rawTask.Priority, len(rawTask.Artifacts), rawTask.DependsOn)

			task := Task{
				ID:          taskID,
				PhaseID:     phaseID,
				Description: rawTask.Description,
				Status:      TaskPending,
				Type:        TaskType(rawTask.Type),
				Priority:    TaskPriority(rawTask.Priority),
				Order:       orderIndex,
				DependsOn:   make([]string, 0),
				Artifacts:   make([]TaskArtifact, 0),
				// Shard routing fields (explicit shard selection)
				Shard:       rawTask.Shard,
				ShardInput:  rawTask.ShardInput,
				ContextFrom: make([]string, 0),
			}
			if task.Priority == "" {
				task.Priority = PriorityNormal
			}

			// Task dependencies
			for _, depIdx := range rawTask.DependsOn {
				if depTaskID, ok := taskIDMap[depIdx]; ok {
					task.DependsOn = append(task.DependsOn, depTaskID)
					logging.CampaignDebug("Task %s depends on task %s", taskID, depTaskID)
				} else {
					logging.Get(logging.CategoryCampaign).Warn("Task %s references unknown dependency index %d", taskID, depIdx)
				}
			}

			// Context injection references (for shard-aware planning)
			// Use globalTaskIDMap for cross-phase references
			for _, ctxIdx := range rawTask.ContextFrom {
				if ctxTaskID, ok := globalTaskIDMap[ctxIdx]; ok {
					task.ContextFrom = append(task.ContextFrom, ctxTaskID)
					logging.CampaignDebug("Task %s will receive context from task %s (global index %d)", taskID, ctxTaskID, ctxIdx)
				} else {
					logging.Get(logging.CategoryCampaign).Warn("Task %s references unknown context source index %d", taskID, ctxIdx)
				}
			}

			// Log explicit shard routing if present
			if task.Shard != "" {
				logging.CampaignDebug("Task %s has explicit shard routing: %s", taskID, task.Shard)
			}

			// Artifacts
			for _, artifactPath := range rawTask.Artifacts {
				artifactType := "/source_file"
				if strings.Contains(artifactPath, "_test") || strings.Contains(artifactPath, "test_") {
					artifactType = "/test_file"
				}
				normalizedPath := normalizePath(artifactPath)
				task.Artifacts = append(task.Artifacts, TaskArtifact{
					Type: artifactType,
					Path: normalizedPath,
				})
			}

			phase.Tasks = append(phase.Tasks, task)
			campaign.TotalTasks++
		}

		campaign.Phases = append(campaign.Phases, phase)
		campaign.TotalPhases++
		logging.CampaignDebug("Added phase %s with %d tasks", phase.ID, len(phase.Tasks))
	}

	logging.Campaign("Campaign structure built: phases=%d, totalTasks=%d", campaign.TotalPhases, campaign.TotalTasks)
	return campaign
}

// validatePlan uses Mangle to validate the plan.
func (d *Decomposer) validatePlan(campaignID string) []PlanValidationIssue {
	logging.CampaignDebug("Validating plan via Mangle kernel")

	issues := make([]PlanValidationIssue, 0)

	// Let Mangle drive validation via validation_error facts
	facts, err := d.kernel.Query("validation_error")
	if err != nil {
		logging.CampaignDebug("No validation errors queried (or query failed): %v", err)
	} else {
		for _, fact := range facts {
			if len(fact.Args) >= 3 {
				phaseID := fmt.Sprintf("%v", fact.Args[0])
				issueType := fmt.Sprintf("%v", fact.Args[1])
				desc := fmt.Sprintf("%v", fact.Args[2])
				issues = append(issues, PlanValidationIssue{
					CampaignID:  campaignID,
					IssueType:   issueType,
					Description: fmt.Sprintf("%s: %s", phaseID, desc),
				})
				logging.CampaignDebug("Validation issue: [%s] %s", issueType, desc)
			}
		}
	}

	logging.CampaignDebug("Validation complete: %d issues found", len(issues))
	return issues
}

// refinePlan asks LLM to refine the plan based on validation issues.
func (d *Decomposer) refinePlan(ctx context.Context, plan *RawPlan, issues []PlanValidationIssue) (*RawPlan, error) {
	if len(issues) == 0 {
		return plan, nil
	}

	logging.Campaign("Refining plan to fix %d validation issues", len(issues))
	timer := logging.StartTimer(logging.CategoryCampaign, "refinePlan")
	defer timer.Stop()

	// Build issues summary
	var issuesSummary strings.Builder
	for _, issue := range issues {
		issuesSummary.WriteString(fmt.Sprintf("- [%s] %s\n", issue.IssueType, issue.Description))
	}

	// Serialize current plan
	planJSON, _ := json.MarshalIndent(plan, "", "  ")

	prompt := fmt.Sprintf(`The following plan has validation issues that need to be fixed:

CURRENT PLAN:
%s

ISSUES:
%s

Please fix these issues and output the corrected plan as JSON.
- For circular dependencies: adjust phase order or dependencies
- For unreachable tasks: add missing task definitions or fix dependency references

Output ONLY valid JSON with the same structure as the input:`, string(planJSON), issuesSummary.String())

	logging.CampaignDebug("Sending refinement request to LLM")
	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM refinement failed: %v", err)
		return nil, err
	}

	// Parse response
	resp = cleanJSONResponse(resp)
	var refinedPlan RawPlan
	if err := json.Unmarshal([]byte(resp), &refinedPlan); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to parse refined plan: %v", err)
		return nil, fmt.Errorf("failed to parse refined plan: %w", err)
	}

	logging.Campaign("Plan refined successfully: %s (phases=%d)", refinedPlan.Title, len(refinedPlan.Phases))
	return &refinedPlan, nil
}

// linkRequirementsToTasks links extracted requirements to tasks.
func (d *Decomposer) linkRequirementsToTasks(requirements []Requirement, campaign *Campaign) {
	logging.CampaignDebug("Linking %d requirements to campaign tasks", len(requirements))
	linkedCount := 0

	for i := range requirements {
		// Simple heuristic: match by keyword overlap
		reqWords := strings.Fields(strings.ToLower(requirements[i].Description))

		for _, phase := range campaign.Phases {
			for _, task := range phase.Tasks {
				taskWords := strings.Fields(strings.ToLower(task.Description))

				// Count matching words
				matches := 0
				for _, rw := range reqWords {
					for _, tw := range taskWords {
						if rw == tw && len(rw) > 3 { // Ignore short words
							matches++
						}
					}
				}

				// If significant overlap, link
				if matches >= 2 {
					requirements[i].CoveredBy = append(requirements[i].CoveredBy, task.ID)
					linkedCount++
					logging.CampaignDebug("Linked requirement %s to task %s (matches=%d)",
						requirements[i].ID, task.ID, matches)
				}
			}
		}
	}

	// Load requirement coverage facts
	for _, req := range requirements {
		for _, taskID := range req.CoveredBy {
			d.kernel.Assert(core.Fact{
				Predicate: "requirement_coverage",
				Args:      []interface{}{req.ID, taskID},
			})
		}
	}

	logging.CampaignDebug("Requirement linking complete: %d links created", linkedCount)
}

func limitString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// cleanJSONResponse removes markdown code fences from JSON response.
func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}

// formatShardList formats available shards for injection into the planner prompt.
func formatShardList(shards []core.ShardInfo) string {
	if len(shards) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("AVAILABLE SHARDS (you can specify any of these for tasks):\n")

	// Group by type for clarity
	groups := make(map[core.ShardType][]core.ShardInfo)
	for _, s := range shards {
		groups[s.Type] = append(groups[s.Type], s)
	}

	typeOrder := []core.ShardType{core.ShardTypeEphemeral, core.ShardTypePersistent, core.ShardTypeUser}
	for _, shardType := range typeOrder {
		if list, ok := groups[shardType]; ok && len(list) > 0 {
			typeLabel := "Ephemeral"
			switch shardType {
			case core.ShardTypePersistent:
				typeLabel = "Specialist"
			case core.ShardTypeUser:
				typeLabel = "User-defined"
			}
			sb.WriteString(fmt.Sprintf("\n[%s shards]\n", typeLabel))
			for _, s := range list {
				desc := s.Description
				if desc == "" {
					desc = "General purpose"
				}
				sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, desc))
			}
		}
	}

	sb.WriteString(`
SHARD ROUTING INSTRUCTIONS:
- For each task, you MAY specify "shard" to route to a specific shard
- Use "shard_input" to provide the exact input for the shard
- Use "context_from" to inject results from previous tasks (array of task indices)
- If shard is not specified, the system infers based on task type

IMPORTANT: For documentation tasks needing directory content awareness:
1. First use "researcher" shard to enumerate/read directory contents
2. Then use "coder" with context_from referencing the research task

Example task with explicit shard routing:
{
  "description": "Read contents of internal/core directory",
  "type": "/research",
  "shard": "researcher",
  "shard_input": "List all files in internal/core and summarize their purpose",
  "order": 0
}

Example task with context injection:
{
  "description": "Create agents.md for internal/core",
  "type": "/file_create",
  "shard": "coder",
  "context_from": [0],
  "order": 1
}
`)
	return sb.String()
}
