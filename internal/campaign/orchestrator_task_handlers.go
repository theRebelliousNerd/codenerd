package campaign

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"codenerd/internal/build"
	"codenerd/internal/core"
	"codenerd/internal/evidence"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/testoutput"
	internaltypes "codenerd/internal/types"
	"crypto/sha256"
)

// spawnTask is the unified entry point for task execution.
func (o *Orchestrator) spawnTask(ctx context.Context, task *Task, intent, input string) (string, error) {
	o.mu.RLock()
	te := o.taskExecutor
	o.mu.RUnlock()

	if te == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	observed, ok := te.(session.ObservedTaskExecutor)
	if !ok {
		return "", fmt.Errorf("task executor %T returns no observed result; a campaign reads each task's verdict", te)
	}
	logging.CampaignDebug("spawnTask: using TaskExecutor for intent=%s", intent)
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       o.withPreviousAttempt(task, input),
	}
	if task != nil {
		ctx = session.WithWriteGuard(ctx, o.writeGuard(task))
	}
	ret, err := observed.ExecuteObserved(ctx, req)
	o.recordAttemptWrites(task, ret.Writes)
	if err != nil {
		// A turn that errored may still have been judged; its verdict is
		// part of why the attempt failed.
		return ret.Output, withSignals(err, returnSignals(ret)...)
	}
	if !ret.Done() {
		return ret.Output, turnNotDoneError(intent, ret)
	}
	return ret.Output, nil
}

// ErrTaskNotDone marks an attempt whose turn ran without an error but whose
// kernel verdict was not /done.
var ErrTaskNotDone = errors.New("the task's turn did not end done")

// turnNotDoneError names what the turn left unmet, from the kernel's own
// turn_missing_evidence, so the failure -- and the retry that reads it -- says
// why rather than only that.
// The verdict and the missing evidence ride on the error as typed signals, so
// the next move is derived from them rather than read back out of this text.
func turnNotDoneError(intent string, ret observation.Return) error {
	if ret.Outcome == "" {
		return fmt.Errorf("%w: the %s turn returned no verdict", ErrTaskNotDone, intent)
	}
	if why := session.DescribeMissingEvidence(ret.Missing); why != "" {
		return withSignals(fmt.Errorf("%w: the %s turn ended %s: %s", ErrTaskNotDone, intent, ret.Outcome, why), returnSignals(ret)...)
	}
	return withSignals(fmt.Errorf("%w: the %s turn ended %s", ErrTaskNotDone, intent, ret.Outcome), returnSignals(ret)...)
}

// withPreviousAttempt carries why the task's last attempt failed into the next
// attempt's input. A retry used to re-spawn the same request without the
// failure that stopped it, and repeated it (ladder C3: a create task retried
// with no word of its failure, and the create-only fallback then refused
// because the first attempt had left the file behind). The attempt is read
// from the live campaign by ID: a replan can orphan the caller's pointer.
func (o *Orchestrator) withPreviousAttempt(task *Task, input string) string {
	if task == nil {
		return input
	}
	var last string
	o.mu.RLock()
	if o.campaign != nil {
	search:
		for i := range o.campaign.Phases {
			for j := range o.campaign.Phases[i].Tasks {
				live := &o.campaign.Phases[i].Tasks[j]
				if live.ID != task.ID {
					continue
				}
				for k := len(live.Attempts) - 1; k >= 0; k-- {
					if a := live.Attempts[k]; a.Outcome == "/failure" && strings.TrimSpace(a.Error) != "" {
						last = a.Error
						break
					}
				}
				break search
			}
		}
	}
	o.mu.RUnlock()
	if last == "" {
		return input
	}
	return input + "\n\n=== THE PREVIOUS ATTEMPT AT THIS TASK FAILED ===\n" + last +
		"\n\nThat attempt's work may still be in the workspace: read what is there before writing, and fix the failure above rather than repeating the attempt."
}

// executeTask executes a single task.
func (o *Orchestrator) executeTask(ctx context.Context, task *Task) (any, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if err := validateTaskEffect(task); err != nil {
		return nil, err
	}
	logging.CampaignDebug("Executing task %s with type %s, shard=%s", task.ID, task.Type, task.Shard)

	// Update task status
	o.updateTaskStatus(task, TaskInProgress)

	if task.Type == TaskTypeTestRun {
		return o.executeTestRunTask(ctx, task)
	}

	// If task has explicit shard specified, use generic shard routing with context injection
	if task.Shard != "" {
		logging.CampaignDebug("Using explicit shard routing: %s", task.Shard)
		return o.executeWithExplicitShard(ctx, task)
	}

	// Fallback to type-based routing for backward compatibility
	switch task.Type {
	case TaskTypeAssaultDiscover:
		logging.CampaignDebug("Delegating to assault discover handler")
		return o.executeAssaultDiscoverTask(ctx, task)
	case TaskTypeAssaultBatch:
		logging.CampaignDebug("Delegating to assault batch handler")
		return o.executeAssaultBatchTask(ctx, task)
	case TaskTypeAssaultTriage:
		logging.CampaignDebug("Delegating to assault triage handler")
		return o.executeAssaultTriageTask(ctx, task)
	case TaskTypeResearch:
		logging.CampaignDebug("Delegating to research task handler")
		return o.executeResearchTask(ctx, task)
	case TaskTypeFileCreate, TaskTypeFileModify:
		logging.CampaignDebug("Delegating to file task handler")
		return o.executeFileTask(ctx, task)
	case TaskTypeTestWrite:
		logging.CampaignDebug("Delegating to test write handler")
		return o.executeTestWriteTask(ctx, task)
	case TaskTypeTestRun:
		logging.CampaignDebug("Delegating to test run handler")
		return o.executeTestRunTask(ctx, task)
	case TaskTypeVerify:
		logging.CampaignDebug("Delegating to verify handler")
		return o.executeVerifyTask(ctx, task)
	case TaskTypeShardSpawn:
		logging.CampaignDebug("Delegating to shard spawn handler")
		return o.executeShardSpawnTask(ctx, task)
	case TaskTypeRefactor:
		logging.CampaignDebug("Delegating to refactor handler")
		return o.executeRefactorTask(ctx, task)
	case TaskTypeIntegrate:
		logging.CampaignDebug("Delegating to integrate handler")
		return o.executeIntegrateTask(ctx, task)
	case TaskTypeDocument:
		logging.CampaignDebug("Delegating to document handler")
		return o.executeDocumentTask(ctx, task)
	case TaskTypeToolCreate:
		logging.CampaignDebug("Delegating to tool create handler (Ouroboros)")
		return o.executeToolCreateTask(ctx, task)
	case TaskTypeCampaignRef:
		logging.CampaignDebug("Delegating to sub-campaign handler")
		return o.executeCampaignRefTask(ctx, task)
	default:
		logging.CampaignDebug("Using generic task handler for type: %s", task.Type)
		return o.executeGenericTask(ctx, task)
	}
}

