package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"fmt"
	"time"
)

// taskFactPredicates are the predicates Task.ToFacts emits, each keyed by the
// task ID in its first argument: every one is retracted before a task's facts
// are reloaded, so a changed task never leaves a stale row behind. A predicate
// ToFacts gains is added here in the same change.
var taskFactPredicates = []string{
	"campaign_task",
	"task_priority",
	"task_order",
	"task_dependency",
	"task_soft_dependency",
	"task_context_from",
	"requires_resource",
	"task_sub_campaign",
	"task_artifact",
	"task_inference",
	"task_attempt",
	"task_attempt_signal",
	"task_replanned_at_cap",
	"task_retry_at",
	"task_error",
	"task_write_target",
	"task_write_ext",
}

func syncCampaignFacts(kernel core.Kernel, previous, next *Campaign, revisionSummary string) error {
	if kernel == nil {
		return ErrNilKernel
	}
	if next == nil {
		return fmt.Errorf("%w: next campaign snapshot is nil", ErrInvalidConfig)
	}

	if _, ok := kernel.(types.KernelTransactor); !ok {
		if err := retractCampaignFacts(kernel, previous); err != nil {
			return fmt.Errorf("retract previous campaign facts: %w", err)
		}
		if err := kernel.LoadFacts(next.ToFacts()); err != nil {
			return fmt.Errorf("load next campaign facts: %w", err)
		}
		if revisionSummary != "" {
			if err := kernel.Assert(core.Fact{
				Predicate: "plan_revision",
				Args:      []any{next.ID, next.RevisionNumber, revisionSummary, time.Now().Unix()},
			}); err != nil {
				return fmt.Errorf("assert plan revision: %w", err)
			}
		}
		return nil
	}

	tx := types.NewKernelTx(kernel)
	queueCampaignFactRetractions(tx, previous)
	tx.LoadFacts(next.ToFacts())
	if revisionSummary != "" {
		tx.Assert(core.Fact{
			Predicate: "plan_revision",
			Args:      []any{next.ID, next.RevisionNumber, revisionSummary, time.Now().Unix()},
		})
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sync campaign facts: %w", err)
	}

	return nil
}

func retractCampaignFacts(kernel core.Kernel, campaign *Campaign) error {
	if kernel == nil || campaign == nil {
		return nil
	}

	retract := func(fact core.Fact) error {
		if err := kernel.RetractFact(fact); err != nil {
			return err
		}
		return nil
	}

	if err := retract(core.Fact{Predicate: "campaign", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_metadata", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_goal", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_progress", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "source_document", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_acceptance", Args: []any{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_acceptance_result", Args: []any{campaign.ID}}); err != nil {
		return err
	}

	for _, profile := range campaign.ContextProfiles {
		if err := retract(core.Fact{Predicate: "context_profile", Args: []any{profile.ID}}); err != nil {
			return err
		}
	}

	for _, phase := range campaign.Phases {
		if err := retract(core.Fact{Predicate: "campaign_phase", Args: []any{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_category", Args: []any{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_objective", Args: []any{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_dependency", Args: []any{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_estimate", Args: []any{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "context_compression", Args: []any{phase.ID}}); err != nil {
			return err
		}

		for _, task := range phase.Tasks {
			for _, pred := range taskFactPredicates {
				if err := retract(core.Fact{Predicate: pred, Args: []any{task.ID}}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func queueCampaignFactRetractions(tx *types.KernelTx, campaign *Campaign) {
	if tx == nil || campaign == nil {
		return
	}

	tx.RetractFact(core.Fact{Predicate: "campaign", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_metadata", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_goal", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_progress", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "source_document", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_acceptance", Args: []any{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_acceptance_result", Args: []any{campaign.ID}})

	for _, profile := range campaign.ContextProfiles {
		tx.RetractFact(core.Fact{Predicate: "context_profile", Args: []any{profile.ID}})
	}

	for _, phase := range campaign.Phases {
		tx.RetractFact(core.Fact{Predicate: "campaign_phase", Args: []any{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_category", Args: []any{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_objective", Args: []any{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_dependency", Args: []any{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_estimate", Args: []any{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "context_compression", Args: []any{phase.ID}})

		for _, task := range phase.Tasks {
			for _, pred := range taskFactPredicates {
				tx.RetractFact(core.Fact{Predicate: pred, Args: []any{task.ID}})
			}
		}
	}
}
