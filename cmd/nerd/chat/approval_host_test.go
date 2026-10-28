package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/approval"

	tea "github.com/charmbracelet/bubbletea"
)

func approvalModel(t *testing.T) (Model, *approval.Broker) {
	t.Helper()
	b, err := approval.NewBroker(approval.Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	m := Model{styles: ui.DefaultStyles(), width: 120, height: 40, sessionID: "s-test"}
	if err := m.attachApprovalHost(b); err != nil {
		t.Fatal(err)
	}
	return m, b
}

type awaited struct {
	grant *approval.Grant
	err   error
}

func openRequest(t *testing.T, b *approval.Broker, intent, target string) <-chan awaited {
	t.Helper()
	subject := approval.Subject{Backend: "modular", Tool: "delete_file", Action: "/delete_file", Target: target, Payload: `{"path":"` + target + `"}`}
	preview, err := approval.BuildPreview(approval.PreviewInput{Summary: "Delete " + target, Arguments: subject.Payload, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan awaited, 1)
	go func() {
		g, _, err := b.Await(context.Background(), intent, subject, preview)
		done <- awaited{g, err}
	}()
	return done
}

// nextUpdate feeds the model the host's next update for intent, as the
// listenApprovals command would.
func nextUpdate(t *testing.T, m Model, intent string) Model {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case r := <-m.approvalHost.Updates():
			model, _ := m.Update(approvalUpdateMsg(r))
			m = model.(Model)
			if r.IntentID == intent && r.Status == approval.StatusAwaitingHuman {
				return m
			}
		case <-deadline:
			t.Fatal("the host was never offered the request")
			return m
		}
	}
}

// pressKey runs one key through Update and then every command it returns,
// the way the Bubble Tea runtime would.
func pressKey(t *testing.T, m Model, r rune) Model {
	t.Helper()
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	m = model.(Model)
	for cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				model, _ = m.Update(c())
				m = model.(Model)
			}
			return m
		}
		if msg == nil {
			return m
		}
		model, cmd = m.Update(msg)
		m = model.(Model)
	}
	return m
}

func TestApprovalHost_KeyDecisionReachesTheBroker(t *testing.T) {
	m, b := approvalModel(t)
	done := openRequest(t, b, "exec-1", "notes.md")
	m = nextUpdate(t, m, "exec-1")
	if m.viewMode != ApprovalView {
		t.Fatalf("view mode = %v; a pending request must open the approval view", m.viewMode)
	}
	m = pressKey(t, m, 'y')
	res := <-done
	if res.err != nil || res.grant == nil {
		t.Fatalf("the approve key did not approve through the host: %v", res.err)
	}
	if m.viewMode != ChatView {
		t.Fatalf("view mode = %v after the last decision, want chat", m.viewMode)
	}
}

func TestApprovalHost_ApproveAllDoesNotApproveARejection(t *testing.T) {
	m, b := approvalModel(t)
	first := openRequest(t, b, "exec-1", "a.md")
	m = nextUpdate(t, m, "exec-1")
	m = pressKey(t, m, 'n') // reject the one on screen
	if res := <-first; !errors.Is(res.err, approval.ErrRejected) {
		t.Fatalf("first = %v, want rejected", res.err)
	}
	second := openRequest(t, b, "exec-2", "b.md")
	m = nextUpdate(t, m, "exec-2")
	m = pressKey(t, m, 'a')
	if res := <-second; res.err != nil {
		t.Fatalf("the viewed pending request was not approved: %v", res.err)
	}
	for _, r := range b.Snapshot() {
		if r.IntentID == "exec-1" && r.Status != approval.StatusRejected {
			t.Fatalf("the rejected request is %s after approve-all", r.Status)
		}
	}
}

func TestApprovalHost_ShutdownCancelsWaitingCalls(t *testing.T) {
	m, b := approvalModel(t)
	done := openRequest(t, b, "exec-1", "a.md")
	_ = nextUpdate(t, m, "exec-1")
	m.detachApprovalHost()
	if res := <-done; !errors.Is(res.err, approval.ErrCanceled) {
		t.Fatalf("a call stayed waiting on a detached TUI: %v", res.err)
	}
}

// /approvals reads the record and decides nothing.
func TestApprovalHost_StatusCommandIsReadOnly(t *testing.T) {
	m, b := approvalModel(t)
	done := openRequest(t, b, "exec-1", "a.md")
	m = nextUpdate(t, m, "exec-1")
	list := formatApprovalStatus(m.approvals, nil)
	pending := b.Snapshot()[0]
	if !strings.Contains(list, pending.ID) || !strings.Contains(list, "awaiting_human") {
		t.Fatalf("listing = %q", list)
	}
	detail := formatApprovalStatus(m.approvals, []string{pending.ID})
	if !strings.Contains(detail, pending.Digest) || !strings.Contains(detail, "Delete a.md") {
		t.Fatalf("detail = %q", detail)
	}
	model, _ := m.handleCmdApprovals("/approvals", []string{"/approvals"})
	_ = model
	if got := b.Snapshot()[0]; got.Status != approval.StatusAwaitingHuman {
		t.Fatalf("the status command changed the request to %s", got.Status)
	}
	if !strings.Contains(formatApprovalStatus(nil, nil), "unavailable") {
		t.Fatal("a missing broker was not reported")
	}
	m.detachApprovalHost()
	<-done
}
