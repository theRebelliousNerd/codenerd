// Package session owns the chat session lifecycle and its persisted state.
//
// The types and load/save helpers in this file describe a chat session's
// persisted state — SessionState, ChatMessage, and SessionHistory, plus
// LoadSessionState, SaveSessionState, LoadSessionHistory, and
// SaveSessionHistory. They lived in internal/init only because `nerd init`
// seeds the first .nerd/session.json, but internal/session is their natural
// owner because it owns the session lifecycle.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionState represents the current session state.
type SessionState struct {
	SessionID    string    `json:"session_id"`
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	TurnCount    int       `json:"turn_count"`

	// Suspension state (for pause/resume)
	Suspended       bool       `json:"suspended"`
	SuspendedAt     *time.Time `json:"suspended_at,omitzero"`
	PendingQuestion string     `json:"pending_question,omitzero"`
	PendingOptions  []string   `json:"pending_options,omitzero"`

	// Context state
	ActiveStrategy string   `json:"active_strategy,omitzero"`
	ActiveGoals    []string `json:"active_goals,omitzero"`
	WorkingFacts   []string `json:"working_facts,omitzero"`

	// Conversation history (stored separately in sessions/ folder)
	HistoryFile string `json:"history_file,omitzero"`
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string    `json:"role"` // "user" or "assistant"
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// SessionHistory represents the full conversation history for a session.
type SessionHistory struct {
	SessionID string        `json:"session_id"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// LoadSessionState loads the session state from .nerd/session.json
func LoadSessionState(workspace string) (*SessionState, error) {
	path := filepath.Join(workspace, ".nerd", "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// SaveSessionState saves the session state to disk.
func SaveSessionState(workspace string, state *SessionState) error {
	path := filepath.Join(workspace, ".nerd", "session.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SaveSessionHistory saves the conversation history to the sessions folder.
func SaveSessionHistory(workspace string, sessionID string, messages []ChatMessage) error {
	sessionsDir := filepath.Join(workspace, ".nerd", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	historyFile := filepath.Join(sessionsDir, sessionID+".json")
	history := SessionHistory{
		SessionID: sessionID,
		Messages:  messages,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// If file exists, preserve CreatedAt
	if existing, err := LoadSessionHistory(workspace, sessionID); err == nil {
		history.CreatedAt = existing.CreatedAt
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyFile, data, 0644)
}

// LoadSessionHistory loads the conversation history for a session.
func LoadSessionHistory(workspace string, sessionID string) (*SessionHistory, error) {
	historyFile := filepath.Join(workspace, ".nerd", "sessions", sessionID+".json")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil, err
	}

	var history SessionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return &history, nil
}
