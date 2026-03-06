// Package campaign implements the Campaign Orchestrator for long-running,
// multi-phase goal execution with context management.
//
// Campaigns are used for:
//   - Greenfield builds from spec documents
//   - Large feature implementations
//   - Codebase-wide stability audits
//   - Migration projects
//   - Any multi-step goal requiring sustained context management
//
// The Campaign Orchestrator follows the OODA loop but with phase-aware
// context paging, progressive refinement, and autopoiesis.
package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CampaignType represents the type of campaign.
type CampaignType string

const (
	CampaignTypeGreenfield         CampaignType = "/greenfield"          // Build from scratch
	CampaignTypeFeature            CampaignType = "/feature"             // Add major feature
	CampaignTypeAudit              CampaignType = "/audit"               // Stability/security audit
	CampaignTypeMigration          CampaignType = "/migration"           // Technology migration
	CampaignTypeRemediation        CampaignType = "/remediation"         // Fix issues across codebase
	CampaignTypeAdversarialAssault CampaignType = "/adversarial_assault" // Long-horizon stress + adversarial probing
	CampaignTypeCustom             CampaignType = "/custom"              // User-defined campaign
)

// CampaignStatus represents the current status of a campaign.
type CampaignStatus string

const (
	StatusPlanning    CampaignStatus = "/planning"    // Initial planning phase
	StatusDecomposing CampaignStatus = "/decomposing" // Breaking down into phases/tasks
	StatusValidating  CampaignStatus = "/validating"  // Validating the plan
	StatusActive      CampaignStatus = "/active"      // Executing
	StatusPaused      CampaignStatus = "/paused"      // Paused (user or system)
	StatusCompleted   CampaignStatus = "/completed"   // Successfully completed
	StatusFailed      CampaignStatus = "/failed"      // Failed (unrecoverable)
)

// PhaseStatus represents the status of a campaign phase.
type PhaseStatus string

const (
	PhasePending    PhaseStatus = "/pending"     // Not started
	PhaseInProgress PhaseStatus = "/in_progress" // Currently executing
	PhaseCompleted  PhaseStatus = "/completed"   // Finished successfully
	PhaseFailed     PhaseStatus = "/failed"      // Failed
	PhaseSkipped    PhaseStatus = "/skipped"     // Skipped (user decision or dependency)
)

// TaskStatus represents the status of a campaign task.
type TaskStatus string

const (
	TaskPending    TaskStatus = "/pending"     // Not started
	TaskInProgress TaskStatus = "/in_progress" // Currently executing
	TaskCompleted  TaskStatus = "/completed"   // Finished successfully
	TaskFailed     TaskStatus = "/failed"      // Failed
	TaskSkipped    TaskStatus = "/skipped"     // Skipped
	TaskBlocked    TaskStatus = "/blocked"     // Blocked by dependency
)

// TaskType represents the type of task.
type TaskType string

const (
	TaskTypeFileCreate  TaskType = "/file_create"  // Create a new file
	TaskTypeFileModify  TaskType = "/file_modify"  // Modify existing file
	TaskTypeTestWrite   TaskType = "/test_write"   // Write tests
	TaskTypeTestRun     TaskType = "/test_run"     // Run tests
	TaskTypeResearch    TaskType = "/research"     // Deep research (spawns researcher shard)
	TaskTypeShardSpawn  TaskType = "/shard_spawn"  // Spawn a shard agent
	TaskTypeToolCreate  TaskType = "/tool_create"  // Create a new tool (autopoiesis)
	TaskTypeVerify      TaskType = "/verify"       // Verification step
	TaskTypeDocument    TaskType = "/document"     // Documentation
	TaskTypeRefactor    TaskType = "/refactor"     // Refactoring
	TaskTypeIntegrate   TaskType = "/integrate"    // Integration step
	TaskTypeCampaignRef TaskType = "/campaign_ref" // Reference to a sub-campaign

	// Adversarial Assault campaign task types (durable, batched).
	TaskTypeAssaultDiscover TaskType = "/assault_discover" // Enumerate targets + create batch tasks
	TaskTypeAssaultBatch    TaskType = "/assault_batch"    // Execute a persisted batch
	TaskTypeAssaultTriage   TaskType = "/assault_triage"   // Summarize results + generate remediation tasks
)

