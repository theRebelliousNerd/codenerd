package campaign

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// The orchestrator measures and executes; campaign_rules.mg judges. This file
// holds three measurements the policy turns into decisions:
//
//   - how much of its budget the context pager uses (context pressure), which
//     decides how many upcoming tasks it prefetches context for;
//   - what a completed task's files measurably carry (placeholders, credentials,
//     unchecked errors, no tests), which decides whether a remediation task is
//     owed;
//   - how far the campaign has come, which decides which milestones are reached.

// prefetchLimit asks the kernel how many upcoming tasks the pager may prefetch
// context for. The pager's usage is asserted first. A kernel that cannot say
// prefetches nothing: prefetching is an optimisation, and the safe answer to
// "unknown pressure" is to not spend budget.
func (o *Orchestrator) prefetchLimit() int {
	if o.contextPager == nil || o.kernel == nil || o.campaign == nil {
		return 0
	}
	o.mu.RLock()
	campaignID := o.campaign.ID
	o.mu.RUnlock()
	used, total, _ := o.contextPager.GetUsage()
	if total <= 0 {
		return 0
	}
	use := core.Fact{Predicate: "campaign_context_use", Args: []any{campaignID, int64(used), int64(total)}}
	if err := o.kernel.RetractFact(core.Fact{Predicate: "campaign_context_use", Args: []any{campaignID}}); err != nil {
		logging.CampaignDebug("context use not retracted: %v", err)
	}
	if err := o.kernel.Assert(use); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("context use not asserted; prefetching nothing: %v", err)
		return 0
	}
	limit, err := o.oneDerivedFor("campaign_prefetch_limit", campaignID)
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("no prefetch decision; prefetching nothing: %v", err)
		return 0
	}
	var n int
	if _, scanErr := fmt.Sscanf(limit, "%d", &n); scanErr != nil || n < 0 {
		return 0
	}
	return n
}

// announceMilestones emits one event for each milestone the kernel derives and
// has not yet seen recorded, then records it. Progress is asserted first so the
// rules judge the campaign as it stands.
func (o *Orchestrator) announceMilestones() {
	if o.kernel == nil || o.campaign == nil {
		return
	}
	o.mu.RLock()
	c := o.campaign
	campaignID := c.ID
	progress := core.Fact{Predicate: "campaign_progress", Args: []any{
		campaignID, int64(c.CompletedPhases), int64(c.TotalPhases), int64(c.CompletedTasks), int64(c.TotalTasks),
	}}
	o.mu.RUnlock()
	_ = o.kernel.RetractFact(core.Fact{Predicate: "campaign_progress", Args: []any{campaignID}})
	if err := o.kernel.Assert(progress); err != nil {
		logging.Get(logging.CategoryCampaign).Warn("campaign progress not asserted; no milestone announced: %v", err)
		return
	}
	pending, err := o.derivedMilestones(campaignID)
	if err != nil {
		logging.CampaignWarn("milestones not derived: %v", err)
		return
	}
	for _, m := range pending {
		name := strings.TrimPrefix(m, "/")
		record := core.Fact{Predicate: "campaign_milestone", Args: []any{
			campaignID, types.MangleAtom(m), "milestone reached: " + name, time.Now().Unix(),
		}}
		if err := o.kernel.Assert(record); err != nil {
			logging.CampaignWarn("milestone %s not recorded; it would be announced again: %v", name, err)
			continue
		}
		logging.Campaign("Milestone reached: %s", name)
		o.emitEvent(EventMilestoneReached, "", "", name, map[string]any{"campaign_id": campaignID, "milestone": name})
	}
}

// derivedMilestones reads the unary-keyed pending milestones of one campaign.
func (o *Orchestrator) derivedMilestones(campaignID string) ([]string, error) {
	return o.derivedFor("milestone_pending", campaignID)
}

// qualityGap is one completed task and the violations the kernel measured in
// its files.
type qualityGap struct {
	task       string
	violations []string
}

