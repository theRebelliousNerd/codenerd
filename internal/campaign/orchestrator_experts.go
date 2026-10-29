package campaign

import (
	"path/filepath"
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
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
// What the decision does to a task's turn: the turn's Target is the task's
// primary file, so the session executor derives the task's experts as it does
// for any turn on that file -- their knowledge in the compile, and the
// consultations the kernel spawns (expert_spawn: /lead or /advise) run before
// the model is called. The turn keeps the task's verb, tools and gates. The
// campaign consults no one itself; it records the selection
// (recordTaskExperts) as events.

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

// recordTaskExperts records which shard agents the kernel selects for the
// task's turn, from the selection the executor itself derives: the task's verb
// on its primary file. It consults no one. The session executor runs every
// consultation the kernel spawns (expert_spawn, internal/session/
// expert_spawn.go) for every entry point, campaign tasks included; a
// campaign-level consultation was a second path that asked the same advisor
// twice per task, and was removed on 2026-10-29.
func (o *Orchestrator) recordTaskExperts(task *Task, verb, target string) {
	sel := o.selectTaskExperts(task, verb, []string{target})
	for _, need := range sel.Needs {
		data := map[string]any{
			"agent": need.Agent, "technologies": need.Technologies, "confidence": need.Confidence,
			"consulted_by": "the task's turn (session executor)",
		}
		switch {
		case need.Spawn == experts.SpawnLead || need.Execute:
			o.emitEvent(EventSpecialistExecutes, task.PhaseID, task.ID, need.Agent, data)
		case need.Spawn == experts.SpawnAdvise || need.Advise:
			o.emitEvent(EventSpecialistConsulted, task.PhaseID, task.ID, need.Agent, data)
		}
	}
}
