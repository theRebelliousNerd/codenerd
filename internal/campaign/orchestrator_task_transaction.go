package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"codenerd/internal/logging"
)

type fileMutationSnapshot struct {
	Path    string
	Exists  bool
	Content []byte
}

// taskExecutionSnapshot captures mutable orchestrator state that must be rolled back.
type taskExecutionSnapshot struct {
	campaign        *Campaign
	taskResults     map[string]string
	taskResultOrder []string
	fileMutations   []fileMutationSnapshot
}

// executeTaskWithRollback wraps mutating task execution in a scoped snapshot.
func (o *Orchestrator) executeTaskWithRollback(ctx context.Context, task *Task) (any, error) {
	return o.withTaskMutationSnapshot(task, func() (any, error) {
		result, err := o.executeTask(ctx, task)
		if err != nil {
			return nil, err
		}
		if err := o.runTaskMicroCheckpoint(ctx, task); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (o *Orchestrator) withTaskExecutionSnapshot(task *Task, run func() (any, error)) (any, error) {
	return o.withTaskMutationSnapshot(task, run)
}

func (o *Orchestrator) withTaskMutationSnapshot(task *Task, run func() (any, error)) (result any, err error) {
	if run == nil {
		return nil, fmt.Errorf("task execution callback is nil")
	}
	if !requiresTaskMutationSnapshot(task) {
		return run()
	}
	if o.shouldGateTask(taskIDOrUnknown(task)) {
		return nil, o.newRiskGateTaskError(task)
	}

	snapshot, snapErr := o.captureTaskExecutionSnapshot(task)
	if snapErr != nil {
		return nil, fmt.Errorf("capture execution snapshot for %s: %w", taskIDOrUnknown(task), snapErr)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			logging.Get(logging.CategoryCampaign).Error(
				"panic during mutating task %s: %v\n%s",
				taskIDOrUnknown(task),
				recovered,
				string(debug.Stack()),
			)
			panicErr := fmt.Errorf("panic during mutating task %s: %v", taskIDOrUnknown(task), recovered)
			if rollbackErr := o.rollbackTaskExecutionSnapshot(snapshot); rollbackErr != nil {
				err = fmt.Errorf("%w (rollback failed: %v)", panicErr, rollbackErr)
			} else {
				err = panicErr
			}
			result = nil
		}
	}()

	result, err = run()
	if err == nil {
		return result, nil
	}

	if rollbackErr := o.rollbackTaskExecutionSnapshot(snapshot); rollbackErr != nil {
		return nil, fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
	}

	return nil, err
}

func requiresTaskMutationSnapshot(task *Task) bool {
	if task == nil {
		return false
	}
	return isMutatingTaskType(task.Type)
}

func (o *Orchestrator) captureTaskExecutionSnapshot(task *Task) (taskExecutionSnapshot, error) {
	var snapshot taskExecutionSnapshot

	o.mu.RLock()
	if o.campaign == nil {
		o.mu.RUnlock()
		return snapshot, fmt.Errorf("no campaign loaded")
	}
	clonedCampaign, err := cloneCampaign(o.campaign)
	o.mu.RUnlock()
	if err != nil {
		return snapshot, fmt.Errorf("clone campaign: %w", err)
	}
	snapshot.campaign = clonedCampaign

	o.resultsMu.RLock()
	snapshot.taskResults = cloneStringMap(o.taskResults)
	snapshot.taskResultOrder = append([]string(nil), o.taskResultOrder...)
	o.resultsMu.RUnlock()

	if task == nil {
		return snapshot, nil
	}

	writeSet := o.resolveTaskWriteSet(task)
	for _, absPath := range writeSet {
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				snapshot.fileMutations = append(snapshot.fileMutations, fileMutationSnapshot{
					Path:   absPath,
					Exists: false,
				})
				continue
			}
			return snapshot, fmt.Errorf("stat path %s: %w", absPath, err)
		}
		if info.IsDir() {
			continue
		}
		content, err := os.ReadFile(absPath)
		if err != nil {
			return snapshot, fmt.Errorf("read snapshot file %s: %w", absPath, err)
		}
		snapshot.fileMutations = append(snapshot.fileMutations, fileMutationSnapshot{
			Path:    absPath,
			Exists:  true,
			Content: content,
		})
	}

	return snapshot, nil
}

