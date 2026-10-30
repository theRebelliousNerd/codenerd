package chat

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
)

// startupScanToWelcome drives the startup scan's completion through Update,
// which unlocks the input and starts the health check, and returns the model
// with the health check's result still unapplied.
func startupScanToWelcome(t *testing.T, m Model) (Model, welcomeHealthCheckMsg) {
	t.Helper()
	m.isBooting = true
	m.bootStage = BootStageScanning
	next, cmd := m.Update(scanCompleteMsg{fileCount: 12, directoryCount: 3, factCount: 400, duration: time.Second})
	m = next.(Model)
	if m.isBooting {
		t.Fatal("fixture: the startup scan did not unlock the input")
	}
	var queue []tea.Cmd
	if cmd != nil {
		queue = append(queue, cmd)
	}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case welcomeHealthCheckMsg:
			return m, msg
		}
	}
	t.Fatal("the startup scan started no health check")
	return m, welcomeHealthCheckMsg{}
}

func roleIndex(m Model, role, contains string) int {
	return slices.IndexFunc(m.history, func(msg Message) bool {
		return msg.Role == role && strings.Contains(msg.Content, contains)
	})
}

// R1-36: the input unlocks when the scan completes while the greeting is a
// model call away, so a request typed at once landed first and the greeting
// arrived below it, as if answering it. The greeting now opens the
// conversation whenever it arrives.
func TestWelcome_OpensTheConversationEvenWhenARequestWasTypedFirst(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	client := NewMockLLMClient()
	client.defaultResp = "Welcome to codeNERD, ready on this project."
	m.client = client
	m, welcome := startupScanToWelcome(t, m)
	if welcome.err != nil || !strings.Contains(welcome.welcome, "Welcome to codeNERD") {
		t.Fatalf("fixture: health check = %+v", welcome)
	}

	m.textarea.SetValue("why does the retry loop stop early?")
	next, _ := m.handleSubmit()
	m = next.(Model)
	next, _ = m.Update(welcome)
	m = next.(Model)

	greeting, request := roleIndex(m, "assistant", "Welcome to codeNERD"), roleIndex(m, "user", "retry loop")
	if greeting < 0 || request < 0 || greeting > request {
		t.Fatalf("greeting at %d, request at %d: the greeting does not open the conversation", greeting, request)
	}
	if view := m.renderHistory(); strings.Index(view, "Welcome to codeNERD") > strings.Index(view, "retry loop") {
		t.Fatal("the rendered conversation shows the greeting below the request")
	}
}

// A health problem found at boot is still reported, wherever it lands.
func TestWelcome_AFailedHealthCheckIsReportedAfterTheRequest(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.textarea.SetValue("why does the retry loop stop early?")
	next, _ := m.handleSubmit()
	m = next.(Model)
	next, _ = m.Update(welcomeHealthCheckMsg{err: errors.New("401 unauthorized"), provider: "configured", model: "configured-model"})
	m = next.(Model)
	if failure := roleIndex(m, "assistant", "LLM health check failed"); failure < roleIndex(m, "user", "retry loop") {
		t.Fatalf("the health failure was not reported after the request (at %d)", failure)
	}
}
