package campaign

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Assault summary export.
//
// An assault campaign leaves its evidence spread across one JSONL file per
// batch under assault/results, one log file per stage run under assault/logs,
// a targets.json and a triage/latest.json. That layout is right for durable,
// resumable execution and useless for answering "what broke". Reading it means
// globbing dozens of files and joining them by hand.
//
// This aggregates the whole run into one report — per stage, per target, worst
// offenders first — and writes it next to the evidence it summarises.

// AssaultStageSummary aggregates one stage across every target.
type AssaultStageSummary struct {
	Stage      AssaultStageKind `json:"stage"`
	Runs       int              `json:"runs"`
	Passed     int              `json:"passed"`
	Failed     int              `json:"failed"`
	Killed     int              `json:"killed"`
	TotalMs    int64            `json:"total_ms"`
	SlowestMs  int64            `json:"slowest_ms"`
	SlowTarget string           `json:"slowest_target,omitzero"`
}

// AssaultTargetSummary aggregates one target across every stage.
type AssaultTargetSummary struct {
	Target      string   `json:"target"`
	Runs        int      `json:"runs"`
	Failed      int      `json:"failed"`
	Killed      int      `json:"killed"`
	TotalMs     int64    `json:"total_ms"`
	FailedStage []string `json:"failed_stages,omitzero"`
	LogPaths    []string `json:"log_paths,omitzero"`
}

// AssaultFailureRecord is one failing stage run, kept for the report.
type AssaultFailureRecord struct {
	Target   string           `json:"target"`
	Stage    AssaultStageKind `json:"stage"`
	Cycle    int              `json:"cycle"`
	Attempt  int              `json:"attempt"`
	ExitCode int              `json:"exit_code"`
	Killed   bool             `json:"killed,omitzero"`
	Reason   string           `json:"reason,omitzero"`
	LogPath  string           `json:"log_path,omitzero"`
	At       time.Time        `json:"at"`
}

// AssaultSummary is the aggregate report for one assault campaign.
type AssaultSummary struct {
	CampaignID  string       `json:"campaign_id"`
	Slug        string       `json:"slug"`
	GeneratedAt time.Time    `json:"generated_at"`
	Scope       AssaultScope `json:"scope,omitzero"`

	Targets      int `json:"targets"`
	Batches      int `json:"batches"`
	BatchesRun   int `json:"batches_run"`
	TotalRuns    int `json:"total_runs"`
	Passed       int `json:"passed"`
	Failed       int `json:"failed"`
	Killed       int `json:"killed"`
	TargetsClean int `json:"targets_clean"`
	TargetsBad   int `json:"targets_failing"`

	Stages  []AssaultStageSummary  `json:"stages,omitzero"`
	Worst   []AssaultTargetSummary `json:"worst_targets,omitzero"`
	Samples []AssaultFailureRecord `json:"failure_samples,omitzero"`

	TriageSummary    string `json:"triage_summary,omitzero"`
	RemediationTasks int    `json:"remediation_tasks"`

	// Incomplete is true when discovery ran but some batches produced no
	// results file. A summary over a partial run must say so: "0 failures" from
	// batches that never executed reads exactly like a clean sweep.
	Incomplete bool `json:"incomplete"`
}

const assaultSummaryFailureSamples = 25

// ExportAssaultSummary aggregates the campaign's assault evidence and writes
// both a JSON and a Markdown report under assault/. It returns the Markdown
// path, which is the one an operator opens.
func (o *Orchestrator) ExportAssaultSummary() (string, *AssaultSummary, error) {
	o.mu.RLock()
	c := o.campaign
	workspace := o.workspace
	o.mu.RUnlock()

	if c == nil {
		return "", nil, ErrNilCampaign
	}
	summary, err := BuildAssaultSummary(workspace, c.ID)
	if err != nil {
		return "", nil, err
	}
	path, err := WriteAssaultSummary(workspace, summary)
	return path, summary, err
}

