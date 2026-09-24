package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/observation"
	toolscore "codenerd/internal/tools/core"
)

// projectTaskReturn shapes a completed task's return for the ONE consumer that
// is another agent: the "CONTEXT FROM TASK" section buildTaskInput pastes into
// every dependent task's prompt.
//
// What was stored there before was json.Marshal of the handler's return value,
// cut at 1000 bytes by completeTask and again at 10240 by storeTaskResult. That is a
// transcript with its head kept and its tail thrown away — the shape most
// likely to end mid-sentence, and the shape that gives a dependent task the
// subagent's PREAMBLE rather than its conclusions, because a shard states its
// plan before its findings. The projection is bounded by structure instead, so
// a downstream task gets the findings, the citations, what changed and what was
// verified whether the upstream shard wrote four hundred bytes or forty
// thousand.
//
// Artifacts come from the task record rather than from the prose. The
// orchestrator already knows what this task was supposed to produce and has
// resolved the paths; reading them back out of the shard's own description of
// its work would be re-deriving a fact that was never in doubt.
func (o *Orchestrator) projectTaskReturn(task *Task, result any) string {
	ret := observation.Return{}
	if task != nil {
		ret.Task = task.Description
		ret.Agent = task.Shard
		if ret.Agent == "" {
			ret.Agent = string(task.Type)
		}
		for _, a := range task.Artifacts {
			if p := strings.TrimSpace(a.Path); p != "" {
				ret.Changed = append(ret.Changed, p)
			}
		}
	}

	switch v := result.(type) {
	case nil:
	case string:
		ret.Output = v
	case map[string]any:
		// The shape every explicit-shard handler returns. The "result" member
		// is the shard's own output; the rest is routing metadata the
		// projection header already carries.
		if out, ok := v["result"].(string); ok {
			ret.Output = out
			if shard, ok := v["shard"].(string); ok && strings.TrimSpace(shard) != "" {
				ret.Agent = shard
			}
			break
		}
		ret.Output = marshalForContext(result)
	default:
		ret.Output = marshalForContext(result)
	}

	return observation.SharedSubagents().
		EncodeReturn(ret, observation.ReturnLimits{}).
		Text(toolscore.SubagentExpandToolName)
}

// marshalForContext renders a handler return value that is not a shard output
// map. A value that will not marshal is reported as such rather than dropped:
// an empty context section reads as "the upstream task found nothing".
func marshalForContext(result any) string {
	data, err := json.Marshal(result)
	if err != nil {
		return "(task result could not be encoded for context injection)"
	}
	return string(data)
}

// storeTaskResult stores a task's result for context injection into dependent tasks.
func (o *Orchestrator) storeTaskResult(taskID, result string) {
	// Compute which results are still needed by pending/active tasks.
	needed := o.computeNeededResultIDs()

	// Stored whole: the projection is bounded by structure (projectTaskReturn),
	// and a byte cut here only ever removed its tail -- the last findings and
	// the handle that redeems the rest.
	o.resultsMu.Lock()
	defer o.resultsMu.Unlock()

	// Maintain insertion/LRU order
	if _, exists := o.taskResults[taskID]; exists {
		for i, id := range o.taskResultOrder {
			if id == taskID {
				o.taskResultOrder = append(o.taskResultOrder[:i], o.taskResultOrder[i+1:]...)
				break
			}
		}
	}
	o.taskResultOrder = append(o.taskResultOrder, taskID)

	o.taskResults[taskID] = result
	logging.CampaignDebug("Stored result for task %s (%d bytes)", taskID, len(result))

	// Prune cache if needed.
	limit := o.policy.TaskResultCacheLimit
	if len(o.taskResultOrder) > limit {
		pruned := 0
		rotations := 0
		for len(o.taskResultOrder) > limit && rotations < len(o.taskResultOrder) {
			oldest := o.taskResultOrder[0]
			o.taskResultOrder = o.taskResultOrder[1:]
			if needed[oldest] {
				// Keep needed results by rotating to the back.
				o.taskResultOrder = append(o.taskResultOrder, oldest)
			} else {
				delete(o.taskResults, oldest)
				pruned++
			}
			rotations++
		}
		if pruned > 0 {
			logging.CampaignDebug("Pruned %d task results (limit=%d)", pruned, limit)
		}
	}
}

// computeNeededResultIDs returns the set of task IDs whose results are referenced
// by pending/in-progress/blocked tasks via ContextFrom.
func (o *Orchestrator) computeNeededResultIDs() map[string]bool {
	needed := make(map[string]bool)
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return needed
	}
	for _, phase := range o.campaign.Phases {
		for _, task := range phase.Tasks {
			if task.Status == TaskPending || task.Status == TaskInProgress || task.Status == TaskBlocked {
				for _, dep := range task.ContextFrom {
					needed[dep] = true
				}
			}
		}
	}
	return needed
}

