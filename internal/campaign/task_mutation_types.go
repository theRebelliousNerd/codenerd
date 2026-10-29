package campaign

// isMutatingTaskType identifies task types that may mutate campaign-controlled
// workspace state and should therefore participate in write-set locking and
// scoped rollback.
func isMutatingTaskType(taskType TaskType) bool {
	switch taskType {
	case TaskTypeFileCreate,
		TaskTypeFileModify,
		TaskTypeTestWrite,
		TaskTypeDocument,
		TaskTypeRefactor,
		TaskTypeIntegrate,
		TaskTypeToolCreate,
		TaskTypeAssaultDiscover,
		TaskTypeAssaultTriage:
		return true
	default:
		return false
	}
}

// requiresCampaignStructuralSnapshot reports whether a task type can restructure
// the campaign PLAN itself (adding/removing phases or tasks on the fly) and thus
// needs a full-campaign snapshot to roll back a partial plan edit on failure.
//
// Only assault task types qualify: they dynamically append discovered/triaged
// tasks to the running phase (see assault_tasks.go). File-writing task types
// (/file_*, /document, /refactor, /integrate, /test_write, /tool_create) do NOT
// restructure the plan — their only durable side effects are file mutations
// (rolled back via the per-task write-set file snapshot) and their own task
// status (owned by handleTaskFailure). Taking a WHOLE-campaign snapshot for them
// and restoring it on failure reverts the committed completions of concurrently
// running sibling tasks and orphans the phase pointer held by runPhase — the
// root cause of the F-SCHED-2 infinite completion→re-dispatch loop. Such tasks
// therefore get file + own-status rollback only, never a campaign swap.
func requiresCampaignStructuralSnapshot(task *Task) bool {
	if task == nil {
		return false
	}
	switch task.Type {
	// Only Discover and Triage restructure the plan (append batch/triage tasks to
	// the running phase). AssaultBatch is intentionally excluded: it is NOT a
	// mutating type (see isMutatingTaskType), so it never reaches a snapshot, and
	// its only writes are idempotent resume-indexed JSONL under .nerd/ that must
	// NOT be rolled back. Listing it here would be dead today and a latent
	// F-SCHED-2 trap if batch ever became mutating.
	case TaskTypeAssaultDiscover,
		TaskTypeAssaultTriage:
		return true
	default:
		return false
	}
}