// executeWithExplicitShard handles tasks with explicitly specified shard routing.
// This enables the campaign system to call ANY shard at will with context injection.
func (o *Orchestrator) executeWithExplicitShard(ctx context.Context, task *Task) (any, error) {
	shardType := task.Shard
	logging.Campaign("Executing task %s with explicit shard: %s", task.ID, shardType)

	// Build input with context injection from dependent tasks AND specialist knowledge
	input := o.buildTaskInputWithSpecialistKnowledge(ctx, task, shardType)
	logging.CampaignDebug("Built shard input (%d bytes) for task %s", len(input), task.ID)

	// Spawn the shard via unified spawnTask
	result, err := o.spawnTask(ctx, task, shardType, input)
	if err != nil {
		// F-DOC-1: /document tasks are the campaign's deliverables (reports,
		// rubrics). The decomposer often routes them to the coder shard, which
		// explores without ever writing (tripping the hollow-success guard) and
		// permanently fails the task — deadlocking the phase. Fall back to direct
		// document generation so the deliverable is still produced and the phase
		// can complete. Non-document tasks keep the hard failure.
		if task.Type == TaskTypeDocument {
			logging.Get(logging.CategoryCampaign).Warn("Shard %s failed for document task %s: %v; falling back to direct document generation", shardType, task.ID, err)
			var targetPath string
			if len(task.Artifacts) > 0 {
				targetPath = task.Artifacts[0].Path
			}
			return o.executeFileTaskFallback(ctx, task, targetPath, fmt.Errorf("shard %s failed: %w", shardType, err))
		}
		logging.Get(logging.CategoryCampaign).Error("Shard %s failed for task %s: %v", shardType, task.ID, err)
		return nil, fmt.Errorf("shard %s failed: %w", shardType, err)
	}

	logging.CampaignDebug("Shard %s completed for task %s, result_len=%d", shardType, task.ID, len(result))

	// F-DURABLE-1: an explicit-shard task that produces analysis (research,
	// audit, review, discovery) rather than a file returns its result only in
	// memory. Persist it as a durable artifact so the findings survive and the
	// phase-checkpoint reviewer can verify a real output. No-op for file/test/
	// tool tasks and when a durable output already exists (see helper).
	o.persistTaskOutputArtifact(task, result)

	// F-HOLLOW-2 (run 14 phase 1: shard=reviewer returned "result":"" for an
	// audit): an analysis task delivered when an output it declares is on
	// disk with content, and nothing else. A done turn that left none fails
	// the attempt, and the campaign's retry carries why. Until 2026-09-23 the
	// answer's shape decided instead -- under 40 runes, or opening "I'll" or
	// "let me" -- and triggered an inline re-spawn with a Go-written prompt,
	// so a terse real finding was redone (sweep finding F12). File, test and
	// tool tasks write their own output and are judged by their write gates.
	if !isFileProducingType(task.Type) && !o.hasDeliverableOnDisk(task) {
		o.emitEvent(EventShardResultEmpty, task.PhaseID, task.ID, fmt.Sprintf("shard %s ended done with nothing to persist", shardType), nil)
		return nil, fmt.Errorf("%w: shard %s ended done for %s with nothing to persist", ErrNoDeliverable, shardType, task.ID)
	}

	return map[string]any{
		"shard":  shardType,
		"result": result,
		"task":   task.ID,
	}, nil
}

// ErrNoDeliverable marks an analysis attempt whose turn ended done but left no
// durable output: no declared output on disk with content, and no answer to
// persist in its place.
var ErrNoDeliverable = errors.New("the task's turn delivered nothing")

// isTrivialResult reports whether an upstream result carries no substantive
// content (countFindingUpstreams). The 40-rune floor rejects empty responses
// and one-line acknowledgements.
func isTrivialResult(s string) bool {
	return len([]rune(strings.TrimSpace(s))) < 40
}

// isFileProducingType reports whether a task type writes its own durable output
// (a file, test, or generated tool). Such tasks legitimately return only a short
// confirmation string, so they are exempt from the empty-result retry and from
// analysis-artifact persistence.
func isFileProducingType(t TaskType) bool {
	switch t {
	case TaskTypeFileCreate, TaskTypeFileModify, TaskTypeTestWrite,
		TaskTypeTestRun, TaskTypeToolCreate, TaskTypeRefactor:
		return true
	}
	return false
}

// isOutputArtifactType reports whether an artifact type is a task's output.
// Input artifacts (/source_file, /knowledge_base) are the material a task
// works on, not what it delivers.
func isOutputArtifactType(t string) bool {
	switch t {
	case "/doc", "/test_file", "/config", "/file":
		return true
	}
	return false
}

// hasDeliverableOnDisk reports whether an output artifact the task holds is on
// disk with content: the evidence an analysis task delivered.
func (o *Orchestrator) hasDeliverableOnDisk(task *Task) bool {
	for _, a := range task.Artifacts {
		if !isOutputArtifactType(a.Type) || a.Path == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(o.workspace, a.Path)); err == nil && fi.Size() > 0 {
			return true
		}
	}
	return false
}

// executeResearchTask spawns a researcher shard.
func (o *Orchestrator) executeResearchTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Spawning researcher shard for task %s", task.ID)
	result, err := o.spawnTask(ctx, task, "/research", o.buildTaskInput(task))
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Researcher shard failed for task %s: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Researcher shard completed for task %s", task.ID)

	// F-DURABLE-1: research/audit tasks previously returned their findings only
	// in memory, leaving nothing on disk. The phase-checkpoint reviewer then
	// correctly reported "no durable discovery outputs" and failed the phase even
	// though the work was done (observed live, run 12 phases 0/1). Persist the
	// findings so they survive the campaign and the reviewer has a real output.
	o.persistTaskOutputArtifact(task, result)

	// F-HOLLOW-1 (run 13 task 0_2: an empty final response marked completed):
	// the task delivered when its findings are on disk. A done turn with
	// nothing to persist fails the attempt, and the campaign's retry carries
	// why; the answer's shape is not evidence either way (sweep finding F12).
	// Whether findings are substantive is the phase checkpoint's judgement.
	if !o.hasDeliverableOnDisk(task) {
		o.emitEvent(EventResearchEmpty, task.PhaseID, task.ID, "research turn ended done with no findings to persist", nil)
		return nil, fmt.Errorf("%w: the research turn for %s ended done with no findings to persist", ErrNoDeliverable, task.ID)
	}
	return map[string]any{"research_result": result}, nil
}

// persistTaskOutputArtifact writes an analysis/research/audit task's textual
// result to a durable campaign artifact and registers it on the task. Such tasks
// return their findings only as an in-memory map, so without this the work leaves
// no durable trace: an audit campaign's findings evaporate when the run ends, and
// the phase-checkpoint reviewer reports "no durable discovery outputs" and fails
// the phase on merit. Persisting the result as a /doc artifact makes the findings
// durable and gives the reviewer something real to verify.
//
// It is a no-op for tasks that produce their own durable output (file/test/tool)
// and for tasks that already carry a durable output artifact on disk. Input
// artifacts (/source_file, /knowledge_base) are the material being audited, not
// outputs, so their presence does not suppress persistence.
func (o *Orchestrator) persistTaskOutputArtifact(task *Task, result string) {
	if task == nil {
		return
	}
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return // nothing to persist
	}
	if isFileProducingType(task.Type) {
		return // these produce their own durable file/test/tool output
	}
	for _, a := range task.Artifacts {
		if isOutputArtifactType(a.Type) && a.Path != "" {
			if _, err := os.Stat(filepath.Join(o.workspace, a.Path)); err == nil {
				return // a durable output already exists; don't duplicate it
			}
		}
	}
	relPath := o.defaultTaskArtifactPath(task)
	fullPath := filepath.Join(o.workspace, relPath)
	// This is the one direct write the orchestrator keeps, and only because it
	// targets the campaign's own artifacts directory. Refuse anything that
	// resolves outside it: repository files go through the VirtualStore.
	if !o.insideCampaignArtifacts(fullPath) {
		logging.Get(logging.CategoryCampaign).Warn("Refusing durable-output write outside the campaign artifacts dir: %s", fullPath)
		return
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Failed to create artifact dir for task %s: %v", task.ID, err)
		return
	}
	header := fmt.Sprintf("# %s\n\n_Durable output for task %s (%s)._\n\n", task.Description, task.ID, task.Type)
	if err := os.WriteFile(fullPath, []byte(header+trimmed+"\n"), 0o644); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Failed to persist output artifact for task %s: %v", task.ID, err)
		return
	}
	task.Artifacts = append(task.Artifacts, TaskArtifact{Type: "/doc", Path: relPath})
	logging.Campaign("Persisted durable output artifact for task %s: %s (%d bytes)", task.ID, relPath, len(trimmed))
	o.emitEvent(EventArtifactPersisted, task.PhaseID, task.ID, relPath, nil)
}

