package campaign

import (
	"testing"
	"time"
)

// =============================================================================
// CAMPAIGN TYPE TESTS
// =============================================================================

func TestCampaignTypeConstants(t *testing.T) {
	types := []CampaignType{
		CampaignTypeGreenfield,
		CampaignTypeFeature,
		CampaignTypeAudit,
		CampaignTypeMigration,
		CampaignTypeRemediation,
		CampaignTypeCustom,
	}

	for _, ct := range types {
		if string(ct) == "" {
			t.Errorf("CampaignType %v has empty string value", ct)
		}
		// All campaign types should start with /
		if string(ct)[0] != '/' {
			t.Errorf("CampaignType %v should start with /", ct)
		}
	}
}

func TestCampaignStatusConstants(t *testing.T) {
	statuses := []CampaignStatus{
		StatusPlanning,
		StatusDecomposing,
		StatusValidating,
		StatusActive,
		StatusPaused,
		StatusCompleted,
		StatusFailed,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("CampaignStatus %v has empty string value", s)
		}
	}
}

func TestPhaseStatusConstants(t *testing.T) {
	statuses := []PhaseStatus{
		PhasePending,
		PhaseInProgress,
		PhaseCompleted,
		PhaseFailed,
		PhaseSkipped,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("PhaseStatus %v has empty string value", s)
		}
	}
}

func TestTaskStatusConstants(t *testing.T) {
	statuses := []TaskStatus{
		TaskPending,
		TaskInProgress,
		TaskCompleted,
		TaskFailed,
		TaskSkipped,
		TaskBlocked,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("TaskStatus %v has empty string value", s)
		}
	}
}

func TestTaskTypeConstants(t *testing.T) {
	types := []TaskType{
		TaskTypeFileCreate,
		TaskTypeFileModify,
		TaskTypeTestWrite,
		TaskTypeTestRun,
		TaskTypeResearch,
		TaskTypeShardSpawn,
		TaskTypeToolCreate,
		TaskTypeVerify,
		TaskTypeDocument,
		TaskTypeRefactor,
		TaskTypeIntegrate,
	}

	for _, tt := range types {
		if string(tt) == "" {
			t.Errorf("TaskType %v has empty string value", tt)
		}
	}
}

func TestTaskPriorityConstants(t *testing.T) {
	priorities := []TaskPriority{
		PriorityCritical,
		PriorityHigh,
		PriorityNormal,
		PriorityLow,
	}

	for _, p := range priorities {
		if string(p) == "" {
			t.Errorf("TaskPriority %v has empty string value", p)
		}
	}
}

func TestDependencyTypeConstants(t *testing.T) {
	deps := []DependencyType{
		DepHard,
		DepSoft,
		DepArtifact,
	}

	for _, d := range deps {
		if string(d) == "" {
			t.Errorf("DependencyType %v has empty string value", d)
		}
	}
}

// =============================================================================
// CAMPAIGN STRUCT TESTS
// =============================================================================

func TestCampaign_Creation(t *testing.T) {
	now := time.Now()
	c := Campaign{
		ID:             "test-campaign-1",
		Type:           CampaignTypeFeature,
		Title:          "Test Campaign",
		Goal:           "Implement new feature X",
		SourceMaterial: []string{"/docs/spec.md"},
		Status:         StatusPlanning,
		CreatedAt:      now,
		UpdatedAt:      now,
		Confidence:     0.85,
		ContextBudget:  100000,
	}

	if c.ID != "test-campaign-1" {
		t.Errorf("Expected ID 'test-campaign-1', got %q", c.ID)
	}

	if c.Type != CampaignTypeFeature {
		t.Errorf("Expected Type CampaignTypeFeature, got %v", c.Type)
	}

	if c.Confidence != 0.85 {
		t.Errorf("Expected Confidence 0.85, got %f", c.Confidence)
	}
}

func TestCampaign_WithPhases(t *testing.T) {
	c := Campaign{
		ID:    "test-campaign",
		Title: "Multi-phase Campaign",
		Phases: []Phase{
			{
				ID:     "phase-1",
				Name:   "Setup",
				Order:  0,
				Status: PhaseCompleted,
			},
			{
				ID:     "phase-2",
				Name:   "Implementation",
				Order:  1,
				Status: PhaseInProgress,
			},
			{
				ID:     "phase-3",
				Name:   "Testing",
				Order:  2,
				Status: PhasePending,
			},
		},
		TotalPhases:     3,
		CompletedPhases: 1,
	}

	if len(c.Phases) != 3 {
		t.Errorf("Expected 3 phases, got %d", len(c.Phases))
	}

	if c.CompletedPhases != 1 {
		t.Errorf("Expected 1 completed phase, got %d", c.CompletedPhases)
	}
}

// =============================================================================
// PHASE STRUCT TESTS
// =============================================================================

