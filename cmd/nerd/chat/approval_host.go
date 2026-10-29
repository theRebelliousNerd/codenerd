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
	m.approvals = b
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
		// A request the person already stepped aside from does not pull them
		// back; one they have not seen does.
		for _, r := range pending {
			if !m.approvalAside[r.ID] {
				m.approvalAside = nil
				m.viewMode = ApprovalView
				break
			}
		}
	case m.viewMode == ApprovalView:
		m.viewMode = ChatView
		m.approvalAside = nil
	default:
		m.approvalAside = nil
	}
	return m
}

// stepAsideFromApproval closes the modal review without deciding. The requests
// keep waiting (their worker stays blocked until they are decided, expire or
// the session closes), so this says so and how to come back.
func (m Model) stepAsideFromApproval() Model {
	pending := m.approvalHost.Pending()
	aside := make(map[string]bool, len(pending))
	for _, r := range pending {
		aside[r.ID] = true
	}
	m.approvalAside = aside
	m.viewMode = ChatView
	return m.addMessage(Message{Role: "system", Time: time.Now(),
		Content: fmt.Sprintf("🔐 %d approval request(s) still wait for your decision and nothing runs until you decide or they expire. `/approvals review` reopens the review.", len(pending))})
}

// reopenApprovalReview returns to the modal for every pending request.
func (m Model) reopenApprovalReview() (Model, bool) {
	if m.approvalHost == nil || len(m.approvalHost.Pending()) == 0 {
		return m, false
	}
	m.approvalAside = nil
	m.approvalView.SyncRequests(m.approvalHost.Pending())
	m.viewMode = ApprovalView
	return m, true
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
	if k := msg.String(); (k == "q" || k == "esc") && m.approvalHost != nil {
		return m.stepAsideFromApproval(), nil
	}
	var cmd tea.Cmd
	m.approvalView, cmd = m.approvalView.Update(msg)
	return m, cmd
}

func (m Model) renderApprovalView() string {
	return m.styles.Layout.Content.Render(m.approvalView.View())
}

// handleCmdApprovals is the operator's read-only view of the approval record:
// every request the journal retains (approval.journal_retention bounds it) and
// its lifecycle state, or one request in full. It decides nothing; decisions
// are only the review view's keys.
func (m Model) handleCmdApprovals(_ string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) > 1 && parts[1] == "review" {
		if reopened, ok := m.reopenApprovalReview(); ok {
			return reopened, nil
		}
		return m.pushAssistantMsg("No approval request waits for a decision."), nil
	}
	return m.pushAssistantMsg(formatApprovalStatus(m.approvals, parts[1:])), nil
}

func formatApprovalStatus(b *approval.Broker, args []string) string {
	if b == nil {
		return "Human approval is unavailable this session: no approval journal could be opened, so approvable actions are refused."
	}
	if len(args) > 0 {
		r, ok := b.Get(strings.TrimSpace(args[0]))
		if !ok {
			return fmt.Sprintf("No approval request %q.", args[0])
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "**%s** %s\n\n", r.ID, r.Status)
		fmt.Fprintf(&sb, "- action: `%s` on `%s` via %s/%s\n", r.Subject.Action, r.Subject.Target, r.Subject.Backend, r.Subject.Tool)
		fmt.Fprintf(&sb, "- digest: `%s`\n- created %s, expires %s\n", r.Digest, r.CreatedAt.Format(time.RFC3339), r.ExpiresAt.Format(time.RFC3339))
		if r.Decision != nil {
			fmt.Fprintf(&sb, "- decision: %s by %s on %s at %s\n", r.Decision.Verdict, r.Decision.Actor, r.Decision.Channel, r.Decision.DecidedAt.Format(time.RFC3339))
		}
		if r.Claim != nil {
			fmt.Fprintf(&sb, "- claim: intent %s, outcome %q\n", r.Claim.IntentID, r.Claim.Outcome)
		}
		if r.StatusReason != "" {
			fmt.Fprintf(&sb, "- reason: %s\n", r.StatusReason)
		}
		fmt.Fprintf(&sb, "\n```\n%s\n```", r.Preview.String())
		return sb.String()
	}
	all := b.Snapshot()
	if len(all) == 0 {
		return "No human-approval requests in this workspace's journal."
	}
	var sb strings.Builder
	sb.WriteString("**Human-approval requests** (most recent last; `/approvals <id>` for one)\n\n")
	for _, r := range all {
		fmt.Fprintf(&sb, "- `%s` %s — %s (%s)\n", r.ID, r.Status, r.Preview.Summary, r.CreatedAt.Format("2006-01-02 15:04"))
	}
	return sb.String()
}
