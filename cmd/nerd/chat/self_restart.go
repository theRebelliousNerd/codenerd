package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/campaign"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/selfrestart"
	"codenerd/internal/session"
	nerdsystem "codenerd/internal/system"

	tea "github.com/charmbracelet/bubbletea"
)

// Self-restart onto a rebuilt binary (internal/selfrestart). The chat watches
// its own executable; when a rebuild settles it says so, and once the session
// is idle -- never during a turn -- it quits the TUI (which restores the
// terminal) and RunInteractiveChat writes the session's state and execs the
// new build in place, which reads the state back (--restore-session) and
// resumes the conversation. An exec that fails reopens this TUI on the same
// session and says why.

// binaryRebuiltMsg is a settled rebuild of the executable.
type binaryRebuiltMsg struct{ id selfrestart.Identity }

// restartCheckMsg asks whether the session is idle enough to restart.
type restartCheckMsg struct{}

// restartCheckInterval is how often a pending restart asks for idleness.
const restartCheckInterval = time.Second

// rebuildWatchMsg is the started watch: its restarter and channel, for the
// Update thread to keep and listen on. Nil fields mean self-restart is off.
type rebuildWatchMsg struct {
	restarter *selfrestart.Restarter
	rebuilds  <-chan selfrestart.Identity
}

// startRebuildWatch starts watching the executable for rebuilds, off the
// Update thread; its result comes back as rebuildWatchMsg.
func (m Model) startRebuildWatch() tea.Cmd {
	ctx, workspace := m.shutdownCtx, m.workspace
	return func() tea.Msg {
		exe, err := selfrestart.ExecutablePath()
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("self-restart off: executable path unknown: %v", err)
			return rebuildWatchMsg{}
		}
		current, err := selfrestart.IdentityOf(exe)
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("self-restart off: %v", err)
			return rebuildWatchMsg{}
		}
		ch := make(chan selfrestart.Identity, 4)
		if err := selfrestart.Watch(ctx, exe, func(id selfrestart.Identity) {
			select {
			case ch <- id:
			default:
			}
		}); err != nil {
			logging.Get(logging.CategoryBoot).Warn("self-restart off: cannot watch %s: %v", exe, err)
			return rebuildWatchMsg{}
		}
		return rebuildWatchMsg{
			restarter: selfrestart.NewRestarter(current, filepath.Join(workspace, ".nerd", "restart"), nil),
			rebuilds:  ch,
		}
	}
}

// handleRebuildWatch keeps the started watch and listens for rebuilds.
func (m Model) handleRebuildWatch(msg rebuildWatchMsg) (Model, tea.Cmd) {
	if msg.restarter == nil {
		return m, nil
	}
	m.restarter, m.rebuilds = msg.restarter, msg.rebuilds
	return m, listenRebuild(m.rebuilds)
}

func listenRebuild(ch <-chan selfrestart.Identity) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg { return binaryRebuiltMsg{id: <-ch} }
}

func restartCheckTick() tea.Cmd {
	return tea.Tick(restartCheckInterval, func(time.Time) tea.Msg { return restartCheckMsg{} })
}

// handleBinaryRebuilt records a rebuild and starts asking for idleness.
func (m Model) handleBinaryRebuilt(msg binaryRebuiltMsg) (Model, tea.Cmd) {
	listen := listenRebuild(m.rebuilds)
	if m.restarter == nil || !m.restarter.BinaryChanged(msg.id) {
		return m, listen
	}
	if !selfrestart.ExecSupported {
		return m.pushAssistantMsg(fmt.Sprintf("A rebuilt codeNERD (%s) is on disk. This platform cannot restart in place: quit and start `nerd` again to use it.", msg.id.Build)), listen
	}
	if m.restartFailed != nil && !m.restartFailed.Changed(msg.id) {
		return m, listen
	}
	m = m.prepareRecurseForRestart()
	if m.recurse != nil {
		m = m.pushAssistantMsg(fmt.Sprintf("A rebuilt codeNERD (%s) is on disk; pausing `/recurse` after the current attempt to restart onto it and auto-resume.", msg.id.Build))
	} else {
		m = m.pushAssistantMsg(fmt.Sprintf("A rebuilt codeNERD (%s) is on disk; restarting onto it when the session is idle, keeping this conversation.", msg.id.Build))
	}
	return m, tea.Batch(listen, restartCheckTick())
}