func TestPhase_Creation(t *testing.T) {
	p := Phase{
		ID:             "phase-1",
		CampaignID:     "campaign-1",
		Name:           "Implementation Phase",
		Order:          0,
		Status:         PhasePending,
		ContextProfile: "profile-1",
		Objectives: []PhaseObjective{
			{
				Type:               ObjectiveCreate,
				Description:        "Create new service",
				VerificationMethod: VerifyTestsPass,
			},
		},
		EstimatedTasks:      5,
		EstimatedComplexity: "medium",
	}

	if p.ID != "phase-1" {
		t.Errorf("Expected ID 'phase-1', got %q", p.ID)
	}

	if len(p.Objectives) != 1 {
		t.Errorf("Expected 1 objective, got %d", len(p.Objectives))
	}
}

func TestPhase_WithDependencies(t *testing.T) {
	p := Phase{
		ID:     "phase-2",
		Name:   "Testing",
		Status: PhasePending,
		Dependencies: []PhaseDependency{
			{
				DependsOnPhaseID: "phase-1",
				Type:             DepHard,
			},
			{
				DependsOnPhaseID: "phase-0",
				Type:             DepSoft,
			},
		},
	}

	if len(p.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(p.Dependencies))
	}

	// Check first dependency is hard
	if p.Dependencies[0].Type != DepHard {
		t.Errorf("Expected first dependency to be hard, got %v", p.Dependencies[0].Type)
	}
}

func TestPhase_WithTasks(t *testing.T) {
	p := Phase{
		ID:     "phase-1",
		Name:   "Implementation",
		Status: PhaseInProgress,
		Tasks: []Task{
			{
				ID:          "task-1",
				Description: "Create model",
				Status:      TaskCompleted,
				Type:        TaskTypeFileCreate,
				Priority:    PriorityHigh,
			},
			{
				ID:          "task-2",
				Description: "Write tests",
				Status:      TaskInProgress,
				Type:        TaskTypeTestWrite,
				Priority:    PriorityNormal,
				DependsOn:   []string{"task-1"},
			},
		},
	}

	if len(p.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(p.Tasks))
	}

	// Verify task dependency
	if len(p.Tasks[1].DependsOn) != 1 || p.Tasks[1].DependsOn[0] != "task-1" {
		t.Error("Expected task-2 to depend on task-1")
	}
}

// =============================================================================
// TASK STRUCT TESTS
// =============================================================================

func TestTask_Creation(t *testing.T) {
	task := Task{
		ID:              "task-1",
		PhaseID:         "phase-1",
		Description:     "Implement user authentication",
		Status:          TaskPending,
		Type:            TaskTypeFileCreate,
		Priority:        PriorityCritical,
		InferredFrom:    "spec.md:24",
		InferenceConf:   0.9,
		InferenceReason: "Spec requires auth module",
	}

	if task.ID != "task-1" {
		t.Errorf("Expected ID 'task-1', got %q", task.ID)
	}

	if task.InferenceConf != 0.9 {
		t.Errorf("Expected InferenceConf 0.9, got %f", task.InferenceConf)
	}
}

func TestTask_WithArtifacts(t *testing.T) {
	task := Task{
		ID:          "task-1",
		Description: "Create auth service",
		Status:      TaskCompleted,
		Artifacts: []TaskArtifact{
			{
				Type: "/source_file",
				Path: "internal/auth/service.go",
				Hash: "abc123",
			},
			{
				Type: "/test_file",
				Path: "internal/auth/service_test.go",
				Hash: "def456",
			},
		},
	}

	if len(task.Artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(task.Artifacts))
	}
}

func TestTask_WithAttempts(t *testing.T) {
	now := time.Now()
	task := Task{
		ID:          "task-1",
		Description: "Flaky task",
		Status:      TaskCompleted,
		Attempts: []TaskAttempt{
			{
				Number:    1,
				Outcome:   "/failure",
				Timestamp: now.Add(-time.Hour),
				Error:     "Connection timeout",
			},
			{
				Number:    2,
				Outcome:   "/failure",
				Timestamp: now.Add(-30 * time.Minute),
				Error:     "Test assertion failed",
			},
			{
				Number:    3,
				Outcome:   "/success",
				Timestamp: now,
			},
		},
	}

	if len(task.Attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", len(task.Attempts))
	}

	// Verify last attempt was successful
	lastAttempt := task.Attempts[len(task.Attempts)-1]
	if lastAttempt.Outcome != "/success" {
		t.Errorf("Expected last attempt to be success, got %s", lastAttempt.Outcome)
	}
}

// =============================================================================
// CHECKPOINT TESTS
// =============================================================================

func TestCheckpoint(t *testing.T) {
	now := time.Now()
	cp := Checkpoint{
		Type:      "/tests",
		Passed:    true,
		Details:   "All 42 tests passed",
		Timestamp: now,
	}

	if !cp.Passed {
		t.Error("Expected checkpoint to pass")
	}

	if cp.Type != "/tests" {
		t.Errorf("Expected Type '/tests', got %q", cp.Type)
	}
}

// =============================================================================
// CONTEXT PROFILE TESTS
// =============================================================================

