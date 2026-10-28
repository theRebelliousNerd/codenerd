package chat

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/approval"
	"codenerd/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// The chat TUI is the human-approval host (Docs/audits/request-bound-human-
// approval.md). At boot it attaches the one trusted decision channel to the
// Cortex's broker; while any request awaits the person, the approval view is
// modal and every y/n/a key becomes a decision the host commits. Nothing a
// model, a tool or a file produces reaches Decide: the only caller is the key
// path below, through ui.ApprovalDecisionMsg, which only DiffApprovalView
// emits.

// approvalUpdateMsg is one broker transition of a request offered to this
// host.
type approvalUpdateMsg approval.Request

// approvalActor names the person deciding: the OS account running the TUI.
func approvalActor() string {
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return u.Username
	}
	return "local-operator"
}

// attachApprovalHost makes this TUI the broker's decision channel. A nil
// broker (no journal could be opened) leaves approvable actions refused.
func (m *Model) attachApprovalHost(b *approval.Broker) error {
	if b == nil {
		return nil
	}
	channel := fmt.Sprintf("tui:%s:%d", m.sessionID, os.Getpid())
	host, err := b.AttachHost(channel, approvalActor())
	if err != nil {
		return err
	}
	m.approvalHost = host
	m.approvalView = ui.NewDiffApprovalView(m.styles, m.width, m.height)
	return nil
}

// detachApprovalHost releases the channel; requests still awaiting it are
// canceled by the broker, so no call stays blocked on a closed TUI.
func (m *Model) detachApprovalHost() {
	if m.approvalHost != nil {
		m.approvalHost.Detach()
		m.approvalHost = nil
	}
}

// listenApprovals waits for the next transition of a request offered to this
// host. Re-armed after every update, like listenToolEvents.
func (m Model) listenApprovals() tea.Cmd {
	if m.approvalHost == nil {
		return nil
	}
	updates := m.approvalHost.Updates()
	return func() tea.Msg {
		r, ok := <-updates
		if !ok {
			return nil
		}
		return approvalUpdateMsg(r)
	}
}

// handleApprovalUpdate resyncs the view with the host's pending set, opens
// the approval view while anything is pending and closes it when nothing is.
func (m Model) handleApprovalUpdate(r approval.Request) (Model, tea.Cmd) {
	switch r.Status {
	case approval.StatusExpired, approval.StatusCanceled, approval.StatusSuperseded:
		m = m.addMessage(Message{Role: "system", Time: time.Now(),
			Content: fmt.Sprintf("🔐 Approval request %s (%s) %s: %s", r.ID, r.Preview.Summary, r.Status, r.StatusReason)})
	case approval.StatusCompleted, approval.StatusOutcomeUnknown, approval.StatusAdmissionFailed:
		m = m.addMessage(Message{Role: "system", Time: time.Now(),
			Content: fmt.Sprintf("🔐 Approved request %s (%s) %s: %s", r.ID, r.Preview.Summary, r.Status, r.StatusReason)})
	}
	m = m.syncApprovalView()
	return m, m.listenApprovals()
}

func (m Model) syncApprovalView() Model {
	if m.approvalHost == nil {
		return m
	}
	pending := m.approvalHost.Pending()
	m.approvalView.SyncRequests(pending)
	switch {
	case len(pending) > 0:
		m.viewMode = ApprovalView
	case m.viewMode == ApprovalView:
		m.viewMode = ChatView
	}
	return m
}

// handleApprovalDecisions commits the person's decisions through the host and
// reports each outcome. A refusal (expired, revised, already decided) is
// shown, never retried.
func (m Model) handleApprovalDecisions(msg ui.ApprovalDecisionMsg) Model {
	if m.approvalHost == nil {
		return m
	}
	for _, in := range msg.Decisions {
		r, err := m.approvalHost.Decide(in)
		var content string
		switch {
		case err == nil:
			content = fmt.Sprintf("🔐 %s: %s (request %s)", verdictWord(in.Verdict), r.Preview.Summary, r.ID)
		case errors.Is(err, approval.ErrExpired), errors.Is(err, approval.ErrNotDecidable), errors.Is(err, approval.ErrDecisionMismatch):
			content = fmt.Sprintf("🔐 Decision on request %s refused: %v", in.RequestID, err)
		default:
			content = fmt.Sprintf("🔐 Decision on request %s failed: %v", in.RequestID, err)
			logging.Get(logging.CategorySession).Warn("approval decision failed: %v", err)
		}
		m = m.addMessage(Message{Role: "system", Content: content, Time: time.Now()})
	}
	return m.syncApprovalView()
}

func verdictWord(v approval.Verdict) string {
	switch v {
	case approval.VerdictApprove:
		return "Approved"
	case approval.VerdictReject:
		return "Rejected"
	default:
		return "Canceled"
	}
}

// handleApprovalKey routes keys to the modal approval view.
func (m Model) handleApprovalKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.approvalView, cmd = m.approvalView.Update(msg)
	return m, cmd
}

func (m Model) renderApprovalView() string {
	return m.styles.Layout.Content.Render(m.approvalView.View())
}
