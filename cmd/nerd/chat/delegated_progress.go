package chat

import (
	"codenerd/internal/sessionlog"
	"context"
	"fmt"
	"github.com/charmbracelet/bubbletea"
	"sort"
	"strings"
	"time"
	"unicode"
)

// delegatedActivity belongs to the Bubble Tea update loop only. A step survives
// model/tool updates, so its duration is not confused with the current call's.
type delegatedActivity struct {
	Session   string
	Agent     string
	Step      string
	StepAt    time.Time
	Current   string
	CurrentAt time.Time
	CallID    string
}

type delegatedProgressMsg sessionlog.LiveEvent

// sendDelegatedProgress applies bounded backpressure instead of silently dropping
// an agent's work. Application shutdown always unblocks a sender; a cancelled
// agent still delivers its terminal update so its running clock is retired.
func (m Model) sendDelegatedProgress(ctx context.Context, ev sessionlog.LiveEvent) {
	var shutdown <-chan struct{}
	if m.shutdownCtx != nil {
		shutdown = m.shutdownCtx.Done()
	}
	// Prefer delivery even when a tool has just observed cancellation. In
	// particular, its final event must retire the visible running clock.
	select {
	case m.delegatedEvents <- ev:
		return
	default:
	}
	if ev.Kind == sessionlog.EventFinal {
		select {
		case m.delegatedEvents <- ev:
		case <-shutdown:
		}
		return
	}
	select {
	case m.delegatedEvents <- ev:
	case <-ctx.Done():
	case <-shutdown:
	}
}

func (m Model) listenDelegatedProgress() tea.Cmd {
	if m.delegatedEvents == nil {
		return nil
	}
	var shutdown <-chan struct{}
	if m.shutdownCtx != nil {
		shutdown = m.shutdownCtx.Done()
	}
	return func() tea.Msg {
		select {
		case ev := <-m.delegatedEvents:
			return delegatedProgressMsg(ev)
		case <-shutdown:
			return nil
		}
	}
}

// handleDelegatedProgress shows observations as they arrive, even with Glass Box
// disabled. No worker mutates Model; the session on the event is authoritative.
func (m *Model) handleDelegatedProgress(ev sessionlog.LiveEvent) {
	if ev.Session != m.sessionID || ev.Agent == "" {
		return
	}
	if m.delegatedActive == nil {
		m.delegatedActive = make(map[string]delegatedActivity)
	}
	key := ev.Session + "\x00" + ev.Agent
	a := m.delegatedActive[key]
	a.Session, a.Agent = ev.Session, ev.Agent
	text := ev.Text
	switch ev.Kind {
	case "plan":
	case "step":
		a.Step, a.StepAt = text, ev.At
		a.Current, a.CurrentAt = "preparing step", ev.At
	case "tool_started":
		a.Current = strings.TrimSpace(ev.Tool + " " + text)
		a.CurrentAt, a.CallID = ev.At, ev.CallID
		text = "RUN " + a.Current
	case sessionlog.EventToolResult:
		status := ev.Status
		if status == "" {
			status = "DONE"
		}
		if ev.Failed {
			status = "FAIL"
		}
		text = status + " " + strings.TrimSpace(ev.Tool+" "+text)
		if a.CallID == ev.CallID && !a.CurrentAt.IsZero() {
			text += " (" + formatElapsedShort(max(time.Duration(0), ev.At.Sub(a.CurrentAt))) + ")"
			a.Current, a.CurrentAt, a.CallID = "preparing next action", ev.At, ""
		}
	case sessionlog.EventFinal:
		if ev.Failed {
			text += " — error or cancelled"
		}
		delete(m.delegatedActive, key)
	case sessionlog.EventCallStart, sessionlog.EventCallFailed, sessionlog.EventInput:
		a.Current, a.CurrentAt = text, ev.At
	case sessionlog.EventAssistant:
		// Prose stays in scrollback without resetting the step clock.
	default:
		return
	}
	if ev.Kind != sessionlog.EventFinal {
		m.delegatedActive[key] = a
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	if strings.TrimSpace(text) == "" {
		return
	}
	*m = m.addMessage(Message{Role: "progress", Content: "[" + ev.Agent + "] " + text, Time: ev.At})
}

// renderDelegatedProgress keeps the current step and call clocks visible even
// while a tool/model is silent. Full subjects remain in chat; the pinned panel
// stays bounded so a long plan or many delegates cannot consume the terminal.
func (m Model) renderDelegatedProgress(now time.Time) string {
	var keys []string
	for key, a := range m.delegatedActive {
		if a.Session == m.sessionID {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	width := m.wrapWidth
	if width < 1 {
		width = max(m.width-4, 20)
	}
	compact := func(text string) string {
		text = strings.Join(strings.Fields(text), " ")
		runes := []rune(text)
		if len(runes) > width {
			return string(runes[:max(width-1, 1)]) + "…"
		}
		return text
	}
	var lines []string
	for i, key := range keys {
		if len(keys) > maxActivityTrail && i == maxActivityTrail-1 {
			lines = append(lines, fmt.Sprintf("  +%d agents active; activity in chat", len(keys)-i))
			break
		}
		a := m.delegatedActive[key]
		if a.Step != "" {
			lines = append(lines, compact("  ▸ "+a.Agent+" · "+formatElapsedShort(max(time.Duration(0), now.Sub(a.StepAt)))+" · "+a.Step))
		}
		if a.Current != "" {
			lines = append(lines, compact("  ▸ "+a.Agent+" · "+formatElapsedShort(max(time.Duration(0), now.Sub(a.CurrentAt)))+" · "+a.Current))
		}
	}
	for i := range lines {
		lines[i] = m.styles.Text.Muted.Width(width).Render(lines[i])
	}
	return strings.Join(lines, "\n")
}