// prepareRecurseForRestart asks a running /recurse loop to stop cleanly at the
// next attempt boundary so the new binary can take over and auto-resume from
// the journal.
func (m Model) prepareRecurseForRestart() Model {
	if m.recurse == nil || m.recurse.stoppingForRestart {
		return m
	}
	m.recurse.stoppingForRestart = true
	m.resumeRecurse = true
	m.resumeRecurseArgs = slices.Clone(m.recurse.args)
	if err := campaign.RequestRecurseStop(m.workspace); err != nil {
		logging.Get(logging.CategorySession).Warn("self-restart: could not request /recurse boundary stop: %v", err)
	}
	return m
}

// hasPendingRestart reports whether a new binary build is waiting to replace
// this process once it becomes idle.
func (m Model) hasPendingRestart() bool {
	if m.restarter == nil {
		return false
	}
	next, pending := m.restarter.Pending()
	return pending && (m.restartFailed == nil || m.restartFailed.Changed(next))
}

// handleRestartCheck restarts when the session is idle, and asks again later
// when it is not.
func (m Model) handleRestartCheck() (Model, tea.Cmd) {
	if !m.hasPendingRestart() {
		return m, nil
	}
	m = m.prepareRecurseForRestart()
	if !m.restarter.ShouldRestart(m.restartBusy()) {
		return m, restartCheckTick()
	}
	st := m.restartState()
	m.restartRequest = &st
	return m, tea.Quit
}

// restartBusy reports work a restart must not cut: a turn in progress, the
// boot, a person deciding an approval, or an active /recurse or campaign run
// that has not yet settled at its boundary.
func (m Model) restartBusy() bool {
	return m.isLoading || m.isBooting || m.viewMode == ApprovalView || m.recurse != nil || m.campaignOrch != nil
}

// restartState is what the new process restores.
func (m Model) restartState() selfrestart.State {
	st := selfrestart.State{
		SessionID: m.sessionID,
		WorkMode:  string(m.workMode()),
		Draft:     m.textarea.Value(),
	}
	if b := m.currentBrief(); b != nil {
		if data, err := json.Marshal(b); err == nil {
			st.Brief = string(data)
		}
	}
	if m.pendingWork != nil && strings.TrimSpace(m.pendingWork.Input) != "" {
		st.Queued = append(st.Queued, m.pendingWork.Input)
	}
	if m.resumeRecurse {
		st.Recurse = true
		st.RecurseArgs = slices.Clone(m.resumeRecurseArgs)
	} else if m.recurse != nil {
		st.Recurse = true
		st.RecurseArgs = slices.Clone(m.recurse.args)
	}
	// What the restart cuts and the new process starts again by itself: the
	// background jobs (a self-repair diagnosis among them) and the queued
	// turns (a self-repair fix turn among them).
	for _, job := range m.background.list() {
		st.Resume = append(st.Resume, selfrestart.ResumeWork{
			Kind: selfrestart.ResumeBackground, ID: job.ID, Shard: job.Shard, Task: job.Task,
			Target: job.Target, Constraint: job.Constraint, Request: job.Request,
			FollowUp: job.FollowUp, SelfRepair: job.SelfRepair,
		})
	}
	for _, q := range m.turn.queuedWork() {
		w := selfrestart.ResumeWork{Kind: selfrestart.ResumeTurn, Input: q.input, Command: q.command}
		if q.chain != nil {
			w.Step = &selfrestart.ChainStep{Instruction: q.chain.Instruction, Evidence: q.chain.Evidence,
				Verb: q.chain.Verb, Target: q.chain.Target, Autonomous: q.chain.Autonomous, Job: q.chain.Job}
		}
		st.Resume = append(st.Resume, w)
	}
	if m.activeCampaign != nil {
		st.Interrupted = append(st.Interrupted, fmt.Sprintf("campaign %s (%s); its progress is saved and it resumes through the campaign resume path", m.activeCampaign.ID, m.activeCampaign.Title))
	}
	if m.offeredSubtask != nil {
		st.Interrupted = append(st.Interrupted, "the offered next step: "+m.offeredSubtask.Description)
	}
	return st
}

// RunInteractiveChat starts the interactive chat session, and restarts it
// onto a rebuilt binary when the session asked to.
func RunInteractiveChat(cfg Config) error {
	model := InitChat(cfg)
	return runChatProgram(model, nil)
}

// runChatProgram runs the TUI until it quits. A quit carrying a restart
// request saves the session and execs the rebuilt binary (exec nil: the
// platform's); an exec that fails reopens the TUI on the same session with
// the reason.
func runChatProgram(model Model, exec selfrestart.ExecFunc) error {
	for {
		final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		fm, ok := final.(Model)
		if !ok || fm.restartRequest == nil {
			return nil
		}
		var restartErr error
		fm, restartErr = fm.performRestart(exec)
		if restartErr == nil {
			return nil // only an injected exec returns
		}
		model = fm
	}
}

