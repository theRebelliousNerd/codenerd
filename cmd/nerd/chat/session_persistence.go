package chat

import (
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/logging"
	"codenerd/internal/session"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

// saveSessionState saves the current session state and history.
// Implements dual persistence: JSON files + SQLite for redundancy and queryability.
// It returns the failures of the session-state and history writes; the
// compression and SQLite projections are best-effort and logged. A save for
// a session that is no longer open (a deferred save that ran after a
// switch) writes nothing (session_owner.go).
func (m *Model) saveSessionState() error {
	if m.workspace == "" || m.sessionID == "" {
		logging.Session("saveSessionState: early return - workspace=%q, sessionID=%q", m.workspace, m.sessionID)
		return nil
	}
	if !m.selected.open(m.sessionID) {
		logging.Session("saveSessionState: session %s is no longer open; this deferred save is skipped", m.sessionID)
		return nil
	}

	// Always persist session state/history for observability and continuity.
	// Session management should not require `/init` (which is about world-model wiring).
	nerdDir := filepath.Join(m.workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		logging.Get(logging.CategorySession).Error("saveSessionState: failed to create .nerd directory: %v", err)
		return err
	}

	logging.Session("saveSessionState: saving session %s with %d messages, turnCount=%d", m.sessionID, len(m.history), m.turnCount)

	// Update session state
	state := &session.SessionState{
		SessionID:    m.sessionID,
		StartedAt:    time.Now(), // Will be overwritten if exists
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  m.sessionID + ".json",
	}

	// Preserve original StartedAt if session exists
	if existing, err := session.LoadSessionState(m.workspace); err == nil {
		state.StartedAt = existing.StartedAt
	}

	// Save session state (JSON)
	stateErr := session.SaveSessionState(m.workspace, state)
	if stateErr != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session state: %v", stateErr)
	}

	// Convert and save conversation history (JSON)
	messages := make([]session.ChatMessage, len(m.history))
	for i, msg := range m.history {
		messages[i] = session.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	historyErr := session.SaveSessionHistory(m.workspace, m.sessionID, messages)
	if historyErr != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session history: %v", historyErr)
	} else {
		logging.Session("Successfully saved %d messages to %s.json", len(messages), m.sessionID)
	}
	// Persist semantic compression state (best-effort) so we can rehydrate
	// infinite context. Errors at this stage previously vanished into the
	// blank assignment; log them at Warn so triage knows when rehydrate
	// after restart is going to read stale state.
	if m.localDB != nil && m.compressor != nil {
		state := m.compressor.GetState()
		if state != nil && state.SessionID != m.sessionID {
			// The compressor holds another session's memory; storing it under
			// this id would hand that memory to this session on its next load.
			logging.Get(logging.CategorySession).Warn(
				"StoreCompressedState: compressor holds session %q, not %q; not stored", state.SessionID, m.sessionID)
		} else if state != nil {
			data, marshalErr := ctxcompress.MarshalCompressedState(state)
			if marshalErr != nil {
				logging.Get(logging.CategorySession).Warn(
					"StoreCompressedState: marshal failed session=%s turn=%d: %v",
					m.sessionID, state.TurnNumber, marshalErr)
			} else if storeErr := m.localDB.StoreCompressedState(m.sessionID, state.TurnNumber, string(data), state.CompressionRatio); storeErr != nil {
				logging.Get(logging.CategorySession).Warn(
					"StoreCompressedState: persist failed session=%s turn=%d ratio=%.3f: %v",
					m.sessionID, state.TurnNumber, state.CompressionRatio, storeErr)
			}
		}
	}

	// ==========================================================================
	// DUAL PERSISTENCE: Sync to SQLite (knowledge.db session_history table)
	// ==========================================================================
	// This enables Mangle queries against session history via virtual predicates
	if m.localDB != nil {
		m.syncSessionToSQLite()
	}
	return errors.Join(stateErr, historyErr)
}

// hydrateCompressorForSession resets and rehydrates the semantic compressor state for a session.
// This enables infinite-context continuity across restarts and session switches.
func (m *Model) hydrateCompressorForSession(sessionID string) {
	if m.compressor == nil {
		return
	}

	// Always reset to avoid leaking state across sessions.
	m.compressor.Reset()
	m.compressor.SetSessionID(sessionID)

	// Always refresh budget at the end so status bar shows accurate context usage.
	defer m.compressor.RefreshBudget()

	if m.localDB == nil {
		return
	}

	stateJSON, turnNumber, _, err := m.localDB.LoadLatestCompressedState(sessionID)
	if err != nil || strings.TrimSpace(stateJSON) == "" {
		return
	}

	state, err := ctxcompress.UnmarshalCompressedState([]byte(stateJSON))
	if err != nil {
		logging.Session("hydrateCompressorForSession: failed to parse compressed state: %v", err)
		return
	}
	// A row stored under this session but holding another's memory is not
	// this session's memory: it is refused, and the session starts with an
	// empty compressor rather than another conversation's summary.
	if state.SessionID != "" && state.SessionID != sessionID {
		logging.Get(logging.CategorySession).Warn("hydrateCompressorForSession: the state stored for %s belongs to %s; not loaded", sessionID, state.SessionID)
		return
	}
	state.SessionID = sessionID

	if err := m.compressor.LoadState(state); err != nil {
		logging.Session("hydrateCompressorForSession: failed to load compressed state: %v", err)
		return
	}

	// Keep turn counter monotonic if compressed state is ahead.
	if turnNumber > m.turnCount {
		m.turnCount = turnNumber
	}
}

