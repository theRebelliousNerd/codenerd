package chat

import (
	"fmt"
	"time"

	"codenerd/internal/config"
)

// The work mode (config.WorkMode) is the chat's one autonomy switch: Shift+Tab
// cycles it, /yolo jumps to or leaves yolo, --yolo holds yolo
// for the session without persisting it. The kernel sees yolo as yolo_mode.

// workMode is the mode in force: session yolo wins, else the persisted mode.
func (m Model) workMode() config.WorkMode {
	if m.yoloFlag {
		return config.WorkModeYolo
	}
	return m.Config.EffectiveWorkMode()
}

// setWorkMode persists mode and mirrors it into the kernel. Leaving yolo ends
// session yolo too, so the footer never shows one mode while another acts.
func (m Model) setWorkMode(mode config.WorkMode) Model {
	if mode != config.WorkModeYolo {
		m.yoloFlag = false
	}
	if m.Config == nil {
		return m
	}
	m.Config.WorkMode = mode
	m.syncWorkModeFacts()
	if err := m.Config.Save(m.userConfigPath()); err != nil {
		m = m.addMessage(Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Mode %s for this session, but saving failed: %v", workModeLabel(mode), err),
			Time:    time.Now(),
		})
	}
	return m
}

// pausesBefore reports whether a multi-step run stops before its next step:
// explicit after every step, propose before mutations, yolo never.
func (m Model) pausesBefore(isMutation bool) bool {
	switch m.workMode() {
	case config.WorkModeExplicit:
		return true
	case config.WorkModePropose:
		return isMutation
	}
	return false
}

func workModeLabel(mode config.WorkMode) string {
	switch mode {
	case config.WorkModeYolo:
		return "[Y] Yolo"
	case config.WorkModeExplicit:
		return "[E] Explicit"
	}
	return "[P] Propose"
}