// campaign_ref lifecycle contract states.
const (
	CampaignRefLifecycleLinked    = "/linked"    // Reference exists, target status unknown/not yet visible
	CampaignRefLifecycleQueued    = "/queued"    // Target campaign exists but not actively executing
	CampaignRefLifecycleActive    = "/active"    // Target campaign currently executing
	CampaignRefLifecyclePaused    = "/paused"    // Target campaign paused
	CampaignRefLifecycleCompleted = "/completed" // Target campaign completed successfully
	CampaignRefLifecycleFailed    = "/failed"    // Target campaign failed
)

// TaskPriority represents task priority levels.
type TaskPriority string

const (
	PriorityCritical TaskPriority = "/critical"
	PriorityHigh     TaskPriority = "/high"
	PriorityNormal   TaskPriority = "/normal"
	PriorityLow      TaskPriority = "/low"
)

// ObjectiveType represents the type of phase objective.
type ObjectiveType string

const (
	ObjectiveCreate    ObjectiveType = "/create"
	ObjectiveModify    ObjectiveType = "/modify"
	ObjectiveTest      ObjectiveType = "/test"
	ObjectiveResearch  ObjectiveType = "/research"
	ObjectiveValidate  ObjectiveType = "/validate"
	ObjectiveIntegrate ObjectiveType = "/integrate"
	ObjectiveReview    ObjectiveType = "/review"
)

// VerificationMethod represents how a phase is verified.
type VerificationMethod string

const (
	VerifyTestsPass       VerificationMethod = "/tests_pass"
	VerifyBuilds          VerificationMethod = "/builds"
	VerifyManualReview    VerificationMethod = "/manual_review"
	VerifyShardValidate   VerificationMethod = "/shard_validation"
	VerifyNemesisGauntlet VerificationMethod = "/nemesis_gauntlet"
	VerifyNone            VerificationMethod = "/none"
)

// DependencyType represents the type of dependency between phases.
type DependencyType string

const (
	DepHard     DependencyType = "/hard"     // Must complete before dependent can start
	DepSoft     DependencyType = "/soft"     // Preferred but not required
	DepArtifact DependencyType = "/artifact" // Needs output artifact from dependency
)