// syncSessionToSQLite syncs conversation history to knowledge.db for query access.
// Uses turn-based storage to avoid duplicates (SQLite table has unique constraint).
func (m *Model) syncSessionToSQLite() {
	if m.localDB == nil || len(m.history) == 0 {
		return
	}

	for _, t := range sessionTurns(m.history) {
		// Store to SQLite (StoreSessionTurn upserts by session and turn).
		// Intent and atoms JSON are empty for now - can be populated by OODA loop
		if err := m.localDB.StoreSessionTurn(m.sessionID, t.number, t.user, "{}", t.assistant, "[]"); err != nil {
			logging.Get(logging.CategorySession).Warn("session %s turn %d: store to SQLite: %v", m.sessionID, t.number, err)
		}
	}
}

// sessionTurn is one user message and the first assistant reply after it.
type sessionTurn struct {
	number          int
	user, assistant string
}

// sessionTurns pairs each user message with the first assistant message
// that follows it, numbering turns by user message. Assistant messages
// nobody asked for (a background result, a notice) do not shift the
// pairing of the turns after them.
func sessionTurns(history []Message) []sessionTurn {
	var out []sessionTurn
	n := 0
	for i := 0; i < len(history); i++ {
		if history[i].Role != "user" {
			continue
		}
		for j := i + 1; j < len(history) && history[j].Role != "user"; j++ {
			if history[j].Role == "assistant" {
				out = append(out, sessionTurn{number: n, user: history[i].Content, assistant: history[j].Content})
				break
			}
		}
		n++
	}
	return out
}

// loadSelectedSession loads a session from the sessions list and switches to it.
// Saves the current session first, then loads the selected session's history.
func (m Model) loadSelectedSession(sessionID string) (tea.Model, tea.Cmd) {
	// Save current session before switching
	if err := m.saveSessionState(); err != nil {
		logging.Get(logging.CategorySession).Error("loadSelectedSession: saving %s before the switch: %v", m.sessionID, err)
	}

	// Load the selected session's history
	history, err := session.LoadSessionHistory(m.workspace, sessionID)
	if err != nil {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Failed to load session: %v", err),
			Time:    time.Now(),
		})
		m.viewMode = ChatView
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}

	// Switch to the selected session: the outgoing conversation's facts
	// leave the kernel before the selected one's memory is loaded.
	m = m.detachConversation()
	m.sessionID = sessionID
	m.selected.set(sessionID)
	m.history = make([]Message, len(history.Messages))
	for i, msg := range history.Messages {
		m.history[i] = Message{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	m.turnCount = len(m.history) / 2 // Approximate turn count from history

	// Rehydrate compressor state for this session (if available).
	m.hydrateCompressorForSession(sessionID)

	// Update session.json to point to this session
	state := &session.SessionState{
		SessionID:    sessionID,
		StartedAt:    history.CreatedAt,
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  sessionID + ".json",
	}
	_ = session.SaveSessionState(m.workspace, state)

	// Add a system message indicating session switch
	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: fmt.Sprintf("*Loaded session: `%s` (%d messages)*", sessionID, len(history.Messages)),
		Time:    time.Now(),
	})

	// Switch back to chat view
	m.viewMode = ChatView
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	return m, nil
}

func persistAgentProfile(workspace, name, agentType, knowledgePath string, kbSize int, status string) error {
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(filepath.Join(nerdDir, "shards"), 0755); err != nil {
		return err
	}

	agentsPath := filepath.Join(nerdDir, "agents.json")
	reg := Registry{
		Version:   "1.0",
		CreatedAt: time.Now().Format(time.RFC3339),
		Agents:    []Agent{},
	}

	if data, err := os.ReadFile(agentsPath); err == nil {
		_ = json.Unmarshal(data, &reg)
	}

	// Upsert
	found := false
	for i, a := range reg.Agents {
		if strings.EqualFold(a.Name, name) {
			reg.Agents[i].Type = agentType
			reg.Agents[i].KnowledgePath = knowledgePath
			reg.Agents[i].KBSize = kbSize
			reg.Agents[i].Status = status
			found = true
			break
		}
	}
	if !found {
		reg.Agents = append(reg.Agents, Agent{
			Name:          name,
			Type:          agentType,
			KnowledgePath: knowledgePath,
			KBSize:        kbSize,
			Status:        status,
		})
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(agentsPath, data, 0644)
}

func resolveSessionID(session *Session) string {
	if session != nil && strings.TrimSpace(session.SessionID) != "" {
		return session.SessionID
	}
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func resolveTurnCount(session *Session) int {
	if session != nil && session.TurnCount > 0 {
		return session.TurnCount
	}
	return 0
}