// performRestart saves the session and execs the pending build. It returns
// only when the exec failed (or an injected exec returned): the model then
// carries the reason and no longer tries until the next rebuild.
func (m Model) performRestart(exec selfrestart.ExecFunc) (Model, error) {
	st := *m.restartRequest
	m.restartRequest = nil
	if err := m.saveSessionState(); err != nil {
		logging.Get(logging.CategorySession).Warn("self-restart: session save before exec: %v", err)
	}
	if exec != nil {
		m.restarter.SetExec(exec)
	}
	next, _ := m.restarter.Pending()
	err := m.restarter.Restart(st, os.Args)
	if err != nil {
		m.restartFailed = &next
		m = m.pushAssistantMsg(fmt.Sprintf("**Restart failed:** %v", err))
		return m, err
	}
	return m, nil
}

// applyRestore resumes the session a restart handed over: its conversation,
// work mode, brief, held work and draft, with a notice naming the builds and
// what the restart interrupted. It runs once the boot (resumed under the same
// session id) has wired the kernel; history is that session's conversation,
// which the boot command read off the Update thread (performSystemBoot), and
// err why it could not be.
func (m Model) applyRestore(history *session.SessionHistory, err error) Model {
	st := m.restore
	m.restore = nil
	if st == nil {
		return m
	}
	if history == nil && err == nil {
		err = fmt.Errorf("the boot read no conversation for it")
	}
	if err == nil && len(history.Messages) > 0 {
		restored := make([]Message, 0, len(history.Messages))
		for _, msg := range history.Messages {
			restored = append(restored, Message{Role: msg.Role, Content: msg.Content, Time: msg.Time, Request: msg.Request, Answers: msg.Answers})
		}
		m.history = append(restored, m.history...)
		m.turn.seenRequests(maxRequest(m.history))
		m.turnCount = len(history.Messages) / 2
		// Every message moved: the cached renders are keyed by index.
		m.renderedCache = make(map[int]string)
		m.cacheInvalidFrom = 0
	} else if err != nil {
		logging.Get(logging.CategorySession).Warn("self-restart: conversation %s not restored: %v", st.SessionID, err)
	}
	if st.WorkMode == "yolo" && m.workMode() != "yolo" {
		m.yoloFlag = true // the session's yolo, as the flag gives it
		m.syncWorkModeFacts()
	}
	if st.Brief != "" {
		var b perception.Brief
		if err := json.Unmarshal([]byte(st.Brief), &b); err == nil {
			m.writeBrief(&b)
		}
	}
	if len(st.Queued) > 0 {
		m.pendingWork = &pendingWork{Input: st.Queued[0], At: time.Now()}
	}
	if st.Draft != "" {
		m.textarea.SetValue(st.Draft)
	}
	if st.Recurse {
		m.resumeRecurse = true
		m.resumeRecurseArgs = slices.Clone(st.RecurseArgs)
	}
	m.resumeWork = slices.Clone(st.Resume)
	notice := fmt.Sprintf("codeNERD updated (%s → %s build), session restored.", st.OldBuild, st.NewBuild)
	if len(st.Resume) > 0 {
		lines := make([]string, 0, len(st.Resume))
		for _, w := range st.Resume {
			if w.Kind == selfrestart.ResumeBackground {
				lines = append(lines, fmt.Sprintf("background %s job %s: %s", w.Shard, w.ID, oneLine(w.Task)))
			} else {
				lines = append(lines, "queued turn: "+oneLine(w.Input))
			}
		}
		notice += "\n\nResuming once the workspace scan completes:\n- " + strings.Join(lines, "\n- ")
	}
	if len(st.Interrupted) > 0 {
		notice += "\n\nInterrupted by the restart:\n- " + strings.Join(st.Interrupted, "\n- ")
	}
	if len(st.Queued) > 0 {
		notice += "\n\nHeld work carried over: " + st.Queued[0]
	}
	if st.Recurse {
		notice += "\n\nResuming `/recurse` once workspace scan completes."
	}
	return m.pushAssistantMsg(notice)
}

// shouldRebuildWatchedExecutable reports whether a verified turn or kept
// /recurse cycle modified executable source (.go excluding *_test.go, or .mg)
// in the workspace that owns the watched binary.
func (m Model) shouldRebuildWatchedExecutable(changed []string) bool {
	if m.restarter == nil || strings.TrimSpace(m.workspace) == "" || !hasExecutableSourceChanges(changed) {
		return false
	}
	exe := selfrestart.CleanExecutablePath(m.restarter.CurrentPath())
	if exe == "" {
		return false
	}
	wsClean := filepath.Clean(m.workspace)
	rel, err := filepath.Rel(wsClean, filepath.Clean(exe))
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return false
	}
	info, err := os.Stat(filepath.Join(wsClean, "cmd", "nerd"))
	return err == nil && info.IsDir()
}