// resolveFileTaskTargetPath resolves the file-task target path from the task's
// declared outputs: Artifacts[0].Path, else the first exact (non-glob)
// WriteSet entry, else a path extracted from the description. WriteSet entries
// are stored absolute (see normalizeWriteSetPaths); relativize them against the
// orchestrator workspace so the result stays suitable for filepath.Join and for
// the "file:<path>" shard string.
func (o *Orchestrator) resolveFileTaskTargetPath(task *Task) string {
	if task == nil {
		return ""
	}
	if len(task.Artifacts) > 0 {
		if p := strings.TrimSpace(task.Artifacts[0].Path); p != "" {
			return p
		}
	}
	for _, raw := range task.WriteSet {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if containsGlobMeta(p) {
			continue
		}
		if filepath.IsAbs(filepath.FromSlash(p)) {
			ws := ""
			if o != nil {
				ws = o.workspace
			}
			if ws != "" {
				rel, err := filepath.Rel(ws, filepath.FromSlash(p))
				if err != nil {
					continue
				}
				rel = filepath.ToSlash(filepath.Clean(rel))
				if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
					continue
				}
				return rel
			}
		}
		return filepath.ToSlash(filepath.Clean(filepath.FromSlash(p)))
	}
	if task.Description != "" {
		if inferred := extractPathFromDescription(task.Description); inferred != "" {
			return inferred
		}
	}
	return ""
}

// executeFileTask creates or modifies a file using the Coder shard.
func (o *Orchestrator) executeFileTask(ctx context.Context, task *Task) (any, error) {
	targetPath := o.resolveFileTaskTargetPath(task)
	if targetPath == "" {
		taskID := ""
		if task != nil {
			taskID = task.ID
		}
		return nil, fmt.Errorf("file task %s has no target path (no artifact, no write set, none in description)", taskID)
	}
	logging.CampaignDebug("Executing file task %s: path=%s", task.ID, targetPath)

	// F-CAMP-1: resolve the target exactly as the success path verifies it and
	// record whether it is an existing directory. Recurse tasks carry a
	// DIRECTORY as their target (subsystem Paths, e.g. "internal/mangle"), and
	// the fallback below is a creator that writes one file: for a directory
	// target or a modify task its refusal must surface as the task's real
	// error, not a misleading kernel write refusal over a directory.
	fullPath := ""
	if filepath.IsAbs(targetPath) {
		fullPath = filepath.Clean(targetPath)
	} else {
		fullPath = filepath.Join(o.workspace, targetPath)
	}
	isDirectoryTarget := false
	targetExisted := false
	if info, statErr := os.Stat(fullPath); statErr == nil {
		targetExisted = true
		isDirectoryTarget = info.IsDir()
	}
	// For a directory target, success means the workspace changed under it, so
	// take the evidence snapshot before the shard runs (same mechanism
	// executeTestRunTask uses). A directory never stats as a regular file, so
	// the post-shard stat check can never verify it on its own.
	var before string
	if isDirectoryTarget {
		snap, snapErr := evidence.Snapshot(ctx, o.workspace)
		if snapErr != nil {
			return nil, fmt.Errorf("evidence snapshot before shard for directory target %s: %w", targetPath, snapErr)
		}
		before = snap
	}

	// Build task string for coder shard
	// NOTE: Don't use "instruction:<value>" format because strings.Fields() splits on spaces,
	// causing multi-word instructions to be truncated. Use simpler format where bare words
	// are joined into the instruction by parseTask.
	// F-STEP-1: a directory target is a package target, never a file target —
	// reuse the isDirectoryTarget stat above (same test testWriteShardTask
	// uses) so the planner's work steps do not run as file edits.
	action := "create"
	if task.Type == TaskTypeFileModify {
		action = "modify"
	}
	targetLabel := "file:"
	if isDirectoryTarget {
		targetLabel = "package:"
	}
	shardTask := fmt.Sprintf("%s %s%s %s", action, targetLabel, targetPath, o.buildTaskInput(task))
	// F-REC-9: a /file_modify task is satisfied only by changing one of its
	// declared write-set files. Name them so the shard does not guess a new
	// helper file instead. /file_create keeps today's string exactly.
	if task.Type == TaskTypeFileModify {
		shardTask += o.writeSetBriefing(task)
	}
	logging.CampaignDebug("Spawning coder shard: action=%s, path=%s, task=%s", action, targetPath, shardTask)

	// Delegate to coder shard
	result, err := o.spawnTask(ctx, task, "/fix", shardTask)
	if err != nil {
		// F-CAMP-3: once the context is expired or cancelled, any fallback's
		// LLM call can only fail with a bare "context deadline exceeded" that
		// hides the real shard failure. Surface the shard error, wrapped with
		// the task identity, instead of falling back.
		if ctxErr := ctx.Err(); ctxErr != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logging.Get(logging.CategoryCampaign).Warn("Shard failed for task %s with an expired context; surfacing shard error instead of falling back: %v", task.ID, err)
			return nil, fmt.Errorf("coder shard failed for %s task %s on %s: %w", task.Type, task.ID, targetPath, err)
		}
		// F-CAMP-1: the fallback is a creator, never an editor. For a directory
		// target it would generate content and try to write it over the
		// directory (kernel refusal that hides the real failure); for a modify
		// task it would clobber the existing file with generated content. Keep
		// the fallback only for task types it was built for.
		if isDirectoryTarget || task.Type == TaskTypeFileModify {
			logging.Get(logging.CategoryCampaign).Warn("Coder shard failed for %s task %s on %s; refusing create-style fallback so the real failure stays visible", task.Type, task.ID, targetPath)
			return nil, fmt.Errorf("coder shard failed for %s task %s on %s: %w", task.Type, task.ID, targetPath, err)
		}
		logging.Get(logging.CategoryCampaign).Warn("Coder shard failed for task %s, using fallback: %v", task.ID, err)
		// Fallback to direct LLM if shard fails
		return o.executeFileTaskFallback(ctx, task, targetPath, fmt.Errorf("coder shard failed for %s task %s on %s: %w", task.Type, task.ID, targetPath, err))
	}

	logging.CampaignDebug("Coder shard completed for task %s, result_len=%d", task.ID, len(result))

	// CRITICAL: Verify file was actually written
	// Shards may return successfully without calling write_file tool.
	// An empty target or a directory must never count as success: an empty
	// target joins to the workspace root itself, which always stats.
	verified := false
	if targetPath != "" {
		if info, statErr := os.Stat(fullPath); statErr == nil && !info.IsDir() && info.Mode().IsRegular() {
			verified = true
		}
	}
	// F-CAMP-1: a directory target is modified in place, so the success
	// criterion is that the workspace changed under it — measured by the
	// before/after evidence snapshot, never by the stat of the directory
	// itself, which existed before the shard ran.
	if !verified && isDirectoryTarget {
		after, snapErr := evidence.Snapshot(ctx, o.workspace)
		if snapErr != nil {
			return nil, fmt.Errorf("evidence snapshot after shard for directory target %s: %w", targetPath, snapErr)
		}
		verified = after != before
	}
	// Ladder C2: a /file_modify whose planned target did not exist is satisfied
	// by changing existing code where that code is, not by the guessed target
	// appearing. validateFileModifyOutcome, around this call, decides from the
	// attempt's writes; the create-style fallback below must not run for it.
	if !verified && task.Type == TaskTypeFileModify && !targetExisted {
		return map[string]any{"coder_result": result, "path": targetPath}, nil
	}
	if !verified {
		if isDirectoryTarget {
			// F-CAMP-1: a hollow success on a directory target must not fall
			// back to the creator path; that path generates a file and tries to
			// write it over the directory, turning "shard broke the tests" into
			// a kernel write refusal.
			logging.Get(logging.CategoryCampaign).Warn("Coder shard reported success for task %s but changed nothing under %s", task.ID, targetPath)
			return nil, fmt.Errorf("coder shard reported success for %s but changed nothing under %s", task.ID, targetPath)
		}
		logging.Get(logging.CategoryCampaign).Warn("Coder shard returned but file not created or not a regular file: %s, using fallback", fullPath)
		// Shard didn't write file - fall back to direct LLM
		return o.executeFileTaskFallback(ctx, task, targetPath, fmt.Errorf("coder shard reported success for %s but did not write %s", task.ID, targetPath))
	}

	logging.Campaign("File verified after shard execution: %s", fullPath)
	return map[string]any{"coder_result": result, "path": targetPath}, nil
}

