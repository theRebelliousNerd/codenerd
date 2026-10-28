package ui

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/diff"

	tea "github.com/charmbracelet/bubbletea"
)

// testRequest is a host snapshot as the broker would hand it to the view.
func testRequest(id string, fd *diff.FileDiff) approval.Request {
	return approval.Request{
		ID: id, Version: 1, IntentID: "exec-" + id, Digest: "sha256:digest-of-" + id,
		Subject:   approval.Subject{Tool: "delete_file", Action: "/delete_file", Target: id + ".md", Agent: "session"},
		Preview:   approval.Preview{Summary: "Delete " + id + ".md", Diff: fd, Complete: true},
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    approval.StatusAwaitingHuman,
	}
}

func testMutation(id string, fd *diff.FileDiff) *PendingMutation {
	return NewPendingMutation(testRequest(id, fd))
}

func decisionsFrom(t *testing.T, cmd tea.Cmd) []approval.DecisionInput {
	t.Helper()
	if cmd == nil {
		return nil
	}
	var out []approval.DecisionInput
	var walk func(tea.Msg)
	walk = func(msg tea.Msg) {
		switch m := msg.(type) {
		case ApprovalDecisionMsg:
			out = append(out, m.Decisions...)
		case tea.BatchMsg:
			for _, c := range m {
				if c != nil {
					walk(c())
				}
			}
		}
	}
	walk(cmd())
	return out
}

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestApprovalView_DecisionCarriesTheDisplayedRequest(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 100, 40)
	view.AddMutation(testMutation("a", nil))
	var cmd tea.Cmd
	view, cmd = view.Update(runeKey('y'))
	got := decisionsFrom(t, cmd)
	if len(got) != 1 {
		t.Fatalf("decisions = %+v", got)
	}
	d := got[0]
	if d.RequestID != "a" || d.RequestVersion != 1 || d.RequestDigest != "sha256:digest-of-a" || d.Verdict != approval.VerdictApprove || d.EventID == "" {
		t.Fatalf("decision does not name the displayed request: %+v", d)
	}
	// A second press is not a second decision.
	view, cmd = view.Update(runeKey('y'))
	if again := decisionsFrom(t, cmd); len(again) != 0 {
		t.Fatalf("a repeated key produced %d more decisions", len(again))
	}
}

func TestApprovalView_ApproveAllIsExactlyTheViewedUndecidedSet(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 100, 40)
	view.AddMutation(testMutation("first", nil)) // on screen: viewed
	view.AddMutation(testMutation("second", nil))
	view.NextMutation() // second viewed
	if _, ok := view.RejectCurrent("no"); !ok {
		t.Fatal("reject failed")
	}
	view.AddMutation(testMutation("third", nil)) // added after, never on screen

	got := view.ApproveAll()
	if len(got) != 1 || got[0].RequestID != "first" {
		t.Fatalf("ApproveAll = %+v, want only the viewed undecided request", got)
	}
	if view.Mutations[1].Submitted != approval.VerdictReject {
		t.Fatal("ApproveAll turned a rejection into an approval")
	}
	if view.Mutations[2].Submitted != "" {
		t.Fatal("ApproveAll approved a request the person never saw")
	}
}

func TestApprovalView_RevisedRequestMustBeSeenAgain(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 100, 40)
	view.AddMutation(testMutation("a", nil))
	view.AddMutation(testMutation("b", nil))
	// b is revised by the host while a is on screen.
	revised := testRequest("b", nil)
	revised.Version, revised.Digest = 2, "sha256:revised"
	view.SyncRequests([]approval.Request{testRequest("a", nil), revised})
	got := view.ApproveAll()
	if len(got) != 1 || got[0].RequestID != "a" {
		t.Fatalf("ApproveAll after revision = %+v", got)
	}
	view.NextMutation()
	in, ok := view.ApproveCurrent()
	if !ok || in.RequestVersion != 2 || in.RequestDigest != "sha256:revised" {
		t.Fatalf("decision on the revised request = %+v, %v", in, ok)
	}
}

func TestApprovalView_RendersReviewLimits(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 120, 60)
	r := testRequest("big", nil)
	r.Preview.Complete = false
	r.Preview.Incomplete = []string{"big.md is binary; its bytes are not shown"}
	r.Preview.Unknowns = []string{"network effects"}
	r.Preview.Text = `{"path":"big.md"}`
	view.AddMutation(NewPendingMutation(r))
	view.ShowWarnings = false // limits are not behind the toggle
	view.updateContent()
	out := stripANSI(view.Viewport.View())
	for _, want := range []string{"Not shown: big.md is binary", "Not modeled: network effects", `Arguments: {"path":"big.md"}`, "sha256:digest-of-big", "Request: big v1"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered view lacks %q:\n%s", want, out)
		}
	}
}

func TestApprovalView_WhitespaceFilterSaysWhatItHides(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 120, 60)
	fd := diff.ComputeDiff("a.go", "a.go", "x := 1\ny\n", "x  :=  1\ny\n")
	view.AddMutation(testMutation("ws", fd))
	view.ToggleIgnoreWhitespace()
	out := stripANSI(view.Viewport.View())
	if !strings.Contains(out, "whitespace-only changed line(s) hidden") {
		t.Fatalf("hidden whitespace lines are not announced:\n%s", out)
	}
}
