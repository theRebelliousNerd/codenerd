package chat

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
)

// What the running work has done, as the harness measured it. A question
// about how the work is going (side_turns.mg /status) is answered with this,
// and perception reads it with every side turn (SessionContext.RunningWork),
// so no reply about the work is a guess or a promise to check on it: a side
// turn has no other way to look.
//
// The measurements:
//   - what runs and for how long: the foreground execution's label and the
//     time its cancel was registered (turnState.runningSince);
//   - what it has called: each agent's transcript progress under the
//     execution's task id (foregroundTask, sessionlog.ProgressForTask): model
//     calls, tool results and errors, the last tool, and what it waits on now;
//   - what it has written: running_work_wrote, which the kernel derives from
//     file_written rows stamped since the execution began;
//   - background jobs, the same way (jobProgress).

// foregroundTask is the sessionlog task id the agents of foreground
// execution id record their transcripts under.
func foregroundTask(id uint64) string { return fmt.Sprintf("turn-%d", id) }

// runningWorkReport renders the running work's progress at now; "" when
// nothing runs.
func (m Model) runningWorkReport(now time.Time) string {
	if m.turn != nil {
		m.turn.decide.Lock()
		defer m.turn.decide.Unlock()
	}
	return m.runningWorkReportLocked(now)
}

// runningWorkReportLocked is runningWorkReport for a caller that holds
// turnState.decide (it asserts running_work_began).
func (m Model) runningWorkReportLocked(now time.Time) string {
	var b strings.Builder
	if owner, label, began := m.turn.runningSince(); owner != 0 {
		what := label
		if what == "" {
			what = "the last message, still being read and routed"
		}
		if began.IsZero() {
			fmt.Fprintf(&b, "Running: %s; it has not started yet.\n", what)
		} else {
			fmt.Fprintf(&b, "Running: %s, for %s.\n", what, now.Sub(began).Round(time.Second))
		}
		agents := sessionlog.Current().ProgressForTask(m.sessionID, foregroundTask(owner))
		if len(agents) == 0 {
			b.WriteString("- No agent has started on it yet.\n")
		}
		for _, p := range agents {
			fmt.Fprintf(&b, "- %s\n", p.Text(now))
		}
		if !began.IsZero() {
			if wrote := m.runningWorkWrote(began); len(wrote) > 0 {
				fmt.Fprintf(&b, "Written since it began: %s.\n", strings.Join(wrote, ", "))
			} else {
				b.WriteString("Written since it began: nothing yet.\n")
			}
		}
	}
	for _, job := range m.background.list() {
		fmt.Fprintf(&b, "In the background: %s: %s, for %s.\n", job.Shard, oneLine(job.Task), now.Sub(job.Started).Round(time.Second))
		for _, line := range m.jobProgress(job, now) {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	return strings.TrimSpace(b.String())
}

// runningWorkWrote asks the kernel what has been written since began
// (running_work_wrote, side_turns.mg).
func (m Model) runningWorkWrote(began time.Time) []string {
	if m.kernel == nil {
		return nil
	}
	tx := m.kernel.Transaction()
	tx.Retract("running_work_began")
	tx.Assert(core.Fact{Predicate: "running_work_began", Args: []any{began.Unix()}})
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryKernel).Warn("running work: assert its start: %v", err)
		return nil
	}
	rows, err := m.kernel.Query("running_work_wrote")
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("running work: query what it wrote: %v", err)
		return nil
	}
	var out []string
	for _, r := range rows {
		if len(r.Args) == 1 {
			out = append(out, types.ExtractString(r.Args[0]))
		}
	}
	sort.Strings(out)
	return out
}