// BuildAssaultSummary reads the on-disk assault evidence for a campaign.
func BuildAssaultSummary(workspace, campaignID string) (*AssaultSummary, error) {
	slug := sanitizeCampaignID(campaignID)
	assaultDir := filepath.Join(workspace, ".nerd", "campaigns", slug, "assault")

	if _, err := os.Stat(assaultDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no assault evidence for %s: %s does not exist", campaignID, assaultDir)
		}
		return nil, err
	}

	summary := &AssaultSummary{
		CampaignID:  campaignID,
		Slug:        slug,
		GeneratedAt: time.Now().UTC(),
	}

	if data, err := os.ReadFile(filepath.Join(assaultDir, "targets.json")); err == nil {
		var targets assaultTargetsFile
		if json.Unmarshal(data, &targets) == nil {
			summary.Scope = targets.Scope
			summary.Targets = len(targets.Targets)
		}
	}

	batchIDs := map[string]bool{}
	if entries, err := os.ReadDir(filepath.Join(assaultDir, "batches")); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				batchIDs[strings.TrimSuffix(e.Name(), ".json")] = false
			}
		}
	}
	summary.Batches = len(batchIDs)

	stages := map[AssaultStageKind]*AssaultStageSummary{}
	targets := map[string]*AssaultTargetSummary{}

	resultsDir := filepath.Join(assaultDir, "results")
	entries, err := os.ReadDir(resultsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read assault results dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		batchIDs[strings.TrimSuffix(e.Name(), ".jsonl")] = true
		results, rerr := readAssaultResults(filepath.Join(resultsDir, e.Name()))
		if rerr != nil {
			return nil, rerr
		}
		for _, r := range results {
			summary.TotalRuns++

			st := stages[r.Stage]
			if st == nil {
				st = &AssaultStageSummary{Stage: r.Stage}
				stages[r.Stage] = st
			}
			st.Runs++
			st.TotalMs += r.DurationMs
			if r.DurationMs > st.SlowestMs {
				st.SlowestMs = r.DurationMs
				st.SlowTarget = r.Target
			}

			tg := targets[r.Target]
			if tg == nil {
				tg = &AssaultTargetSummary{Target: r.Target}
				targets[r.Target] = tg
			}
			tg.Runs++
			tg.TotalMs += r.DurationMs

			failed := r.ExitCode != 0 || r.Killed || r.Error != ""
			switch {
			case r.Killed:
				summary.Killed++
				st.Killed++
				tg.Killed++
			case !failed:
				summary.Passed++
				st.Passed++
			}
			if failed {
				summary.Failed++
				st.Failed++
				tg.Failed++
				tg.FailedStage = appendUniqueString(tg.FailedStage, string(r.Stage))
				if r.LogPath != "" {
					tg.LogPaths = appendUniqueString(tg.LogPaths, r.LogPath)
				}
				if len(summary.Samples) < assaultSummaryFailureSamples {
					reason := r.Error
					if reason == "" && r.KillReason != "" {
						reason = r.KillReason
					}
					summary.Samples = append(summary.Samples, AssaultFailureRecord{
						Target:   r.Target,
						Stage:    r.Stage,
						Cycle:    r.Cycle,
						Attempt:  r.Attempt,
						ExitCode: r.ExitCode,
						Killed:   r.Killed,
						Reason:   reason,
						LogPath:  r.LogPath,
						At:       r.StartedAt,
					})
				}
			}
		}
	}

	for _, ran := range batchIDs {
		if ran {
			summary.BatchesRun++
		}
	}
	summary.Incomplete = summary.Batches > 0 && summary.BatchesRun < summary.Batches

	summary.Stages = make([]AssaultStageSummary, 0, len(stages))
	for _, st := range stages {
		summary.Stages = append(summary.Stages, *st)
	}
	sort.Slice(summary.Stages, func(i, j int) bool { return summary.Stages[i].Stage < summary.Stages[j].Stage })

	worst := make([]AssaultTargetSummary, 0, len(targets))
	for _, tg := range targets {
		if tg.Failed > 0 {
			summary.TargetsBad++
			worst = append(worst, *tg)
		} else {
			summary.TargetsClean++
		}
	}
	sort.Slice(worst, func(i, j int) bool {
		if worst[i].Failed != worst[j].Failed {
			return worst[i].Failed > worst[j].Failed
		}
		return worst[i].Target < worst[j].Target
	})
	summary.Worst = worst

	if data, err := os.ReadFile(filepath.Join(assaultDir, "triage", "latest.json")); err == nil {
		var triage assaultTriageOutput
		if json.Unmarshal(data, &triage) == nil {
			summary.TriageSummary = strings.TrimSpace(triage.Summary)
			summary.RemediationTasks = len(triage.RecommendedTasks)
		}
	}

	return summary, nil
}

// WriteAssaultSummary persists the report as summary.json and summary.md under
// the campaign's assault directory, returning the Markdown path.
func WriteAssaultSummary(workspace string, summary *AssaultSummary) (string, error) {
	if summary == nil {
		return "", fmt.Errorf("nil assault summary")
	}
	dir := filepath.Join(workspace, ".nerd", "campaigns", summary.Slug, "assault")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create assault dir: %w", err)
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal assault summary: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), data, 0o644); err != nil {
		return "", fmt.Errorf("write assault summary json: %w", err)
	}

	mdPath := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(mdPath, []byte(RenderAssaultSummaryMarkdown(summary)), 0o644); err != nil {
		return "", fmt.Errorf("write assault summary markdown: %w", err)
	}
	return mdPath, nil
}

