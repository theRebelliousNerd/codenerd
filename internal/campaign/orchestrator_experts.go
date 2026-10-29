package campaign

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/session"
)

// Which shard agent a campaign task needs is the kernel's decision
// (expert_needed, specialist_should_execute, specialist_should_advise in
// policy/shard_experts.mg and policy/shards.mg), asked through
// internal/experts with the task's files -- the same derivation chat
// delegation and every executor compile use. The campaign used to carry its
// own rules for it (prefer_specialist_for_task, campaign_task_shard_override,
// a campaign delegate_task), each joining shard_can_handle, which nothing
// produced; none of them could fire, and no campaign task reached a shard
// agent.
//
// What the decision does to a task's turn:
//   - every task: the turn's Target is the task's primary file, so the
//     executor's compile derives the task's experts (neededExperts) and
//     carries their knowledge, its language and its file context, as a chat
//     turn on that file would;
//   - specialist_should_advise: before the turn, the agent is consulted
//     ("/consult/<agent>") on the campaign's own task executor, and its advice
//     is part of the turn's input;
//   - specialist_should_execute: the agent's match is strong enough to own the
//     task. The turn keeps the task's verb, tools and gates (a campaign task's
//     obligations are its verb's), and the agent's knowledge drives it through
//     the compile; it is not also consulted.

// taskTargets is the task's declared files: its artifacts and its write set,
// workspace-relative, globs left out (a glob names no file to judge).
func (o *Orchestrator) taskTargets(task *Task) []string {
	if task == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(raw string) {
		p := strings.TrimSpace(raw)
		if p == "" || containsGlobMeta(p) {
			return
		}
		p = filepath.FromSlash(p)
		if filepath.IsAbs(p) {
			if o.workspace == "" {
				return
			}
			rel, err := filepath.Rel(o.workspace, p)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return
			}
			p = rel
		}
		p = filepath.ToSlash(filepath.Clean(p))
		if p == "." || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, a := range task.Artifacts {
		add(a.Path)
	}
	for _, w := range task.WriteSet {
		add(w)
	}
	return out
}

// selectTaskExperts asks the kernel which shard agents the task's files need.
// A kernel that cannot answer leaves the task without one, and says so.
func (o *Orchestrator) selectTaskExperts(task *Task, verb string, targets []string) experts.Selection {
	if o.kernel == nil || len(targets) == 0 {
		return experts.Selection{}
	}
	// Complexity is the kernel's (task_complexity in policy/shard_experts.mg,
	// derived from the task's files and verb), not the phase's estimate.
	sel, err := experts.Select(o.kernel, experts.Task{
		Verb:      verb,
		Targets:   targets,
		Workspace: o.workspace,
	})
	if err != nil {
		logging.Get(logging.CategoryCampaign).Warn("shard expert selection for task %s failed: %v", task.ID, err)
		return experts.Selection{}
	}
	if agents := sel.Agents(); len(agents) > 0 {
		logging.Campaign("Task %s (%s on %v) needs shard agents %v", task.ID, verb, targets, agents)
	}
	return sel
}

// withExpertAdvice consults each agent the kernel says should advise on the
// task and appends the advice to input. A consultation that fails is named in
// the input, so the turn does not take silence for agreement.
func (o *Orchestrator) withExpertAdvice(ctx context.Context, observed session.ObservedTaskExecutor, task *Task, sel experts.Selection, input string) string {
	var sb strings.Builder
	for _, need := range sel.Needs {
		if !need.Advise || need.Execute {
			continue
		}
		o.emitEvent(EventSpecialistConsulted, task.PhaseID, task.ID, need.Agent, map[string]any{
			"agent": need.Agent, "technologies": need.Technologies, "confidence": need.Confidence,
		})
		ret, err := observed.ExecuteObserved(ctx, session.TaskRequest{
			IntentVerb: "/consult/" + need.Agent,
			Task:       consultationTask(task, need),
		})
		advice := strings.TrimSpace(ret.Output)
		switch {
		case err != nil:
			logging.Get(logging.CategoryCampaign).Warn("Shard agent %s could not advise on task %s: %v", need.Agent, task.ID, err)
			fmt.Fprintf(&sb, "\n\n=== SHARD AGENT %s WAS ASKED TO ADVISE AND FAILED ===\n%v\n", need.Agent, err)
		case advice == "":
			fmt.Fprintf(&sb, "\n\n=== SHARD AGENT %s WAS ASKED TO ADVISE AND GAVE NO ADVICE ===\n", need.Agent)
		default:
			fmt.Fprintf(&sb, "\n\n=== ADVICE FROM SHARD AGENT %s (domain: %s) ===\n%s\n", need.Agent, strings.Join(need.Technologies, ", "), advice)
		}
	}
	if sb.Len() == 0 {
		return input
	}
	return input + sb.String()
}

// consultationTask is what an advising agent is asked: the task and the files
// that tie it to the agent's domain.
func consultationTask(task *Task, need experts.Need) string {
	files := need.Files
	if len(files) == 0 {
		files = []string{"(the task's files)"}
	}
	return fmt.Sprintf("ADVISORY REQUEST from a campaign task. Another agent will make the change; give the domain advice it needs.\n\n"+
		"Task: %s\nFiles in your domain (%s): %s\n\n"+
		"Name the pitfalls, conventions and checks this change must respect in your domain. Do not write the change yourself.",
		task.Description, strings.Join(need.Technologies, ", "), strings.Join(files, ", "))
}