// executeFileTaskFallback generates a deliverable document directly when the
// shard that should have written it failed; cause is that failure. It writes
// documents only. Code and configuration carry the turn's obligations -- tests,
// coverage, vet, the verdict -- which a bare generation call does not, so a
// coder stopped on them fails its task instead of landing the file another way
// (campaign 1284b6bb: "hollow success blocked: turn created Go source ...
// without a test file", then a request to "Generate the following file").
// Every failure leads with cause: the task's error is what the retry and the
// replanner read, and the fallback's own refusal is not why the work stopped.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string, cause error) (any, error) {
	logging.CampaignDebug("Executing file task fallback for %s via direct LLM", task.ID)
	fail := func(err error) error {
		if cause == nil {
			return err
		}
		return fmt.Errorf("%w; then the fallback: %w", cause, err)
	}

	// If no target path, try to extract from task description.
	if targetPath == "" {
		targetPath = extractPathFromDescription(task.Description)
		if targetPath != "" {
			logging.CampaignDebug("Extracted target path from description: %s", targetPath)
		}
	}
	// F-TASK-1: the decomposer frequently emits artifact-producing tasks
	// (/document, /file_create) with no target path/artifact. Failing them
	// permanently deadlocks the phase (a failed task blocks phase completion).
	// Write to a deterministic campaign artifact path instead so the work
	// product is preserved and the phase can proceed. Tasks that mutate a
	// specific existing file (/file_modify, /refactor, /integrate) still require
	// an explicit path — defaulting one would be meaningless.
	if targetPath == "" {
		if task.Type == TaskTypeDocument || task.Type == TaskTypeFileCreate {
			targetPath = o.defaultTaskArtifactPath(task)
			logging.Campaign("No target path for %s task %s; defaulting to campaign artifact %s", task.Type, task.ID, targetPath)
		} else {
			logging.Get(logging.CategoryCampaign).Error("No target path for file task %s and could not extract from description", task.ID)
			return nil, fail(fmt.Errorf("no target path specified for file task %s", task.ID))
		}
	}

	// Path traversal guard
	cleanPath := filepath.Clean(targetPath)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return nil, fail(fmt.Errorf("path traversal attempt blocked for path: %s", targetPath))
	}
	targetPath = cleanPath

	// The fallback is a creator, never an editor. Refuse to overwrite an
	// existing file before generating anything: a hollow coder result must fail
	// the task (retry -> attempt-cap re-plan), never truncate real work through
	// this back door. An existing DIRECTORY is refused too (F-CAMP-1): this
	// path can only write one file, and aiming it at a directory previously
	// produced a misleading kernel write refusal instead of the real failure.
	statPath := filepath.Join(o.workspace, targetPath)
	if info, statErr := os.Stat(statPath); statErr == nil {
		if info.IsDir() {
			logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: target %s is an existing directory; the fallback writes a single file, not a directory", task.ID, targetPath)
			return nil, fail(fmt.Errorf("fallback refused for %s: target %s is an existing directory and the fallback cannot write a directory", task.ID, targetPath))
		}
		if info.Mode().IsRegular() {
			logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: target %s already exists; modify tasks need the coder path", task.ID, targetPath)
			return nil, fail(fmt.Errorf("fallback refused for %s: target %s exists (%d bytes); modify tasks need the coder path", task.ID, targetPath, info.Size()))
		}
	}

	// Documents only: see the doc comment. Checked before anything is
	// generated, so a refused target costs no model call.
	if !isDeliverableDocument(targetPath) {
		logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: %s is not a document; a coder stopped on code fails its task", task.ID, targetPath)
		return nil, fail(fmt.Errorf("fallback refused for %s: %s is not a document; direct generation writes deliverables, never code or configuration", task.ID, targetPath))
	}

	// Front door only: repository writes go through the VirtualStore so
	// permitted/3, the Dreamer gate and the FileWriteValidator apply. Never
	// fall back to a direct write around them.
	if o.virtualStore == nil {
		logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: no VirtualStore attached; refusing to write %s around the front door", task.ID, targetPath)
		return nil, fail(fmt.Errorf("fallback refused for %s: virtualStore is nil, cannot write %s through the VirtualStore", task.ID, targetPath))
	}

	// Holographic context: the fallback prompt carries upstream durable
	// findings after the description and before the file target, so a direct-LLM
	// document still sees what to write from. Prompt-only; the written file is
	// the shard result, never this input section.
	taskBlock := task.Description
	if upstream := o.upstreamArtifactContext(task); upstream != "" {
		taskBlock = taskBlock + "\n\n" + upstream
	}
	prompt := fmt.Sprintf(`Generate the following file:
Task: %s
Target Path: %s

Output ONLY the file content, no explanation or markdown fences:`, taskBlock, targetPath)

	content, err := o.llmClient.Complete(ctx, prompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM file generation failed for task %s: %v", task.ID, err)
		return nil, fail(fmt.Errorf("generate %s: %w", targetPath, err))
	}

	// Extract code block from LLM response (removes reasoning traces and markdown fences)
	lang := getLangFromPath(targetPath)
	content = extractCodeBlock(content, lang)
	logging.CampaignDebug("Extracted code block for %s (lang=%s, %d bytes)", targetPath, lang, len(content))

	// F-DOC-2: guard against pathological model repetition loops (observed live:
	// Grok emitting "1. End. 2. Finish." x1500 as a 19KB artifact). Without this
	// the degenerate output passes the non-empty check and is counted as task
	// success. Retry once with an explicit anti-repetition instruction; if the
	// model still degenerates, persist an honest placeholder rather than garbage.
	if isDegenerateGeneration(content) {
		logging.Get(logging.CategoryCampaign).Warn("Fallback generation for %s is degenerate (%d bytes); retrying with anti-repetition guard", task.ID, len(content))
		retryPrompt := prompt + "\n\nIMPORTANT: Produce a concise, non-repetitive document. Do NOT repeat words, phrases, or numbered lines. Stop as soon as the content is complete."
		if retried, rerr := o.llmClient.Complete(ctx, retryPrompt); rerr == nil {
			if rc := extractCodeBlock(retried, lang); rc != "" && !isDegenerateGeneration(rc) {
				content = rc
				logging.Campaign("Anti-repetition retry recovered a coherent document for %s (%d bytes)", task.ID, len(content))
			} else {
				content = degradedGenerationPlaceholder(task, targetPath)
				logging.Get(logging.CategoryCampaign).Warn("Retry still degenerate for %s; writing honest degraded placeholder", task.ID)
			}
		} else {
			content = degradedGenerationPlaceholder(task, targetPath)
			logging.Get(logging.CategoryCampaign).Warn("Anti-repetition retry failed for %s (%v); writing honest degraded placeholder", task.ID, rerr)
		}
		o.emitEvent(EventGenerationDegraded, "", task.ID, "fallback document generation was degenerate", nil)
	}

	fullPath := filepath.Join(o.workspace, targetPath)
	logging.CampaignDebug("Writing generated file via VirtualStore: %s (%d bytes)", fullPath, len(content))
	// Front door only: the write goes through the VirtualStore so permitted/3,
	// the Dreamer gate and the FileWriteValidator apply. A direct os.WriteFile
	// here bypassed every one of them (observed live: a 251-line policy file
	// truncated to a 9-line stub).
	// The router executes only what the kernel has permitted, and permission
	// derives from a pending_action fact the caller asserts first — the same
	// contract the session executor follows. Assert it so the constitution
	// decides (a critical path or an unsafe target is denied there, not here).
	actionID := fmt.Sprintf("campaign-fallback-%s", task.ID)
	// permitted/3 is matched against the exact payload the action is routed
	// with: the canonical JSON the session executor asserts (json.Marshal of
	// the args; over the cap refused, never truncated). This used to assert a
	// {"content_bytes":N} summary instead, so no fallback write was ever
	// permitted -- the kernel refused every document the model had just been
	// asked to generate.
	payload := map[string]any{"content": content}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fail(fmt.Errorf("encode the fallback write for %s: %w", targetPath, err))
	}
	if len(payloadJSON) > session.MaxActionPayloadBytes {
		return nil, fail(fmt.Errorf("fallback refused for %s: the generated %s is %d bytes encoded, over the %d-byte action payload cap",
			task.ID, targetPath, len(payloadJSON), session.MaxActionPayloadBytes))
	}
	if o.kernel != nil {
		pending := core.Fact{
			Predicate: "pending_action",
			Args:      []any{actionID, core.MangleAtom("/write_file"), fullPath, string(payloadJSON), time.Now().Unix()},
		}
		if err := o.kernel.Assert(pending); err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Failed to assert pending_action for fallback write %s: %v", fullPath, err)
		} else {
			defer func() { _ = o.kernel.RetractFact(pending) }()
		}
	}
	writeResult, err := o.virtualStore.RouteActionResult(ctx, core.Fact{
		Predicate: "next_action",
		Args:      []any{actionID, "write_file", fullPath, payload},
	})
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("VirtualStore write failed for fallback file %s: %v", fullPath, err)
		return nil, fail(err)
	}
	if !writeResult.Success {
		logging.Get(logging.CategoryCampaign).Warn("VirtualStore refused fallback write for %s: %s", fullPath, writeResult.Error)
		return nil, fail(fmt.Errorf("fallback write for %s refused by VirtualStore: %s", targetPath, writeResult.Error))
	}

	logging.CampaignDebug("File fallback completed via VirtualStore: %s", fullPath)
	return map[string]any{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code using the Tester shard.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	targetPath := o.resolveFileTaskTargetPath(task)
	logging.CampaignDebug("Executing test write task %s: target=%s", task.ID, targetPath)

	// Build task string for tester shard
	shardTask := o.testWriteShardTask(task, targetPath)

	// Delegate to tester shard
	result, err := o.spawnTask(ctx, task, "/test", shardTask)
	if err != nil {
		// F-CAMP-3: an expired or cancelled context makes any downstream
		// fallback's LLM call fail with a bare "context deadline exceeded" that
		// hides the real tester failure. Surface the shard error instead.
		if ctxErr := ctx.Err(); ctxErr != nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test write task %s with an expired context; surfacing shard error instead of falling back: %v", task.ID, err)
			return nil, fmt.Errorf("tester shard failed for test_write task %s on %s: %w", task.ID, targetPath, err)
		}
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test write task %s, falling back to coder: %v", task.ID, err)
		// Fallback to coder shard for test generation
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Test write task completed: %s", task.ID)
	return map[string]any{"tester_result": result, "target": targetPath}, nil
}

