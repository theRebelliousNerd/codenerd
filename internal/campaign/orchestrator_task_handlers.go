package campaign

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	internaltypes "codenerd/internal/types"
)

// spawnTask is the unified entry point for task execution.
func (o *Orchestrator) spawnTask(ctx context.Context, intent string, task string) (string, error) {
	o.mu.RLock()
	te := o.taskExecutor
	o.mu.RUnlock()

	if te == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	logging.CampaignDebug("spawnTask: using TaskExecutor for intent=%s", intent)
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return te.Execute(ctx, req)
}

// executeTask executes a single task.
func (o *Orchestrator) executeTask(ctx context.Context, task *Task) (any, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	logging.CampaignDebug("Executing task %s with type %s, shard=%s", task.ID, task.Type, task.Shard)

	// Update task status
	o.updateTaskStatus(task, TaskInProgress)

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
	result, err := o.spawnTask(ctx, shardType, input)
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
			return o.executeFileTaskFallback(ctx, task, targetPath)
		}
		logging.Get(logging.CategoryCampaign).Error("Shard %s failed for task %s: %v", shardType, task.ID, err)
		return nil, fmt.Errorf("shard %s failed: %w", shardType, err)
	}

	logging.CampaignDebug("Shard %s completed for task %s, result_len=%d", shardType, task.ID, len(result))

	// F-HOLLOW-2: an explicit analysis shard can return an empty/near-empty result
	// on a package-audit task (observed live, run 14 phase 1: shard=reviewer
	// returned "result":"" for invariant/error-contract audits; shard=testarchitect
	// returned a 367-byte stub). The checkpoint reviewer then correctly fails the
	// phase for missing findings. Retry once via the research path, which reliably
	// produces written findings for audit tasks (run 14 phase 0 researcher tasks
	// each delivered 7-11KB). Skip file/test/tool tasks, which legitimately return
	// only a short confirmation after writing their own durable output.
	if needsAnalysisRetry(result) && !isFileProducingType(task.Type) {
		logging.Get(logging.CategoryCampaign).Warn("Explicit-shard task %s (shard=%s) returned a non-deliverable result (%d bytes, empty-or-intent-stub); retrying via research path", task.ID, shardType, len(strings.TrimSpace(result)))
		retryInput := task.Description + "\n\nIMPORTANT: Do NOT describe what you WILL do and do NOT return a plan. Perform the audit NOW and report concrete findings in your final response, with file+symbol anchors where possible (or an explicit \"no issues found\" for a surface you checked). Do NOT return an empty response."
		if retried, rerr := o.spawnTask(ctx, "/research", retryInput); rerr == nil && !needsAnalysisRetry(retried) {
			result = retried
			logging.Campaign("Research-path retry recovered a substantive result for %s (%d bytes)", task.ID, len(result))
		} else {
			logging.Get(logging.CategoryCampaign).Warn("Explicit-shard task %s still returned no substantive output after research-path retry", task.ID)
			o.emitEvent(EventShardResultEmpty, task.PhaseID, task.ID, fmt.Sprintf("shard %s returned no substantive output after retry", shardType), nil)
		}
	}

	// F-DURABLE-1: an explicit-shard task that produces analysis (research,
	// audit, review, discovery) rather than a file returns its result only in
	// memory. Persist it as a durable artifact so the findings survive and the
	// phase-checkpoint reviewer can verify a real output. No-op for file/test/
	// tool tasks and when a durable output already exists (see helper).
	o.persistTaskOutputArtifact(task, result)

	return map[string]any{
		"shard":  shardType,
		"result": result,
		"task":   task.ID,
	}, nil
}

// isTrivialResult reports whether a shard result carries no substantive content
// and is therefore not worth persisting or counting as a real deliverable. The
// 40-rune floor rejects empty responses and one-line acknowledgements while
// admitting even a terse but real finding.
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

// intentStubPrefixes open a plan-only ("I will do X") response rather than actual
// findings. Matched case-insensitively against the trimmed result prefix.
var intentStubPrefixes = []string{
	"i'll ", "i will ", "i am going to ", "i'm going to ", "let me ",
	"i plan to ", "i intend to ", "i would ", "i'm planning", "i am planning",
	"first, i", "here is my plan", "here's my plan", "my plan", "plan:",
}

