package chat

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Background read-only work. When the kernel derives route_background() — a
// delegation whose intent only reads (routing_arbitration.mg) — the shard runs
// off the turn: the chat answers at once and stays open, and the result comes
// back as a message, as shard facts, and as background_job/4 rows the main
// agent and every shard see while the job runs. A mutation never runs here: it
// stays in the foreground under its verification loop and the work mode.

// backgroundJob is one shard task running off the turn.
type backgroundJob struct {
	ID    string
	Shard string
	Task  string
	// Target and Constraint are the routed intent's, passed to the executor
	// as fields (intentTaskRequest); a self-diagnosis job has neither.
	Target     string
	Constraint string
	// Request is the person's words the job was started from.
	Request string
	Started time.Time
	// FollowUp, when set, is submitted as a turn of its own once the job
	// succeeds, with the result appended: the next step of a chain (a self
	// diagnosis proposes its fix). The work mode decides whether it runs.
	FollowUp string
	// Session is the chat session that started the job (startBackground);
	// its result settles there even if the user has moved to another one.
	Session string
	// RequestID is the request the job's result answers (applyAnswer): the
	// turn that started it only acknowledged it.
	RequestID uint64
}

// backgroundJobs is shared across Model copies: Update works on values, and
// a job started on one copy must be seen by the copy that renders the footer.
type backgroundJobs struct {
	mu      sync.Mutex
	running map[string]backgroundJob
	cancel  map[string]context.CancelFunc
	// stopped latches a stop the user asked for, so a job whose work had
	// already returned when the stop came still settles as stopped and does
	// not start its next step.
	stopped map[string]bool
}

func newBackgroundJobs() *backgroundJobs {
	return &backgroundJobs{running: map[string]backgroundJob{}, cancel: map[string]context.CancelFunc{}, stopped: map[string]bool{}}
}

// add records a running job and the cancel that stops it.
func (b *backgroundJobs) add(job backgroundJob, cancel context.CancelFunc) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running[job.ID] = job
	if cancel != nil {
		b.cancel[job.ID] = cancel
	}
}