// RenderAssaultSummaryMarkdown formats the report for a human.
func RenderAssaultSummaryMarkdown(s *AssaultSummary) string {
	if s == nil {
		return "# Assault summary\n\nNo data.\n"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Assault summary — %s\n\n", s.CampaignID)
	fmt.Fprintf(&sb, "Generated %s\n\n", s.GeneratedAt.Format(time.RFC3339))

	if s.Incomplete {
		sb.WriteString("> **Partial run.** ")
		fmt.Fprintf(&sb, "%d of %d batches produced results. Counts below cover only the batches that ran — "+
			"a low failure count here is not a clean sweep.\n\n", s.BatchesRun, s.Batches)
	}

	sb.WriteString("## Totals\n\n")
	sb.WriteString("| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(&sb, "| Scope | %s |\n", strings.TrimPrefix(string(s.Scope), "/"))
	fmt.Fprintf(&sb, "| Targets discovered | %d |\n", s.Targets)
	fmt.Fprintf(&sb, "| Batches (run/total) | %d / %d |\n", s.BatchesRun, s.Batches)
	fmt.Fprintf(&sb, "| Stage runs | %d |\n", s.TotalRuns)
	fmt.Fprintf(&sb, "| Passed | %d |\n", s.Passed)
	fmt.Fprintf(&sb, "| Failed | %d |\n", s.Failed)
	fmt.Fprintf(&sb, "| Killed (timeout) | %d |\n", s.Killed)
	fmt.Fprintf(&sb, "| Targets clean | %d |\n", s.TargetsClean)
	fmt.Fprintf(&sb, "| Targets failing | %d |\n", s.TargetsBad)
	fmt.Fprintf(&sb, "| Remediation tasks proposed | %d |\n\n", s.RemediationTasks)

	if len(s.Stages) > 0 {
		sb.WriteString("## By stage\n\n")
		sb.WriteString("| Stage | Runs | Passed | Failed | Killed | Total (s) | Slowest target |\n")
		sb.WriteString("|---|---:|---:|---:|---:|---:|---|\n")
		for _, st := range s.Stages {
			fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %.1f | %s (%.1fs) |\n",
				strings.TrimPrefix(string(st.Stage), "/"), st.Runs, st.Passed, st.Failed, st.Killed,
				float64(st.TotalMs)/1000, st.SlowTarget, float64(st.SlowestMs)/1000)
		}
		sb.WriteString("\n")
	}

	if len(s.Worst) > 0 {
		sb.WriteString("## Failing targets\n\n")
		sb.WriteString("| Target | Failed | Killed | Stages | Total (s) |\n|---|---:|---:|---|---:|\n")
		for _, tg := range s.Worst {
			fmt.Fprintf(&sb, "| %s | %d | %d | %s | %.1f |\n",
				tg.Target, tg.Failed, tg.Killed,
				strings.Join(trimAtomList(tg.FailedStage), ", "), float64(tg.TotalMs)/1000)
		}
		sb.WriteString("\n")
	} else if s.TotalRuns > 0 {
		sb.WriteString("## Failing targets\n\nNone.\n\n")
	}

	if len(s.Samples) > 0 {
		sb.WriteString("## Failure samples\n\n")
		for _, f := range s.Samples {
			fmt.Fprintf(&sb, "- **%s** / %s (cycle %d, attempt %d) exit=%d",
				f.Target, strings.TrimPrefix(string(f.Stage), "/"), f.Cycle, f.Attempt, f.ExitCode)
			if f.Killed {
				sb.WriteString(" **killed**")
			}
			if f.Reason != "" {
				fmt.Fprintf(&sb, " — %s", truncateForDisplay(f.Reason, 160))
			}
			if f.LogPath != "" {
				fmt.Fprintf(&sb, "\n  - log: `%s`", f.LogPath)
			}
			sb.WriteString("\n")
		}
		if s.Failed > len(s.Samples) {
			fmt.Fprintf(&sb, "\n_%d further failures omitted; see `results/`._\n", s.Failed-len(s.Samples))
		}
		sb.WriteString("\n")
	}

	if s.TriageSummary != "" {
		sb.WriteString("## Triage\n\n```\n")
		sb.WriteString(s.TriageSummary)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

func trimAtomList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.TrimPrefix(s, "/"))
	}
	return out
}

func appendUniqueString(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