// looksLikeIntentStub reports whether a result is a short plan-only preamble
// ("I'll audit internal/world for ... Starting with ...") that clears the
// emptiness floor but contains no findings (observed live, run 15 phases 2/3/4:
// the checkpoint reviewer flagged artifacts as "only an intent stub"). Only short
// results qualify: a long result that opens with a planning phrase has still done
// the work, so the rune cap avoids false positives on real analysis.
func looksLikeIntentStub(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if len([]rune(t)) > 600 {
		return false
	}
	for _, p := range intentStubPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// needsAnalysisRetry reports whether an analysis-task result is not a real
// deliverable — either trivially empty or a plan-only intent stub — and should
// be retried with a stronger, execute-now instruction.
func needsAnalysisRetry(result string) bool {
	return isTrivialResult(result) || looksLikeIntentStub(result)
}

// analyticalVerifyKeywords mark a /verify task whose real objective is analytical
// review (produce findings) rather than a build check (run go build).
var analyticalVerifyKeywords = []string{
	"inspect", "audit", "review", "identify", "analyze", "analyse", "assess",
	"examine", "evaluate", "defect", "vulnerab", "contract drift", "api contract",
	"error-handling", "error handling", "code smell", "anti-pattern", "antipattern",
	"race condition", "lifecycle", "ownership", "invariant", "risk", "finding",
}

// isAnalyticalVerifyDescription reports whether a /verify task's description asks
// for analytical review (which must produce durable findings) rather than a plain
// build/compile check (which the go-build path handles). Build-verification tasks
// stay on the build path for backward compatibility.
func isAnalyticalVerifyDescription(desc string) bool {
	d := strings.ToLower(desc)
	for _, k := range analyticalVerifyKeywords {
		if strings.Contains(d, k) {
			return true
		}
	}
	return false
}

// executeResearchTask spawns a researcher shard.
func (o *Orchestrator) executeResearchTask(ctx context.Context, task *Task) (any, error) {
	logging.CampaignDebug("Spawning researcher shard for task %s", task.ID)
	result, err := o.spawnTask(ctx, "/research", o.buildTaskInput(task))
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Researcher shard failed for task %s: %v", task.ID, err)
		return nil, err
	}
	logging.CampaignDebug("Researcher shard completed for task %s", task.ID)

	// F-HOLLOW-1: a research subagent can return an empty final response (observed
	// live, run 13 task 0_2: "Fallback parse: empty response ... EOF") yet be
	// marked "completed successfully", producing a hollow success that delivers
	// nothing durable — the phase-checkpoint reviewer then correctly fails the
	// phase for the missing artifact. Retry once, explicitly demanding the written
	// findings, before accepting an empty deliverable.
	if needsAnalysisRetry(result) {
		logging.Get(logging.CategoryCampaign).Warn("Research task %s returned a non-deliverable result (%d bytes, empty-or-intent-stub); retrying once", task.ID, len(strings.TrimSpace(result)))
		retryInput := task.Description + "\n\nIMPORTANT: Do NOT describe what you WILL do and do NOT return a plan. Perform the work NOW and report concrete findings as a complete written report in your final response (or an explicit \"no issues found\" for a surface you checked). Do NOT return an empty response."
		if retried, rerr := o.spawnTask(ctx, "/research", retryInput); rerr == nil && !needsAnalysisRetry(retried) {
			result = retried
			logging.Campaign("Research retry recovered a substantive result for %s (%d bytes)", task.ID, len(result))
		} else {
			logging.Get(logging.CategoryCampaign).Warn("Research task %s still returned no substantive output after retry", task.ID)
			o.emitEvent(EventResearchEmpty, task.PhaseID, task.ID, "research task returned no substantive output after retry", nil)
		}
	}

	// F-DURABLE-1: research/audit tasks previously returned their findings only
	// in memory, leaving nothing on disk. The phase-checkpoint reviewer then
	// correctly reported "no durable discovery outputs" and failed the phase even
	// though the work was done (observed live, run 12 phases 0/1). Persist the
	// findings so they survive the campaign and the reviewer has a real output.
	o.persistTaskOutputArtifact(task, result)
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
	if isTrivialResult(trimmed) {
		return // nothing substantial worth persisting
	}
	if isFileProducingType(task.Type) {
		return // these produce their own durable file/test/tool output
	}
	for _, a := range task.Artifacts {
		switch a.Type {
		case "/doc", "/test_file", "/config", "/file":
			if a.Path != "" {
				if _, err := os.Stat(filepath.Join(o.workspace, a.Path)); err == nil {
					return // a durable output already exists; don't duplicate it
				}
			}
		}
	}
	relPath := o.defaultTaskArtifactPath(task)
	fullPath := filepath.Join(o.workspace, relPath)
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

	// Build task string for coder shard
	// NOTE: Don't use "instruction:<value>" format because strings.Fields() splits on spaces,
	// causing multi-word instructions to be truncated. Use simpler format where bare words
	// are joined into the instruction by parseTask.
	action := "create"
	if task.Type == TaskTypeFileModify {
		action = "modify"
	}
	shardTask := fmt.Sprintf("%s file:%s %s", action, targetPath, o.buildTaskInput(task))
	logging.CampaignDebug("Spawning coder shard: action=%s, path=%s, task=%s", action, targetPath, shardTask)

	// Delegate to coder shard
	result, err := o.spawnTask(ctx, "/fix", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Coder shard failed for task %s, using fallback: %v", task.ID, err)
		// Fallback to direct LLM if shard fails
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	logging.CampaignDebug("Coder shard completed for task %s, result_len=%d", task.ID, len(result))

	// CRITICAL: Verify file was actually written
	// Shards may return successfully without calling write_file tool.
	// An empty target or a directory must never count as success: an empty
	// target joins to the workspace root itself, which always stats.
	verified := false
	var fullPath string
	if targetPath != "" {
		if filepath.IsAbs(targetPath) {
			fullPath = filepath.Clean(targetPath)
		} else {
			fullPath = filepath.Join(o.workspace, targetPath)
		}
		if info, statErr := os.Stat(fullPath); statErr == nil && !info.IsDir() && info.Mode().IsRegular() {
			verified = true
		}
	}
	if !verified {
		if fullPath == "" {
			fullPath = filepath.Join(o.workspace, targetPath)
		}
		logging.Get(logging.CategoryCampaign).Warn("Coder shard returned but file not created or not a regular file: %s, using fallback", fullPath)
		// Shard didn't write file - fall back to direct LLM
		return o.executeFileTaskFallback(ctx, task, targetPath)
	}

	logging.Campaign("File verified after shard execution: %s", fullPath)
	return map[string]any{"coder_result": result, "path": targetPath}, nil
}

// executeFileTaskFallback uses direct LLM when shard is unavailable.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string) (any, error) {
	logging.CampaignDebug("Executing file task fallback for %s via direct LLM", task.ID)

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
			return nil, fmt.Errorf("no target path specified for file task %s", task.ID)
		}
	}

	// Path traversal guard
	cleanPath := filepath.Clean(targetPath)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return nil, fmt.Errorf("path traversal attempt blocked for path: %s", targetPath)
	}
	targetPath = cleanPath

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
		return nil, err
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
	logging.CampaignDebug("Writing generated file: %s (%d bytes)", fullPath, len(content))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to create directory for %s: %v", fullPath, err)
		return nil, err
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Failed to write file %s: %v", fullPath, err)
		return nil, err
	}

	logging.CampaignDebug("File fallback completed: %s", fullPath)
	return map[string]any{"path": fullPath, "size": len(content)}, nil
}

