package chat

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// /yolo toggles autonomous mode: codeNERD makes every decision it can without
// asking. The setting persists to .nerd/config.json so CLI commands (notably
// `nerd campaign recurse --waves 0`) and later chat sessions agree.
//
// Yolo resolves CHOICE points, never SAFETY verdicts: constitutional denials,
// risk-gate refusals, and safety validators still refuse, and unbounded runs
// still stop on the stall fuse.

func (m Model) handleCmdYolo(input string, parts []string) (tea.Model, tea.Cmd) {
	_ = input
	on := true
	if len(parts) > 1 {
		switch parts[1] {
		case "on":
			on = true
		case "off":
			on = false
		case "status":
			return m.pushYoloNote(yoloStatusText(m.yoloEnabled())), nil
		default:
			return m.pushYoloNote("Usage: `/yolo [on|off|status]` — bare `/yolo` toggles."), nil
		}
	} else {
		on = !m.yoloEnabled()
	}
	m = m.setYolo(on)
	if on {
		return m.pushYoloNote("Yolo mode ON: I will decide everything I can without asking. Safety denials still apply — this resolves choices, never verdicts."), nil
	}
	return m.pushYoloNote("Yolo mode OFF: back to asking."), nil
}

func (m Model) yoloEnabled() bool {
	return m.Config != nil && m.Config.YoloMode()
}

func (m Model) setYolo(on bool) Model {
	if m.Config == nil {
		return m
	}
	m.Config.Yolo = on
	if err := m.Config.Save(m.userConfigPath()); err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Yolo mode %s for this session, but saving failed: %v", onOff(on), err),
			Time:    time.Now(),
		})
	}
	return m
}

func (m Model) pushYoloNote(content string) Model {
	m = m.addMessage(Message{Role: "assistant", Content: content, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m
}

func yoloStatusText(on bool) string {
	if on {
		return "Yolo mode is ON: deciding without asking (safety denials still apply)."
	}
	return "Yolo mode is OFF."
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}
