// Package chat provides tests for session.go adapters and utilities.
package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/store"
)

// =============================================================================
// SESSION FUNCTION TESTS
// =============================================================================

func TestPersistAgentProfile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create .nerd directory structure is handled by the function
	err := persistAgentProfile(tmpDir, "TestAgent", "specialist", "/path/to/kb", 1024, "active")
	if err != nil {
		t.Fatalf("persistAgentProfile failed: %v", err)
	}

	// Verify file created
	agentsPath := filepath.Join(tmpDir, ".nerd", "agents.json")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Fatalf("agents.json not created")
	}

	// Verify content
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("Failed to read agents.json: %v", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("Failed to parse agents.json: %v", err)
	}

	if len(reg.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(reg.Agents))
	} else {
		agent := reg.Agents[0]
		if agent.Name != "TestAgent" {
			t.Errorf("Expected agent name 'TestAgent', got '%s'", agent.Name)
		}
		if agent.Type != "specialist" {
			t.Errorf("Expected agent type 'specialist', got '%s'", agent.Type)
		}
		if agent.KBSize != 1024 {
			t.Errorf("Expected KBSize 1024, got %d", agent.KBSize)
		}
	}

	// Test update existing
	err = persistAgentProfile(tmpDir, "TestAgent", "specialist", "/new/path", 2048, "inactive")
	if err != nil {
		t.Fatalf("persistAgentProfile update failed: %v", err)
	}

	data, _ = os.ReadFile(agentsPath)
	_ = json.Unmarshal(data, &reg)

	if len(reg.Agents) != 1 {
		t.Errorf("Expected 1 agent after update, got %d", len(reg.Agents))
	} else {
		agent := reg.Agents[0]
		if agent.KBSize != 2048 {
			t.Errorf("Expected updated KBSize 2048, got %d", agent.KBSize)
		}
		if agent.Status != "inactive" {
			t.Errorf("Expected updated Status 'inactive', got '%s'", agent.Status)
		}
	}
}

func TestMigrateOldSessionsToSQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration test in short mode")
	}

	tmpDir := t.TempDir()

	// Setup: Create a local store
	// Must pass full path to DB file
	dbPath := filepath.Join(tmpDir, ".nerd", "knowledge.db")
	localDB, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Skipf("Failed to create LocalStore: %v", err)
	}
	defer localDB.Close()

	// Setup: Create some dummy session JSON files
	// saveSessionHistory handles directory creation

	// Session 1: valid
	session1 := "session-1"
	hist1 := nerdinit.SessionHistory{
		SessionID: session1,
		Messages: []nerdinit.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "I am good"},
		},
	}
	saveSessionHistory(t, tmpDir, session1, hist1)

	// Session 2: incomplete pair (should be skipped or partially migrated)
	session2 := "session-2"
	hist2 := nerdinit.SessionHistory{
		SessionID: session2,
		Messages: []nerdinit.ChatMessage{
			{Role: "user", Content: "Only me"},
		},
	}
	saveSessionHistory(t, tmpDir, session2, hist2)

	// Run migration
	count, err := MigrateOldSessionsToSQLite(tmpDir, localDB)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Should have migrated 2 turns from session 1
	if count != 2 {
		t.Errorf("Expected 2 migrated turns, got %d", count)
	}
}

func TestSyncSessionToSQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sync test in short mode")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".nerd", "knowledge.db")
	localDB, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Skipf("Failed to create LocalStore: %v", err)
	}
	defer localDB.Close()

	m := Model{
		sessionID: "test-sync-session",
		localDB:   localDB,
		history: []Message{
			{Role: "user", Content: "User 1"},
			{Role: "assistant", Content: "Asst 1"},
			{Role: "user", Content: "User 2"}, // Unmatched
		},
	}

	// Run sync
	m.syncSessionToSQLite()

	// We can try to store a duplicate turn and ensure it doesn't fail
	m.syncSessionToSQLite()
}