// executeTestWriteTask writes tests for existing code using the Tester shard.
func (o *Orchestrator) executeTestWriteTask(ctx context.Context, task *Task) (any, error) {
	// Get target file from artifacts
	var targetPath string
	if len(task.Artifacts) > 0 {
		targetPath = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test write task %s: target=%s", task.ID, targetPath)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("generate_tests file:%s %s", targetPath, o.buildTaskInput(task))
	logging.CampaignDebug("Spawning tester shard for test generation")

	// Delegate to tester shard
	result, err := o.spawnTask(ctx, "/test", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test write task %s, falling back to coder: %v", task.ID, err)
		// Fallback to coder shard for test generation
		return o.executeFileTask(ctx, task)
	}

	logging.CampaignDebug("Test write task completed: %s", task.ID)
	return map[string]any{"tester_result": result, "target": targetPath}, nil
}

// executeTestRunTask runs tests using the Tester shard.
func (o *Orchestrator) executeTestRunTask(ctx context.Context, task *Task) (any, error) {
	// Get target from artifacts or use default
	target := "./..."
	if len(task.Artifacts) > 0 {
		target = task.Artifacts[0].Path
	}
	logging.CampaignDebug("Executing test run task %s: target=%s", task.ID, target)

	// Build task string for tester shard
	shardTask := fmt.Sprintf("run_tests package:%s %s", target, o.buildTaskInput(task))
	logging.CampaignDebug("Spawning tester shard for test execution")

	// Delegate to tester shard
	result, err := o.spawnTask(ctx, "/test", shardTask)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Tester shard failed for test run task %s, using direct execution: %v", task.ID, err)
		// Fallback to direct execution
		cmd := tactile.Command{
			Binary:           "go",
			Arguments:        []string{"test", target},
			WorkingDirectory: o.workspace,
			Limits: &tactile.ResourceLimits{
				TimeoutMs: 300 * 1000,
			},
		}
		logging.CampaignDebug("Executing tests directly via tactile: go test %s", target)
		res, execErr := o.executor.Execute(ctx, cmd)
		output := ""
		if res != nil {
			output = res.Output()
			// Truncate massive output to avoid OOM
			if len(output) > 1024*1024 { // 1MB limit
				output = output[:1024*1024] + "\n... (output truncated)"
			}
		}
		if execErr != nil {
			logging.Get(logging.CategoryCampaign).Error("Test execution failed: %v", execErr)
			return map[string]any{"output": output, "passed": false}, execErr
		}
		logging.Campaign("Tests passed via direct execution")
		return map[string]any{"output": output, "passed": true}, nil
	}

	logging.CampaignDebug("Test run task completed: %s", task.ID)
	return map[string]any{"tester_result": result, "target": target}, nil
}

