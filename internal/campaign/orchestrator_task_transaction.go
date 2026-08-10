package campaign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"

	"codenerd/internal/logging"
)

type fileMutationSnapshot struct {
	Path    string
	Exists  bool
	Content []byte
}

// taskExecutionSnapshot captures mutable orchestrator state that must be rolled back.
//
// There are two rollback scopes:
//   - Structural (assault) tasks capture the whole campaign + task-result cache so a
//     partial plan edit can be fully reverted (campaign != nil).
//   - Non-structural mutating tasks capture only the failing task's own status, so a
//     failure rolls back files + that task's status WITHOUT swapping o.campaign or
//     clobbering concurrently-running sibling tasks (campaign == nil). This is the
//     fix for the F-SCHED-2 infinite re-dispatch loop.
type taskExecutionSnapshot struct {
	campaign        *Campaign
	taskResults     map[string]string
	taskResultOrder []string
	fileMutations   []fileMutationSnapshot
	declaredGlobs   []string
	globPreMatches  map[string]map[string]struct{}

	// Scoped (non-structural) rollback state.
	scopedTask   *Task
	scopedStatus TaskStatus
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
	if err == nil && task != nil && task.Type == TaskTypeFileModify {
		err = validateFileModifyOutcome(task, snapshot)
	}
	if err == nil {
		return result, nil
	}

	if rollbackErr := o.rollbackTaskExecutionSnapshot(snapshot); rollbackErr != nil {
		return nil, fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
	}

	return nil, err
}

func validateFileModifyOutcome(task *Task, snapshot taskExecutionSnapshot) error {
	newMatches, err := listNewBroadGlobMatches(snapshot)
	if err != nil {
		return fmt.Errorf("verify file_modify broad-glob contract for %s: %w", taskIDOrUnknown(task), err)
	}
	if len(newMatches) > 0 {
		return fmt.Errorf("task %s (%s) created new file(s) matching declared broad glob: %v (broad globs do not grant exact-path authority; provenance unknown)", taskIDOrUnknown(task), task.Type, newMatches)
	}
	modified := false
	var deletedPath string
	var readErr error
	var readErrPath string
	for _, fs := range snapshot.fileMutations {
		if !fs.Exists {
			continue
		}
		current, err := os.ReadFile(fs.Path)
		if err != nil {
			if os.IsNotExist(err) {
				if deletedPath == "" {
					deletedPath = fs.Path
				}
				continue
			}
			if readErr == nil {
				readErr = err
				readErrPath = fs.Path
			}
			continue
		}
		if !bytes.Equal(current, fs.Content) {
			modified = true
		}
	}
	if deletedPath != "" {
		return fmt.Errorf("task %s (%s) deleted pre-existing file %s (file_modify must modify, not delete)", taskIDOrUnknown(task), task.Type, deletedPath)
	}
	if readErr != nil {
		return fmt.Errorf("verify file_modify outcome for %s: read %s: %w", taskIDOrUnknown(task), readErrPath, readErr)
	}
	if modified {
		return nil
	}
	return fmt.Errorf("task %s (%s) modified no pre-existing file in its declared write set", taskIDOrUnknown(task), task.Type)
}

func requiresTaskMutationSnapshot(task *Task) bool {
	if task == nil {
		return false
	}
	return isMutatingTaskType(task.Type)
}

func (o *Orchestrator) captureTaskExecutionSnapshot(task *Task) (taskExecutionSnapshot, error) {
	var snapshot taskExecutionSnapshot

	if requiresCampaignStructuralSnapshot(task) {
		// Structural (assault) task: full-campaign transactional rollback.
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
	} else if task != nil {
		// Non-structural mutating task: scope rollback to this task's own status
		// (looked up in the live campaign) plus its file mutations. Never snapshot
		// the whole campaign — doing so reverts concurrent siblings' completions and
		// orphans runPhase's phase pointer (F-SCHED-2).
		snapshot.scopedTask = task
		snapshot.scopedStatus = o.currentTaskStatus(task.ID)
	}

	if task == nil {
		return snapshot, nil
	}

	writeSet := o.resolveTaskWriteSet(task)
	snapshot.globPreMatches = make(map[string]map[string]struct{})
	seenGlobs := make(map[string]struct{})
	for _, candidate := range writeSet {
		if !containsGlobMeta(candidate) {
			continue
		}
		if _, err := filepath.Match(candidate, ""); err != nil {
			return snapshot, fmt.Errorf("invalid glob pattern %q: %w", candidate, err)
		}
		if _, exists := seenGlobs[candidate]; exists {
			continue
		}
		matches, err := filepath.Glob(candidate)
		if err != nil {
			return snapshot, fmt.Errorf("invalid glob pattern %q: %w", candidate, err)
		}
		preMatches := make(map[string]struct{})
		for _, match := range matches {
			clean := filepath.Clean(match)
			info, err := os.Stat(clean)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return snapshot, fmt.Errorf("stat glob pre-match %s for %q: %w", clean, candidate, err)
			}
			if info.IsDir() {
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			preMatches[clean] = struct{}{}
		}
		seenGlobs[candidate] = struct{}{}
		snapshot.declaredGlobs = append(snapshot.declaredGlobs, candidate)
		snapshot.globPreMatches[candidate] = preMatches
	}
	sort.Strings(snapshot.declaredGlobs)
	expanded, err := expandSnapshotPaths(writeSet)
	if err != nil {
		return snapshot, err
	}
	for _, absPath := range expanded {
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

func containsGlobMeta(path string) bool {
	for _, r := range path {
		if r == '*' || r == '?' || r == '[' {
			return true
		}
	}
	return false
}

func expandSnapshotPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(paths)*2)
	var expanded []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if containsGlobMeta(p) {
			if _, err := filepath.Match(p, ""); err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", p, err)
			}
			matches, err := filepath.Glob(p)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", p, err)
			}
			if len(matches) == 0 {
				continue
			}
			for _, m := range matches {
				clean := filepath.Clean(m)
				if _, ok := seen[clean]; !ok {
					seen[clean] = struct{}{}
					expanded = append(expanded, clean)
				}
			}
			continue
		}
		clean := filepath.Clean(p)
		if _, ok := seen[clean]; !ok {
			seen[clean] = struct{}{}
			expanded = append(expanded, clean)
		}
	}
	if len(expanded) == 0 {
		return nil, nil
	}
	sort.Strings(expanded)
	deduped := expanded[:0]
	var prev string
	for i, v := range expanded {
		if i == 0 || v != prev {
			deduped = append(deduped, v)
			prev = v
		}
	}
	return deduped, nil
}

