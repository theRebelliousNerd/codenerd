package chat

import (
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/retrieval"
	"codenerd/internal/transparency"
	"context"
	"fmt"
	"strings"
)

// seedIssueFacts runs a bounded sparse-retrieval pass over the workspace and
// asserts the issue_* / candidate_file / keyword_hit / tiered_context_file /
// issue_context EDB surface. This drives issue-aware spreading activation and
// prompt atom selection.
//
// It used to extract keywords and stop there, so the SparseRetriever that boot
// builds into SystemComponents.Retriever was never called — it was not even
// copied onto the model — and the kernel only ever learned about files the user
// had named by hand. Two of the facts it did assert were
// silently dropped by the kernel besides: issue_keyword's weight and
// tiered_context_file's relevance are declared /number, and a fractional float
// in a /number slot is rejected outright, which took out every keyword weight
// except the 1.0 on mentioned files. Both scales now go through
// types.PercentFromRatio inside the retrieval transducer.
func (m *Model) seedIssueFacts(ctx context.Context, intent perception.Intent, rawInput string) {
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

	if ctx == nil {
		ctx = context.Background()
	}

	// The budget is deliberately separate from the LLM timeouts: this is
	// filesystem work on the user's turn, and a repository too large to finish
	// inside it must yield the tiers that completed rather than stall the loop.
	var bus *transparency.GlassBoxEventBus
	if m.glassBoxEnabled {
		bus = m.glassBoxEventBus
	}

	if _, err := retrieval.SeedIssueFacts(ctx, m.kernel, retrieval.SeedRequest{
		IssueID:   retrieval.NewIssueID(),
		IssueText: issueText,
		WorkDir:   m.workspace,
		Retriever: m.retriever,
		// m.embeddingEngine is cortex.EmbeddingEngine: nil when Ollama was
		// unavailable at boot (the factory leaves it nil deliberately), in
		// which case the seed keeps the heuristic Tier 4 fallback.
		EmbeddingEngine: m.embeddingEngine,
		Timeout:         retrieval.DefaultSeedTimeout,
		GlassBox:        bus,
		TurnID:          m.turnCount,
	}); err != nil {
		logging.Context("[seedIssueFacts] retrieval seed failed: %v", err)
	}
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

// fastPathKernelUpdate asserts the current user_intent and clears stale turn
// facts in the background after a conversational fast-path response. The
// caller guards on a non-nil kernel.
func (m Model) fastPathKernelUpdate(intent perception.Intent) {
	defer func() {
		if r := recover(); r != nil {
			logging.Routing("[processInput] FAST-PATH async kernel update recovered panic: %v", r)
		}
	}()
	tx := m.kernel.Transaction()
	// Retract stale facts from previous turns
	tx.Retract("shard_result")
	tx.Retract("pending_test")
	tx.Retract("pending_review")
	tx.Retract("interrupt_requested")
	tx.Retract("execution_result")
	tx.Retract("routing_result")
	tx.Retract("pending_action")
	tx.Retract("delegate_task")
	tx.Retract("trace_recall_result")
	tx.Retract("learning_recall_result")
	tx.Retract("delegation_candidate")
	tx.Retract("multi_step_signal")
	tx.Retract("intent_signal")
	tx.Retract("current_understanding")
	tx.Retract("llm_suggested_mode")
	tx.Retract("candidate_mode")
	tx.Retract("best_candidate_priority")
	tx.Retract("derived_mode")
	tx.Retract("derived_primary_shard")
	tx.Retract("derived_context_priority")
	tx.Retract("derived_tool_priority")
	// Assert current intent
	intentID := "/current_intent"
	tx.RetractFact(core.Fact{Predicate: "user_intent", Args: []any{intentID}})
	tx.RetractFact(core.Fact{Predicate: "processed_intent", Args: []any{intentID}})
	tx.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []any{intentID, intent.Category, intent.Verb, intent.Target, intent.Constraint},
	})
	tx.Assert(core.Fact{Predicate: "processed_intent", Args: []any{intentID}})
	if err := tx.Commit(); err != nil {
		logging.Routing("[processInput] FAST-PATH async kernel update commit error: %v", err)
	} else {
		logging.Routing("[processInput] FAST-PATH async kernel update complete")
	}
}