func (o *Orchestrator) rollbackTaskExecutionSnapshot(snapshot taskExecutionSnapshot) error {
	if snapshot.campaign == nil {
		return fmt.Errorf("snapshot campaign is nil")
	}

	for _, fs := range snapshot.fileMutations {
		if fs.Exists {
			if err := os.MkdirAll(filepath.Dir(fs.Path), 0755); err != nil {
				return fmt.Errorf("rollback mkdir %s: %w", fs.Path, err)
			}
			if err := os.WriteFile(fs.Path, fs.Content, 0644); err != nil {
				return fmt.Errorf("rollback write %s: %w", fs.Path, err)
			}
			continue
		}
		if err := os.Remove(fs.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rollback remove %s: %w", fs.Path, err)
		}
	}

	o.mu.Lock()
	currentCampaign := o.campaign
	o.campaign = snapshot.campaign
	o.mu.Unlock()

	o.resultsMu.Lock()
	o.taskResults = cloneStringMap(snapshot.taskResults)
	o.taskResultOrder = append([]string(nil), snapshot.taskResultOrder...)
	o.resultsMu.Unlock()

	if err := o.rollbackCampaignFacts(currentCampaign, snapshot.campaign); err != nil {
		return err
	}

	o.assertCampaignConfigFacts()
	if o.contextPager != nil && snapshot.campaign.ContextBudget > 0 {
		o.contextPager.SetBudget(snapshot.campaign.ContextBudget)
	}

	return nil
}

func (o *Orchestrator) rollbackCampaignFacts(currentCampaign, restoredCampaign *Campaign) error {
	if o.kernel == nil {
		return nil
	}

	if currentCampaign != nil {
		currentClone, err := cloneCampaign(currentCampaign)
		if err != nil {
			return fmt.Errorf("clone current campaign: %w", err)
		}
		currentFacts := currentClone.ToFacts()
		if len(currentFacts) > 0 {
			if err := o.kernel.RetractExactFactsBatch(currentFacts); err != nil {
				return fmt.Errorf("retract current campaign facts: %w", err)
			}
		}
	}

	if restoredCampaign != nil {
		restoredClone, err := cloneCampaign(restoredCampaign)
		if err != nil {
			return fmt.Errorf("clone restored campaign: %w", err)
		}
		if err := o.kernel.LoadFacts(restoredClone.ToFacts()); err != nil {
			return fmt.Errorf("restore campaign facts: %w", err)
		}
	}

	return nil
}

func cloneCampaign(src *Campaign) (*Campaign, error) {
	if src == nil {
		return nil, nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var cloned Campaign
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return make(map[string]string)
	}
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func taskIDOrUnknown(task *Task) string {
	if task == nil || task.ID == "" {
		return "<unknown>"
	}
	return task.ID
}

func (o *Orchestrator) newRiskGateTaskError(task *Task) error {
	taskID := taskIDOrUnknown(task)

	o.mu.RLock()
	decision := o.riskDecision
	mode := normalizeRiskGateMode(o.config.RiskGateMode)
	o.mu.RUnlock()

	if decision == nil {
		return fmt.Errorf("risk gate blocked mutating task %s (mode=%s)", taskID, mode)
	}

	return fmt.Errorf(
		"risk gate blocked mutating task %s (score=%d threshold=%d override=%s mode=%s snapshot=%s)",
		taskID,
		decision.Score,
		decision.Threshold,
		decision.OverrideLevel,
		mode,
		decision.SnapshotID,
	)
}