func TestContextProfile(t *testing.T) {
	cp := ContextProfile{
		ID:              "profile-1",
		RequiredSchemas: []string{"file_topology", "symbol_graph", "diagnostic"},
		RequiredTools:   []string{"read_file", "write_file", "exec_cmd"},
		FocusPatterns:   []string{"internal/auth/**/*.go", "cmd/server/**/*.go"},
	}

	if cp.ID != "profile-1" {
		t.Errorf("Expected ID 'profile-1', got %q", cp.ID)
	}

	if len(cp.RequiredSchemas) != 3 {
		t.Errorf("Expected 3 schemas, got %d", len(cp.RequiredSchemas))
	}
}

// =============================================================================
// LEARNING TESTS
// =============================================================================

func TestLearning(t *testing.T) {
	now := time.Now()
	l := Learning{
		Type:      "/success_pattern",
		Pattern:   "test_first",
		Fact:      "Writing tests first reduces iteration time",
		AppliedAt: now,
	}

	if l.Type != "/success_pattern" {
		t.Errorf("Expected Type '/success_pattern', got %q", l.Type)
	}
}

// =============================================================================
// SOURCE DOCUMENT TESTS
// =============================================================================

func TestSourceDocument(t *testing.T) {
	now := time.Now()
	doc := SourceDocument{
		CampaignID: "campaign-1",
		Path:       "/docs/api-spec.md",
		Type:       "/spec",
		ParsedAt:   now,
		Summary:    "API specification for user service",
	}

	if doc.Type != "/spec" {
		t.Errorf("Expected Type '/spec', got %q", doc.Type)
	}
}

// =============================================================================
// REQUIREMENT TESTS
// =============================================================================

func TestRequirement(t *testing.T) {
	req := Requirement{
		ID:          "req-1",
		CampaignID:  "campaign-1",
		Description: "User must be able to authenticate with email/password",
		Priority:    "high",
		Source:      "spec.md:42",
		CoveredBy:   []string{"task-1", "task-2"},
	}

	if req.ID != "req-1" {
		t.Errorf("Expected ID 'req-1', got %q", req.ID)
	}

	if len(req.CoveredBy) != 2 {
		t.Errorf("Expected 2 covering tasks, got %d", len(req.CoveredBy))
	}
}

// =============================================================================
// CAMPAIGN SHARD TESTS
// =============================================================================

func TestCampaignShard(t *testing.T) {
	cs := CampaignShard{
		CampaignID: "campaign-1",
		ShardID:    "shard-abc123",
		ShardType:  "researcher",
		Task:       "Research OAuth2 best practices",
		Status:     "active",
	}

	if cs.ShardType != "researcher" {
		t.Errorf("Expected ShardType 'researcher', got %q", cs.ShardType)
	}
}

// =============================================================================
// PHASE OBJECTIVE TESTS
// =============================================================================

func TestPhaseObjective(t *testing.T) {
	objectives := []PhaseObjective{
		{
			Type:               ObjectiveCreate,
			Description:        "Create user service",
			VerificationMethod: VerifyTestsPass,
		},
		{
			Type:               ObjectiveTest,
			Description:        "Achieve 80% coverage",
			VerificationMethod: VerifyBuilds,
		},
		{
			Type:               ObjectiveReview,
			Description:        "Code review by senior dev",
			VerificationMethod: VerifyManualReview,
		},
	}

	if len(objectives) != 3 {
		t.Errorf("Expected 3 objectives, got %d", len(objectives))
	}

	// Verify objective types
	if objectives[0].Type != ObjectiveCreate {
		t.Errorf("Expected ObjectiveCreate, got %v", objectives[0].Type)
	}
}

// =============================================================================
// TASK ARTIFACT TESTS
// =============================================================================

func TestTaskArtifact(t *testing.T) {
	artifacts := []TaskArtifact{
		{
			Type: "/source_file",
			Path: "internal/service.go",
			Hash: "sha256:abc123",
		},
		{
			Type: "/test_file",
			Path: "internal/service_test.go",
		},
		{
			Type: "/config",
			Path: "config/app.yaml",
		},
	}

	for _, a := range artifacts {
		if a.Path == "" {
			t.Error("Artifact path should not be empty")
		}
	}

	// First artifact should have hash
	if artifacts[0].Hash == "" {
		t.Error("First artifact should have hash")
	}
}

// =============================================================================
// PHASE DEPENDENCY TESTS
// =============================================================================

func TestPhaseDependency(t *testing.T) {
	deps := []PhaseDependency{
		{
			DependsOnPhaseID: "phase-1",
			Type:             DepHard,
		},
		{
			DependsOnPhaseID: "phase-2",
			Type:             DepSoft,
		},
		{
			DependsOnPhaseID: "phase-3",
			Type:             DepArtifact,
		},
	}

	if len(deps) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(deps))
	}

	// Verify hard dependency
	if deps[0].Type != DepHard {
		t.Errorf("Expected DepHard, got %v", deps[0].Type)
	}
}
