package chat

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func scrollbackModel(t *testing.T, messages int) Model {
	t.Helper()
	m := NewTestModel()
	m = m.handleWindowSizeMsg(windowSizeMsg{Width: 100, Height: 30})
	for i := range messages {
		m = m.addMessage(Message{Role: RoleAssistant, Content: fmt.Sprintf("message-%04d", i), Time: time.Now()})
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m
}

// A long session's earlier messages are reachable: scrolling up at the top of
// the rendered window loads the page before it and keeps the reader's place.
func TestScrollback_ScrollingPastTheTopLoadsEarlierMessages(t *testing.T) {
	m := scrollbackModel(t, 3*historyRenderLimit)
	if got := m.historyStart(); got != 2*historyRenderLimit {
		t.Fatalf("historyStart = %d, want %d", got, 2*historyRenderLimit)
	}
	if view := m.renderHistory(); strings.Contains(view, "message-0000") || !strings.Contains(view, "earlier message(s)") {
		t.Fatalf("the default window must hold the tail and name what is above it")
	}

	m.viewport.GotoTop()
	top := m.viewport.TotalLineCount()
	if !m.loadEarlierHistory() {
		t.Fatal("scrolling up at the top loaded nothing")
	}
	if got := m.historyStart(); got != historyRenderLimit {
		t.Fatalf("after one load historyStart = %d, want %d", got, historyRenderLimit)
	}
	if grown := m.viewport.TotalLineCount() - top; grown <= 0 || m.viewport.YOffset != grown {
		t.Fatalf("the reader's place moved: offset %d, lines added %d", m.viewport.YOffset, grown)
	}

	m.viewport.GotoTop()
	m.loadEarlierHistory()
	if m.historyStart() != 0 {
		t.Fatalf("the whole history is not loaded: start %d", m.historyStart())
	}
	if view := m.renderHistory(); !strings.Contains(view, "message-0000") || strings.Contains(view, "earlier message(s)") {
		t.Fatalf("the first message must be rendered and the marker gone")
	}
	m.viewport.GotoTop()
	if m.loadEarlierHistory() {
		t.Fatal("nothing is left to load")
	}
}

// New output does not pull the view away from someone reading back; sending a
// message returns to the live tail.
func TestScrollback_NewOutputDoesNotYankAReaderToTheBottom(t *testing.T) {
	m := scrollbackModel(t, historyRenderLimit)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	if !m.readingBack || m.viewport.AtBottom() {
		t.Fatalf("page up did not start reading back (readingBack=%v)", m.readingBack)
	}
	at := m.viewport.YOffset

	m = m.pushAssistantMsg("fresh output while reading back")
	if m.viewport.YOffset != at {
		t.Fatalf("new output moved the view from %d to %d", at, m.viewport.YOffset)
	}

	m = m.pushUserMsg("my next message")
	if m.readingBack || !m.viewport.AtBottom() {
		t.Fatal("sending a message must return to the live tail")
	}
}