// appendQualityRemediation reads the kernel's remediation_task_needed for the
// phase's completed file tasks and appends one fix task per task, naming what
// was measured. The world model is refreshed first: a task has just written
// files the boot scan never saw. The target of each remediation is recorded
// (task_remediation_target), which retires the debt in the policy and keeps a
// fix that falls short from being re-spawned. It returns how many tasks it
// appended.
func (o *Orchestrator) appendQualityRemediation(ctx context.Context, phaseID string) (int, error) {
	if o.kernel == nil || o.campaign == nil {
		return 0, nil
	}
	o.refreshWorldModel(ctx)

	rows, err := o.kernel.Query("remediation_task_needed")
	if err != nil {
		return 0, fmt.Errorf("query remediation_task_needed: %w", err)
	}
	needed := map[string][]string{}
	for _, f := range rows {
		if len(f.Args) >= 2 {
			task := types.ExtractString(f.Args[0])
			needed[task] = append(needed[task], types.ExtractString(f.Args[1]))
		}
	}
	if len(needed) == 0 {
		return 0, nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	c := o.campaign
	var phase *Phase
	for i := range c.Phases {
		if c.Phases[i].ID == phaseID {
			phase = &c.Phases[i]
			break
		}
	}
	if phase == nil {
		return 0, fmt.Errorf("quality remediation: phase %s is not in the campaign", phaseID)
	}
	var gaps []qualityGap
	for _, t := range phase.Tasks {
		if t.Status != TaskCompleted || (t.Type != TaskTypeFileCreate && t.Type != TaskTypeFileModify) {
			continue
		}
		if v, ok := needed[t.ID]; ok {
			sort.Strings(v)
			gaps = append(gaps, qualityGap{task: t.ID, violations: v})
		}
	}
	if len(gaps) == 0 {
		return 0, nil
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].task < gaps[j].task })

	previous, err := cloneCampaign(c)
	if err != nil {
		return 0, fmt.Errorf("quality remediation: %w", err)
	}
	var targets []core.Fact
	for _, gap := range gaps {
		orig := phase.taskByID(gap.task)
		scope := taskWriteScope(orig)
		if len(scope) == 0 {
			continue // a file task with no declared target is refused before it runs
		}
		var names []string
		for _, v := range gap.violations {
			names = append(names, strings.TrimPrefix(v, "/"))
		}
		id := fmt.Sprintf("%s_fix", orig.ID)
		phase.Tasks = append(phase.Tasks, Task{
			ID:      id,
			PhaseID: phase.ID,
			Description: fmt.Sprintf(
				"Task %q is complete, but the files it wrote carry measured quality violations: %s.\n\n"+
					"Fix them in %s: finish what is marked unfinished, move credentials out of source, check the errors that are dropped, add the missing tests. "+
					"Do not delete working behaviour to satisfy a measurement.",
				orig.Description, strings.Join(names, ", "), strings.Join(scope, ", ")),
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			PlannedType: TaskTypeFileModify,
			Priority:    PriorityHigh,
			WriteSet:    scope,
			DependsOn:   []string{orig.ID},
		})
		c.TotalTasks++
		for _, v := range gap.violations {
			targets = append(targets, core.Fact{Predicate: "task_remediation_target", Args: []any{id, orig.ID, types.MangleAtom(v)}})
		}
	}
	if len(targets) == 0 {
		return 0, nil
	}
	if err := syncCampaignFacts(o.kernel, previous, c, fmt.Sprintf("phase %s: quality remediation appended", phase.ID)); err != nil {
		return 0, fmt.Errorf("quality remediation: %w", err)
	}
	if err := o.kernel.AssertBatch(targets); err != nil {
		return 0, fmt.Errorf("quality remediation: targets not recorded: %w", err)
	}
	o.persistCampaign(fmt.Sprintf("quality remediation %s", phase.ID))
	appended := len(gaps)
	logging.Campaign("Quality remediation appended to %s: %d task(s)", phase.ID, appended)
	o.emitEvent(EventQualityRemediation, phase.ID, "", fmt.Sprintf("%d fix task(s) appended for measured quality violations", appended), nil)
	return appended, nil
}

func (p *Phase) taskByID(id string) *Task {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			return &p.Tasks[i]
		}
	}
	return nil
}

// taskWriteScope is where a remediation may write: the files the task declared
// or produced.
func taskWriteScope(t *Task) []string {
	if t == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range t.WriteSet {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, a := range t.Artifacts {
		if a.Path != "" && !seen[a.Path] && isOutputArtifactType(a.Type) {
			seen[a.Path] = true
			out = append(out, a.Path)
		}
	}
	sort.Strings(out)
	return out
}

// refreshWorldModel re-measures the workspace and applies the delta, so rules
// over file measurements judge what the tasks wrote and not what the boot scan
// saw. A failure is logged and judged on what the kernel holds.
func (o *Orchestrator) refreshWorldModel(ctx context.Context) {
	if o.workspace == "" {
		return
	}
	res, err := world.NewScanner().ScanWorkspaceIncremental(ctx, o.workspace, nil, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		logging.CampaignWarn("world model refresh before the quality gate failed: %v", err)
		return
	}
	if err := world.ApplyIncrementalResult(o.kernel, res); err != nil {
		logging.CampaignWarn("world model refresh not applied: %v", err)
	}
}