// testWriteShardTask builds the tester shard task string so the label matches
// what the target is: an existing directory is a package target, any other
// non-empty target is a file target, and an empty target carries no dangling
// "file:" label. The full path resolves the same way executeFileTask does.
func (o *Orchestrator) testWriteShardTask(task *Task, targetPath string) string {
	if targetPath != "" {
		var fullPath string
		if filepath.IsAbs(targetPath) {
			fullPath = filepath.Clean(targetPath)
		} else {
			fullPath = filepath.Join(o.workspace, targetPath)
		}
		if info, statErr := os.Stat(fullPath); statErr == nil && info.IsDir() {
			return fmt.Sprintf("generate_tests package:%s %s", targetPath, o.buildTaskInput(task))
		}
		return fmt.Sprintf("generate_tests file:%s %s", targetPath, o.buildTaskInput(task))
	}
	return fmt.Sprintf("generate_tests %s", o.buildTaskInput(task))
}

// executeTestRunTask executes the declared check through the gated VirtualStore.
// Model prose is never an execution receipt. Test-writing and expert consultation
// remain separate tasks; running an already identified test needs no model call.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	o.mu.Lock()
	task.TestWitness = nil
	o.mu.Unlock()
	if o.virtualStore == nil {
		return nil, fmt.Errorf("test execution requires VirtualStore")
	}
	target := "./..."
	if len(task.Artifacts) > 0 {
		target = task.Artifacts[0].Path
	}
	if target == "" || strings.HasPrefix(target, "-") || strings.ContainsAny(target, "\\ :;|&$`\"'\n\r\t(){}[]<>") {
		return nil, fmt.Errorf("invalid test package target %q", target)
	}
	if o.kernel == nil {
		return nil, fmt.Errorf("test execution requires kernel authorization")
	}
	before, err := evidence.Snapshot(ctx, o.workspace)
	if err != nil {
		return nil, err
	}
	// Workspace build tags: a tagless run in codeNERD's own tree tests a
	// different build (sqlite-vec files excluded). No-op elsewhere.
	cmdParts := append([]string{"go", "test", "-count=1"}, build.TestTagsForWorkspace(o.workspace)...)
	command := strings.Join(append(cmdParts, target), " ")
	actionID := "campaign-check-" + task.ID
	// campaign.test_run_timeout, in the seconds the run_tests action takes.
	// permitted/3 matches the payload the action is routed with, so the
	// asserted JSON and the routed map carry the same number.
	timeoutSec := int(o.policy.TestRunTimeout.Seconds())
	payload := map[string]any{"timeout": timeoutSec}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	pending := core.Fact{Predicate: "pending_action", Args: []any{actionID, core.MangleAtom("/run_tests"), command, string(payloadJSON), time.Now().Unix()}}
	if err := o.kernel.Assert(pending); err != nil {
		return nil, err
	}
	defer o.kernel.RetractFact(pending)
	result, err := o.virtualStore.RouteActionResult(ctx, core.Fact{Predicate: "next_action", Args: []any{actionID, "run_tests", command, payload}})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, withSignals(fmt.Errorf("test execution failed: %s", testFailureSummary(target, result.Error, result.Output)), "/tests_red")
	}
	after, err := evidence.Snapshot(ctx, o.workspace)
	if err != nil {
		return nil, err
	}
	if before != after {
		return nil, fmt.Errorf("workspace changed during test execution; witness invalidated")
	}
	witness := &TestExecutionWitness{Snapshot: after, Command: command, OutputHash: fmt.Sprintf("%x", sha256.Sum256([]byte(result.Output))), RecordedAt: time.Now().UTC()}
	o.mu.Lock()
	task.TestWitness = witness
	o.mu.Unlock()
	return map[string]any{"target": target, "passed": true, "output": result.Output, "witness": witness, "witness_source": "virtual_store/run_tests"}, nil
}

