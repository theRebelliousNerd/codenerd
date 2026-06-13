package chat

import (
	ctxcompress "codenerd/internal/context"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"

	"codenerd/internal/store"
	"encoding/json"
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
func (m *Model) saveSessionState() {
	if m.workspace == "" || m.sessionID == "" {
		logging.Session("saveSessionState: early return - workspace=%q, sessionID=%q", m.workspace, m.sessionID)
		return
	}

	// Always persist session state/history for observability and continuity.
	// Session management should not require `/init` (which is about world-model wiring).
	nerdDir := filepath.Join(m.workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		logging.Get(logging.CategorySession).Error("saveSessionState: failed to create .nerd directory: %v", err)
		return
	}

	logging.Session("saveSessionState: saving session %s with %d messages, turnCount=%d", m.sessionID, len(m.history), m.turnCount)

	// Update session state
	state := &nerdinit.SessionState{
		SessionID:    m.sessionID,
		StartedAt:    time.Now(), // Will be overwritten if exists
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  m.sessionID + ".json",
	}

	// Preserve original StartedAt if session exists
	if existing, err := nerdinit.LoadSessionState(m.workspace); err == nil {
		state.StartedAt = existing.StartedAt
	}

	// Save session state (JSON)
	if err := nerdinit.SaveSessionState(m.workspace, state); err != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session state: %v", err)
	}

	// Convert and save conversation history (JSON)
	messages := make([]nerdinit.ChatMessage, len(m.history))
	for i, msg := range m.history {
		messages[i] = nerdinit.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Time:    msg.Time,
		}
	}
	if err := nerdinit.SaveSessionHistory(m.workspace, m.sessionID, messages); err != nil {
		logging.Get(logging.CategorySession).Error("Failed to save session history: %v", err)
	} else {
		logging.Session("Successfully saved %d messages to %s.json", len(messages), m.sessionID)
	}

	// Persist semantic compression state (best-effort) so we can rehydrate
	// infinite context. Errors at this stage previously vanished into the
	// blank assignment; log them at Warn so triage knows when rehydrate
	// after restart is going to read stale state.
	if m.localDB != nil && m.compressor != nil {
		state := m.compressor.GetState()
		if state != nil {
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

	// Process message pairs (user + assistant = 1 turn)
	// History format: [user1, asst1, user2, asst2, ...]
	for i := 0; i < len(m.history)-1; i += 2 {
		userMsg := m.history[i]
		asstMsg := m.history[i+1]

		// Skip if not a proper user-assistant pair
		if userMsg.Role != "user" || asstMsg.Role != "assistant" {
			continue
		}

		turnNumber := i / 2

		// Store to SQLite (StoreSessionTurn handles duplicates gracefully)
		// Intent and atoms JSON are empty for now - can be populated by OODA loop
		err := m.localDB.StoreSessionTurn(
			m.sessionID,
			turnNumber,
			userMsg.Content,
			"{}", // intent_json placeholder
			asstMsg.Content,
			"[]", // atoms_json placeholder
		)
		if err != nil {
			// Log but don't fail - JSON is the primary store
			// Duplicate key errors are expected and harmless
			continue
		}
	}
}

// loadSelectedSession loads a session from the sessions list and switches to it.
// Saves the current session first, then loads the selected session's history.
func (m Model) loadSelectedSession(sessionID string) (tea.Model, tea.Cmd) {
	// Save current session before switching
	m.saveSessionState()

	// Load the selected session's history
	history, err := nerdinit.LoadSessionHistory(m.workspace, sessionID)
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

	// Switch to the selected session
	m.sessionID = sessionID
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
	state := &nerdinit.SessionState{
		SessionID:    sessionID,
		StartedAt:    history.CreatedAt,
		LastActiveAt: time.Now(),
		TurnCount:    m.turnCount,
		HistoryFile:  sessionID + ".json",
	}
	_ = nerdinit.SaveSessionState(m.workspace, state)

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

// MigrateOldSessionsToSQLite migrates all existing JSON session files to SQLite.
// This enables querying historical sessions via virtual predicates.
// Safe to call multiple times - uses INSERT OR IGNORE for idempotency.
func MigrateOldSessionsToSQLite(workspace string, localDB *store.LocalStore) (int, error) {
	if localDB == nil {
		return 0, nil
	}

	// List all session JSON files
	sessionIDs, err := nerdinit.ListSessionHistories(workspace)
	if err != nil {
		return 0, err
	}

	migratedTurns := 0

	for _, sessionID := range sessionIDs {
		history, err := nerdinit.LoadSessionHistory(workspace, sessionID)
		if err != nil {
			continue // Skip corrupted sessions
		}

		// Process message pairs
		for i := 0; i < len(history.Messages)-1; i += 2 {
			userMsg := history.Messages[i]
			asstMsg := history.Messages[i+1]

			if userMsg.Role != "user" || asstMsg.Role != "assistant" {
				continue
			}

			turnNumber := i / 2

			err := localDB.StoreSessionTurn(
				sessionID,
				turnNumber,
				userMsg.Content,
				"{}",
				asstMsg.Content,
				"[]",
			)
			if err == nil {
				migratedTurns++
			}
		}
	}

	return migratedTurns, nil
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
