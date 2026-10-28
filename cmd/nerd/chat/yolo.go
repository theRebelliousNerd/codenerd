package chat

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// /yolo toggles the yolo work mode (config.WorkMode): codeNERD makes every
// decision it can without asking. It persists as work_mode so CLI commands
// (notably `nerd campaign recurse --waves 0`) and later chat sessions agree.

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
		return m.pushYoloNote("Yolo mode ON: no clarification round-trips, learning auto-consent, unbounded /recurse allowed. Safety denials, error notices, and loop stalls still surface — this resolves choices, never verdicts."), nil
	}
	return m.pushYoloNote("Yolo mode OFF: " + workModeLabel(m.workMode()) + ", back to asking."), nil
}

// yoloEnabled is the work mode in force being yolo (work_mode.go).
func (m Model) yoloEnabled() bool {
	return m.workMode() == config.WorkModeYolo
}

// setYolo jumps to yolo, or leaves it for propose; /yolo off from session
// yolo keeps whichever non-yolo mode is persisted.
func (m Model) setYolo(on bool) Model {
	if on {
		return m.setWorkMode(config.WorkModeYolo)
	}
	if mode := m.Config.EffectiveWorkMode(); mode != config.WorkModeYolo {
		return m.setWorkMode(mode)
	}
	return m.setWorkMode(config.WorkModePropose)
}

// syncYoloFact mirrors the yolo switch into the kernel as the yolo_mode
// fact, which the ambiguity-driven clarification rules consult. Config is
// the source of truth (it persists); the fact is derived session state.
// A nil kernel (pre-boot toggle) is fine: the boot hook re-syncs from
// config once the kernel exists.
func (m Model) syncYoloFact() {
	if m.kernel == nil {
		return
	}
	if m.yoloEnabled() {
		if err := m.kernel.Assert(core.Fact{Predicate: "yolo_mode"}); err != nil {
			logging.Get(logging.CategoryKernel).Error("yolo: failed to assert yolo_mode: %v", err)
		}
		return
	}
	if err := m.kernel.Retract("yolo_mode"); err != nil {
		logging.Get(logging.CategoryKernel).Error("yolo: failed to retract yolo_mode: %v", err)
	}
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
		return "Yolo mode is ON: no clarification round-trips, learning auto-consent, unbounded /recurse allowed (safety denials, error notices, and stalls still apply)."
	}
	return "Yolo mode is OFF."
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}