func hasExecutableSourceChanges(changed []string) bool {
	for _, p := range changed {
		clean := strings.ToLower(strings.TrimSpace(p))
		switch {
		case strings.HasSuffix(clean, "_test.go"):
			continue
		case strings.HasSuffix(clean, ".go"), strings.HasSuffix(clean, ".mg"):
			return true
		}
	}
	return false
}

// resumeRestoredWork starts again what the restart cut (applyRestore kept
// it): each background job under its own id, and each queued turn back on
// the queue, which runs it once nothing is in flight. It runs when the
// startup scan is done, as a resumed /recurse does.
func (m Model) resumeRestoredWork() (Model, tea.Cmd) {
	work := m.resumeWork
	m.resumeWork = nil
	var cmds []tea.Cmd
	for _, w := range work {
		switch w.Kind {
		case selfrestart.ResumeBackground:
			job := backgroundJob{
				ID: w.ID, Shard: w.Shard, Task: w.Task, Target: w.Target, Constraint: w.Constraint,
				Request: w.Request, FollowUp: w.FollowUp, SelfRepair: w.SelfRepair, Started: time.Now(),
			}
			cmds = append(cmds, m.startBackground(job, job.Task))
		case selfrestart.ResumeTurn:
			q := queuedMessage{input: w.Input, command: w.Command}
			if s := w.Step; s != nil {
				q.chain = &chainStep{Instruction: s.Instruction, Evidence: s.Evidence, Verb: s.Verb, Target: s.Target, Autonomous: s.Autonomous, Job: s.Job}
			}
			m.turn.enqueue(q)
		default:
			logging.Get(logging.CategorySession).Warn("self-restart: resumed work of unknown kind %q not started: %+v", w.Kind, w)
		}
	}
	return m, tea.Batch(cmds...)
}

// queuedWork is the messages waiting as work, in order: what a restart
// carries over.
func (t *turnState) queuedWork() []queuedMessage {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []queuedMessage
	for _, q := range t.queued {
		if !q.pending {
			out = append(out, q)
		}
	}
	return out
}

// selfRebuildHooks build and self-check a rebuilt binary; nil fields (or a
// nil hooks value) are the production ones. Tests inject them.
type selfRebuildHooks struct {
	Build func(ctx context.Context, workspace, out string) error
	Check func(ctx context.Context, workspace, binary string, changed []string) error
}

// rebuildWatchedExecutable compiles ./cmd/nerd into a sibling temp file,
// proves the build (selfCheckBinary), and only then renames it over the
// watched executable, so the self-restart watcher picks up a build that runs
// -- without ETXTBSY or partial reads. A broken build never replaces a working
// one: the failure is logged as an ERROR, which the problems log makes a
// self_problem a self-repair can take up, and the running binary stays. A
// rebuild runs only after a verified turn changed source, so a failure does
// not loop: the next try is the next fix.
func (m Model) rebuildWatchedExecutable(ctx context.Context, changed []string) error {
	if !m.shouldRebuildWatchedExecutable(changed) {
		return nil
	}
	err := m.rebuildAndInstall(ctx, changed)
	if err != nil {
		logging.Get(logging.CategoryBuild).Error("self-rebuild of %s failed; the running binary is unchanged: %v", build.SelfBinaryPackage, err)
		m.selfRepairRebuilt("", err)
		return err
	}
	return nil
}