// testFailureSummary leads a failed run's error with the parsed failure list
// instead of the raw log. Downstream only the head of this error survives —
// the repro task carries 220 chars of "last error" — so a summary that opens
// with "exit status 1" followed by megabytes of log tells the agent the suite
// is red without ever saying which tests broke. Counts and names first, raw
// output (capped) after.
func testFailureSummary(target, runErr, output string) string {
	var sb strings.Builder
	counts := testoutput.Parse(output)
	if counts.Parsed && counts.Failed > 0 {
		// Named failures are authoritative: the generic line matcher also
		// counts package summary lines ("FAIL\tpkg"), so the raw count can
		// exceed the number of tests that actually broke.
		failed := counts.Failed
		names := counts.FailedNames
		if len(names) > 0 {
			failed = len(names)
		}
		fmt.Fprintf(&sb, "%d failed / %d passed in %s", failed, counts.Passed, target)
		const maxNames = 8
		shown := names
		if len(shown) > maxNames {
			shown = shown[:maxNames]
		}
		if len(shown) > 0 {
			fmt.Fprintf(&sb, ": %s", strings.Join(shown, ", "))
		}
		if len(names) > len(shown) {
			fmt.Fprintf(&sb, " (+%d more)", len(names)-len(shown))
		}
		if first := firstFailureDetail(output); first != "" {
			fmt.Fprintf(&sb, ". First: %s", first)
		}
	} else {
		fmt.Fprintf(&sb, "%s (output did not parse as test results)", runErr)
	}
	const maxRaw = 4000
	raw := strings.TrimSpace(output)
	if len(raw) > maxRaw {
		raw = raw[:maxRaw] + "\n... (raw output truncated)"
	}
	if raw != "" {
		sb.WriteString("\n" + raw)
	}
	return sb.String()
}

// firstFailureDetail returns the first assertion line of the first failing
// test. Verbose go output prints a test's log lines BEFORE its `--- FAIL:`
// marker, so it searches backward from the marker for the nearest
// `_test.go:NN: message` line.
func firstFailureDetail(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "--- FAIL:") {
			continue
		}
		for j := i - 1; j >= 0 && j >= i-10; j-- {
			trimmed := strings.TrimSpace(lines[j])
			if strings.Contains(trimmed, "_test.go:") {
				if len(trimmed) > 200 {
					trimmed = trimmed[:200] + "..."
				}
				return trimmed
			}
			if strings.HasPrefix(trimmed, "=== RUN") || strings.HasPrefix(trimmed, "--- ") {
				break
			}
		}
		return strings.TrimSpace(line)
	}
	return ""
}

// executeVerifyTask judges a /verify task the way the kernel decides
// (verify_task_route): a build when the task's phase wrote code, a review of
// the phase's artifacts otherwise. The build is evidence only about code; a
// phase that wrote Markdown is not verified by `go build ./...`.
func (o *Orchestrator) executeVerifyTask(ctx context.Context, task *Task) (any, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	// Holographic context + honesty: log the upstream injection even on the
	// build path (which otherwise has no shard input), then fail a hollow
	// report target while upstream holds real findings.
	_ = o.upstreamArtifactContext(task)
	if herr := o.checkVerifyHollowReport(task); herr != nil {
		logging.Get(logging.CategoryCampaign).Error("Verify task %s rejected hollow report: %v", task.ID, herr)
		return nil, herr
	}
	route, err := o.oneDerivedFor("verify_task_route", task.ID)
	if err != nil {
		return nil, fmt.Errorf("verify task %s: %w", task.ID, err)
	}
	switch route {
	case "/build":
	case "/review":
		logging.Campaign("Verify task %s: its phase wrote no code; the kernel routes it to a review of the phase's artifacts", task.ID)
		return o.executeResearchTask(ctx, task)
	default:
		return nil, fmt.Errorf("verify task %s: the kernel derived verify_task_route %s, which this orchestrator cannot run", task.ID, route)
	}

	logging.CampaignDebug("Executing verify task %s: go build ./...", task.ID)
	// Run build verification for this task
	cmd := tactile.Command{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: o.policy.VerifyBuildTimeout.Milliseconds(),
		},
	}
	res, err := o.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	// The executor reports a command that ran and failed as a result with a
	// non-zero exit, not as an error (runBuildCheckpoint reads it the same
	// way); only exit 0 verifies.
	if err == nil && res != nil && res.ExitCode != 0 {
		err = withSignals(fmt.Errorf("go build ./... exited %d:\n%s", res.ExitCode, output), "/build_failed")
	}
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Verify task %s failed: %v", task.ID, err)
		return map[string]any{
			"task_id":  task.ID,
			"output":   output,
			"verified": false,
		}, err
	}
	logging.Campaign("Verify task %s passed", task.ID)
	return map[string]any{
		"task_id":  task.ID,
		"output":   output,
		"verified": true,
	}, nil
}

// executeShardSpawnTask spawns a specialized shard.
// executeShardSpawnTask spawns a specialized shard.
func (o *Orchestrator) executeShardSpawnTask(ctx context.Context, task *Task) (any, error) {
	// Extract shard type from description
	intent := "/fix" // Default
	logging.CampaignDebug("Executing shard spawn task %s: intent=%s", task.ID, intent)
	// Holographic context: shard-spawn inputs carry upstream durable findings.
	result, err := o.spawnTask(ctx, task, intent, o.buildTaskInput(task))
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Shard spawn task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Shard spawn task completed: %s", task.ID)
	return map[string]any{"shard_result": result}, nil
}

// executeRefactorTask refactors existing code using the Coder shard.
// executeRefactorTask refactors existing code using the Coder shard.
func (o *Orchestrator) executeRefactorTask(ctx context.Context, task *Task) (any, error) {
	// Get target files from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing refactor task %s: path=%s", task.ID, targetPath)

	// Build task string for coder shard. Holographic context: the instruction
	// carries upstream durable findings via buildTaskInput.
	shardTask := fmt.Sprintf("refactor file:%s instruction:%s", targetPath, o.buildTaskInput(task))
	logging.CampaignDebug("Spawning coder shard for refactoring")

	// Delegate to coder shard
	result, err := o.spawnTask(ctx, task, "/fix", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Refactor shard failed for task %s, falling back to file task: %v", task.ID, err)
		// Fallback to generic file task
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Refactor task completed: %s", task.ID)
	return map[string]any{"coder_result": result, "path": targetPath}, nil
}

// executeIntegrateTask integrates components.
func (o *Orchestrator) executeIntegrateTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing integrate task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeDocumentTask generates documentation.
func (o *Orchestrator) executeDocumentTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Executing document task %s via file task", task.ID)
	return o.executeFileTask(ctx, task)
}

