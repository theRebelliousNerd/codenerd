package tools

import "context"

// CampaignCheck is the acceptance command a campaign declared for itself
// (Acceptance.Command, run by runAcceptanceRound), handed to a task's turn
// so the turn can run the same judge the campaign will run after it.
//
// On campaign 7b853890 two acceptance-fix attempts made the right edit but
// could not run the checker themselves; each re-run cost a whole acceptance
// round. The check rides the task context from the orchestrator's spawnTask
// to the run_check tool, the way the write set rides TestScope and the write
// guard rides the session's writeGuardKey: the executor path between them
// (TaskExecutor.ExecuteObserved) derives no new context, so what spawnTask
// sets is what the tool reads.
type CampaignCheck struct {
	// CampaignID is the campaign that declared the check.
	CampaignID string
	// TaskID is the task whose turn may run it.
	TaskID string
	// Argv is the exact command line, run as argv[0] with argv[1:]. It is
	// never joined into a shell string and never takes model input.
	Argv []string
}

type campaignCheckKey struct{}

// WithCampaignCheck returns a context in which the run_check tool runs check.
func WithCampaignCheck(ctx context.Context, check CampaignCheck) context.Context {
	check.Argv = append([]string(nil), check.Argv...)
	return context.WithValue(ctx, campaignCheckKey{}, check)
}

// CampaignCheckFrom returns the check ctx carries. ok is false outside a
// campaign task, and when the carried argv names no binary: a check with no
// command is not a check, and the tool fails closed without one.
func CampaignCheckFrom(ctx context.Context) (check CampaignCheck, ok bool) {
	if ctx == nil {
		return CampaignCheck{}, false
	}
	check, ok = ctx.Value(campaignCheckKey{}).(CampaignCheck)
	if !ok || len(check.Argv) == 0 || check.Argv[0] == "" {
		return CampaignCheck{}, false
	}
	check.Argv = append([]string(nil), check.Argv...)
	return check, true
}