func (m Model) rebuildAndInstall(ctx context.Context, changed []string) error {
	exe := selfrestart.CleanExecutablePath(m.restarter.CurrentPath())
	tmp := fmt.Sprintf("%s.rebuild-%d.tmp", exe, os.Getpid())
	defer func() { _ = os.Remove(tmp) }()

	buildFn, check := defaultSelfBuild, m.selfCheckBinary
	if h := m.rebuildHooks; h != nil {
		if h.Build != nil {
			buildFn = h.Build
		}
		if h.Check != nil {
			check = h.Check
		}
	}
	if err := buildFn(ctx, m.workspace, tmp); err != nil {
		return fmt.Errorf("go build %s: %w", build.SelfBinaryPackage, err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return fmt.Errorf("chmod rebuilt binary: %w", err)
	}
	if err := check(ctx, m.workspace, tmp, changed); err != nil {
		return fmt.Errorf("self-check of the rebuilt binary: %w", err)
	}
	if err := os.Rename(tmp, exe); err != nil {
		return fmt.Errorf("install rebuilt binary over %s: %w", exe, err)
	}
	id, err := selfrestart.IdentityOf(exe)
	if err == nil {
		m.restarter.BinaryChanged(id)
	}
	m.selfRepairRebuilt(id.Build, nil)
	logging.Get(logging.CategoryBoot).Info("self-rebuild: built, self-checked and installed %s after source changes", exe)
	return nil
}

func defaultSelfBuild(ctx context.Context, workspace, out string) error {
	output, err := build.BuildBinary(ctx, workspace, out, build.SelfBinaryPackage)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// selfCheckBinary proves a rebuilt binary before it replaces the running one,
// each step under logging.self_repair_check_timeout_sec: it starts (--help),
// its engine loads every changed .mg file (check-mangle), and go vet passes
// on every changed package. The proof is cheap and needs no model.
func (m Model) selfCheckBinary(ctx context.Context, workspace, binary string, changed []string) error {
	timeout := m.Config.Logging.SelfRepairCheckTimeout()
	run := func(name string, argv []string, env []string) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, argv...)
		cmd.Dir = workspace
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("`%s %s` failed: %w\n%s", filepath.Base(name), strings.Join(argv, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	env := build.AllowedEnv(workspace)
	if err := run(binary, []string{"--help"}, env); err != nil {
		return err
	}
	mg, pkgs := changedSources(workspace, changed)
	if len(mg) > 0 {
		if err := run(binary, append([]string{"check-mangle"}, mg...), env); err != nil {
			return err
		}
	}
	if len(pkgs) > 0 {
		goEnv, argv := build.GoInvocation(workspace, workspace, append(append([]string{"vet"}, build.TestTagsForWorkspace(workspace)...), pkgs...))
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", argv...)
		cmd.Dir = workspace
		cmd.Env = goEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("`go vet %s` failed: %w\n%s", strings.Join(pkgs, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// changedSources splits changed paths into the workspace-relative .mg files
// that still exist and the ./-relative packages of the non-test .go files
// that still exist, each sorted and unique.
func changedSources(workspace string, changed []string) (mg, pkgs []string) {
	seenMG, seenPkg := map[string]bool{}, map[string]bool{}
	for _, p := range changed {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, p)
		}
		rel, err := filepath.Rel(workspace, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			continue // deleted: the build itself covered it
		}
		rel = filepath.ToSlash(rel)
		switch {
		case strings.HasSuffix(rel, ".mg") && !seenMG[rel]:
			seenMG[rel] = true
			mg = append(mg, rel)
		case strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go"):
			pkg := "./" + filepath.ToSlash(filepath.Dir(rel))
			if !seenPkg[pkg] {
				seenPkg[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
	}
	sort.Strings(mg)
	sort.Strings(pkgs)
	return mg, pkgs
}

// readRestore reads the restore file the launch named, once.
func readRestore(path string) (*selfrestart.State, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	st, err := selfrestart.ReadState(path)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// policyReloadMsg is one dev-mode policy hot reload's outcome.
type policyReloadMsg struct{ ev nerdsystem.PolicyReloadEvent }

func listenPolicyReloads(ch <-chan nerdsystem.PolicyReloadEvent) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return policyReloadMsg{ev: ev}
	}
}

// handlePolicyReload shows a dev-mode reload: live, or failed with the old
// program kept.
func (m Model) handlePolicyReload(msg policyReloadMsg) (Model, tea.Cmd) {
	files := strings.Join(msg.ev.Files, ", ")
	if msg.ev.Err != nil {
		disposition := ""
		if msg.ev.Reload.Disposition == "rejected" {
			disposition = "; the running policy is unchanged"
		}
		m = m.pushAssistantMsg(fmt.Sprintf("**Policy reload failed** (%s)%s (stage %s, candidate %s, active %s):\n\n```\n%v\n```", files, disposition, msg.ev.Reload.Stage, msg.ev.Reload.Candidate, msg.ev.Reload.Active, msg.ev.Err))
	} else {
		m = m.pushAssistantMsg(fmt.Sprintf("Policy reloaded from the source tree: %s is live (generation %s).", files, msg.ev.Reload.Active))
	}
	var ch <-chan nerdsystem.PolicyReloadEvent
	if m.cortex != nil {
		ch = m.cortex.PolicyReloads()
	}
	return m, listenPolicyReloads(ch)
}

// restoreSessionID is the session a restart hands over, or "" for a fresh one.
func (m Model) restoreSessionID() string {
	if m.restore == nil {
		return ""
	}
	return m.restore.SessionID
}