// executeToolCreateTask triggers tool generation via kernel-mediated autopoiesis.
// It asserts missing_tool_for fact to the kernel, which derives delegate_task(/tool_generator, ...).
// The autopoiesis orchestrator listens for these derived facts and generates the tool.
func (o *Orchestrator) executeToolCreateTask(ctx context.Context, task *Task) (any, error) {
	logging.Campaign("Executing tool create task %s (Ouroboros)", task.ID)
	// Extract tool capability from task description or artifacts
	// For tool creation, the Path field contains the tool/capability name
	capability := task.Description
	if len(task.Artifacts) > 0 && task.Artifacts[0].Path != "" {
		capability = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Tool capability requested: %s", capability)

	// Generate intent ID for this tool creation request
	intentID := fmt.Sprintf("campaign_%s_task_%s", o.campaign.ID, task.ID)
	logging.CampaignDebug("Tool creation intent ID: %s", intentID)

	// Assert missing_tool_for to kernel - this triggers the policy rules:
	// 1. delegate_task(/tool_generator, Cap, /pending) derives
	// 2. next_action(/generate_tool) derives
	// 3. Autopoiesis orchestrator picks up the delegation
	err := o.kernel.Assert(core.Fact{
		Predicate: "missing_tool_for",
		Args:      []any{intentID, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert missing_tool_for: %w", err)
	}

	// Also assert goal_requires so the policy can derive properly
	err = o.kernel.Assert(core.Fact{
		Predicate: "goal_requires",
		Args:      []any{o.campaign.Goal, capability},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assert goal_requires: %w", err)
	}

	// Emit event for visibility
	o.emitEvent(EventToolGenerationRequested, "", task.ID, capability, map[string]any{
		"intent_id":  intentID,
		"capability": capability,
	})

	// capability originates from LLM-authored task text, so it must be escaped
	// before being interpolated into a Mangle query. Raw interpolation of a
	// value containing a quote or newline produced a malformed query whose
	// error was then swallowed by the `err == nil` guard below, leaving this
	// loop to spin for the full 30 minutes and report "pending" as a success.
	quotedCapability := strconv.Quote(capability)

	// Poll for tool_ready or tool_registered fact (with timeout)
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// A query that errors every tick can never succeed; bail out rather than
	// burning the whole timeout on it.
	const maxConsecutiveQueryErrors = 3
	queryErrors := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			// Tool generation timed out - return partial success
			// The tool may still be generating in the background
			return map[string]any{
				"status":     "pending",
				"capability": capability,
				"message":    "tool generation initiated but not yet complete",
			}, nil
		case <-ticker.C:
			// Check if tool is now registered
			facts, err := o.kernel.Query(fmt.Sprintf(`tool_registered(%s)`, quotedCapability))
			if err == nil && len(facts) > 0 {
				return map[string]any{
					"status":     "complete",
					"capability": capability,
					"tool_name":  capability,
				}, nil
			}

			// Also check has_capability
			capFacts, capErr := o.kernel.Query(fmt.Sprintf(`has_capability(%s)`, quotedCapability))
			if capErr == nil && len(capFacts) > 0 {
				return map[string]any{
					"status":     "complete",
					"capability": capability,
				}, nil
			}

			if err != nil || capErr != nil {
				queryErrors++
				logging.CampaignWarn("tool_create poll query failed for capability %q (%d/%d): tool_registered=%v has_capability=%v",
					capability, queryErrors, maxConsecutiveQueryErrors, err, capErr)
				if queryErrors >= maxConsecutiveQueryErrors {
					return nil, fmt.Errorf("tool registration poll for %q failed %d consecutive times: %w",
						capability, queryErrors, cmp.Or(err, capErr))
				}
			} else {
				queryErrors = 0
			}
		}
	}
}

// executeCampaignRefTask handles a sub-campaign reference.
// Currently it validates the sub-campaign ID and logs the intent.
// In a full fractal implementation, this would spawn a child Orchestrator.
func (o *Orchestrator) executeCampaignRefTask(ctx context.Context, task *Task) (any, error) {
	_ = ctx
	logging.CampaignDebug("Executing campaign ref task %s", task.ID)
	if task.SubCampaignID == "" {
		logging.Get(logging.CategoryCampaign).Error("Task %s has type /campaign_ref but no sub_campaign_id", task.ID)
		return nil, fmt.Errorf("task %s has type /campaign_ref but no sub_campaign_id", task.ID)
	}

	failurePolicy := normalizeCampaignRefFailurePolicy(task.CampaignRefFailurePolicy)
	inheritance := normalizeCampaignRefInheritance(task.CampaignRefInheritance)
	subStatus, found := o.lookupCampaignStatus(task.SubCampaignID)
	lifecycle := CampaignRefLifecycleLinked
	if found {
		lifecycle = campaignRefLifecycleFromStatus(subStatus)
	}

	envelope := CampaignRefResult{
		Version:       1,
		SubCampaignID: task.SubCampaignID,
		Status:        lifecycle,
		Artifacts:     []string{},
		LearnedFacts:  []string{},
		Checkpoints:   0,
		FailurePolicy: failurePolicy,
		Inheritance:   inheritance,
	}
	eventData := map[string]any{
		"sub_campaign_id": task.SubCampaignID,
		"lifecycle":       envelope.Status,
		"failure_policy":  string(envelope.FailurePolicy),
	}
	if found {
		eventData["sub_campaign_status"] = string(subStatus)
	} else {
		eventData["sub_campaign_status"] = "/unknown"
	}

	if lifecycle == CampaignRefLifecycleFailed {
		envelope.FailureSummary = fmt.Sprintf("sub-campaign %s is in failed state", task.SubCampaignID)
		envelope.Status, envelope.LearnedFacts = applyCampaignRefFailurePolicy(failurePolicy, envelope.LearnedFacts)
		eventData["mapped_lifecycle"] = envelope.Status

		o.emitEvent(EventSubCampaignReferenced, "", task.ID, fmt.Sprintf("Linking sub-campaign %s", task.SubCampaignID), eventData)
		if failurePolicy == CampaignRefPolicyPropagate {
			return nil, fmt.Errorf("%s", envelope.FailureSummary)
		}

		logging.Campaign("Linked sub-campaign %s with policy %s -> %s", task.SubCampaignID, failurePolicy, envelope.Status)
		return envelope, nil
	}

	o.emitEvent(EventSubCampaignReferenced, "", task.ID, fmt.Sprintf("Linking sub-campaign %s", task.SubCampaignID), eventData)
	logging.Campaign("Linked sub-campaign %s with lifecycle %s", task.SubCampaignID, envelope.Status)
	return envelope, nil
}

func (o *Orchestrator) lookupCampaignStatus(campaignID string) (CampaignStatus, bool) {
	if o.kernel == nil || campaignID == "" {
		return "", false
	}

	facts, err := o.kernel.Query("campaign")
	if err != nil {
		logging.CampaignWarn("failed to query campaign status for %s: %v", campaignID, err)
		return "", false
	}

	// Walk reverse to favor newest asserted campaign status.
	for i := len(facts) - 1; i >= 0; i-- {
		fact := facts[i]
		if len(fact.Args) < 5 {
			continue
		}
		if internaltypes.ExtractString(fact.Args[0]) != campaignID {
			continue
		}
		return CampaignStatus(internaltypes.ExtractString(fact.Args[4])), true
	}
	return "", false
}

func normalizeCampaignRefFailurePolicy(policy CampaignRefFailurePolicy) CampaignRefFailurePolicy {
	switch policy {
	case CampaignRefPolicyAbsorb, CampaignRefPolicyTransform, CampaignRefPolicyPropagate:
		return policy
	default:
		return CampaignRefPolicyPropagate
	}
}

func normalizeCampaignRefInheritance(inheritance *CampaignRefInheritance) CampaignRefInheritance {
	normalized := CampaignRefInheritance{
		FactsScope:  "campaign_namespace_readonly",
		FSScope:     "child_snapshot_rw",
		MemoryScope: "scoped_vector_campaign_namespace",
		ToolScope:   "parent_tool_allowlist",
	}
	if inheritance == nil {
		return normalized
	}

	if strings.TrimSpace(inheritance.FactsScope) != "" {
		normalized.FactsScope = strings.TrimSpace(inheritance.FactsScope)
	}
	if strings.TrimSpace(inheritance.FSScope) != "" {
		normalized.FSScope = strings.TrimSpace(inheritance.FSScope)
	}
	if strings.TrimSpace(inheritance.MemoryScope) != "" {
		normalized.MemoryScope = strings.TrimSpace(inheritance.MemoryScope)
	}
	if strings.TrimSpace(inheritance.ToolScope) != "" {
		normalized.ToolScope = strings.TrimSpace(inheritance.ToolScope)
	}

	return normalized
}

func applyCampaignRefFailurePolicy(policy CampaignRefFailurePolicy, learnedFacts []string) (string, []string) {
	switch policy {
	case CampaignRefPolicyAbsorb:
		return CampaignRefLifecycleCompleted, append(learnedFacts, "/campaign_ref_failure_absorbed")
	case CampaignRefPolicyTransform:
		return CampaignRefLifecycleCompleted, append(learnedFacts, "/campaign_ref_failure_transformed")
	default:
		return CampaignRefLifecycleFailed, learnedFacts
	}
}

// executeGenericTask runs a generic task via shard delegation.
func (o *Orchestrator) executeGenericTask(ctx context.Context, task *Task) (any, error) {
	if task == nil || (task.Description == "" && task.ShardInput == "") {
		return nil, fmt.Errorf("task description cannot be empty")
	}
	logging.CampaignDebug("Executing generic task %s via coder shard", task.ID)
	// Holographic context: generic inputs carry upstream durable findings.
	result, err := o.spawnTask(ctx, task, "/fix", o.buildTaskInput(task))
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Generic task %s failed: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Generic task completed: %s", task.ID)
	return map[string]any{"result": result}, nil
}

// extractCodeBlock extracts code from LLM response that may contain markdown fences.
// Returns the code inside ```lang or ``` blocks, or the original text if no fences found.
func extractCodeBlock(text, lang string) string {
	// Look for ```lang or ``` blocks
	patterns := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
		"```\r\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(text, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(text[start:], "```")
			if end != -1 {
				return strings.TrimSpace(text[start : start+end])
			}
		}
	}

	// If no code block found, return the whole text (might be raw code)
	return strings.TrimSpace(text)
}

// isDegenerateGeneration reports whether text looks like a pathological model
// repetition loop rather than a real deliverable — e.g. Grok emitting
// "1. End. 2. Finish. 3. Complete. 4. Done." hundreds of times (observed live in
// campaign_e6f9b0eb, a 19KB artifact of near-zero information). Such output
// otherwise passes the non-empty write check in the fallback path and is silently
// counted as task success, defeating the hollow-success guard. The heuristic is
// deliberately conservative (only fires on extreme, unambiguous degeneracy) so it
// never rejects a legitimately terse or identifier-dense document.
func isDegenerateGeneration(text string) bool {
	const minTokens = 200 // short outputs are never flagged
	fields := strings.Fields(text)
	if len(fields) < minTokens {
		return false
	}
	// Normalize each token to its letters-only lowercase form so the numeric
	// counters ("1." "2." ...) collapse to empty and the cycling words
	// ("end" "finish" ...) collapse together.
	vocab := make(map[string]int, len(fields))
	words := 0
	for _, f := range fields {
		norm := strings.ToLower(strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r)
		}))
		if norm == "" {
			continue // pure counter/punctuation token
		}
		vocab[norm]++
		words++
	}
	if words == 0 {
		return true // nothing but counters/punctuation
	}
	// Vocabulary ratio: distinct words / total words. Real prose sits well above
	// 0.1; a handful of words cycling thousands of times sits near zero.
	ratio := float64(len(vocab)) / float64(words)
	if ratio < 0.03 {
		return true
	}
	// Absolute floor: a very long output built from a tiny vocabulary is
	// degenerate even if the ratio math is skewed by a long non-repeating prefix.
	if words > 400 && len(vocab) < 25 {
		return true
	}
	return false
}

// degradedGenerationPlaceholder returns a short, honest Markdown note recording
// that the model failed to produce a coherent deliverable for this task. Writing
// this instead of the raw degenerate output keeps the phase progressing (a hard
// task failure would deadlock phase completion — the trap F-TASK-1/F-DOC-1 fixed)
// while refusing to launder model garbage into a silent "success": a downstream
// checkpoint or human reader sees the truth.
func degradedGenerationPlaceholder(task *Task, targetPath string) string {
	return fmt.Sprintf(`# Generation Degraded

The document generation for task %s did not produce a coherent result: the
model returned degenerate, repetitive output that was rejected by the campaign
fallback's quality guard. This placeholder is written so the deliverable path
(%s) exists and the phase can proceed, but the task did NOT genuinely succeed.

## Original task
%s

_Regenerate this artifact with a healthier model/config before relying on it._
`, task.ID, targetPath, strings.TrimSpace(task.Description))
}

// getLangFromPath returns the language identifier for a file path.
// isDeliverableDocument reports whether path names a prose document -- the
// only kind of file the direct-generation fallback may write.
func isDeliverableDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".txt", ".rst":
		return true
	}
	return false
}

func getLangFromPath(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	switch ext {
	case "go":
		return "go"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "kt":
		return "kotlin"
	case "py":
		return "python"
	case "sql":
		return "sql"
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "md":
		return "markdown"
	default:
		return ext
	}
}

var descriptionPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)create\s+(\S+\.\w+)`),  // "Create internal/domain/foo.go"
	regexp.MustCompile(`(?i)file[:\s]+(\S+\.\w+)`), // "file: path/to/file.go"
	regexp.MustCompile(`(?i)(\S+/\S+\.\w+)`),       // Any path with / and extension
	regexp.MustCompile(`(?i)internal/\S+\.\w+`),    // internal/... paths
	regexp.MustCompile(`(?i)cmd/\S+\.\w+`),         // cmd/... paths
	regexp.MustCompile(`(?i)pkg/\S+\.\w+`),         // pkg/... paths
}

// defaultTaskArtifactPath returns a deterministic, workspace-relative artifact
// path for an artifact-producing task that the decomposer left without a target
// path. Documents land under the campaign's own artifact directory (which is
// inside .nerd/, already excluded from world scans), so they never pollute the
// audited source tree and are stable across retries.
func (o *Orchestrator) defaultTaskArtifactPath(task *Task) string {
	id := strings.ReplaceAll(strings.TrimPrefix(task.ID, "/"), "/", "_")
	if id == "" {
		id = "task"
	}
	campID := "campaign"
	if o.campaign != nil {
		campID = campaignSlug(o.campaign.ID)
	}
	return filepath.ToSlash(filepath.Join(".nerd", "campaigns", campID, "artifacts", id+".md"))
}

// insideCampaignArtifacts reports whether an absolute path resolves inside
// this campaign's artifacts directory (`.nerd/campaigns/<slug>/artifacts`).
// The durable-output writer is the orchestrator's one direct file write and
// is allowed only there; every repository write goes through the VirtualStore.
func (o *Orchestrator) insideCampaignArtifacts(fullPath string) bool {
	campID := "campaign"
	if o.campaign != nil {
		campID = campaignSlug(o.campaign.ID)
	}
	root := filepath.Clean(filepath.Join(o.workspace, ".nerd", "campaigns", campID, "artifacts"))
	rel, err := filepath.Rel(root, filepath.Clean(fullPath))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// extractPathFromDescription attempts to extract a file path from a task description.
// Looks for common patterns like "Create internal/domain/foo.go" or "file: path/to/file.go"
func extractPathFromDescription(desc string) string {
	for _, re := range descriptionPathPatterns {
		matches := re.FindStringSubmatch(desc)
		if len(matches) > 1 {
			path := matches[1]
			// Validate it looks like a real path
			if strings.Contains(path, "/") && strings.Contains(path, ".") {
				return path
			}
		} else if len(matches) == 1 {
			path := matches[0]
			if strings.Contains(path, "/") && strings.Contains(path, ".") {
				return path
			}
		}
	}

	return ""
}
