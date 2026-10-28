package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Background read-only work. When the kernel derives route_background() — a
// delegation whose intent only reads (routing_arbitration.mg) — the shard runs
// off the turn: the chat answers at once and stays open, and the result comes
// back as a message, as shard facts, and as background_job/4 rows the main
// agent and every shard see while the job runs. A mutation never runs here: it
// stays in the foreground under its verification loop and the work mode.

// backgroundJob is one shard task running off the turn.
type backgroundJob struct {
	ID      string
	Shard   string
	Task    string
	Started time.Time
	// FollowUp, when set, is submitted as a turn of its own once the job
	// succeeds, with the result appended: the next step of a chain (a self
	// diagnosis proposes its fix). The work mode decides whether it runs.
	FollowUp string
}

// backgroundJobs is shared across Model copies: Update works on values, and
// a job started on one copy must be seen by the copy that renders the footer.
type backgroundJobs struct {
	mu      sync.Mutex
	running map[string]backgroundJob
}

func newBackgroundJobs() *backgroundJobs {
	return &backgroundJobs{running: map[string]backgroundJob{}}
}

func (b *backgroundJobs) add(job backgroundJob) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running[job.ID] = job
}

func (b *backgroundJobs) remove(id string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.running, id)
}

// list is the running jobs, oldest first.
func (b *backgroundJobs) list() []backgroundJob {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]backgroundJob, 0, len(b.running))
	for _, j := range b.running {
		out = append(out, j)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Started.Before(out[k].Started) })
	return out
}

// backgroundStartMsg is a turn that handed its work to the background: the
// surface is shown now, and the job's command is started by Update.
type backgroundStartMsg struct {
	Surface string
	Job     backgroundJob
}

// backgroundDoneMsg is a finished background job.
type backgroundDoneMsg struct {
	Job     backgroundJob
	Surface string
	Result  string
	Facts   []core.Fact
	Err     error
}

// backgroundStartSurface is the main agent's reply plus what went to the
// background, so the user knows the conversation is open.
func backgroundStartSurface(response string, job backgroundJob) string {
	var b strings.Builder
	if r := strings.TrimSpace(response); r != "" {
		b.WriteString(r)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "_The %s shard is working on this in the background: %s_\n\nKeep talking; its result will appear here.", job.Shard, oneLine(job.Task))
	return b.String()
}

// startBackground records the job, marks it in the kernel, and returns the
// command that runs it.
func (m Model) startBackground(job backgroundJob, input string) tea.Cmd {
	m.background.add(job)
	m.assertBackgroundJob(job, "/running")
	return func() tea.Msg {
		ctx := m.shutdownCtx
		if ctx == nil {
			ctx = context.Background()
		}
		ret, err := m.spawnTaskWithContext(ctx, job.Shard, job.Task, m.buildSessionContext(ctx), types.PriorityNormal)
		result := ret.Output
		var facts []core.Fact
		if m.shardMgr != nil {
			facts = m.shardMgr.ResultToFacts(job.ID, job.Shard, job.Task, result, err)
		}
		done := backgroundDoneMsg{Job: job, Result: result, Facts: facts, Err: err}
		if err == nil {
			// Reviewer and tester output is read for the user, as in the
			// foreground; the raw output stays attached.
			if interpretsOutput(job.Shard) {
				done.Surface = m.formatInterpretedResult(ctx, input, job.Shard, job.Task, result, "")
			} else {
				done.Surface = result
			}
		}
		return done
	}
}

// finishBackground applies a finished job to the session: its facts, its
// kernel row, the message, and the chain's next step.
func (m Model) finishBackground(msg backgroundDoneMsg) (Model, tea.Cmd) {
	m.background.remove(msg.Job.ID)
	if m.kernel != nil && len(msg.Facts) > 0 {
		if err := m.kernel.LoadFacts(msg.Facts); err != nil {
			logging.Get(logging.CategoryKernel).Warn("background %s: load result facts: %v", msg.Job.ID, err)
		}
	}
	elapsed := time.Since(msg.Job.Started).Round(time.Second)
	if msg.Err != nil {
		m.assertBackgroundJob(msg.Job, "/failed")
		return m.pushAssistantMsg(fmt.Sprintf("**Background %s failed** after %s: %s\n\nTask: %s", msg.Job.Shard, elapsed, msg.Err, oneLine(msg.Job.Task))), nil
	}
	m.assertBackgroundJob(msg.Job, "/done")
	m.storeShardResult(msg.Job.Shard, msg.Job.Task, msg.Result, msg.Facts)
	m = m.pushAssistantMsg(fmt.Sprintf("**Background %s finished** (%s): %s\n\n%s", msg.Job.Shard, elapsed, oneLine(msg.Job.Task), msg.Surface))
	if msg.Job.FollowUp == "" || m.isLoading {
		return m, nil
	}
	// The chain's next step is a turn like any other: perception reads it and
	// the kernel's work admission decides whether it runs, is proposed, or
	// waits for go.
	m.isLoading = true
	return m, m.processInput(msg.Job.FollowUp + "\n\n" + msg.Result)
}

// assertBackgroundJob replaces the job's background_job row with its status.
func (m Model) assertBackgroundJob(job backgroundJob, status string) {
	if m.kernel == nil {
		return
	}
	_ = m.kernel.RetractFact(core.Fact{Predicate: "background_job", Args: []any{job.ID}})
	if err := m.kernel.Assert(core.Fact{Predicate: "background_job", Args: []any{
		job.ID, job.Shard, job.Task, types.MangleAtom(status),
	}}); err != nil {
		logging.Get(logging.CategoryKernel).Warn("background %s: assert background_job: %v", job.ID, err)
	}
}

// backgroundFooter names the running jobs for the footer; empty when none.
func (m Model) backgroundFooter() string {
	jobs := m.background.list()
	if len(jobs) == 0 {
		return ""
	}
	names := make([]string, 0, len(jobs))
	for _, j := range jobs {
		names = append(names, j.Shard)
	}
	return "bg: " + strings.Join(names, ", ")
}

// interpretsOutput reports a shard whose findings are read for the user before
// they are shown (formatInterpretedResult), in the foreground or not.
func interpretsOutput(shard string) bool {
	return shard == "reviewer" || shard == "tester"
}
