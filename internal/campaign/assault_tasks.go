package campaign

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

type assaultTargetsFile struct {
	CampaignID string       `json:"campaign_id"`
	CreatedAt  time.Time    `json:"created_at"`
	Scope      AssaultScope `json:"scope"`
	Targets    []string     `json:"targets"`
	Include    []string     `json:"include,omitempty"`
	Exclude    []string     `json:"exclude,omitempty"`
}

type assaultBatchFile struct {
	CampaignID string    `json:"campaign_id"`
	BatchID    string    `json:"batch_id"`
	CreatedAt  time.Time `json:"created_at"`
	Targets    []string  `json:"targets"`
}

type assaultResult struct {
	CampaignID string           `json:"campaign_id"`
	BatchID    string           `json:"batch_id"`
	Target     string           `json:"target"`
	Cycle      int              `json:"cycle"`
	Stage      AssaultStageKind `json:"stage"`
	Attempt    int              `json:"attempt"`
	StartedAt  time.Time        `json:"started_at"`
	DurationMs int64            `json:"duration_ms"`
	ExitCode   int              `json:"exit_code"`
	Killed     bool             `json:"killed,omitempty"`
	KillReason string           `json:"kill_reason,omitempty"`
	Truncated  bool             `json:"truncated,omitempty"`
	LogPath    string           `json:"log_path"`
	Error      string           `json:"error,omitempty"`
}

type stageOutcome struct {
	ExitCode   int
	Killed     bool
	KillReason string
	Truncated  bool
	Error      string
}

type assaultFailure struct {
	Target    string
	Stage     AssaultStageKind
	Attempt   int
	Cycle     int
	LogPath   string
	BatchID   string
	StartedAt time.Time
	ExitCode  int
	Error     string
}

type assaultTriageOutput struct {
	Summary            string                   `json:"summary"`
	RecommendedTasks   []assaultRemediationTask `json:"recommended_tasks"`
	AdditionalMetadata map[string]interface{}   `json:"metadata,omitempty"`
}

