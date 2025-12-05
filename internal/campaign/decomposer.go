package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
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
	kernel     *core.RealKernel
	llmClient  perception.LLMClient
	workspace  string
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
	Goal           string       // High-level goal description
	SourcePaths    []string     // Paths to spec docs, requirements, etc.
	CampaignType   CampaignType // Type of campaign
	UserHints      []string     // Optional user guidance
	MaxPhases      int          // Max phases (0 = unlimited)
	ContextBudget  int          // Token budget (0 = default 100k)
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

	// Set defaults
	if req.ContextBudget == 0 {
		req.ContextBudget = 100000 // 100k tokens default
	}

	// Step 1: Ingest source documents
	sourceDocs, sourceContent, err := d.ingestSourceDocuments(ctx, campaignID, req.SourcePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest source documents: %w", err)
	}

	// Step 2: Extract requirements from source documents
	requirements, err := d.extractRequirements(ctx, campaignID, sourceContent)
	if err != nil {
		return nil, fmt.Errorf("failed to extract requirements: %w", err)
	}

	// Step 3: LLM proposes phases and tasks
	rawPlan, err := d.llmProposePlan(ctx, req, sourceContent, requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to propose plan: %w", err)
	}

	// Step 4: Convert to Campaign structure
	campaign := d.buildCampaign(campaignID, req, rawPlan)

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

// ingestSourceDocuments reads and parses source documents.
func (d *Decomposer) ingestSourceDocuments(ctx context.Context, campaignID string, paths []string) ([]SourceDocument, map[string]string, error) {
	docs := make([]SourceDocument, 0)
	content := make(map[string]string)

	for _, path := range paths {
		// Resolve path
		fullPath := path
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(d.workspace, path)
		}

		// Read file
		data, err := os.ReadFile(fullPath)
		if err != nil {
			// Try glob pattern
			matches, _ := filepath.Glob(fullPath)
			if len(matches) == 0 {
				continue // Skip missing files
			}
			for _, match := range matches {
				data, err = os.ReadFile(match)
				if err != nil {
					continue
				}
				docType := d.inferDocType(match)
				docs = append(docs, SourceDocument{
					CampaignID: campaignID,
					Path:       match,
					Type:       docType,
					ParsedAt:   time.Now(),
				})
				content[match] = string(data)
			}
			continue
		}

		docType := d.inferDocType(fullPath)
		docs = append(docs, SourceDocument{
			CampaignID: campaignID,
			Path:       fullPath,
			Type:       docType,
			ParsedAt:   time.Now(),
		})
		content[fullPath] = string(data)
	}

	return docs, content, nil
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

	// Combine content for analysis
	var combined strings.Builder
	for path, text := range content {
		combined.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", path, text))
	}

	// Limit content size
	contentStr := combined.String()
	if len(contentStr) > 50000 {
		contentStr = contentStr[:50000] + "\n...[truncated]..."
	}

	prompt := fmt.Sprintf(`Analyze these source documents and extract discrete requirements.

For each requirement, output JSON:
{
  "requirements": [
    {"id": "REQ001", "description": "...", "priority": "/critical|/high|/normal|/low", "source": "filename"},
    ...
  ]
}

Source Documents:
%s

Output ONLY valid JSON:`, contentStr)

	resp, err := d.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse response
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
		// Return empty if parsing fails
		return nil, nil
	}

	requirements := make([]Requirement, len(parsed.Requirements))
	for i, r := range parsed.Requirements {
		requirements[i] = Requirement{
			ID:          fmt.Sprintf("/req_%s_%s", campaignID[10:], r.ID),
			CampaignID:  campaignID,
			Description: r.Description,
			Priority:    r.Priority,
			Source:      r.Source,
		}
	}

	return requirements, nil
}

// RawPlan represents the LLM's proposed plan structure.
type RawPlan struct {
	Title      string     `json:"title"`
	Confidence float64    `json:"confidence"`
	Phases     []RawPhase `json:"phases"`
}

// RawPhase represents a proposed phase.
type RawPhase struct {
	Name               string   `json:"name"`
	Order              int      `json:"order"`
	Description        string   `json:"description"`
	ObjectiveType      string   `json:"objective_type"`
	VerificationMethod string   `json:"verification_method"`
	Complexity         string   `json:"complexity"`
	DependsOn          []int    `json:"depends_on"` // Indices of dependent phases
	Tasks              []RawTask `json:"tasks"`
	FocusPatterns      []string `json:"focus_patterns"`
	RequiredTools      []string `json:"required_tools"`
}

// RawTask represents a proposed task.
type RawTask struct {
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	DependsOn   []int    `json:"depends_on"` // Indices of dependent tasks in same phase
	Artifacts   []string `json:"artifacts"`
}