// executeVerifyTask runs verification (build, lint, etc.).
// executeVerifyTask runs verification (build, lint, etc.).
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
	// F-VERIFY-1: /verify historically meant "go build ./...". But decomposers of
	// audit/remediation campaigns emit the phase's actual analytical work (inspect
	// logic defects, error-handling mistakes, API-contract drift) as /verify tasks.
	// Running only a build ignores the task description, produces no findings, and
	// leaves the phase's real deliverable missing — the checkpoint reviewer then
	// correctly fails the phase (observed live, run 13 phase 1: /verify tasks 1_1,
	// 1_2, 1_5, 1_6 delivered nothing). Route analytical verify tasks to the
	// research+persist path (which retries on empty and writes a durable artifact);
	// keep the build path for genuine build-verification tasks.
	if isAnalyticalVerifyDescription(task.Description) {
		logging.Campaign("Verify task %s is analytical (audit/review); routing to research+persist instead of go build", task.ID)
		return o.executeResearchTask(ctx, task)
	}

	logging.CampaignDebug("Executing verify task %s: go build ./...", task.ID)
	// Run build verification for this task
	cmd := tactile.Command{
		Binary:           "go",
		Arguments:        []string{"build", "./..."},
		WorkingDirectory: o.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 300 * 1000, // 5 minutes
		},
	}
	res, err := o.executor.Execute(ctx, cmd)
	output := ""
	if res != nil {
		output = res.Output()
	}
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Verify task %s failed: %v", task.ID, err)
		return map[string]any{
			"task_id": task.ID,
			"output":   output,
			"verified": false,
		}, err
	}
	logging.Campaign("Verify task %s passed", task.ID)
	return map[string]any{
		"task_id": task.ID,
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
	result, err := o.spawnTask(ctx, intent, o.buildTaskInput(task))
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
	result, err := o.spawnTask(ctx, "/fix", shardTask)
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
	result, err := o.spawnTask(ctx, "/fix", o.buildTaskInput(task))
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