type assaultRemediationTask struct {
	Type        string   `json:"type"` // "/shard_task" | "/tool_create"
	Priority    string   `json:"priority,omitempty"`
	Description string   `json:"description"`
	Shard       string   `json:"shard,omitempty"`
	ShardInput  string   `json:"shard_input,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
}

func (o *Orchestrator) executeAssaultDiscoverTask(ctx context.Context, task *Task) (any, error) {
	if o.campaign == nil {
		return nil, fmt.Errorf("no campaign loaded")
	}
	cfg := o.getAssaultConfig()
	assaultDir, slug := o.assaultDir()

	batchesDir := filepath.Join(assaultDir, "batches")
	resultsDir := filepath.Join(assaultDir, "results")
	logsDir := filepath.Join(assaultDir, "logs")
	triageDir := filepath.Join(assaultDir, "triage")
	for _, dir := range []string{assaultDir, batchesDir, resultsDir, logsDir, triageDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create assault directory %s: %w", dir, err)
		}
	}

	phase1ID, existingBatchTasks, ok := o.phaseIDByOrder(1)
	if !ok {
		return nil, fmt.Errorf("assault execution phase not found (order=1)")
	}
	if existingBatchTasks > 0 {
		// Likely already discovered; keep it idempotent.
		return map[string]interface{}{
			"campaign_id":   o.campaign.ID,
			"campaign_slug": slug,
			"scope":         cfg.Scope,
			"batches":       existingBatchTasks,
			"status":        "already_discovered",
			"targets_path":  normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "targets.json")),
			"results_dir":   normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "results")),
		}, nil
	}

	targets, err := o.discoverAssaultTargets(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets discovered (scope=%s include=%v exclude=%v)", cfg.Scope, cfg.Include, cfg.Exclude)
	}
	sort.Strings(targets)

	// Persist targets list for durability and offline analysis.
	targetsPath := filepath.Join(assaultDir, "targets.json")
	tf := assaultTargetsFile{
		CampaignID: o.campaign.ID,
		CreatedAt:  time.Now(),
		Scope:      cfg.Scope,
		Targets:    targets,
		Include:    cfg.Include,
		Exclude:    cfg.Exclude,
	}
	data, _ := json.MarshalIndent(tf, "", "  ")
	if err := os.WriteFile(targetsPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write targets file: %w", err)
	}

	batches := chunkStrings(targets, cfg.BatchSize)
	newTasks := make([]Task, 0, len(batches))
	for i, batchTargets := range batches {
		batchID := fmt.Sprintf("batch_%04d", i)
		bf := assaultBatchFile{
			CampaignID: o.campaign.ID,
			BatchID:    batchID,
			CreatedAt:  time.Now(),
			Targets:    batchTargets,
		}
		batchPath := filepath.Join(batchesDir, batchID+".json")
		batchData, _ := json.MarshalIndent(bf, "", "  ")
		if err := os.WriteFile(batchPath, batchData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write batch file %s: %w", batchPath, err)
		}

		relBatchPath, _ := filepath.Rel(o.workspace, batchPath)
		newTasks = append(newTasks, Task{
			ID:          fmt.Sprintf("/task_%s_%d_%d", o.campaign.ID[10:], 1, i),
			PhaseID:     phase1ID,
			Description: fmt.Sprintf("Execute assault %s (%d targets)", batchID, len(batchTargets)),
			Status:      TaskPending,
			Type:        TaskTypeAssaultBatch,
			Priority:    PriorityNormal,
			Order:       i,
			Artifacts: []TaskArtifact{{
				Type: "/assault_batch",
				Path: normalizePath(relBatchPath),
			}},
		})
	}

	if err := o.appendTasksToPhase(phase1ID, newTasks); err != nil {
		return nil, err
	}

	logging.Campaign("Assault discovery complete: campaign=%s scope=%s targets=%d batches=%d",
		o.campaign.ID, cfg.Scope, len(targets), len(batches))

	return map[string]interface{}{
		"campaign_id":   o.campaign.ID,
		"campaign_slug": slug,
		"scope":         cfg.Scope,
		"targets":       len(targets),
		"batches":       len(batches),
		"targets_path":  normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "targets.json")),
		"results_dir":   normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "results")),
	}, nil
}

func (o *Orchestrator) executeAssaultBatchTask(ctx context.Context, task *Task) (any, error) {
	if o.campaign == nil {
		return nil, fmt.Errorf("no campaign loaded")
	}
	cfg := o.getAssaultConfig()
	assaultDir, slug := o.assaultDir()

	batchPath, ok := findArtifactPath(task, "/assault_batch")
	if !ok {
		return nil, fmt.Errorf("assault batch task %s missing /assault_batch artifact", task.ID)
	}
	fullBatchPath := filepath.Join(o.workspace, filepath.FromSlash(batchPath))
	data, err := os.ReadFile(fullBatchPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch file %s: %w", fullBatchPath, err)
	}
	var batch assaultBatchFile
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, fmt.Errorf("failed to parse batch file %s: %w", fullBatchPath, err)
	}

	resultsDir := filepath.Join(assaultDir, "results")
	logsDir := filepath.Join(assaultDir, "logs", batch.BatchID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create results dir: %w", err)
	}
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs dir: %w", err)
	}

	resultsPath := filepath.Join(resultsDir, batch.BatchID+".jsonl")
	completed := readAssaultResultIndex(resultsPath)

	exec := newAssaultExecutor(o.workspace, cfg.LogMaxBytes, time.Duration(cfg.DefaultTimeoutSeconds)*time.Second)

	wrote := 0
	skipped := 0
	passed := 0
	failed := 0

	for cycle := 1; cycle <= cfg.Cycles; cycle++ {
		for _, target := range batch.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			for _, stage := range cfg.Stages {
				for attempt := 1; attempt <= stage.Repeat; attempt++ {
					key := assaultResultKey(cycle, stage.Kind, attempt, target)
					if completed[key] {
						skipped++
						continue
					}

					start := time.Now()
					record := assaultResult{
						CampaignID: o.campaign.ID,
						BatchID:    batch.BatchID,
						Target:     target,
						Cycle:      cycle,
						Stage:      stage.Kind,
						Attempt:    attempt,
						StartedAt:  start,
						ExitCode:   -1,
					}

					logPath := filepath.Join(logsDir, fmt.Sprintf("%s_%s.log", strings.TrimPrefix(string(stage.Kind), "/"), shortHash(key)))
					relLogPath, _ := filepath.Rel(o.workspace, logPath)
					record.LogPath = normalizePath(relLogPath)

					stageOK, stageOut := o.runAssaultStage(ctx, exec, cfg, stage, target, logPath)
					record.DurationMs = time.Since(start).Milliseconds()
					record.Truncated = stageOut.Truncated
					record.ExitCode = stageOut.ExitCode
					record.Killed = stageOut.Killed
					record.KillReason = stageOut.KillReason
					record.Error = stageOut.Error

					if stageOK {
						passed++
					} else {
						failed++
					}

					if err := appendJSONL(resultsPath, record); err != nil {
						return nil, fmt.Errorf("failed to append results: %w", err)
					}
					wrote++
					completed[key] = true
				}
			}
		}
	}

	return map[string]interface{}{
		"campaign_id":   o.campaign.ID,
		"campaign_slug": slug,
		"batch_id":      batch.BatchID,
		"targets":       len(batch.Targets),
		"cycles":        cfg.Cycles,
		"stages":        len(cfg.Stages),
		"wrote_results": wrote,
		"skipped":       skipped,
		"passed":        passed,
		"failed":        failed,
		"results_path":  normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "results", batch.BatchID+".jsonl")),
	}, nil
}

func (o *Orchestrator) runAssaultStage(
	ctx context.Context,
	exec tactile.Executor,
	cfg AssaultConfig,
	stage AssaultStage,
	target string,
	logPath string,
) (ok bool, out stageOutcome) {
	switch stage.Kind {
	case AssaultStageNemesisReview:
		if !cfg.EnableNemesis {
			return true, stageOutcome{ExitCode: 0}
		}
		if o.shardMgr == nil {
			writeTextFileBestEffort(logPath, "nemesis skipped: shard manager unavailable\n")
			return true, stageOutcome{ExitCode: 0}
		}

		dir := targetToDir(target)
		taskStr := fmt.Sprintf("review:%s", filepath.Join(o.workspace, filepath.FromSlash(dir)))
		result, err := o.shardMgr.Spawn(ctx, "nemesis", taskStr)
		content := "nemesis review\n\n" + taskStr + "\n\n" + result
		if err != nil {
			content += "\n\nERROR: " + err.Error()
			writeTextFileBestEffort(logPath, content)
			return false, stageOutcome{ExitCode: 1, Error: err.Error()}
		}
		writeTextFileBestEffort(logPath, content)
		lower := strings.ToLower(result)
		if strings.Contains(lower, "defeated") || strings.Contains(lower, "attack succeeded") ||
			(strings.Contains(lower, "verdict") && strings.Contains(lower, "fail")) {
			return false, stageOutcome{ExitCode: 1, Error: "nemesis found weaknesses"}
		}
		return true, stageOutcome{ExitCode: 0}

	case AssaultStageGoVet:
		return o.runCommandStage(ctx, exec, stage, "go", []string{"vet", target}, logPath)

	case AssaultStageGoTest:
		return o.runCommandStage(ctx, exec, stage, "go", []string{"test", "-count=1", target}, logPath)

	case AssaultStageGoTestRace:
		return o.runCommandStage(ctx, exec, stage, "go", []string{"test", "-race", "-count=1", target}, logPath)

	case AssaultStageCommand:
		cmdLine := strings.ReplaceAll(stage.Command, "{{target}}", target)
		bin, args := shellForCommand(cmdLine)
		return o.runCommandStage(ctx, exec, stage, bin, args, logPath)

	default:
		writeTextFileBestEffort(logPath, fmt.Sprintf("unknown assault stage kind: %s\n", stage.Kind))
		return false, stageOutcome{ExitCode: 2, Error: "unknown stage kind"}
	}
}

func (o *Orchestrator) runCommandStage(
	ctx context.Context,
	exec tactile.Executor,
	stage AssaultStage,
	binary string,
	args []string,
	logPath string,
) (bool, stageOutcome) {
	cmd := tactile.Command{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: o.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(stage.TimeoutSeconds) * 1000,
		},
	}
	res, err := exec.Execute(ctx, cmd)
	if err != nil {
		content := fmt.Sprintf("%s %s\n\nERROR: %s\n", binary, strings.Join(args, " "), err.Error())
		writeTextFileBestEffort(logPath, content)
		return false, stageOutcome{ExitCode: 1, Error: err.Error()}
	}

	output := res.Output()
	header := fmt.Sprintf(
		"%s %s\nexit_code=%d duration=%s killed=%v truncated=%v\n\n",
		binary, strings.Join(args, " "),
		res.ExitCode, res.Duration, res.Killed, res.Truncated,
	)
	writeTextFileBestEffort(logPath, header+output)

	ok := res.Success && res.ExitCode == 0
	out := stageOutcome{
		ExitCode:   res.ExitCode,
		Killed:     res.Killed,
		KillReason: res.KillReason,
		Truncated:  res.Truncated,
		Error:      res.Error,
	}
	if !ok && out.Error == "" && res.ExitCode != 0 {
		out.Error = fmt.Sprintf("exit code %d", res.ExitCode)
	}
	return ok, out
}

func (o *Orchestrator) executeAssaultTriageTask(ctx context.Context, task *Task) (any, error) {
	if o.campaign == nil {
		return nil, fmt.Errorf("no campaign loaded")
	}
	cfg := o.getAssaultConfig()
	assaultDir, slug := o.assaultDir()
	resultsDir := filepath.Join(assaultDir, "results")

	files, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read assault results dir %s: %w", resultsDir, err)
	}

	total := 0
	success := 0
	failures := make([]assaultFailure, 0)

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".jsonl") {
			continue
		}
		path := filepath.Join(resultsDir, f.Name())
		items, err := readAssaultResults(path)
		if err != nil {
			return nil, err
		}
		for _, r := range items {
			total++
			if r.ExitCode == 0 && r.Error == "" {
				success++
				continue
			}
			failures = append(failures, assaultFailure{
				Target:    r.Target,
				Stage:     r.Stage,
				Attempt:   r.Attempt,
				Cycle:     r.Cycle,
				LogPath:   r.LogPath,
				BatchID:   r.BatchID,
				StartedAt: r.StartedAt,
				ExitCode:  r.ExitCode,
				Error:     r.Error,
			})
		}
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].StartedAt.After(failures[j].StartedAt) })

	summary := buildAssaultSummary(total, success, failures, cfg.MaxRemediationTasks)

	triageOut := assaultTriageOutput{
		Summary: summary,
		AdditionalMetadata: map[string]interface{}{
			"campaign_id":  o.campaign.ID,
			"results_dir":  normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "results")),
			"total":        total,
			"success":      success,
			"failures":     len(failures),
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	recommended := o.llmAssaultRemediationPlan(ctx, cfg, summary)
	if len(recommended) == 0 {
		recommended = deterministicRemediationTasks(failures, cfg.MaxRemediationTasks)
	}
	triageOut.RecommendedTasks = recommended

	triagePath := filepath.Join(assaultDir, "triage", fmt.Sprintf("triage_%s.json", time.Now().Format("20060102T150405")))
	if err := os.MkdirAll(filepath.Dir(triagePath), 0755); err == nil {
		if data, err := json.MarshalIndent(triageOut, "", "  "); err == nil {
			_ = os.WriteFile(triagePath, data, 0644)
			_ = os.WriteFile(filepath.Join(assaultDir, "triage", "latest.json"), data, 0644)
		}
	}

	phase3ID, existing, ok := o.phaseIDByOrder(3)
	if !ok {
		return nil, fmt.Errorf("remediation phase not found (order=3)")
	}
	if existing > 0 {
		// Keep triage idempotent: if remediation tasks already exist, don't duplicate them.
		return map[string]interface{}{
			"campaign_id":             o.campaign.ID,
			"campaign_slug":           slug,
			"total_results":           total,
			"success":                 success,
			"failures":                len(failures),
			"triage_path":             normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "triage", "latest.json")),
			"status":                  "already_triaged",
			"remediation_tasks_added": 0,
		}, nil
	}

	limit := cfg.MaxRemediationTasks
	if limit <= 0 {
		limit = 25
	}
	if len(recommended) > limit {
		recommended = recommended[:limit]
	}

	remediationTasks := make([]Task, 0, len(recommended))
	for i, rt := range recommended {
		priority := TaskPriority(rt.Priority)
		if priority == "" {
			priority = PriorityHigh
		}
		newTask := Task{
			ID:          fmt.Sprintf("/task_%s_%d_%d", o.campaign.ID[10:], 3, existing+i),
			PhaseID:     phase3ID,
			Description: rt.Description,
			Status:      TaskPending,
			Priority:    priority,
			Order:       existing + i,
			Type:        TaskTypeRefactor,
		}

		switch strings.TrimSpace(rt.Type) {
		case "/tool_create":
			newTask.Type = TaskTypeToolCreate
			newTask.Shard = ""
			newTask.ShardInput = ""
		default:
			newTask.Shard = rt.Shard
			if newTask.Shard == "" {
				newTask.Shard = "coder"
			}
			if strings.TrimSpace(rt.ShardInput) != "" {
				newTask.ShardInput = rt.ShardInput
			} else {
				newTask.ShardInput = rt.Description
			}
		}

		for _, ap := range rt.Artifacts {
			ap = strings.TrimSpace(ap)
			if ap == "" {
				continue
			}
			newTask.Artifacts = append(newTask.Artifacts, TaskArtifact{Type: "/assault_artifact", Path: normalizePath(ap)})
		}
		remediationTasks = append(remediationTasks, newTask)
	}

	if err := o.appendTasksToPhase(phase3ID, remediationTasks); err != nil {
		return nil, err
	}

	logging.Campaign("Assault triage complete: failures=%d remediation_tasks=%d triage=%s",
		len(failures), len(remediationTasks), normalizePath(triagePath))

	return map[string]interface{}{
		"campaign_id":                  o.campaign.ID,
		"campaign_slug":                slug,
		"total_results":                total,
		"success":                      success,
		"failures":                     len(failures),
		"triage_path":                  normalizePath(filepath.Join(".nerd", "campaigns", slug, "assault", "triage", "latest.json")),
		"remediation_tasks_added":      len(remediationTasks),
		"remediation_phase_task_count": existing + len(remediationTasks),
	}, nil
}

func (o *Orchestrator) llmAssaultRemediationPlan(ctx context.Context, cfg AssaultConfig, summary string) []assaultRemediationTask {
	if o.llmClient == nil {
		return nil
	}

	promptProvider := o.promptProvider
	if promptProvider == nil {
		promptProvider = NewStaticPromptProvider()
	}

	assaultPrompt, err := promptProvider.GetPrompt(ctx, RoleAssault, o.campaign.ID)
	if err != nil || strings.TrimSpace(assaultPrompt) == "" {
		assaultPrompt = AssaultLogic
	}

	userPrompt := fmt.Sprintf(`ASSAULT RESULTS SUMMARY:
%s

Generate a remediation plan as JSON.

Constraints:
- Create at most %d remediation tasks.
- Prefer shard tasks (shard=coder) that reference concrete artifact paths (logs/results).
- If you need a missing capability (e.g., fuzz harness), emit a /tool_create task.

Output ONLY valid JSON:
{
  "summary": "string",
  "recommended_tasks": [
    {
      "type": "/shard_task|/tool_create",
      "priority": "/critical|/high|/normal|/low",
      "description": "string",
      "shard": "coder|tester|reviewer|nemesis|researcher",
      "shard_input": "string",
      "artifacts": ["path1","path2"]
    }
  ]
}
`, summary, cfg.MaxRemediationTasks)

	resp, err := o.llmClient.Complete(ctx, assaultPrompt+"\n\n"+userPrompt)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Assault triage LLM call failed: %v", err)
		return nil
	}
	resp = cleanJSONResponse(resp)

	var out assaultTriageOutput
	if err := json.Unmarshal([]byte(resp), &out); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("Assault triage JSON parse failed: %v", err)
		return nil
	}
	if len(out.RecommendedTasks) == 0 {
		return nil
	}
	return out.RecommendedTasks
}

func deterministicRemediationTasks(failures []assaultFailure, limit int) []assaultRemediationTask {
	if limit <= 0 {
		limit = 25
	}

	out := make([]assaultRemediationTask, 0, min(limit, len(failures)))
	for i := 0; i < len(failures) && len(out) < limit; i++ {
		f := failures[i]
		desc := fmt.Sprintf("Fix failing assault target %s (%s)", f.Target, strings.TrimPrefix(string(f.Stage), "/"))
		out = append(out, assaultRemediationTask{
			Type:        "/shard_task",
			Priority:    "/high",
			Description: desc,
			Shard:       "coder",
			ShardInput: fmt.Sprintf(
				"Fix the failure for assault target %s (stage=%s cycle=%d attempt=%d, exit_code=%d). Read the log at %s, identify root cause, patch code, and re-run `go test -count=1 %s`.\n\nBe surgical; add tests if needed; avoid unrelated refactors.",
				f.Target, f.Stage, f.Cycle, f.Attempt, f.ExitCode, f.LogPath, f.Target,
			),
			Artifacts: []string{f.LogPath},
		})
	}
	return out
}

func buildAssaultSummary(total, success int, failures []assaultFailure, maxExamples int) string {
	failCount := len(failures)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("total_results=%d success=%d failures=%d\n", total, success, failCount))
	if failCount == 0 {
		sb.WriteString("No failures detected.\n")
		return sb.String()
	}
	sb.WriteString("Recent failures (most recent first):\n")
	if maxExamples <= 0 {
		maxExamples = 25
	}
	for i := 0; i < len(failures) && i < maxExamples; i++ {
		f := failures[i]
		sb.WriteString(fmt.Sprintf("- [%s] %s stage=%s cycle=%d attempt=%d exit_code=%d log=%s\n",
			f.BatchID, f.Target, f.Stage, f.Cycle, f.Attempt, f.ExitCode, f.LogPath))
	}
	return sb.String()
}

func (o *Orchestrator) getAssaultConfig() AssaultConfig {
	if o == nil || o.campaign == nil || o.campaign.Assault == nil {
		return DefaultAssaultConfig()
	}
	cfg := o.campaign.Assault.Normalize()
	// Persist normalization for long-horizon durability.
	o.campaign.Assault = &cfg
	return cfg
}

func (o *Orchestrator) assaultDir() (dir string, slug string) {
	slug = sanitizeCampaignID(o.campaign.ID)
	return filepath.Join(o.workspace, ".nerd", "campaigns", slug, "assault"), slug
}

func chunkStrings(items []string, size int) [][]string {
	if size <= 0 {
		size = 10
	}
	out := make([][]string, 0, (len(items)/size)+1)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

func normalizePrefix(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "/")
	s = normalizePath(s)
	s = strings.TrimSuffix(s, "/")
	return s
}

func normalizePrefixes(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = normalizePrefix(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func hasPrefix(path, prefix string) bool {
	if prefix == "" {
		return false
	}
	path = normalizePath(path)
	prefix = normalizePrefix(prefix)
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func matchesInclude(path string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	for _, p := range include {
		if hasPrefix(path, p) {
			return true
		}
	}
	return false
}

func matchesExclude(path string, exclude []string) bool {
	if len(exclude) == 0 {
		return false
	}
	for _, p := range exclude {
		if hasPrefix(path, p) {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:10]
}

func assaultResultKey(cycle int, stage AssaultStageKind, attempt int, target string) string {
	return fmt.Sprintf("%d|%s|%d|%s", cycle, stage, attempt, target)
}

func appendJSONL(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func readAssaultResults(path string) ([]assaultResult, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open assault results %s: %w", path, err)
	}
	defer f.Close()

	results := make([]assaultResult, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r assaultResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func readAssaultResultIndex(resultsPath string) map[string]bool {
	index := make(map[string]bool)
	results, err := readAssaultResults(resultsPath)
	if err != nil {
		return index
	}
	for _, r := range results {
		index[assaultResultKey(r.Cycle, r.Stage, r.Attempt, r.Target)] = true
	}
	return index
}

func findArtifactPath(task *Task, artifactType string) (string, bool) {
	if task == nil {
		return "", false
	}
	for _, a := range task.Artifacts {
		if a.Type == artifactType && a.Path != "" {
			return a.Path, true
		}
	}
	if len(task.Artifacts) > 0 && task.Artifacts[0].Path != "" {
		return task.Artifacts[0].Path, true
	}
	return "", false
}

func targetToDir(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "./")
	target = strings.TrimSuffix(target, "/...")
	if target == "" {
		return "."
	}
	return target
}

func writeTextFileBestEffort(path, content string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(content), 0644)
}

func newAssaultExecutor(workspace string, maxOutputBytes int64, defaultTimeout time.Duration) tactile.Executor {
	cfg := tactile.DefaultExecutorConfig()
	cfg.DefaultWorkingDir = workspace
	cfg.MaxOutputBytes = maxOutputBytes
	cfg.DefaultTimeout = defaultTimeout
	if cfg.MaxTimeout < 2*time.Hour {
		cfg.MaxTimeout = 2 * time.Hour
	}
	if defaultTimeout > cfg.MaxTimeout {
		cfg.MaxTimeout = defaultTimeout
	}
	return tactile.NewDirectExecutorWithConfig(cfg)
}

func shellForCommand(cmdLine string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", cmdLine}
	}
	return "bash", []string{"-c", cmdLine}
}

func (o *Orchestrator) discoverAssaultTargets(ctx context.Context, cfg AssaultConfig) ([]string, error) {
	// Go-first: use go list when go.mod exists.
	if _, err := os.Stat(filepath.Join(o.workspace, "go.mod")); err == nil {
		return o.discoverGoTargets(ctx, cfg)
	}

	// Generic fallback: treat repo or include prefixes as coarse targets.
	if cfg.Scope == AssaultScopeRepo || len(cfg.Include) == 0 {
		return []string{"./..."}, nil
	}

	out := make([]string, 0, len(cfg.Include))
	for _, inc := range cfg.Include {
		inc = normalizePrefix(inc)
		if inc == "" || inc == "." {
			out = append(out, "./...")
			continue
		}
		out = append(out, "./"+inc+"/...")
	}
	return uniqueStrings(out), nil
}

func (o *Orchestrator) discoverGoTargets(ctx context.Context, cfg AssaultConfig) ([]string, error) {
	if o.executor == nil {
		o.executor = tactile.NewDirectExecutor()
	}

	timeout := cfg.DefaultTimeoutSeconds
	if timeout <= 0 {
		timeout = 900
	}

	// Ask go list for directories so we can group by subsystem/module.
	cmd := tactile.Command{
		Binary:           "go",
		Arguments:        []string{"list", "-f", "{{.Dir}}", "./..."},
		WorkingDirectory: o.workspace,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(timeout) * 1000,
		},
	}

	res, err := o.executor.Execute(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}
	out := res.Output()

	lines := strings.Split(out, "\n")
	pkgDirs := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rel, err := filepath.Rel(o.workspace, line)
		if err != nil {
			continue
		}
		rel = normalizePath(rel)
		if rel == "" {
			continue
		}
		pkgDirs = append(pkgDirs, rel)
	}

	include := normalizePrefixes(cfg.Include)
	exclude := normalizePrefixes(cfg.Exclude)

	targetSet := make(map[string]struct{})
	switch cfg.Scope {
	case AssaultScopeRepo:
		targetSet["./..."] = struct{}{}

	case AssaultScopePackage:
		for _, dir := range pkgDirs {
			if !matchesInclude(dir, include) || matchesExclude(dir, exclude) {
				continue
			}
			if dir == "." {
				targetSet["./"] = struct{}{}
				continue
			}
			targetSet["./"+dir] = struct{}{}
		}

	case AssaultScopeModule:
		for _, dir := range pkgDirs {
			key := groupModule(dir)
			if key == "" {
				continue
			}
			if !matchesInclude(key, include) || matchesExclude(key, exclude) {
				continue
			}
			if key == "." {
				targetSet["./"] = struct{}{}
				continue
			}
			targetSet["./"+key+"/..."] = struct{}{}
		}

	case AssaultScopeSubsystem, "":
		for _, dir := range pkgDirs {
			key := groupSubsystem(dir)
			if key == "" {
				continue
			}
			if !matchesInclude(key, include) || matchesExclude(key, exclude) {
				continue
			}
			if key == "." {
				targetSet["./"] = struct{}{}
				continue
			}
			targetSet["./"+key+"/..."] = struct{}{}
		}

	default:
		return nil, fmt.Errorf("unsupported assault scope: %s", cfg.Scope)
	}

	targets := make([]string, 0, len(targetSet))
	for t := range targetSet {
		targets = append(targets, t)
	}
	return targets, nil
}

func groupModule(relDir string) string {
	relDir = normalizePath(relDir)
	if relDir == "" {
		return ""
	}
	if relDir == "." {
		return "."
	}
	parts := strings.Split(relDir, "/")
	return parts[0]
}

func groupSubsystem(relDir string) string {
	relDir = normalizePath(relDir)
	if relDir == "" {
		return ""
	}
	if relDir == "." {
		return "."
	}
	parts := strings.Split(relDir, "/")
	if len(parts) >= 2 && (parts[0] == "internal" || parts[0] == "cmd" || parts[0] == "pkg") {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func (o *Orchestrator) phaseIDByOrder(order int) (phaseID string, existingTasks int, ok bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return "", 0, false
	}
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].Order == order {
			return o.campaign.Phases[i].ID, len(o.campaign.Phases[i].Tasks), true
		}
	}
	return "", 0, false
}

func (o *Orchestrator) appendTasksToPhase(phaseID string, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	// Update campaign structure first.
	o.mu.Lock()
	if o.campaign == nil {
		o.mu.Unlock()
		return fmt.Errorf("no campaign loaded")
	}

	var phase *Phase
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == phaseID {
			phase = &o.campaign.Phases[i]
			break
		}
	}
	if phase == nil {
		o.mu.Unlock()
		return fmt.Errorf("phase not found: %s", phaseID)
	}

	for i := range tasks {
		phase.Tasks = append(phase.Tasks, tasks[i])
		o.campaign.TotalTasks++
	}
	phase.EstimatedTasks = len(phase.Tasks)

	phaseEstimatedTasks := phase.EstimatedTasks
	phaseComplexity := phase.EstimatedComplexity
	campaignID := o.campaign.ID
	completedPhases := o.campaign.CompletedPhases
	totalPhases := o.campaign.TotalPhases
	completedTasks := o.campaign.CompletedTasks
	totalTasks := o.campaign.TotalTasks
	o.campaign.UpdatedAt = time.Now()

	o.mu.Unlock()

	// Inject new task facts into kernel (plus refresh progress fact).
	facts := make([]core.Fact, 0, len(tasks)*6)
	for i := range tasks {
		facts = append(facts, tasks[i].ToFacts()...)
	}
	if err := o.kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("failed to load new assault task facts: %w", err)
	}

	// Refresh campaign_progress fact for logic consumers.
	_ = o.kernel.RetractFact(core.Fact{Predicate: "campaign_progress", Args: []interface{}{campaignID}})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "campaign_progress",
		Args:      []interface{}{campaignID, completedPhases, totalPhases, completedTasks, totalTasks},
	})

	// Refresh phase_estimate for the mutated phase (best-effort).
	_ = o.kernel.RetractFact(core.Fact{Predicate: "phase_estimate", Args: []interface{}{phaseID}})
	_ = o.kernel.Assert(core.Fact{
		Predicate: "phase_estimate",
		Args:      []interface{}{phaseID, phaseEstimatedTasks, phaseComplexity},
	})

	// Persist campaign immediately for long-horizon durability.
	o.mu.Lock()
	_ = o.saveCampaign()
	o.mu.Unlock()

	return nil
}