// llmProposePlan asks LLM to propose a plan structure.
func (d *Decomposer) llmProposePlan(ctx context.Context, req DecomposeRequest, content map[string]string, requirements []Requirement) (*RawPlan, error) {
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

	// Add source content (truncated)
	contextBuilder.WriteString("SOURCE DOCUMENTS:\n")
	for path, text := range content {
		if len(text) > 10000 {
			text = text[:10000] + "\n...[truncated]..."
		}
		contextBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", path, text))
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
		ID:             campaignID,
		Type:           req.CampaignType,
		Title:          plan.Title,
		Goal:           req.Goal,
		SourceMaterial: req.SourcePaths,
		Status:         StatusValidating,
		CreatedAt:      now,
		UpdatedAt:      now,
		Confidence:     plan.Confidence,
		ContextBudget:  req.ContextBudget,
		Phases:         make([]Phase, 0),
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

	// Query for validation issues
	facts, err := d.kernel.Query("plan_validation_issue")
	if err == nil {
		for _, fact := range facts {
			if len(fact.Args) >= 3 {
				if factCampaignID, ok := fact.Args[0].(string); ok && factCampaignID == campaignID {
					issues = append(issues, PlanValidationIssue{
						CampaignID:  campaignID,
						IssueType:   fmt.Sprintf("%v", fact.Args[1]),
						Description: fmt.Sprintf("%v", fact.Args[2]),
					})
				}
			}
		}
	}

	// Additional validation: check for circular dependencies
	circularDeps := d.detectCircularDependencies(campaignID)
	for _, dep := range circularDeps {
		issues = append(issues, PlanValidationIssue{
			CampaignID:  campaignID,
			IssueType:   "/circular_dependency",
			Description: dep,
		})
	}

	// Check for unreachable tasks
	unreachable := d.detectUnreachableTasks(campaignID)
	for _, task := range unreachable {
		issues = append(issues, PlanValidationIssue{
			CampaignID:  campaignID,
			IssueType:   "/unreachable_task",
			Description: task,
		})
	}

	return issues
}

// detectCircularDependencies checks for circular phase dependencies.
func (d *Decomposer) detectCircularDependencies(campaignID string) []string {
	issues := make([]string, 0)

	// Get all phases
	phaseFacts, _ := d.kernel.Query("campaign_phase")
	phases := make(map[string]int) // phaseID -> order

	for _, fact := range phaseFacts {
		if len(fact.Args) >= 5 {
			if factCampaignID, ok := fact.Args[1].(string); ok && factCampaignID == campaignID {
				phaseID := fmt.Sprintf("%v", fact.Args[0])
				order := 0
				if o, ok := fact.Args[3].(int); ok {
					order = o
				} else if o, ok := fact.Args[3].(int64); ok {
					order = int(o)
				}
				phases[phaseID] = order
			}
		}
	}

	// Get dependencies
	depFacts, _ := d.kernel.Query("phase_dependency")
	for _, fact := range depFacts {
		if len(fact.Args) >= 2 {
			phaseID := fmt.Sprintf("%v", fact.Args[0])
			depPhaseID := fmt.Sprintf("%v", fact.Args[1])

			if phaseOrder, ok := phases[phaseID]; ok {
				if depOrder, depOk := phases[depPhaseID]; depOk {
					if depOrder >= phaseOrder {
						issues = append(issues, fmt.Sprintf("Phase %s depends on %s but has equal or earlier order", phaseID, depPhaseID))
					}
				}
			}
		}
	}

	return issues
}

// detectUnreachableTasks checks for tasks with unresolvable dependencies.
func (d *Decomposer) detectUnreachableTasks(campaignID string) []string {
	issues := make([]string, 0)

	// Get all tasks
	taskFacts, _ := d.kernel.Query("campaign_task")
	tasks := make(map[string]bool) // taskID -> exists

	for _, fact := range taskFacts {
		if len(fact.Args) >= 2 {
			taskID := fmt.Sprintf("%v", fact.Args[0])
			tasks[taskID] = true
		}
	}

	// Get dependencies
	depFacts, _ := d.kernel.Query("task_dependency")
	for _, fact := range depFacts {
		if len(fact.Args) >= 2 {
			taskID := fmt.Sprintf("%v", fact.Args[0])
			depTaskID := fmt.Sprintf("%v", fact.Args[1])

			if _, ok := tasks[taskID]; ok {
				if _, depOk := tasks[depTaskID]; !depOk {
					issues = append(issues, fmt.Sprintf("Task %s depends on non-existent task %s", taskID, depTaskID))
				}
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