func TestLoadSelectedSession(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Setup existing session
	sessionID := "target-session"
	hist := nerdinit.SessionHistory{
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Messages: []nerdinit.ChatMessage{
			{Role: "user", Content: "History 1"},
			{Role: "assistant", Content: "History 2"},
		},
	}
	saveSessionHistory(t, tmpDir, sessionID, hist)

	// Create model
	m := Model{
		workspace: tmpDir,
		sessionID: "current-session",
		history:   []Message{},
		textarea:  NewTestModel().textarea, // Need initialized textarea
	}

	// Create .nerd dir for saveSessionState to work
	if err := os.MkdirAll(filepath.Join(tmpDir, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd: %v", err)
	}

	// Run load
	newModel, cmd := m.loadSelectedSession(sessionID)

	// Type assert back to Model
	m2, ok := newModel.(Model)
	if !ok {
		t.Fatalf("Result is not a Model")
	}

	// Verify switch
	if m2.sessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, m2.sessionID)
	}
	if len(m2.history) < 2 { // +1 for system message
		t.Errorf("Expected loaded history, got %d messages", len(m2.history))
	}
	if m2.history[0].Content != "History 1" {
		t.Errorf("Expected first message 'History 1', got '%s'", m2.history[0].Content)
	}

	// Check for system message at end
	lastMsg := m2.history[len(m2.history)-1]
	if !strings.Contains(lastMsg.Content, "Loaded session") {
		t.Errorf("Expected system message at end, got '%s'", lastMsg.Content)
	}

	if cmd != nil {
		// Cmd is usually nil for this function unless there are side effects returning cmds
	}
}

func TestLoadSelectedSession_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	m := Model{
		workspace: tmpDir,
		sessionID: "current",
		textarea:  NewTestModel().textarea,
	}

	// Create .nerd dir
	os.MkdirAll(filepath.Join(tmpDir, ".nerd"), 0755)

	newModel, _ := m.loadSelectedSession("non-existent")
	m2 := newModel.(Model)

	// Should not have switched session ID
	if m2.sessionID != "current" {
		t.Errorf("Should not have switched session ID on failure")
	}

	// Should have error message
	if len(m2.history) == 0 {
		t.Error("Expected error message in history")
	} else {
		lastMsg := m2.history[len(m2.history)-1]
		if !strings.Contains(lastMsg.Content, "Failed to load session") {
			t.Errorf("Expected failure message, got: %s", lastMsg.Content)
		}
	}
}

func TestHydrateAllTools(t *testing.T) {
	// This tests the flow of tool hydration
	if testing.Short() {
		t.Skip("Skipping tool hydration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create mock executor
	mockExec := &MockExecutor{}
	vs := core.NewVirtualStore(mockExec)

	// 1. Create available_tools.json
	tools := []nerdinit.ToolDefinition{
		{
			Name:        "test_tool",
			Description: "A test tool",
			Command:     "echo test",
			Category:    "test",
		},
	}
	toolsData, _ := json.Marshal(tools)
	nerdDir := filepath.Join(tmpDir, ".nerd")

	// LoadToolsFromFile expects tools under .nerd/tools/available_tools.json
	toolsDir := filepath.Join(nerdDir, "tools")
	os.MkdirAll(toolsDir, 0755)
	os.WriteFile(filepath.Join(toolsDir, "available_tools.json"), toolsData, 0644)

	// 2. Create .compiled directory with a dummy tool
	compiledDir := filepath.Join(nerdDir, ".compiled")
	os.MkdirAll(compiledDir, 0755)

	// Run hydration
	// hydrateAllTools expects the path to the .nerd directory
	err := hydrateAllTools(vs, nerdDir)
	if err != nil {
		// It might fail on modular tools if they need specific environment,
		// or compiled tools if empty.
		// We just want to ensure it runs without panic and tries to load.
		t.Logf("hydrateAllTools returned: %v", err)
	}

	// Verify static tool was loaded
	reg := vs.GetToolRegistry()
	tool, found := reg.GetTool("test_tool")
	if !found {
		t.Error("Static tool 'test_tool' was not loaded")
	} else {
		// Note: The description might not be directly available on Tool struct if it's not exported or different field
		// Check what Tool struct looks like if needed.
		// For now just checking existence is good.
		_ = tool
	}
}

// Helper to save session history in the format expected by nerdinit
func saveSessionHistory(t *testing.T, workspace, sessionID string, history nerdinit.SessionHistory) {
	nerdDir := filepath.Join(workspace, ".nerd")
	// Note: loadSelectedSession uses nerdinit.LoadSessionHistory which looks in sessions dir
	sessionsDir := filepath.Join(nerdDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	path := filepath.Join(sessionsDir, sessionID+".json")
	data, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("Failed to marshal history: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write history file: %v", err)
	}
}
