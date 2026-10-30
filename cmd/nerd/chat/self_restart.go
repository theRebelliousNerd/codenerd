package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		exe, err := os.Executable()
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("self-restart off: executable path unknown: %v", err)
			return rebuildWatchMsg{}
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
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
	m = m.pushAssistantMsg(fmt.Sprintf("A rebuilt codeNERD (%s) is on disk; restarting onto it when the session is idle, keeping this conversation.", msg.id.Build))
	return m, tea.Batch(listen, restartCheckTick())
}

// handleRestartCheck restarts when the session is idle, and asks again later
// when it is not.
func (m Model) handleRestartCheck() (Model, tea.Cmd) {
	if m.restarter == nil || m.restartFailed {
		return m, nil
	}
	if !m.restarter.ShouldRestart(m.restartBusy()) {
		return m, restartCheckTick()
	}
	st := m.restartState()
	m.restartRequest = &st
	return m, tea.Quit
}

// restartBusy reports work a restart must not cut: a turn in progress, the
// boot, or a person deciding an approval.
func (m Model) restartBusy() bool {
	return m.isLoading || m.isBooting || m.viewMode == ApprovalView
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
	for _, job := range m.background.list() {
		st.Interrupted = append(st.Interrupted, fmt.Sprintf("background job %s (%s: %s)", job.ID, job.Shard, job.Task))
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
	err := m.restarter.Restart(st, os.Args)
	if err != nil {
		m.restartFailed = true
		m = m.pushAssistantMsg(fmt.Sprintf("**Restart failed:** %v", err))
		return m, err
	}
	return m, nil
}

// applyRestore resumes the session a restart handed over: its conversation,
// work mode, brief, held work and draft, with a notice naming the builds and
// what the restart interrupted. It runs once the boot (resumed under the same
// session id) has wired the kernel.
func (m Model) applyRestore() Model {
	st := m.restore
	m.restore = nil
	if st == nil {
		return m
	}
	if history, err := session.LoadSessionHistory(m.workspace, st.SessionID); err == nil && len(history.Messages) > 0 {
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
	notice := fmt.Sprintf("codeNERD updated (%s → %s build), session restored.", st.OldBuild, st.NewBuild)
	if len(st.Interrupted) > 0 {
		notice += "\n\nInterrupted by the restart:\n- " + strings.Join(st.Interrupted, "\n- ")
	}
	if len(st.Queued) > 0 {
		notice += "\n\nHeld work carried over: " + st.Queued[0]
	}
	return m.pushAssistantMsg(notice)
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
		m = m.pushAssistantMsg(fmt.Sprintf("**Policy reload failed** (%s); the running policy is unchanged:\n\n```\n%v\n```", files, msg.ev.Err))
	} else {
		m = m.pushAssistantMsg(fmt.Sprintf("Policy reloaded from the source tree: %s is live.", files))
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