// getTaskResult retrieves a stored task result for context injection.
func (o *Orchestrator) getTaskResult(taskID string) (string, bool) {
	o.resultsMu.RLock()
	defer o.resultsMu.RUnlock()
	result, ok := o.taskResults[taskID]
	return result, ok
}

// buildTaskInput constructs the input for a shard: the task's
// ShardInput/Description, then what the campaign hands it (taskContextSection).
func (o *Orchestrator) buildTaskInput(ctx context.Context, task *Task) (string, error) {
	if task == nil {
		return "", nil
	}
	// Start with explicit shard input if provided, otherwise use description
	input := task.ShardInput
	if input == "" {
		input = task.Description
	}

	// Holographic context: every shard input carries the returns of its
	// context edges the kernel keeps (task_context_projection), the upstream
	// evidence it selects (task_evidence), whole, digested or by handle, and
	// the code its brief names (task_preload).
	// Prompt-only: persistTaskOutputArtifact stores the shard's result, never
	// this input, so the section is not re-persisted as the task's own artifact.
	upstream, err := o.taskContextSection(ctx, task)
	if err != nil {
		return "", err
	}
	if upstream != "" {
		if input != "" {
			input += "\n\n"
		}
		input += upstream
	}

	return input, nil
}

// renderContextFrom writes the stored return of each named task, in order: a
// task with no stored return (not run, or pruned) contributes nothing.
func (o *Orchestrator) renderContextFrom(ids []string) string {
	var parts []string
	for _, depID := range ids {
		result, ok := o.getTaskResult(depID)
		if !ok || result == "" {
			continue
		}
		parts = append(parts, "=== CONTEXT FROM TASK "+depID+" ===\n"+result)
		logging.CampaignDebug("Injected context from task %s (%d bytes)", depID, len(result))
	}
	return strings.Join(parts, "\n\n")
}

// writeSetBriefing tells a file task what it may change and what will
// satisfy it. A /file_modify task that creates a new file is refused by
// validateFileModifyOutcome, so the existing files are named here rather
// than left to the shard to guess. When nothing the write set declares
// exists, the plan guessed the target (ladder C2): the briefing names the
// guess as absent, and the change belongs in the existing code wherever it is.
func (o *Orchestrator) writeSetBriefing(task *Task) string {
	if task == nil {
		return ""
	}
	if task.Type != TaskTypeFileModify {
		return ""
	}
	writeSet := o.resolveTaskWriteSet(task)
	if len(writeSet) == 0 {
		return ""
	}
	// Directories that never carry a task's own change: version control,
	// runtime state, vendored copies, and fixture corpora.
	skipDir := map[string]bool{
		".git":     true,
		".nerd":    true,
		"vendor":   true,
		"testdata": true,
	}
	seen := make(map[string]bool)
	var relPaths []string
	workspace := ""
	if o != nil {
		workspace = o.workspace
	}
	toRel := func(abs string) string {
		slash := filepath.ToSlash(filepath.Clean(abs))
		if workspace != "" {
			if rel, err := filepath.Rel(workspace, filepath.FromSlash(slash)); err == nil {
				rel = filepath.ToSlash(rel)
				if rel != "." && rel != "" && !strings.HasPrefix(rel, "../") {
					return rel
				}
			}
		}
		return slash
	}
	var absent []string
	for _, entry := range writeSet {
		hostPath := filepath.FromSlash(entry)
		info, err := os.Stat(hostPath)
		if err != nil {
			if os.IsNotExist(err) && !containsGlobMeta(entry) {
				absent = append(absent, toRel(entry))
			}
			continue
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				continue
			}
			rel := toRel(entry)
			if !seen[rel] {
				seen[rel] = true
				relPaths = append(relPaths, rel)
			}
			continue
		}
		root := hostPath
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && skipDir[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel := toRel(filepath.ToSlash(path))
			if !seen[rel] {
				seen[rel] = true
				relPaths = append(relPaths, rel)
			}
			return nil
		})
	}
	list := func(paths []string) string {
		sort.Strings(paths)
		if len(paths) <= 40 {
			return strings.Join(paths, "\n")
		}
		return strings.Join(paths[:40], "\n") + fmt.Sprintf("\n... and %d more", len(paths)-40)
	}
	var b strings.Builder
	if len(relPaths) == 0 {
		if len(absent) == 0 {
			return ""
		}
		b.WriteString("\n\nTHIS TASK'S PLANNED TARGET DOES NOT EXIST (workspace-relative):\n")
		b.WriteString(list(absent))
		b.WriteString("\nThis task modifies existing code, and the plan guessed where that code lives. Creating the file above does not satisfy it: find the existing file that holds the code this task changes and make the change there. Add tests next to the code you changed.")
		return b.String()
	}
	b.WriteString("\n\nFILES THIS TASK MAY MODIFY (workspace-relative):\n")
	b.WriteString(list(relPaths))
	b.WriteString("\nThis task modifies existing files. Creating a new file does not satisfy it: the change must land in one of the files above, and a new helper file that nothing calls is not a change. Add tests next to the code you changed.")
	return b.String()
}
