package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Decomposer creates campaign plans through LLM + Mangle collaboration.
// It parses messy specifications and user goals into structured, validated plans.
type Decomposer struct {
	kernel    *core.RealKernel
	llmClient perception.LLMClient
	workspace string
}

// NewDecomposer creates a new decomposer.
func NewDecomposer(kernel *core.RealKernel, llmClient perception.LLMClient, workspace string) *Decomposer {
	return &Decomposer{
		kernel:    kernel,
		llmClient: llmClient,
		workspace: workspace,
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

// Decompose creates a campaign plan through LLM + Mangle collaboration.
func (d *Decomposer) Decompose(ctx context.Context, req DecomposeRequest) (*DecomposeResult, error) {
	// Generate campaign ID
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	safeCampaignID := sanitizeCampaignID(campaignID)

	// Set defaults
	if req.ContextBudget == 0 {
		req.ContextBudget = 100000 // 100k tokens default
	}

	kbPath := filepath.Join(d.workspace, ".nerd", "campaigns", safeCampaignID, "knowledge.db")

	// Step 1: Ingest source documents
	sourceDocs, fileMeta, err := d.ingestSourceDocuments(ctx, campaignID, req.SourcePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest source documents: %w", err)
	}

	// Seed metadata + goal signals for Mangle-driven selection
	d.seedDocFacts(campaignID, req.Goal, fileMeta)

	// Step 1b: Ingest into campaign knowledge store (vectors + graph) for retrieval
	if err := d.ingestIntoKnowledgeStore(ctx, campaignID, kbPath, fileMeta); err != nil {
		fmt.Printf("[campaign] warning: knowledge ingestion failed: %v\n", err)
	}

	// Step 2: Extract requirements from source documents
	requirements, err := d.extractRequirementsSmart(ctx, campaignID, req.Goal, kbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract requirements: %w", err)
	}

	// Step 3: LLM proposes phases and tasks
	rawPlan, err := d.llmProposePlan(ctx, req, kbPath, fileMeta, requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to propose plan: %w", err)
	}

	// Step 4: Convert to Campaign structure
	campaign := d.buildCampaign(campaignID, req, rawPlan)
	campaign.SourceDocs = sourceDocs
	campaign.KnowledgeBase = kbPath

	// Step 5: Load into Mangle for validation
	facts := campaign.ToFacts()
	if err := d.kernel.LoadFacts(facts); err != nil {
		return nil, fmt.Errorf("failed to load campaign facts: %w", err)
	}

	// Step 6: Mangle validates (circular deps, unreachable tasks, etc.)
	issues := d.validatePlan(campaignID)

	// Step 7: If issues, attempt LLM refinement
	if len(issues) > 0 {
		refinedPlan, err := d.refinePlan(ctx, rawPlan, issues)
		if err == nil && refinedPlan != nil {
			campaign = d.buildCampaign(campaignID, req, refinedPlan)
			// Reload and revalidate
			d.kernel.Retract("campaign")
			d.kernel.Retract("campaign_phase")
			d.kernel.Retract("campaign_task")
			d.kernel.LoadFacts(campaign.ToFacts())
			issues = d.validatePlan(campaignID)
		}
	}

	// Step 8: Link requirements to tasks
	d.linkRequirementsToTasks(requirements, campaign)

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
	docs := make([]SourceDocument, 0)
	meta := make([]FileMetadata, 0)

	for _, path := range paths {
		// Check for cancellation between file reads
		select {
		case <-ctx.Done():
			return docs, meta, ctx.Err()
		default:
		}
		// Resolve path
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(d.workspace, path)
		}

		stat, err := os.Stat(fullPath)
		if err != nil {
			// Try glob pattern
			matches, _ := filepath.Glob(fullPath)
			if len(matches) == 0 {
				continue // Skip missing files
			}
			for _, match := range matches {
				mds, mmeta := d.readDocumentsFromPath(match, campaignID)
				docs = append(docs, mds...)
				meta = append(meta, mmeta...)
			}
			continue
		}

		if stat.IsDir() {
			mds, mmeta := d.readDocumentsFromDir(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		} else {
			mds, mmeta := d.readDocumentsFromPath(fullPath, campaignID)
			docs = append(docs, mds...)
			meta = append(meta, mmeta...)
		}
	}

	return docs, meta, nil
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
	meta = append(meta, FileMetadata{
		Path:       path,
		Type:       docType,
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime(),
	})
	return docs, meta
}

// ingestIntoKnowledgeStore persists all document chunks into the campaign knowledge DB (vectors + KG).
func (d *Decomposer) ingestIntoKnowledgeStore(ctx context.Context, campaignID, dbPath string, files []FileMetadata) error {
	if len(files) == 0 {
		return nil
	}

	ingestor, err := NewDocumentIngestor(dbPath, embedding.DefaultConfig())
	if err != nil {
		return err
	}
	defer ingestor.Close()

	for _, fm := range files {
		data, err := os.ReadFile(fm.Path)
		if err != nil {
			continue
		}
		payload := map[string]string{fm.Path: string(data)}
		_, _ = ingestor.Ingest(ctx, campaignID, payload)
	}

	return nil
}

// seedDocFacts pushes lightweight document metadata into the kernel for logic-based selection.
func (d *Decomposer) seedDocFacts(campaignID, goal string, files []FileMetadata) {
	if d.kernel == nil {
		return
	}

	// Campaign goal fact already loaded later; still record a preliminary goal signal for selection rules.
	_ = d.kernel.Assert(core.Fact{
		Predicate: "campaign_goal",
		Args:      []interface{}{campaignID, goal},
	})

	for _, fm := range files {
		_ = d.kernel.Assert(core.Fact{
			Predicate: "doc_metadata",
			Args:      []interface{}{campaignID, fm.Path, fm.Type, fm.SizeBytes, fm.ModifiedAt.Unix()},
		})
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
			prompt := fmt.Sprintf(`Analyze this source document chunk and extract discrete requirements.
Return JSON only:
{
  "requirements": [
    {"id": "REQ001", "description": "...", "priority": "/critical|/high|/normal|/low", "source": "filename"}
  ]
}

Document: %s
Chunk: %d of %d
Content:
%s
`, path, idx+1, len(chunks), chunk)

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
func (d *Decomposer) extractRequirementsSmart(ctx context.Context, campaignID, goal, kbPath string) ([]Requirement, error) {
	if d.llmClient == nil {
		return nil, nil
	}

	questions := d.generateDiscoveryQuestions(goal)
	if len(questions) == 0 {
		return nil, nil
	}

	store, err := store.NewLocalStore(kbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open knowledge store: %w", err)
	}
	defer store.Close()

	reqs := make([]Requirement, 0)
	seen := make(map[string]bool)
	reqCounter := 0

	for _, q := range questions {
		entries, err := store.VectorRecallSemanticFiltered(ctx, q, 6, "campaign_id", campaignID)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			continue
		}

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
			continue
		}

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
		}
	}

	return reqs, nil
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
	DependsOn   []int    `json:"depends_on"` // Indices of dependent tasks in same phase
	Artifacts   []string `json:"artifacts"`
}

// llmProposePlan asks LLM to propose a plan structure using retrieved context.
func (d *Decomposer) llmProposePlan(ctx context.Context, req DecomposeRequest, kbPath string, files []FileMetadata, requirements []Requirement) (*RawPlan, error) {
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

	// Add source metadata
	if len(files) > 0 {
		contextBuilder.WriteString("SOURCE DOCUMENTS (metadata):\n")
		for _, f := range files {
			contextBuilder.WriteString(fmt.Sprintf("- %s (%s, %d bytes, modified %s)\n", f.Path, f.Type, f.SizeBytes, f.ModifiedAt.Format(time.RFC3339)))
		}
		contextBuilder.WriteString("\n")
	}

	// Retrieve goal-focused snippets for context
	if kbPath != "" {
		if ls, err := store.NewLocalStore(kbPath); err == nil {
			defer ls.Close()
			entries, _ := ls.VectorRecallSemanticFiltered(ctx, req.Goal, 6, "campaign_id", fmt.Sprintf("/campaign_%s", sanitizeCampaignID(req.Goal)))
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

	prompt := fmt.Sprintf(`You are a project planning expert. Create a detailed, executable plan.

%s

Create a campaign plan with phases and tasks. Each phase should have:
- Clear objective and verification method
- Concrete, actionable tasks
- Proper dependencies
- Context focus patterns (file globs)
- Required tools

Task types: /file_create, /file_modify, /test_write, /test_run, /research, /shard_spawn, /tool_create, /verify, /document, /refactor, /integrate

Output JSON:
{
  "title": "Campaign Title",
  "confidence": 0.0-1.0,
  "phases": [
    {
      "name": "Phase Name",
      "order": 0,
      "description": "What this phase accomplishes",
      "objective_type": "/create|/modify|/test|/research|/validate|/integrate|/review",
      "verification_method": "/tests_pass|/builds|/manual_review|/shard_validation|/none",
      "complexity": "/low|/medium|/high|/critical",
      "depends_on": [phase_indices],
      "focus_patterns": ["internal/core/*", "pkg/**/*.go"],
      "required_tools": ["fs_read", "fs_write", "exec_cmd"],
      "tasks": [
        {
          "description": "Specific task description",
          "type": "/file_create|/file_modify|/test_write|/test_run|/research|/verify|/document",
          "priority": "/critical|/high|/normal|/low",
          "depends_on": [task_indices_in_this_phase],
          "artifacts": ["/path/to/file.go"]
        }
      ]
    }
  ]
}

Output ONLY valid JSON:`, contextBuilder.String())

	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	resp = cleanJSONResponse(resp)
	var plan RawPlan
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return &plan, nil
}

// buildCampaign converts a RawPlan to a Campaign.
func (d *Decomposer) buildCampaign(campaignID string, req DecomposeRequest, plan *RawPlan) *Campaign {
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
	phaseIDMap := make(map[int]string) // Map order -> phaseID
	for i, rawPhase := range plan.Phases {
		phaseID := fmt.Sprintf("/phase_%s_%d", campaignID[10:], i)
		phaseIDMap[i] = phaseID

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
			Order:          rawPhase.Order,
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
			}
		}

		// Build tasks
		taskIDMap := make(map[int]string)
		for j, rawTask := range rawPhase.Tasks {
			taskID := fmt.Sprintf("/task_%s_%d_%d", campaignID[10:], i, j)
			taskIDMap[j] = taskID

			task := Task{
				ID:          taskID,
				PhaseID:     phaseID,
				Description: rawTask.Description,
				Status:      TaskPending,
				Type:        TaskType(rawTask.Type),
				Priority:    TaskPriority(rawTask.Priority),
				DependsOn:   make([]string, 0),
				Artifacts:   make([]TaskArtifact, 0),
			}

			// Task dependencies
			for _, depIdx := range rawTask.DependsOn {
				if depTaskID, ok := taskIDMap[depIdx]; ok {
					task.DependsOn = append(task.DependsOn, depTaskID)
				}
			}

			// Artifacts
			for _, artifactPath := range rawTask.Artifacts {
				artifactType := "/source_file"
				if strings.Contains(artifactPath, "_test") || strings.Contains(artifactPath, "test_") {
					artifactType = "/test_file"
				}
				task.Artifacts = append(task.Artifacts, TaskArtifact{
					Type: artifactType,
					Path: artifactPath,
				})
			}

			phase.Tasks = append(phase.Tasks, task)
			campaign.TotalTasks++
		}

		campaign.Phases = append(campaign.Phases, phase)
		campaign.TotalPhases++
	}

	return campaign
}

// validatePlan uses Mangle to validate the plan.
func (d *Decomposer) validatePlan(campaignID string) []PlanValidationIssue {
	issues := make([]PlanValidationIssue, 0)

	// Let Mangle drive validation via validation_error facts
	facts, err := d.kernel.Query("validation_error")
	if err == nil {
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
			}
		}
	}

	return issues
}

// refinePlan asks LLM to refine the plan based on validation issues.
func (d *Decomposer) refinePlan(ctx context.Context, plan *RawPlan, issues []PlanValidationIssue) (*RawPlan, error) {
	if len(issues) == 0 {
		return plan, nil
	}

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

	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
	resp = cleanJSONResponse(resp)
	var refinedPlan RawPlan
	if err := json.Unmarshal([]byte(resp), &refinedPlan); err != nil {
		return nil, fmt.Errorf("failed to parse refined plan: %w", err)
	}

	return &refinedPlan, nil
}

// linkRequirementsToTasks links extracted requirements to tasks.
func (d *Decomposer) linkRequirementsToTasks(requirements []Requirement, campaign *Campaign) {
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
}

// cleanJSONResponse removes markdown code fences from JSON response.
func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return strings.TrimSpace(resp)
}