// settle retires job id once. It reports whether the job was running (a
// second done message for it was not) and whether a stop was latched for it.
func (b *backgroundJobs) settle(id string) (wasRunning, stopped bool) {
	if b == nil {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if cancel, ok := b.cancel[id]; ok {
		cancel()
		delete(b.cancel, id)
	}
	_, wasRunning = b.running[id]
	stopped = b.stopped[id]
	delete(b.running, id)
	delete(b.stopped, id)
	return wasRunning, stopped
}

// cancelAll stops every running job; each still reports back, as stopped.
func (b *backgroundJobs) cancelAll() []backgroundJob {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var stopped []backgroundJob
	for id, job := range b.running {
		b.stopped[id] = true
		if cancel, ok := b.cancel[id]; ok {
			cancel()
		}
		stopped = append(stopped, job)
	}
	return stopped
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
	// Reflection is the recall the handing-off turn ran: Update publishes it
	// before starting the job, so the job's session context carries the
	// request's own hits, not whatever an earlier turn left.
	Reflection *ReflectionState
}

// backgroundDoneMsg is a finished background job.
type backgroundDoneMsg struct {
	Job     backgroundJob
	Surface string
	Result  string
	Facts   []core.Fact
	Err     error
}

// backgroundTask is the task of a read-only delegation. A /test query ("run
// the tests and tell me what fails") runs the suite: formatShardTask turns a
// /test with a named target into write_tests, which creates files, and the
// background lane starts nothing that writes (and, as a /query, is never
// held by the work mode).
func (m Model) backgroundTask(intent perception.Intent) string {
	if intent.Verb == "/test" {
		return withTaskConstraint("run_tests", intent.Constraint)
	}
	return m.formatShardTaskWithContext(intent.Verb, intent.Target, intent.Constraint, m.workspace, m.lastShardResult)
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
	parent := m.shutdownCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	job.Session = m.sessionID
	// The job's agents file under this session, tagged with the job's id, so
	// the main agent can read their progress while they run (jobProgress).
	if m.usageTracker != nil {
		ctx = usage.NewContext(ctx, m.usageTracker)
	}
	ctx = sessionlog.WithTask(usage.WithSessionID(ctx, m.sessionID), job.ID)
	m.background.add(job, cancel)
	m.assertBackgroundJob(job, "/running")
	return func() tea.Msg {
		req := shardTypeToTaskRequest(job.Shard, job.Task)
		req.Target, req.Constraint, req.Request = job.Target, job.Constraint, job.Request
		ret, err := m.runBackgroundUntilVerified(ctx, req)
		result := ret.Output
		var facts []core.Fact
		if m.shardMgr != nil {
			facts = m.shardMgr.ResultToFacts(job.ID, job.Shard, job.Task, result, err)
		}
		if err != nil {
			err = withPartialState(err, ret)
		}
		done := backgroundDoneMsg{Job: job, Result: result, Facts: facts, Err: err}
		if err == nil {
			// Reviewer and tester output is read for the user, as in the
			// foreground; the raw output stays attached.
			if interpretsOutput(job.Shard) {
				var warning string
				if job.Shard == reviewerShard {
					warning = m.checkReviewFindings(ctx, job.ID, result)
				}
				done.Surface = m.formatInterpretedResult(ctx, input, job.Shard, job.Task, result, warning)
			} else {
				done.Surface = result
			}
		}
		return done
	}
}

// runBackgroundUntilVerified runs the job's task and, while the turn's verdict
// says the work is not verified, sends it out again with what is missing
// (session.RunUntilVerified, policy/task_verdict.mg). A job used to be
// reported "finished" whatever the verdict said. It ends when the turn is
// verified, its report names an obstacle, a dispatch makes no progress, or the
// job is stopped; an end short of verified is an error naming the gap, and the
// job reports as failed.
func (m Model) runBackgroundUntilVerified(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	var k types.Kernel
	if m.kernel != nil {
		k = m.kernel
	}
	sessionCtx := m.buildSessionContext(ctx)
	return session.RunUntilVerified(ctx, k, func(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
		return m.spawnTaskWithContext(ctx, req, sessionCtx, types.PriorityNormal)
	}, req)
}

// finishBackground applies a finished job to the session that started it:
// its facts, its kernel row, the message, and the chain's next step. A
// second done message for a job is ignored. A stop the user asked for is
// honored even when the work had already returned: the result is shown and
// the next step is not started. A busy chat queues the next step instead of
// dropping it. A job started in another session settles there.
func (m Model) finishBackground(msg backgroundDoneMsg) (Model, tea.Cmd) {
	wasRunning, stopped := m.background.settle(msg.Job.ID)
	if !wasRunning {
		logging.Get(logging.CategorySession).Warn("background %s: done again after it settled; ignored", msg.Job.ID)
		return m, nil
	}
	if m.kernel != nil && len(msg.Facts) > 0 {
		if err := m.kernel.LoadFacts(msg.Facts); err != nil {
			logging.Get(logging.CategoryKernel).Warn("background %s: load result facts: %v", msg.Job.ID, err)
		}
	}
	elapsed := time.Since(msg.Job.Started).Round(time.Second)
	if msg.Err != nil {
		m.assertBackgroundJob(msg.Job, "/failed")
	} else {
		m.assertBackgroundJob(msg.Job, "/done")
	}
	report := backgroundReport(msg, stopped, elapsed)
	if msg.Job.Session != "" && msg.Job.Session != m.sessionID {
		return m.settleElsewhere(msg, stopped, report), nil
	}
	if msg.Err != nil {
		return m.pushAssistantMsg(report), nil
	}
	m.storeShardResult(msg.Job.Shard, msg.Job.Task, msg.Result, msg.Facts)
	if msg.Job.FollowUp == "" || stopped {
		return m.pushAssistantMsg(report), nil
	}
	// The chain's next step is a turn like any other: perception reads it and
	// the kernel's work admission decides whether it runs, is proposed, or
	// waits for go. A turn in flight makes it wait its turn, not vanish.
	next := msg.Job.FollowUp + "\n\n" + msg.Result
	if m.isLoading || m.turn.current() != 0 {
		m.turn.enqueue(queuedMessage{input: next})
		return m.pushAssistantMsg(report + "\n\n_Its next step is queued for when the current turn finishes._"), nil
	}
	m = m.pushAssistantMsg(report)
	m.isLoading = true
	// The next step is the chain's, not a request the person made.
	return m, m.processInput(next, 0)
}

// backgroundReport is the message for a settled job.
func backgroundReport(msg backgroundDoneMsg, stopped bool, elapsed time.Duration) string {
	job := msg.Job
	switch {
	case msg.Err != nil && (stopped || errors.Is(msg.Err, context.Canceled)):
		return fmt.Sprintf("**Stopped background %s** after %s, as you asked: %s", job.Shard, elapsed, oneLine(job.Task))
	case errors.Is(msg.Err, session.ErrNotVerified):
		// The shard's last report is what the person needs: it says what is
		// left, or what the obstacle is.
		report := fmt.Sprintf("**Background %s did not finish verified** after %s: %s\n\nTask: %s", job.Shard, elapsed, msg.Err, oneLine(job.Task))
		if last := strings.TrimSpace(msg.Result); last != "" {
			report += "\n\nIts last report:\n\n" + last
		}
		return report
	case msg.Err != nil:
		return fmt.Sprintf("**Background %s failed** after %s: %s\n\nTask: %s", job.Shard, elapsed, msg.Err, oneLine(job.Task))
	case stopped:
		report := fmt.Sprintf("**Stopped background %s** after %s, as you asked; it had already finished its work: %s\n\n%s", job.Shard, elapsed, oneLine(job.Task), msg.Surface)
		if job.FollowUp != "" {
			report += "\n\n_Its next step was not started._"
		}
		return report
	default:
		return fmt.Sprintf("**Background %s finished** (%s): %s\n\n%s", job.Shard, elapsed, oneLine(job.Task), msg.Surface)
	}
}

// settleElsewhere delivers a job's result to the session that started it,
// which is not the one open now: the report is appended to that session's
// saved history and only a notice is shown here. The open session does not
// adopt the result or the chain's next step.
func (m Model) settleElsewhere(msg backgroundDoneMsg, stopped bool, report string) Model {
	if msg.Err == nil && !stopped && msg.Job.FollowUp != "" {
		report += "\n\n_Its next step was not started: this session was not open when the job finished. Ask for it here to continue._"
	}
	if err := appendSessionMessage(m.workspace, msg.Job.Session, Message{Role: "assistant", Content: report, Time: time.Now(), Answers: msg.Job.RequestID}); err != nil {
		return m.pushAssistantMsg(fmt.Sprintf("_A background %s started in session `%s` finished, but its result could not be added to that session (%v), so here it is:_\n\n%s", msg.Job.Shard, msg.Job.Session, err, report))
	}
	return m.pushAssistantMsg(fmt.Sprintf("_The background %s started in session `%s` finished; its result was added to that session._", msg.Job.Shard, msg.Job.Session))
}

// appendSessionMessage appends one message to a saved session's history.
func appendSessionMessage(workspace, sessionID string, msg Message) error {
	history, err := session.LoadSessionHistory(workspace, sessionID)
	if err != nil {
		return err
	}
	messages := append(history.Messages, session.ChatMessage{Role: msg.Role, Content: msg.Content, Time: msg.Time, Request: msg.Request, Answers: msg.Answers})
	return session.SaveSessionHistory(workspace, sessionID, messages)
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

// jobProgress is what each running job's agents have done so far, one line per
// agent, read from the transcripts the harness writes (sessionlog.Progress).
func (m Model) jobProgress(job backgroundJob, now time.Time) []string {
	var lines []string
	for _, p := range sessionlog.Current().ProgressForTask(job.Session, job.ID) {
		lines = append(lines, p.Text(now))
	}
	return lines
}

// refreshJobProgress replaces the kernel's background_job_progress facts with
// each running job's progress now. The main agent reads them in its context
// every turn (routing_arbitration.mg), so "how is it going?" is answered from
// what the job has done, not guessed.
func (m Model) refreshJobProgress(now time.Time) {
	if m.kernel == nil {
		return
	}
	_ = m.kernel.Retract("background_job_progress")
	for _, job := range m.background.list() {
		for _, line := range m.jobProgress(job, now) {
			if err := m.kernel.Assert(core.Fact{Predicate: "background_job_progress", Args: []any{job.ID, line}}); err != nil {
				logging.Get(logging.CategoryKernel).Warn("background %s: assert progress: %v", job.ID, err)
			}
		}
	}
}