// Campaign represents a long-running, multi-phase goal.
type Campaign struct {
	ID             string           `json:"id"`
	Type           CampaignType     `json:"type"`
	Title          string           `json:"title"`
	Goal           string           `json:"goal"`            // High-level goal description
	SourceMaterial []string         `json:"source_material"` // Paths to spec docs
	SourceDocs     []SourceDocument `json:"source_docs,omitempty"`
	KnowledgeBase  string           `json:"knowledge_base,omitempty"` // Path to campaign knowledge DB
	Status         CampaignStatus   `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Confidence     float64          `json:"confidence"` // LLM's confidence in the plan (0.0-1.0)

	// Structure
	Phases          []Phase          `json:"phases"`
	ContextProfiles []ContextProfile `json:"context_profiles,omitempty"`

	// Progress
	CompletedPhases int `json:"completed_phases"`
	TotalPhases     int `json:"total_phases"`
	CompletedTasks  int `json:"completed_tasks"`
	TotalTasks      int `json:"total_tasks"`

	// Context management
	ContextBudget      int     `json:"context_budget"`      // Total token budget
	ContextUsed        int     `json:"context_used"`        // Currently used tokens
	ContextUtilization float64 `json:"context_utilization"` // 0.0-1.0

	// Revision tracking
	RevisionNumber int    `json:"revision_number"`
	LastRevision   string `json:"last_revision_summary"`

	// Learnings (autopoiesis)
	Learnings []Learning `json:"learnings,omitempty"`

	// Optional: adversarial assault configuration (for CampaignTypeAdversarialAssault).
	Assault *AssaultConfig `json:"assault,omitempty"`
}

// Phase represents a distinct phase within a campaign.
type Phase struct {
	ID             string      `json:"id"`
	CampaignID     string      `json:"campaign_id"`
	Name           string      `json:"name"`
	Order          int         `json:"order"` // Execution order (0-based)
	Category       string      `json:"category"`
	Status         PhaseStatus `json:"status"`
	ContextProfile string      `json:"context_profile"` // ID of the context profile

	// Objectives
	Objectives []PhaseObjective `json:"objectives"`

	// Tasks
	Tasks []Task `json:"tasks"`

	// Dependencies
	Dependencies []PhaseDependency `json:"dependencies,omitempty"`

	// Estimates
	EstimatedTasks      int    `json:"estimated_tasks"`
	EstimatedComplexity string `json:"estimated_complexity"` // /low, /medium, /high, /critical

	// Checkpoints
	Checkpoints []Checkpoint `json:"checkpoints,omitempty"`

	// Compression (after completion)
	CompressedSummary string    `json:"compressed_summary,omitempty"`
	OriginalAtomCount int       `json:"original_atom_count,omitempty"`
	CompressedAt      time.Time `json:"compressed_at,omitempty"`
}

// PhaseObjective describes what a phase aims to accomplish.
type PhaseObjective struct {
	Type               ObjectiveType      `json:"type"`
	Description        string             `json:"description"`
	VerificationMethod VerificationMethod `json:"verification_method"`
}

// PhaseDependency represents a dependency between phases.
type PhaseDependency struct {
	DependsOnPhaseID string         `json:"depends_on_phase_id"`
	Type             DependencyType `json:"type"`
}

// Task represents an atomic unit of work within a phase.
type Task struct {
	ID          string       `json:"id"`
	PhaseID     string       `json:"phase_id"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status"`
	Type        TaskType     `json:"type"`
	Priority    TaskPriority `json:"priority"`
	Order       int          `json:"order"`

	// Dependencies
	DependsOn []string `json:"depends_on,omitempty"` // Task IDs this depends on
	SoftDeps  []string `json:"soft_deps,omitempty"`  // Soft dependencies (preferred order)
	Resources []string `json:"resources,omitempty"`  // Required resources (semaphores)
	WriteSet  []string `json:"write_set,omitempty"`  // Canonical file paths this task is allowed to mutate

	// Shard routing (explicit shard selection, overrides type-based inference)
	Shard       string   `json:"shard,omitempty"`        // Which shard to use (e.g., "coder", "researcher")
	ShardInput  string   `json:"shard_input,omitempty"`  // Full input to pass to shard
	ContextFrom []string `json:"context_from,omitempty"` // Task IDs to pull results from for context injection

	// Recursion
	SubCampaignID string `json:"sub_campaign_id,omitempty"` // If set, this task is a sub-campaign

	// Sub-campaign contract (TaskTypeCampaignRef)
	CampaignRefFailurePolicy CampaignRefFailurePolicy `json:"campaign_ref_failure_policy,omitempty"`
	CampaignRefInheritance   *CampaignRefInheritance  `json:"campaign_ref_inheritance,omitempty"`

	// Artifacts produced
	Artifacts []TaskArtifact `json:"artifacts,omitempty"`

	// Provenance
	InferredFrom    string  `json:"inferred_from,omitempty"` // What this was derived from
	InferenceConf   float64 `json:"inference_confidence"`    // Confidence of inference
	InferenceReason string  `json:"inference_reason,omitempty"`

	// Execution tracking
	Attempts  []TaskAttempt `json:"attempts,omitempty"`
	LastError string        `json:"last_error,omitempty"`
	// Backoff control (persisted for long-horizon durability)
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
}

// CampaignRefFailurePolicy controls how parent task status reacts to child status.
type CampaignRefFailurePolicy string

const (
	CampaignRefPolicyPropagate CampaignRefFailurePolicy = "/propagate"
	CampaignRefPolicyAbsorb    CampaignRefFailurePolicy = "/absorb"
	CampaignRefPolicyTransform CampaignRefFailurePolicy = "/transform"
)

