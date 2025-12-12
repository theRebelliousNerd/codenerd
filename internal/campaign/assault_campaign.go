package campaign

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewAdversarialAssaultCampaign builds a durable, batched campaign that can
// sweep a large repo in stages, persist results under .nerd/campaigns/<id>/assault,
// and then generate remediation tasks based on observed failures.
//
// This is intentionally deterministic (no LLM decomposition) to avoid exploding
// a 50k-file repo into per-file tasks.
func NewAdversarialAssaultCampaign(workspace string, cfg AssaultConfig) *Campaign {
	cfg = cfg.Normalize()

	now := time.Now()
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	slug := sanitizeCampaignID(campaignID)
	kbPath := filepath.Join(workspace, ".nerd", "campaigns", slug, "knowledge.db")

	title := fmt.Sprintf("Adversarial Assault (%s)", strings.TrimPrefix(string(cfg.Scope), "/"))
	goal := fmt.Sprintf(
		"Run an adversarial assault campaign (scope=%s, cycles=%d, batch_size=%d, stages=%d) and persist results under .nerd/campaigns/%s/assault, then triage and generate remediation tasks.",
		cfg.Scope,
		cfg.Cycles,
		cfg.BatchSize,
		len(cfg.Stages),
		slug,
	)

	c := &Campaign{
		ID:              campaignID,
		Type:            CampaignTypeAdversarialAssault,
		Title:           title,
		Goal:            goal,
		SourceMaterial:  []string{},
		KnowledgeBase:   kbPath,
		Status:          StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
		Confidence:      1.0,
		ContextBudget:   100000,
		Phases:          make([]Phase, 0, 4),
		ContextProfiles: make([]ContextProfile, 0, 4),
		TotalPhases:     0,
		TotalTasks:      0,
		Assault:         &cfg,
	}

	// Minimal context profiles (used primarily for prompt assembly and future paging).
	for i := 0; i < 4; i++ {
		profileID := fmt.Sprintf("/profile_%s_%d", campaignID[10:], i)
		profile := ContextProfile{
			ID:              profileID,
			RequiredSchemas: []string{"file_topology", "symbol_graph", "diagnostic"},
			RequiredTools:   []string{"exec_cmd", "run_tests", "build_project"},
			FocusPatterns:   []string{"**/*"},
		}
		c.ContextProfiles = append(c.ContextProfiles, profile)
	}

	phase0 := Phase{
		ID:             fmt.Sprintf("/phase_%s_%d", campaignID[10:], 0),
		CampaignID:     campaignID,
		Name:           "Discovery",
		Order:          0,
		Category:       "/analysis",
		Status:         PhasePending,
		ContextProfile: c.ContextProfiles[0].ID,
		Objectives: []PhaseObjective{{
			Type:               ObjectiveResearch,
			Description:        "Discover targets and generate batched assault tasks",
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      1,
		EstimatedComplexity: "/medium",
		Tasks: []Task{{
			ID:          fmt.Sprintf("/task_%s_%d_%d", campaignID[10:], 0, 0),
			PhaseID:     fmt.Sprintf("/phase_%s_%d", campaignID[10:], 0),
			Description: "Enumerate repo targets (packages/subsystems) and create persisted batch tasks",
			Status:      TaskPending,
			Type:        TaskTypeAssaultDiscover,
			Priority:    PriorityCritical,
			Order:       0,
		}},
	}
	phase1 := Phase{
		ID:             fmt.Sprintf("/phase_%s_%d", campaignID[10:], 1),
		CampaignID:     campaignID,
		Name:           "Assault Execution",
		Order:          1,
		Category:       "/test",
		Status:         PhasePending,
		ContextProfile: c.ContextProfiles[1].ID,
		Objectives: []PhaseObjective{{
			Type:               ObjectiveTest,
			Description:        "Execute assault batches and persist results",
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      0, // Filled during discovery
		EstimatedComplexity: "/high",
		Tasks:               []Task{},
		Dependencies: []PhaseDependency{{
			DependsOnPhaseID: phase0.ID,
			Type:             DepHard,
		}},
	}
	phase2 := Phase{
		ID:             fmt.Sprintf("/phase_%s_%d", campaignID[10:], 2),
		CampaignID:     campaignID,
		Name:           "Triage",
		Order:          2,
		Category:       "/review",
		Status:         PhasePending,
		ContextProfile: c.ContextProfiles[2].ID,
		Objectives: []PhaseObjective{{
			Type:               ObjectiveReview,
			Description:        "Summarize assault results and propose remediation tasks",
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      1,
		EstimatedComplexity: "/medium",
		Tasks: []Task{{
			ID:          fmt.Sprintf("/task_%s_%d_%d", campaignID[10:], 2, 0),
			PhaseID:     fmt.Sprintf("/phase_%s_%d", campaignID[10:], 2),
			Description: "Triage assault results and generate remediation plan",
			Status:      TaskPending,
			Type:        TaskTypeAssaultTriage,
			Priority:    PriorityHigh,
			Order:       0,
		}},
		Dependencies: []PhaseDependency{{
			DependsOnPhaseID: phase1.ID,
			Type:             DepHard,
		}},
	}
	phase3 := Phase{
		ID:             fmt.Sprintf("/phase_%s_%d", campaignID[10:], 3),
		CampaignID:     campaignID,
		Name:           "Remediation",
		Order:          3,
		Category:       "/remediation",
		Status:         PhasePending,
		ContextProfile: c.ContextProfiles[3].ID,
		Objectives: []PhaseObjective{{
			Type:               ObjectiveModify,
			Description:        "Apply fixes driven by triage findings",
			VerificationMethod: VerifyTestsPass,
		}},
		EstimatedTasks:      0, // Filled during triage
		EstimatedComplexity: "/critical",
		Tasks:               []Task{},
		Dependencies: []PhaseDependency{{
			DependsOnPhaseID: phase2.ID,
			Type:             DepHard,
		}},
		Checkpoints: []Checkpoint{{
			Type:      string(VerifyTestsPass),
			Passed:    false,
			Timestamp: time.Time{},
		}},
	}

	c.Phases = append(c.Phases, phase0, phase1, phase2, phase3)
	c.TotalPhases = len(c.Phases)
	for _, p := range c.Phases {
		c.TotalTasks += len(p.Tasks)
	}

	return c
}

