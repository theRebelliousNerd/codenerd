package chat

import (
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"context"
	"fmt"
	"strings"
	"time"
)

// seedIssueFacts extracts issue text + keywords from user input and asserts issue_* facts.
// This drives issue-aware spreading activation and prompt atom selection.
func (m *Model) seedIssueFacts(intent perception.Intent, rawInput string) {
	if m.kernel == nil {
		return
	}

	// Only seed for verbs that are typically issue-driven.
	switch intent.Verb {
	case "/fix", "/debug", "/review", "/security":
	default:
		return
	}

	issueText := strings.TrimSpace(rawInput)
	if issueText == "" {
		return
	}

	// Keep the stored issue text bounded to avoid EDB bloat.
	const maxIssueChars = 4000
	if len(issueText) > maxIssueChars {
		issueText = issueText[:maxIssueChars]
	}

	issueID := fmt.Sprintf("/issue_%d", time.Now().UnixNano())
	keywords := retrieval.ExtractKeywords(issueText)

	facts := make([]core.Fact, 0, 1+len(keywords.Weights)+len(keywords.MentionedFiles))
	facts = append(facts, core.Fact{
		Predicate: "issue_text",
		Args:      []any{issueID, issueText},
	})

	for kw, weight := range keywords.Weights {
		if strings.TrimSpace(kw) == "" {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "issue_keyword",
			Args:      []any{issueID, kw, weight},
		})
	}

	for _, file := range keywords.MentionedFiles {
		if strings.TrimSpace(file) == "" {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "file_mentioned",
			Args:      []any{file, issueID},
		})
	}

	// GAP-017 FIX: Assert tiered_context_file facts for issue-driven file relevance
	// Tier 1: Directly mentioned files (highest relevance)
	// This enables the activation engine to boost these files in context selection
	for i, file := range keywords.MentionedFiles {
		if strings.TrimSpace(file) == "" {
			continue
		}
		// tiered_context_file(IssueID, File, Tier, Relevance, TokenCount)
		// Tier 1 for directly mentioned, relevance decreases by position
		relevance := 1.0 - (float64(i) * 0.1)
		if relevance < 0.5 {
			relevance = 0.5
		}
		facts = append(facts, core.Fact{
			Predicate: "tiered_context_file",
			Args:      []any{issueID, file, "/tier1", relevance, 0},
		})
	}

	_ = m.kernel.LoadFacts(facts)
}

// seedCampaignFacts asserts campaign context facts for spreading activation and JIT selection.
// GAP-002 FIX: This enables the activation engine and JIT compiler to be campaign-aware.
func (m *Model) seedCampaignFacts() {
	if m.kernel == nil || m.activeCampaign == nil {
		return
	}

	c := m.activeCampaign
	facts := make([]core.Fact, 0, 10)

	// current_campaign(CampaignID)
	facts = append(facts, core.Fact{
		Predicate: "current_campaign",
		Args:      []any{c.ID},
	})

	// Find current phase (first non-completed phase)
	var currentPhase *campaign.Phase
	for i := range c.Phases {
		if c.Phases[i].Status != campaign.PhaseCompleted {
			currentPhase = &c.Phases[i]
			break
		}
	}

	if currentPhase != nil {
		// current_phase(PhaseID)
		facts = append(facts, core.Fact{
			Predicate: "current_phase",
			Args:      []any{currentPhase.ID},
		})

		// phase_objective(PhaseID, ObjectiveIndex, Description)
		for i, obj := range currentPhase.Objectives {
			objID := fmt.Sprintf("/obj_%s_%d", currentPhase.ID, i)
			facts = append(facts, core.Fact{
				Predicate: "phase_objective",
				Args:      []any{currentPhase.ID, objID, obj.Description},
			})
		}

		// Find next pending task
		for _, task := range currentPhase.Tasks {
			if task.Status == campaign.TaskPending || task.Status == campaign.TaskInProgress {
				// next_campaign_task(TaskID)
				facts = append(facts, core.Fact{
					Predicate: "next_campaign_task",
					Args:      []any{task.ID},
				})

				// task_artifact(TaskID, ArtifactType, Path)
				for _, artifact := range task.Artifacts {
					facts = append(facts, core.Fact{
						Predicate: "task_artifact",
						Args:      []any{task.ID, artifact.Type, artifact.Path},
					})
				}
				break // Only the next task
			}
		}
	}

	_ = m.kernel.LoadFacts(facts)
}

// runClarifierShard invokes the requirements_interrogator shard synchronously to gather clarifying questions.
func (m Model) runClarifierShard(ctx context.Context, goal string) (string, error) {
	if m.shardMgr == nil && m.taskExecutor == nil {
		return "", fmt.Errorf("no executor available: both taskExecutor and shardMgr are nil")
	}

	cctx, cancel := context.WithTimeout(ctx, config.GetLLMTimeouts().ArticulationTimeout)
	defer cancel()

	result, err := m.spawnTask(cctx, "requirements_interrogator", goal)
	if err != nil {
		return "", err
	}
	return result, nil
}