// CampaignRefInheritance defines parent -> child scope inheritance.
type CampaignRefInheritance struct {
	FactsScope  string `json:"facts_scope,omitempty"`
	FSScope     string `json:"fs_scope,omitempty"`
	MemoryScope string `json:"memory_scope,omitempty"`
	ToolScope   string `json:"tool_scope,omitempty"`
}

// CampaignRefResult is the typed result envelope returned from /campaign_ref tasks.
type CampaignRefResult struct {
	Version        int                      `json:"version"`
	SubCampaignID  string                   `json:"sub_campaign_id"`
	Status         string                   `json:"status"`
	Artifacts      []string                 `json:"artifacts,omitempty"`
	LearnedFacts   []string                 `json:"learned_facts,omitempty"`
	Checkpoints    int                      `json:"checkpoints"`
	FailureSummary string                   `json:"failure_summary,omitempty"`
	FailurePolicy  CampaignRefFailurePolicy `json:"failure_policy"`
	Inheritance    CampaignRefInheritance   `json:"inheritance"`
}

// TaskArtifact represents an artifact produced by a task.
type TaskArtifact struct {
	Type string `json:"type"` // /source_file, /test_file, /config, /shard_agent, /knowledge_base, /doc
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"`
}

// TaskAttempt tracks an execution attempt for a task.
type TaskAttempt struct {
	Number    int       `json:"number"`
	Outcome   string    `json:"outcome"` // /success, /failure, /partial
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// Checkpoint represents a verification checkpoint for a phase.
type Checkpoint struct {
	Type      string    `json:"type"` // /tests, /build, /lint, /coverage, /manual, /integration
	Passed    bool      `json:"passed"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ContextProfile defines what context a phase needs.
type ContextProfile struct {
	ID              string   `json:"id"`
	RequiredSchemas []string `json:"required_schemas"` // Schema sections needed
	RequiredTools   []string `json:"required_tools"`   // Tools permitted
	FocusPatterns   []string `json:"focus_patterns"`   // Glob patterns for files
}

// Learning represents something learned during campaign execution.
type Learning struct {
	Type      string    `json:"type"` // /success_pattern, /failure_pattern, /preference, /optimization
	Pattern   string    `json:"pattern"`
	Fact      string    `json:"fact"`
	AppliedAt time.Time `json:"applied_at"`
}

// SourceDocument represents a source document ingested for the campaign.
type SourceDocument struct {
	CampaignID string    `json:"campaign_id"`
	Path       string    `json:"path"`
	Type       string    `json:"type"` // /spec, /requirements, /design, /readme, /api_doc, /tutorial
	ParsedAt   time.Time `json:"parsed_at"`
	Summary    string    `json:"summary,omitempty"`
}

// FileMetadata captures lightweight document metadata for selection and retrieval.
type FileMetadata struct {
	Path            string    `json:"path"`
	Type            string    `json:"type"`
	SizeBytes       int64     `json:"size_bytes"`
	ModifiedAt      time.Time `json:"modified_at"`
	Tags            []string  `json:"tags,omitempty"`
	Layer           string    `json:"layer,omitempty"`
	LayerConfidence float64   `json:"layer_confidence,omitempty"`
	LayerReason     string    `json:"layer_reason,omitempty"`
}

// Requirement represents a requirement extracted from source documents.
type Requirement struct {
	ID          string   `json:"id"`
	CampaignID  string   `json:"campaign_id"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Source      string   `json:"source"`               // Which document this came from
	CoveredBy   []string `json:"covered_by,omitempty"` // Task IDs that cover this
}

// CampaignShard tracks a shard spawned as part of campaign execution.
type CampaignShard struct {
	CampaignID string `json:"campaign_id"`
	ShardID    string `json:"shard_id"`
	ShardType  string `json:"shard_type"`
	Task       string `json:"task"`
	Status     string `json:"status"`
}

// ShardResult represents the result from a shard execution.
type ShardResult struct {
	ShardID    string    `json:"shard_id"`
	ResultType string    `json:"result_type"` // /success, /failure, /partial, /knowledge
	ResultData string    `json:"result_data"`
	Timestamp  time.Time `json:"timestamp"`
}

// Progress represents campaign progress for display.
type Progress struct {
	CampaignID      string   `json:"campaign_id"`
	CampaignTitle   string   `json:"campaign_title"`
	CampaignStatus  string   `json:"campaign_status"`
	CurrentPhase    string   `json:"current_phase"`
	CurrentPhaseIdx int      `json:"current_phase_idx"`
	CompletedPhases int      `json:"completed_phases"`
	TotalPhases     int      `json:"total_phases"`
	PhaseProgress   float64  `json:"phase_progress"`   // 0.0-1.0
	OverallProgress float64  `json:"overall_progress"` // 0.0-1.0
	CurrentTask     string   `json:"current_task"`
	CompletedTasks  int      `json:"completed_tasks"`
	TotalTasks      int      `json:"total_tasks"`
	ActiveShards    []string `json:"active_shards"`
	ContextUsage    float64  `json:"context_usage"` // 0.0-1.0
	Learnings       int      `json:"learnings_count"`
	Replans         int      `json:"replans_count"`
	Errors          []string `json:"errors,omitempty"`
}

// ReplanTrigger represents a reason to replan the campaign.
type ReplanTrigger struct {
	CampaignID  string    `json:"campaign_id"`
	Reason      string    `json:"reason"` // /task_failed, /new_requirement, /user_feedback, /dependency_change, /blocked
	TriggeredAt time.Time `json:"triggered_at"`
	Details     string    `json:"details,omitempty"`
}

// PlanValidationIssue represents an issue found during plan validation.
type PlanValidationIssue struct {
	CampaignID  string `json:"campaign_id"`
	IssueType   string `json:"issue_type"` // /missing_dependency, /circular_dependency, /unreachable_task, /ambiguous_goal
	Description string `json:"description"`
}

// ToFacts converts a Campaign to Mangle facts for kernel loading.
func (c *Campaign) ToFacts() []core.Fact {
	logging.CampaignDebug("Converting campaign %s to Mangle facts", c.ID)

	facts := make([]core.Fact, 0)

	source := ""
	if len(c.SourceMaterial) > 0 {
		source = c.SourceMaterial[0]
	}

	// Main campaign fact
	facts = append(facts, core.Fact{
		Predicate: "campaign",
		Args:      []interface{}{c.ID, string(c.Type), c.Title, source, string(c.Status)},
	})

	// Campaign metadata
	facts = append(facts, core.Fact{
		Predicate: "campaign_metadata",
		Args:      []interface{}{c.ID, c.CreatedAt.Unix(), len(c.Phases), int64(c.Confidence * 100)}, // Scale 0.0-1.0 → 0-100 integer
	})

	// Campaign goal
	facts = append(facts, core.Fact{
		Predicate: "campaign_goal",
		Args:      []interface{}{c.ID, c.Goal},
	})

	// Progress
	facts = append(facts, core.Fact{
		Predicate: "campaign_progress",
		Args:      []interface{}{c.ID, c.CompletedPhases, c.TotalPhases, c.CompletedTasks, c.TotalTasks},
	})

	// Context profiles
	for i := range c.ContextProfiles {
		facts = append(facts, c.ContextProfiles[i].ToFacts()...)
	}

	// Source documents
	for _, doc := range c.SourceDocs {
		facts = append(facts, core.Fact{
			Predicate: "source_document",
			Args:      []interface{}{c.ID, doc.Path, doc.Type, doc.ParsedAt.Unix()},
		})
	}

	// Phases
	for i := range c.Phases {
		facts = append(facts, c.Phases[i].ToFacts()...)
	}

	logging.CampaignDebug("Campaign %s converted: %d total facts (phases=%d, profiles=%d, docs=%d)",
		c.ID, len(facts), len(c.Phases), len(c.ContextProfiles), len(c.SourceDocs))

	return facts
}

// ToFacts converts a Phase to Mangle facts.
func (p *Phase) ToFacts() []core.Fact {
	logging.CampaignDebug("Converting phase %s (%s) to Mangle facts", p.ID, p.Name)

	facts := make([]core.Fact, 0)

	// Phase fact
	facts = append(facts, core.Fact{
		Predicate: "campaign_phase",
		Args:      []interface{}{p.ID, p.CampaignID, p.Name, p.Order, string(p.Status), p.ContextProfile},
	})

	// Phase category for build topology enforcement
	category := normalizeCategory(p.Category)
	p.Category = category
	if category != "" {
		facts = append(facts, core.Fact{
			Predicate: "phase_category",
			Args:      []interface{}{p.ID, category},
		})
	}

	// Phase objectives
	for _, obj := range p.Objectives {
		facts = append(facts, core.Fact{
			Predicate: "phase_objective",
			Args:      []interface{}{p.ID, string(obj.Type), obj.Description, string(obj.VerificationMethod)},
		})
	}

	// Phase dependencies
	for _, dep := range p.Dependencies {
		facts = append(facts, core.Fact{
			Predicate: "phase_dependency",
			Args:      []interface{}{p.ID, dep.DependsOnPhaseID, string(dep.Type)},
		})
	}

	// Phase estimates
	facts = append(facts, core.Fact{
		Predicate: "phase_estimate",
		Args:      []interface{}{p.ID, p.EstimatedTasks, p.EstimatedComplexity},
	})

	// Tasks
	for idx := range p.Tasks {
		if p.Tasks[idx].Order == 0 {
			p.Tasks[idx].Order = idx
		}
		facts = append(facts, p.Tasks[idx].ToFacts()...)
	}

	// Compression (if completed)
	if p.CompressedSummary != "" {
		facts = append(facts, core.Fact{
			Predicate: "context_compression",
			Args:      []interface{}{p.ID, p.CompressedSummary, p.OriginalAtomCount, p.CompressedAt.Unix()},
		})
	}

	return facts
}

// ToFacts converts a Task to Mangle facts.
func (t *Task) ToFacts() []core.Fact {
	facts := make([]core.Fact, 0)

	// Task fact
	facts = append(facts, core.Fact{
		Predicate: "campaign_task",
		Args:      []interface{}{t.ID, t.PhaseID, t.Description, string(t.Status), string(t.Type)},
	})

	// Task priority
	facts = append(facts, core.Fact{
		Predicate: "task_priority",
		Args:      []interface{}{t.ID, string(t.Priority)},
	})

	// Task order (stable deterministic ordering)
	facts = append(facts, core.Fact{
		Predicate: "task_order",
		Args:      []interface{}{t.ID, t.Order},
	})

	// Task dependencies
	for _, depID := range t.DependsOn {
		facts = append(facts, core.Fact{
			Predicate: "task_dependency",
			Args:      []interface{}{t.ID, depID},
		})
	}

	// Soft dependencies
	for _, depID := range t.SoftDeps {
		facts = append(facts, core.Fact{
			Predicate: "task_soft_dependency",
			Args:      []interface{}{t.ID, depID},
		})
	}

	// Resources
	for _, res := range t.Resources {
		facts = append(facts, core.Fact{
			Predicate: "requires_resource",
			Args:      []interface{}{t.ID, res},
		})
	}

	// Recursion
	if t.SubCampaignID != "" {
		facts = append(facts, core.Fact{
			Predicate: "task_sub_campaign",
			Args:      []interface{}{t.ID, t.SubCampaignID},
		})
	}

	// Artifacts
	for _, artifact := range t.Artifacts {
		path := normalizePath(artifact.Path)
		facts = append(facts, core.Fact{
			Predicate: "task_artifact",
			Args:      []interface{}{t.ID, artifact.Type, path, artifact.Hash},
		})
	}

	// Inference
	if t.InferredFrom != "" {
		facts = append(facts, core.Fact{
			Predicate: "task_inference",
			Args:      []interface{}{t.ID, t.InferredFrom, t.InferenceConf, t.InferenceReason},
		})
	}

	// Attempts
	for _, attempt := range t.Attempts {
		facts = append(facts, core.Fact{
			Predicate: "task_attempt",
			Args:      []interface{}{t.ID, attempt.Number, attempt.Outcome, attempt.Timestamp.Unix()},
		})
	}

	// Retry backoff window
	if !t.NextRetryAt.IsZero() {
		facts = append(facts, core.Fact{
			Predicate: "task_retry_at",
			Args:      []interface{}{t.ID, t.NextRetryAt.Unix()},
		})
	}

	// Error
	if t.LastError != "" {
		facts = append(facts, core.Fact{
			Predicate: "task_error",
			Args:      []interface{}{t.ID, "execution_error", t.LastError},
		})
	}

	// Deterministic write contract
	for _, writePath := range t.DeterministicWriteSet() {
		facts = append(facts, core.Fact{
			Predicate: "task_write_target",
			Args:      []interface{}{t.ID, writePath},
		})
	}

	return facts
}

// DeterministicWriteSet returns a canonical, stable write_set for task contracts.
// It prefers explicit WriteSet contracts and falls back to Artifacts for compatibility.
func (t *Task) DeterministicWriteSet() []string {
	paths := make([]string, 0, len(t.WriteSet)+len(t.Artifacts))
	paths = append(paths, t.WriteSet...)
	if len(paths) == 0 {
		for _, artifact := range t.Artifacts {
			if artifact.Path != "" {
				paths = append(paths, artifact.Path)
			}
		}
	}
	if len(paths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(paths))
	writeSet := make([]string, 0, len(paths))
	for _, candidate := range paths {
		path := normalizePath(candidate)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		writeSet = append(writeSet, path)
	}
	sort.Strings(writeSet)
	return writeSet
}

func campaignRefLifecycleFromStatus(status CampaignStatus) string {
	switch status {
	case StatusPlanning, StatusDecomposing, StatusValidating:
		return CampaignRefLifecycleQueued
	case StatusActive:
		return CampaignRefLifecycleActive
	case StatusPaused:
		return CampaignRefLifecyclePaused
	case StatusCompleted:
		return CampaignRefLifecycleCompleted
	case StatusFailed:
		return CampaignRefLifecycleFailed
	default:
		return CampaignRefLifecycleLinked
	}
}

func campaignRefLifecycleContractMap() map[string]string {
	return map[string]string{
		string(StatusPlanning):    CampaignRefLifecycleQueued,
		string(StatusDecomposing): CampaignRefLifecycleQueued,
		string(StatusValidating):  CampaignRefLifecycleQueued,
		string(StatusActive):      CampaignRefLifecycleActive,
		string(StatusPaused):      CampaignRefLifecyclePaused,
		string(StatusCompleted):   CampaignRefLifecycleCompleted,
		string(StatusFailed):      CampaignRefLifecycleFailed,
		"/unknown":                CampaignRefLifecycleLinked,
	}
}

// ToFacts converts a ContextProfile to Mangle facts.
func (cp *ContextProfile) ToFacts() []core.Fact {
	return []core.Fact{{
		Predicate: "context_profile",
		Args: []interface{}{
			cp.ID,
			joinStrings(cp.RequiredSchemas),
			joinStrings(cp.RequiredTools),
			joinStrings(cp.FocusPatterns),
		},
	}}
}

// Helper to join strings with commas.
func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

// normalizeCategory coerces category strings into canonical /atom form with a default.
func normalizeCategory(category string) string {
	return normalizePhaseCategory(category)
}

func normalizeAtomValue(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return ""
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return normalized
}

func defaultTaskTypeForCategory(category string) TaskType {
	switch normalizeCategory(category) {
	case "/research":
		return TaskTypeResearch
	case "/test", "/testing":
		return TaskTypeTestWrite
	case "/verify", "/validation", "/audit", "/review":
		return TaskTypeVerify
	case "/document", "/documentation", "/docs":
		return TaskTypeDocument
	case "/integrate", "/integration":
		return TaskTypeIntegrate
	case "/refactor":
		return TaskTypeRefactor
	case "/scaffold", "/create":
		return TaskTypeFileCreate
	default:
		return TaskTypeFileModify
	}
}

func normalizeTaskType(taskType string, fallback TaskType) TaskType {
	normalized := normalizeAtomValue(taskType)
	switch normalized {
	case string(TaskTypeFileCreate):
		return TaskTypeFileCreate
	case string(TaskTypeFileModify):
		return TaskTypeFileModify
	case string(TaskTypeTestWrite):
		return TaskTypeTestWrite
	case string(TaskTypeTestRun):
		return TaskTypeTestRun
	case string(TaskTypeResearch):
		return TaskTypeResearch
	case string(TaskTypeShardSpawn):
		return TaskTypeShardSpawn
	case string(TaskTypeToolCreate):
		return TaskTypeToolCreate
	case string(TaskTypeVerify):
		return TaskTypeVerify
	case string(TaskTypeDocument):
		return TaskTypeDocument
	case string(TaskTypeRefactor):
		return TaskTypeRefactor
	case string(TaskTypeIntegrate):
		return TaskTypeIntegrate
	case string(TaskTypeCampaignRef):
		return TaskTypeCampaignRef
	case string(TaskTypeAssaultDiscover):
		return TaskTypeAssaultDiscover
	case string(TaskTypeAssaultBatch):
		return TaskTypeAssaultBatch
	case string(TaskTypeAssaultTriage):
		return TaskTypeAssaultTriage
	case "/code", "/modify", "/edit", "/patch":
		return TaskTypeFileModify
	case "/create", "/new_file":
		return TaskTypeFileCreate
	case "/test":
		return TaskTypeTestWrite
	case "/docs", "/documentation":
		return TaskTypeDocument
	case "/review", "/audit", "/validate":
		return TaskTypeVerify
	}
	if fallback == "" {
		return TaskTypeFileModify
	}
	return fallback
}

func normalizeTaskPriority(priority string) TaskPriority {
	switch normalizeAtomValue(priority) {
	case string(PriorityCritical), "/urgent":
		return PriorityCritical
	case string(PriorityHigh), "/important":
		return PriorityHigh
	case string(PriorityLow):
		return PriorityLow
	case string(PriorityNormal), "/medium", "/default":
		return PriorityNormal
	default:
		return PriorityNormal
	}
}

func defaultObjectiveTypeForCategory(category string) ObjectiveType {
	switch normalizeCategory(category) {
	case "/research":
		return ObjectiveResearch
	case "/test", "/testing":
		return ObjectiveTest
	case "/verify", "/validation", "/audit":
		return ObjectiveValidate
	case "/integrate", "/integration":
		return ObjectiveIntegrate
	case "/review":
		return ObjectiveReview
	case "/scaffold", "/create":
		return ObjectiveCreate
	default:
		return ObjectiveModify
	}
}

func normalizeObjectiveType(objective string, fallback ObjectiveType) ObjectiveType {
	switch normalizeAtomValue(objective) {
	case string(ObjectiveCreate):
		return ObjectiveCreate
	case string(ObjectiveModify), "/code":
		return ObjectiveModify
	case string(ObjectiveTest), "/testing":
		return ObjectiveTest
	case string(ObjectiveResearch):
		return ObjectiveResearch
	case string(ObjectiveValidate), "/verify", "/validation", "/audit":
		return ObjectiveValidate
	case string(ObjectiveIntegrate), "/integration":
		return ObjectiveIntegrate
	case string(ObjectiveReview):
		return ObjectiveReview
	}
	if fallback == "" {
		return ObjectiveModify
	}
	return fallback
}

func normalizeVerificationMethod(method string) VerificationMethod {
	switch normalizeAtomValue(method) {
	case string(VerifyTestsPass), "/test", "/tests", "/testing":
		return VerifyTestsPass
	case string(VerifyBuilds), "/build":
		return VerifyBuilds
	case string(VerifyManualReview), "/review":
		return VerifyManualReview
	case string(VerifyShardValidate), "/validate", "/validation":
		return VerifyShardValidate
	case string(VerifyNemesisGauntlet), "/gauntlet", "/nemesis":
		return VerifyNemesisGauntlet
	case string(VerifyNone), "":
		return VerifyNone
	default:
		return VerifyNone
	}
}

// normalizePath cleans filesystem paths and converts separators to slash form.
func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}