func (o *Orchestrator) rollbackTaskExecutionSnapshot(snapshot taskExecutionSnapshot) error {
	// File mutations are always reverted, for both structural and scoped snapshots.
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

	// NOTE: No broad-glob deletion here. New files matching a declared broad glob
	// have unknown provenance (human or concurrent agent) and are left visible for
	// operator recovery. Fail-closed detection is handled in validateFileModifyOutcome
	// via listNewBroadGlobMatches. Exact non-glob missing-path entries remain governed
	// by fileMutations snapshot rollback above.

	// Scoped (non-structural) rollback: restore only the failing task's own status
	// in place. This keeps o.campaign's identity and all sibling task state intact,
	// so concurrent completions survive and runPhase's phase pointer is never
	// orphaned. The failed task's terminal status is subsequently owned by
	// handleTaskFailure; this restore just undoes the in-progress transition.
	if snapshot.campaign == nil {
		if snapshot.scopedTask != nil {
			o.updateTaskStatus(snapshot.scopedTask, snapshot.scopedStatus)
		}
		return nil
	}

	// Structural (assault) rollback: full-campaign transactional restore.
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

// listNewBroadGlobMatches returns a sorted, deduplicated list of current
// regular-file matches for each declared broad glob that were absent from
// that glob's pre-task match set. It is detection-only: callers decide
// whether to fail closed or keep files visible for operator recovery.
// Exact non-glob paths are governed by fileMutations snapshot rollback.
func listNewBroadGlobMatches(snapshot taskExecutionSnapshot) ([]string, error) {
	if len(snapshot.declaredGlobs) == 0 {
		return nil, nil
	}
	newFilesSet := make(map[string]struct{})
	for _, glob := range snapshot.declaredGlobs {
		if glob == "" {
			continue
		}
		if !containsGlobMeta(glob) {
			continue
		}
		if _, err := filepath.Match(glob, ""); err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", glob, err)
		}
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, fmt.Errorf("glob %q failed: %w", glob, err)
		}
		seen := make(map[string]struct{}, len(matches))
		for _, m := range matches {
			clean := filepath.Clean(m)
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			if preSet, ok := snapshot.globPreMatches[glob]; ok {
				if _, existed := preSet[clean]; existed {
					continue
				}
			}
			if _, ok := newFilesSet[clean]; ok {
				continue
			}
			info, err := os.Stat(clean)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("stat %s: %w", clean, err)
			}
			if info.IsDir() {
				continue
			}
			newFilesSet[clean] = struct{}{}
		}
	}
	if len(newFilesSet) == 0 {
		return nil, nil
	}
	newFiles := make([]string, 0, len(newFilesSet))
	for p := range newFilesSet {
		newFiles = append(newFiles, p)
	}
	sort.Strings(newFiles)
	deduped := newFiles[:0]
	var prev string
	for i, v := range newFiles {
		if i == 0 || v != prev {
			deduped = append(deduped, v)
			prev = v
		}
	}
	return deduped, nil
}

// currentTaskStatus returns the status of a task (by ID) from the live campaign,
// or TaskPending if the task cannot be found.
func (o *Orchestrator) currentTaskStatus(taskID string) TaskStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return TaskPending
	}
	for i := range o.campaign.Phases {
		for j := range o.campaign.Phases[i].Tasks {
			if o.campaign.Phases[i].Tasks[j].ID == taskID {
				return o.campaign.Phases[i].Tasks[j].Status
			}
		}
	}
	return TaskPending
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
	maps.Copy(cloned, src)
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
