package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"fmt"
	"time"
)

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
				Args:      []interface{}{next.ID, next.RevisionNumber, revisionSummary, time.Now().Unix()},
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
			Args:      []interface{}{next.ID, next.RevisionNumber, revisionSummary, time.Now().Unix()},
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

	if err := retract(core.Fact{Predicate: "campaign", Args: []interface{}{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_metadata", Args: []interface{}{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_goal", Args: []interface{}{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "campaign_progress", Args: []interface{}{campaign.ID}}); err != nil {
		return err
	}
	if err := retract(core.Fact{Predicate: "source_document", Args: []interface{}{campaign.ID}}); err != nil {
		return err
	}

	for _, profile := range campaign.ContextProfiles {
		if err := retract(core.Fact{Predicate: "context_profile", Args: []interface{}{profile.ID}}); err != nil {
			return err
		}
	}

	for _, phase := range campaign.Phases {
		if err := retract(core.Fact{Predicate: "campaign_phase", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_category", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_objective", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_dependency", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "phase_estimate", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}
		if err := retract(core.Fact{Predicate: "context_compression", Args: []interface{}{phase.ID}}); err != nil {
			return err
		}

		for _, task := range phase.Tasks {
			if err := retract(core.Fact{Predicate: "campaign_task", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_priority", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_order", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_dependency", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_soft_dependency", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "requires_resource", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_sub_campaign", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_artifact", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_inference", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_attempt", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_retry_at", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_error", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
			if err := retract(core.Fact{Predicate: "task_write_target", Args: []interface{}{task.ID}}); err != nil {
				return err
			}
		}
	}

	return nil
}

func queueCampaignFactRetractions(tx *types.KernelTx, campaign *Campaign) {
	if tx == nil || campaign == nil {
		return
	}

	tx.RetractFact(core.Fact{Predicate: "campaign", Args: []interface{}{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_metadata", Args: []interface{}{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_goal", Args: []interface{}{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "campaign_progress", Args: []interface{}{campaign.ID}})
	tx.RetractFact(core.Fact{Predicate: "source_document", Args: []interface{}{campaign.ID}})

	for _, profile := range campaign.ContextProfiles {
		tx.RetractFact(core.Fact{Predicate: "context_profile", Args: []interface{}{profile.ID}})
	}

	for _, phase := range campaign.Phases {
		tx.RetractFact(core.Fact{Predicate: "campaign_phase", Args: []interface{}{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_category", Args: []interface{}{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_objective", Args: []interface{}{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_dependency", Args: []interface{}{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "phase_estimate", Args: []interface{}{phase.ID}})
		tx.RetractFact(core.Fact{Predicate: "context_compression", Args: []interface{}{phase.ID}})

		for _, task := range phase.Tasks {
			tx.RetractFact(core.Fact{Predicate: "campaign_task", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_priority", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_order", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_dependency", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_soft_dependency", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "requires_resource", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_sub_campaign", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_artifact", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_inference", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_attempt", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_retry_at", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_error", Args: []interface{}{task.ID}})
			tx.RetractFact(core.Fact{Predicate: "task_write_target", Args: []interface{}{task.ID}})
		}
	}
}

func cloneCampaignForMutation(campaign *Campaign) *Campaign {
	if campaign == nil {
		return nil
	}

	clone := *campaign
	if campaign.Phases != nil {
		clone.Phases = make([]Phase, len(campaign.Phases))
		for i := range campaign.Phases {
			phase := campaign.Phases[i]
			clone.Phases[i] = phase
			if phase.Objectives != nil {
				clone.Phases[i].Objectives = append([]PhaseObjective(nil), phase.Objectives...)
			}
			if phase.Dependencies != nil {
				clone.Phases[i].Dependencies = append([]PhaseDependency(nil), phase.Dependencies...)
			}
			if phase.Tasks != nil {
				clone.Phases[i].Tasks = make([]Task, len(phase.Tasks))
				for j := range phase.Tasks {
					task := phase.Tasks[j]
					clone.Phases[i].Tasks[j] = task
					if task.DependsOn != nil {
						clone.Phases[i].Tasks[j].DependsOn = append([]string(nil), task.DependsOn...)
					}
					if task.SoftDeps != nil {
						clone.Phases[i].Tasks[j].SoftDeps = append([]string(nil), task.SoftDeps...)
					}
					if task.Resources != nil {
						clone.Phases[i].Tasks[j].Resources = append([]string(nil), task.Resources...)
					}
					if task.Artifacts != nil {
						clone.Phases[i].Tasks[j].Artifacts = append([]TaskArtifact(nil), task.Artifacts...)
					}
					if task.Attempts != nil {
						clone.Phases[i].Tasks[j].Attempts = append([]TaskAttempt(nil), task.Attempts...)
					}
					if task.WriteSet != nil {
						clone.Phases[i].Tasks[j].WriteSet = append([]string(nil), task.WriteSet...)
					}
					if task.ContextFrom != nil {
						clone.Phases[i].Tasks[j].ContextFrom = append([]string(nil), task.ContextFrom...)
					}
				}
			}
		}
	}

	return &clone
}
