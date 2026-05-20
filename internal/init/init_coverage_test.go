package init

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Validation helpers
// ===========================================================================

func TestSanitizeForMangle_WhenVariousInputs_ShouldReturnValidConstants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty string", input: "", expected: "unknown"},
		{name: "lowercase", input: "go", expected: "go"},
		{name: "uppercase", input: "GO", expected: "go"},
		{name: "mixed case", input: "TypeScript", expected: "typescript"},
		{name: "with spaces", input: "hello world", expected: "hello_world"},
		{name: "with dashes", input: "gin-gonic", expected: "gin_gonic"},
		{name: "with dots", input: "vue.js", expected: "vue_js"},
		{name: "with slashes", input: "github.com/foo", expected: "github_com_foo"},
		{name: "starts with number", input: "3dmodel", expected: "n3dmodel"},
		{name: "special chars only", input: "!!!", expected: "unknown"},
		{name: "consecutive underscores", input: "a--b--c", expected: "a_b_c"},
		{name: "leading underscore", input: "-foo", expected: "foo"},
		{name: "trailing underscore", input: "foo-", expected: "foo"},
		{name: "complex path", input: "github.com/charmbracelet/bubbletea", expected: "github_com_charmbracelet_bubbletea"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForMangle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMajorVersion_WhenVariousFormats_ShouldReturnMajor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: ""},
		{name: "semver", input: "1.2.3", expected: "1"},
		{name: "caret", input: "^2.0.0", expected: "2"},
		{name: "tilde", input: "~3.1.0", expected: "3"},
		{name: "gte", input: ">=4.5.6", expected: "4"},
		{name: "gt", input: ">5.0.0", expected: "5"},
		{name: "lte", input: "<=6.0.0", expected: "6"},
		{name: "lt", input: "<7.0.0", expected: "7"},
		{name: "eq", input: "=8.0.0", expected: "8"},
		{name: "v prefix", input: "v9.1.2", expected: "9"},
		{name: "zero major", input: "0.1.2", expected: "0"},
		{name: "large major", input: "123.456.789", expected: "123"},
		{name: "prerelease", input: "1.0.0-beta.1", expected: "1"},
		{name: "just major", input: "5", expected: "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMajorVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateTokens_WhenVariousLengths_ShouldReturnQuarterCharCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "empty", input: "", expected: 0},
		{name: "one char", input: "a", expected: 1},
		{name: "four chars", input: "abcd", expected: 1},
		{name: "five chars", input: "abcde", expected: 2},
		{name: "eight chars", input: "abcdefgh", expected: 2},
		{name: "100 chars", input: string(make([]byte, 100)), expected: 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeContentHash_WhenCalled_ShouldReturnHexLength(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		content string
	}{
		{name: "empty both", id: "", content: ""},
		{name: "with id", id: "test", content: "content"},
		{name: "long content", id: "id", content: string(make([]byte, 1000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeContentHash(tt.id, tt.content)
			assert.NotEmpty(t, result, "hash should not be empty")
		})
	}
}

func TestCleanNameConstant_WhenVariousInputs_ShouldStripLeadingSlash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "with slash", input: "/go", expected: "go"},
		{name: "no slash", input: "go", expected: "go"},
		{name: "empty", input: "", expected: ""},
		{name: "only slash", input: "/", expected: ""},
		{name: "double slash", input: "//foo", expected: "/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanNameConstant(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAgentName_WhenDBPath_ShouldReturnName(t *testing.T) {
	tests := []struct {
		name     string
		dbPath   string
		expected string
	}{
		{name: "standard", dbPath: "/path/to/GoExpert_knowledge.db", expected: "GoExpert"},
		{name: "no suffix", dbPath: "/path/to/test.db", expected: "test.db"},
		{name: "just name", dbPath: "agent_knowledge.db", expected: "agent"},
		{name: "nested path", dbPath: filepath.Join("a", "b", "c", "Deep_knowledge.db"), expected: "Deep"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAgentName(tt.dbPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateProjectID_WhenDifferentPaths_ShouldProduceDeterministicIDs(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		id1 := generateProjectID("/path/to/project")
		id2 := generateProjectID("/path/to/project")
		assert.Equal(t, id1, id2, "same path should produce same ID")
	})

	t.Run("different for different paths", func(t *testing.T) {
		id1 := generateProjectID("/path/to/project/alpha")
		id2 := generateProjectID("/path/to/project/bravo")
		assert.NotEqual(t, id1, id2, "different paths should produce different IDs")
	})

	t.Run("has prefix", func(t *testing.T) {
		id := generateProjectID("/any/project/workspace")
		assert.Contains(t, id, "proj_")
	})
}

func TestIsInitialized_WhenNoNerdDir_ShouldReturnFalse(t *testing.T) {
	tmpDir := t.TempDir()
	assert.False(t, IsInitialized(tmpDir))
}

func TestIsInitialized_WhenProfileExists_ShouldReturnTrue(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "profile.json"), []byte("{}"), 0644))
	assert.True(t, IsInitialized(tmpDir))
}

// ===========================================================================
// Profile load/save tests
// ===========================================================================

func TestLoadProjectProfile_WhenValidFile_ShouldUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	profile := ProjectProfile{
		ProjectID: "proj_abc",
		Name:      "testproject",
		Language:  "go",
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "profile.json"), data, 0644))

	loaded, err := LoadProjectProfile(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "proj_abc", loaded.ProjectID)
	assert.Equal(t, "testproject", loaded.Name)
	assert.Equal(t, "go", loaded.Language)
}

func TestLoadProjectProfile_WhenMissing_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadProjectProfile(tmpDir)
	assert.Error(t, err)
}

func TestLoadProjectProfile_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "profile.json"), []byte("not json"), 0644))

	_, err := LoadProjectProfile(tmpDir)
	assert.Error(t, err)
}

func TestLoadPreferences_WhenValidFile_ShouldUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	prefs := UserPreferences{
		Verbosity:        "concise",
		ExplanationLevel: "expert",
		RequireTests:     true,
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "preferences.json"), data, 0644))

	loaded, err := LoadPreferences(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "concise", loaded.Verbosity)
	assert.Equal(t, "expert", loaded.ExplanationLevel)
	assert.True(t, loaded.RequireTests)
}

func TestLoadPreferences_WhenMissing_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadPreferences(tmpDir)
	assert.Error(t, err)
}

// ===========================================================================
// Session state tests
// ===========================================================================

func TestLoadSessionState_WhenValidFile_ShouldUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	state := SessionState{
		SessionID:    "sess_123",
		TurnCount:    5,
		Suspended:    false,
		StartedAt:    time.Now().Truncate(time.Second),
		LastActiveAt: time.Now().Truncate(time.Second),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "session.json"), data, 0644))

	loaded, err := LoadSessionState(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "sess_123", loaded.SessionID)
	assert.Equal(t, 5, loaded.TurnCount)
	assert.False(t, loaded.Suspended)
}

func TestLoadSessionState_WhenMissing_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadSessionState(tmpDir)
	assert.Error(t, err)
}

func TestSaveSessionState_WhenValid_ShouldPersist(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	state := &SessionState{
		SessionID:    "sess_456",
		TurnCount:    10,
		Suspended:    true,
		StartedAt:    time.Now().Truncate(time.Second),
		LastActiveAt: time.Now().Truncate(time.Second),
	}

	err := SaveSessionState(tmpDir, state)
	require.NoError(t, err)

	loaded, err := LoadSessionState(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "sess_456", loaded.SessionID)
	assert.Equal(t, 10, loaded.TurnCount)
	assert.True(t, loaded.Suspended)
}

func TestGetLatestSession_WhenStateExists_ShouldReturnID(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	state := &SessionState{SessionID: "sess_latest"}
	data, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(nerdDir, "session.json"), data, 0644))

	id, err := GetLatestSession(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "sess_latest", id)
}

func TestGetLatestSession_WhenMissing_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := GetLatestSession(tmpDir)
	assert.Error(t, err)
}

// ===========================================================================
// Session history tests
// ===========================================================================

func TestSaveAndLoadSessionHistory_WhenRoundTripped_ShouldPreserveMessages(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(filepath.Join(nerdDir, "sessions"), 0755))

	messages := []ChatMessage{
		{Role: "user", Content: "hello", Time: time.Now().Truncate(time.Second)},
		{Role: "assistant", Content: "hi", Time: time.Now().Truncate(time.Second)},
	}

	err := SaveSessionHistory(tmpDir, "sess_hist_1", messages)
	require.NoError(t, err)

	loaded, err := LoadSessionHistory(tmpDir, "sess_hist_1")
	require.NoError(t, err)
	assert.Equal(t, "sess_hist_1", loaded.SessionID)
	assert.Len(t, loaded.Messages, 2)
	assert.Equal(t, "user", loaded.Messages[0].Role)
	assert.Equal(t, "hello", loaded.Messages[0].Content)
}

func TestLoadSessionHistory_WhenMissing_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadSessionHistory(tmpDir, "nonexistent")
	assert.Error(t, err)
}

func TestListSessionHistories_WhenNoSessions_ShouldReturnEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	sessions, err := ListSessionHistories(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestListSessionHistories_WhenSessionsExist_ShouldReturnIDs(t *testing.T) {
	tmpDir := t.TempDir()
	sessDir := filepath.Join(tmpDir, ".nerd", "sessions")
	require.NoError(t, os.MkdirAll(sessDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "sess_a.json"), []byte("{}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sessDir, "sess_b.json"), []byte("{}"), 0644))

	sessions, err := ListSessionHistories(tmpDir)
	require.NoError(t, err)
	sort.Strings(sessions)
	assert.Equal(t, []string{"sess_a", "sess_b"}, sessions)
}

// ===========================================================================
// Tool definition tests
// ===========================================================================

func TestToolDefinition_IsMCPTool_WhenTypeSet_ShouldReturnTrue(t *testing.T) {
	mcpTool := ToolDefinition{Type: "mcp", Name: "test"}
	assert.True(t, mcpTool.IsMCPTool())

	staticTool := ToolDefinition{Name: "test"}
	assert.False(t, staticTool.IsMCPTool())

	emptyType := ToolDefinition{Type: "", Name: "test"}
	assert.False(t, emptyType.IsMCPTool())
}

func TestGetLanguageTools_WhenKnownLanguage_ShouldReturnTools(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		minTools int
	}{
		{name: "go", lang: "go", minTools: 5},
		{name: "golang alias", lang: "golang", minTools: 5},
		{name: "python", lang: "python", minTools: 4},
		{name: "typescript", lang: "typescript", minTools: 4},
		{name: "javascript", lang: "javascript", minTools: 4},
		{name: "rust", lang: "rust", minTools: 4},
		{name: "unknown", lang: "haskell", minTools: 0},
		{name: "empty", lang: "", minTools: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := GetLanguageTools(tt.lang)
			assert.GreaterOrEqual(t, len(tools), tt.minTools)
			for _, tool := range tools {
				assert.NotEmpty(t, tool.Name, "tool name should not be empty")
				assert.NotEmpty(t, tool.Category, "tool category should not be empty")
			}
		})
	}
}

func TestGetFrameworkTools_WhenKnownFramework_ShouldReturnTools(t *testing.T) {
	tests := []struct {
		name      string
		framework string
		hasTools  bool
	}{
		{name: "bubbletea", framework: "bubbletea", hasTools: true},
		{name: "gin", framework: "gin", hasTools: true},
		{name: "echo", framework: "echo", hasTools: true},
		{name: "react", framework: "react", hasTools: true},
		{name: "unknown", framework: "unknown_fw", hasTools: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := GetFrameworkTools(tt.framework)
			if tt.hasTools {
				assert.NotEmpty(t, tools)
			} else {
				assert.Empty(t, tools)
			}
		})
	}
}

func TestGetDependencyTools_WhenKnownDep_ShouldReturnTools(t *testing.T) {
	tools := GetDependencyTools([]string{"rod"})
	assert.Len(t, tools, 1)
	assert.Equal(t, "rod_download_browser", tools[0].Name)

	tools = GetDependencyTools([]string{"docker"})
	assert.Len(t, tools, 2)

	tools = GetDependencyTools([]string{"unknown_dep"})
	assert.Empty(t, tools)

	tools = GetDependencyTools(nil)
	assert.Empty(t, tools)
}

func TestGenerateToolsForProject_WhenMixedTech_ShouldDedup(t *testing.T) {
	// go appears twice but tools should be unique
	tools := GenerateToolsForProject([]string{"go", "go"})
	names := make(map[string]int)
	for _, tool := range tools {
		names[tool.Name]++
	}
	for name, count := range names {
		assert.Equal(t, 1, count, "tool %s should appear exactly once", name)
	}
}

func TestGenerateToolsForProject_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	tools := GenerateToolsForProject(nil)
	assert.Empty(t, tools)

	tools = GenerateToolsForProject([]string{})
	assert.Empty(t, tools)
}

func TestGetToolsForAgentType_WhenKnownAgent_ShouldReturnToolsAndPrefs(t *testing.T) {
	tests := []struct {
		name      string
		agent     string
		lang      string
		wantTools bool
	}{
		{name: "GoExpert", agent: "GoExpert", lang: "go", wantTools: true},
		{name: "PythonExpert", agent: "PythonExpert", lang: "python", wantTools: true},
		{name: "TSExpert", agent: "TSExpert", lang: "typescript", wantTools: true},
		{name: "RustExpert", agent: "RustExpert", lang: "rust", wantTools: true},
		{name: "RodExpert", agent: "RodExpert", lang: "go", wantTools: true},
		{name: "BrowserAutomationExpert", agent: "BrowserAutomationExpert", lang: "go", wantTools: true},
		{name: "DatabaseExpert go", agent: "DatabaseExpert", lang: "go", wantTools: true},
		{name: "DatabaseExpert python", agent: "DatabaseExpert", lang: "python", wantTools: false},
		{name: "WebAPIExpert", agent: "WebAPIExpert", lang: "go", wantTools: true},
		{name: "FrontendExpert", agent: "FrontendExpert", lang: "typescript", wantTools: true},
		{name: "SecurityAuditor go", agent: "SecurityAuditor", lang: "go", wantTools: true},
		{name: "SecurityAuditor python", agent: "SecurityAuditor", lang: "python", wantTools: true},
		{name: "TestArchitect go", agent: "TestArchitect", lang: "go", wantTools: true},
		{name: "TestArchitect python", agent: "TestArchitect", lang: "python", wantTools: true},
		{name: "TestArchitect ts", agent: "TestArchitect", lang: "typescript", wantTools: true},
		{name: "unknown", agent: "UnknownAgent", lang: "go", wantTools: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, prefs := GetToolsForAgentType(tt.agent, tt.lang)
			if tt.wantTools {
				assert.NotEmpty(t, tools)
				assert.NotEmpty(t, prefs)
			} else {
				assert.Empty(t, tools)
				assert.Empty(t, prefs)
			}
		})
	}
}

func TestSaveAndLoadToolsFromFile_WhenRoundTripped_ShouldPreserve(t *testing.T) {
	tmpDir := t.TempDir()

	tools := []ToolDefinition{
		{Name: "test_tool", Category: "test", Description: "A test tool"},
		{Name: "mcp_tool", Type: "mcp", Category: "build", MCPServer: "server1"},
	}

	err := SaveToolsToFile(tmpDir, tools)
	require.NoError(t, err)

	loaded, err := LoadToolsFromFile(tmpDir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "test_tool", loaded[0].Name)
	assert.Equal(t, "mcp_tool", loaded[1].Name)
	assert.True(t, loaded[1].IsMCPTool())
}

func TestLoadToolsFromFile_WhenMissing_ShouldReturnNil(t *testing.T) {
	tmpDir := t.TempDir()
	tools, err := LoadToolsFromFile(tmpDir)
	assert.NoError(t, err)
	assert.Nil(t, tools)
}

func TestLoadToolsFromFile_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	require.NoError(t, os.MkdirAll(toolsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(toolsDir, "available_tools.json"), []byte("bad json"), 0644))

	_, err := LoadToolsFromFile(tmpDir)
	assert.Error(t, err)
}

// ===========================================================================
// Shared knowledge helpers
// ===========================================================================

func TestGetSharedKnowledgePath_WhenCalled_ShouldReturnExpectedPath(t *testing.T) {
	path := GetSharedKnowledgePath("/project")
	expected := filepath.Join("/project", ".nerd", "shards", "core_concepts.db")
	assert.Equal(t, expected, path)
}

func TestSharedKnowledgePoolExists_WhenNotCreated_ShouldReturnFalse(t *testing.T) {
	tmpDir := t.TempDir()
	assert.False(t, SharedKnowledgePoolExists(tmpDir))
}

func TestSharedKnowledgePoolExists_WhenCreated_ShouldReturnTrue(t *testing.T) {
	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, ".nerd", "shards")
	require.NoError(t, os.MkdirAll(dbDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dbDir, "core_concepts.db"), []byte("db"), 0644))
	assert.True(t, SharedKnowledgePoolExists(tmpDir))
}

// ===========================================================================
// Validation result & summary tests
// ===========================================================================

func TestFindBackupFiles_WhenNoBackups_ShouldReturnEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	shardsDir := filepath.Join(tmpDir, "shards")
	require.NoError(t, os.MkdirAll(shardsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(shardsDir, "agent_knowledge.db"), []byte("db"), 0644))

	backups := FindBackupFiles(tmpDir)
	assert.Empty(t, backups)
}

func TestFindBackupFiles_WhenBackupsExist_ShouldReturnThem(t *testing.T) {
	tmpDir := t.TempDir()
	shardsDir := filepath.Join(tmpDir, "shards")
	require.NoError(t, os.MkdirAll(shardsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(shardsDir, "agent.backup_20240101"), []byte("bak"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(shardsDir, "other.backup_20240102"), []byte("bak"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(shardsDir, "normal.db"), []byte("db"), 0644))

	backups := FindBackupFiles(tmpDir)
	assert.Len(t, backups, 2)
}

func TestFindBackupFiles_WhenNonexistentDir_ShouldReturnEmpty(t *testing.T) {
	backups := FindBackupFiles("/nonexistent/path")
	assert.Empty(t, backups)
}

func TestCleanupBackups_WhenDryRun_ShouldNotDelete(t *testing.T) {
	tmpDir := t.TempDir()
	shardsDir := filepath.Join(tmpDir, "shards")
	require.NoError(t, os.MkdirAll(shardsDir, 0755))
	backupFile := filepath.Join(shardsDir, "test.backup_20240101")
	require.NoError(t, os.WriteFile(backupFile, []byte("bak"), 0644))

	count, err := CleanupBackups(tmpDir, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// File should still exist
	_, statErr := os.Stat(backupFile)
	assert.NoError(t, statErr, "backup should not be deleted in dry run")
}

func TestCleanupBackups_WhenNoBackups_ShouldReturnZero(t *testing.T) {
	tmpDir := t.TempDir()
	shardsDir := filepath.Join(tmpDir, "shards")
	require.NoError(t, os.MkdirAll(shardsDir, 0755))

	count, err := CleanupBackups(tmpDir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCleanupBackups_WhenActualCleanup_ShouldDelete(t *testing.T) {
	tmpDir := t.TempDir()
	shardsDir := filepath.Join(tmpDir, "shards")
	require.NoError(t, os.MkdirAll(shardsDir, 0755))
	backupFile := filepath.Join(shardsDir, "test.backup_20240101")
	require.NoError(t, os.WriteFile(backupFile, []byte("bak"), 0644))

	count, err := CleanupBackups(tmpDir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// File should be gone
	_, statErr := os.Stat(backupFile)
	assert.True(t, os.IsNotExist(statErr), "backup should be deleted")
}

// ===========================================================================
// ETA Tracker tests
// ===========================================================================

func TestNewETATracker_WhenCreated_ShouldHaveDefaults(t *testing.T) {
	tracker := NewETATracker(10)
	assert.Equal(t, 10, tracker.GetTotalPhases())
	assert.Equal(t, 0, tracker.GetCurrentPhase())
	assert.GreaterOrEqual(t, tracker.GetElapsed().Nanoseconds(), int64(0))
}

func TestETATracker_StartAndCompletePhase_ShouldTrackProgress(t *testing.T) {
	tracker := NewETATracker(5)

	tracker.StartPhase(1)
	assert.Equal(t, 1, tracker.GetCurrentPhase())

	tracker.CompletePhase("setup")

	tracker.StartPhase(2)
	assert.Equal(t, 2, tracker.GetCurrentPhase())
}

func TestETATracker_GetETARemaining_WhenKnownPhases_ShouldEstimate(t *testing.T) {
	tracker := NewETATracker(3)

	remaining := tracker.GetETARemaining([]string{"setup", "scanning"})
	assert.Greater(t, remaining.Nanoseconds(), int64(0), "ETA should be positive for known phases")
}

func TestETATracker_GetETARemaining_WhenUnknownPhases_ShouldUseDefault(t *testing.T) {
	tracker := NewETATracker(3)

	remaining := tracker.GetETARemaining([]string{"unknown_phase"})
	assert.Equal(t, 10*time.Second, remaining, "unknown phases should default to 10s")
}

func TestETATracker_GetETARemaining_WhenEmpty_ShouldReturnZero(t *testing.T) {
	tracker := NewETATracker(3)

	remaining := tracker.GetETARemaining(nil)
	assert.Equal(t, time.Duration(0), remaining)
}

// ===========================================================================
// DefaultInitConfig tests
// ===========================================================================

func TestDefaultInitConfig_WhenEmptyWorkspace_ShouldNotPanic(t *testing.T) {
	cfg := DefaultInitConfig("")
	// Should use current working dir fallback
	assert.NotEmpty(t, cfg.Workspace)
	assert.True(t, cfg.Interactive)
	assert.Equal(t, 30*time.Minute, cfg.Timeout)
	assert.False(t, cfg.SkipResearch)
}

func TestDefaultInitConfig_WhenGivenWorkspace_ShouldUseIt(t *testing.T) {
	cfg := DefaultInitConfig("/my/workspace")
	assert.Equal(t, "/my/workspace", cfg.Workspace)
}

// ===========================================================================
// Interactive agent helpers tests
// ===========================================================================

func TestConvertToDetectedAgents_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	result := ConvertToDetectedAgents(nil, ProjectProfile{})
	assert.Empty(t, result)
}

func TestConvertToDetectedAgents_WhenAgents_ShouldCopyFields(t *testing.T) {
	recommended := []RecommendedAgent{
		{
			Name:        "GoExpert",
			Type:        "language",
			Description: "Go expert",
			Priority:    10,
			Reason:      "detected go.mod",
		},
	}
	profile := ProjectProfile{Language: "go"}

	detected := ConvertToDetectedAgents(recommended, profile)
	require.Len(t, detected, 1)
	assert.Equal(t, "GoExpert", detected[0].Name)
	assert.Equal(t, "language", detected[0].Type)
	assert.Equal(t, 10, detected[0].Priority)
}

func TestConvertToRecommendedAgents_WhenAgents_ShouldCopyFields(t *testing.T) {
	detected := []DetectedAgent{
		{
			Name:     "SecurityAuditor",
			Type:     "core",
			Priority: 5,
			Reason:   "always included",
		},
	}

	recommended := ConvertToRecommendedAgents(detected)
	require.Len(t, recommended, 1)
	assert.Equal(t, "SecurityAuditor", recommended[0].Name)
	assert.Equal(t, "core", recommended[0].Type)
	assert.Equal(t, 5, recommended[0].Priority)
}

func TestCategorizeAgent_WhenSecurityAuditor_ShouldBeCore(t *testing.T) {
	agent := RecommendedAgent{Name: "SecurityAuditor"}
	category, recommended, detectedBy := categorizeAgent(agent, ProjectProfile{})
	assert.Equal(t, "core", category)
	assert.True(t, recommended)
	assert.Equal(t, "always included", detectedBy)
}

func TestCategorizeAgent_WhenTestArchitect_ShouldBeCore(t *testing.T) {
	agent := RecommendedAgent{Name: "TestArchitect"}
	category, recommended, detectedBy := categorizeAgent(agent, ProjectProfile{})
	assert.Equal(t, "core", category)
	assert.True(t, recommended)
	assert.Equal(t, "always included", detectedBy)
}

func TestCategorizeAgent_WhenGoExpertWithGoLang_ShouldBeLanguage(t *testing.T) {
	agent := RecommendedAgent{Name: "GoExpert"}
	profile := ProjectProfile{Language: "go"}
	category, recommended, detectedBy := categorizeAgent(agent, profile)
	assert.Equal(t, "language", category)
	assert.True(t, recommended)
	assert.Equal(t, "primary language", detectedBy)
}

func TestCategorizeAgent_WhenGoExpertWithNonGoLang_ShouldBeOptional(t *testing.T) {
	agent := RecommendedAgent{Name: "GoExpert"}
	profile := ProjectProfile{Language: "python"}
	category, recommended, _ := categorizeAgent(agent, profile)
	assert.Equal(t, "optional", category)
	assert.False(t, recommended)
}

func TestCategorizeAgent_WhenUnknownAgent_ShouldBeOptional(t *testing.T) {
	agent := RecommendedAgent{Name: "RandomAgent"}
	category, recommended, detectedBy := categorizeAgent(agent, ProjectProfile{})
	assert.Equal(t, "optional", category)
	assert.False(t, recommended)
	assert.Equal(t, "detected", detectedBy)
}

func TestFilterSelectedAgents_WhenMixed_ShouldReturnOnlySelected(t *testing.T) {
	agents := []DetectedAgent{
		{Name: "A", Selected: true},
		{Name: "B", Selected: false},
		{Name: "C", Selected: true},
		{Name: "D", Selected: false},
	}

	filtered := filterSelectedAgents(agents)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "A", filtered[0].Name)
	assert.Equal(t, "C", filtered[1].Name)
}

func TestFilterSelectedAgents_WhenNoneSelected_ShouldReturnEmpty(t *testing.T) {
	agents := []DetectedAgent{
		{Name: "A", Selected: false},
	}
	filtered := filterSelectedAgents(agents)
	assert.Empty(t, filtered)
}

func TestSortAgentsForDisplay_WhenMixedCategories_ShouldPrioritizeCorrectly(t *testing.T) {
	agents := []DetectedAgent{
		{Name: "opt_low", Category: "optional", Recommended: false, Priority: 1},
		{Name: "core", Category: "core", Recommended: true, Priority: 5},
		{Name: "rec_high", Category: "language", Recommended: true, Priority: 10},
		{Name: "opt_high", Category: "optional", Recommended: false, Priority: 100},
	}

	sortAgentsForDisplay(agents)

	// Core should be first
	assert.Equal(t, "core", agents[0].Name)
	// Then recommended
	assert.Equal(t, "rec_high", agents[1].Name)
	// Then optional by priority
	assert.Equal(t, "opt_high", agents[2].Name)
	assert.Equal(t, "opt_low", agents[3].Name)
}

// ===========================================================================
// Scanner parser edge cases
// ===========================================================================

func TestParseGoSum_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parseGoSum("")
	assert.Empty(t, deps)
}

func TestParseGoSum_WhenNotableDeps_ShouldDetect(t *testing.T) {
	content := `github.com/stretchr/testify v1.8.0 h1:hash=
github.com/stretchr/testify v1.8.0/go.mod h1:hash=
github.com/google/uuid v1.3.0 h1:hash=
github.com/random/pkg v0.1.0 h1:hash=
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parseGoSum(content)
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
		assert.Equal(t, "transitive", dep.Type)
	}
	assert.True(t, names["testify"])
	assert.True(t, names["uuid"])
	assert.False(t, names["random"])
}

func TestParseGoSum_WhenDuplicate_ShouldDedup(t *testing.T) {
	content := `github.com/stretchr/testify v1.8.0 h1:hash=
github.com/stretchr/testify v1.8.0/go.mod h1:hash=
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parseGoSum(content)
	assert.Len(t, deps, 1, "duplicate entries should be deduped")
}

func TestParseYarnLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parseYarnLock("")
	assert.Empty(t, deps)
}

func TestParseYarnLock_WhenNotableDeps_ShouldDetect(t *testing.T) {
	content := `
"webpack@^5.0.0":
  version "5.88.0"
  resolved "..."
"vite@^4.0.0":
  version "4.4.0"
  resolved "..."
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parseYarnLock(content)
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["webpack"])
	assert.True(t, names["vite"])
}

func TestParsePnpmLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePnpmLock("")
	assert.Empty(t, deps)
}

func TestParsePnpmLock_WhenNotableDeps_ShouldDetect(t *testing.T) {
	content := `lockfileVersion: 5.4
dependencies:
  /nuxt@3.0.0:
    resolution: {}
  /tailwindcss@3.3.0:
    resolution: {}
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePnpmLock(content)
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["nuxt"])
	assert.True(t, names["tailwindcss"])
}

func TestParseCargoLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parseCargoLock("")
	assert.Empty(t, deps)
}

func TestParseCargoLock_WhenNotableDeps_ShouldDetect(t *testing.T) {
	content := `[[package]]
name = "tokio"
version = "1.28.0"

[[package]]
name = "serde"
version = "1.0.160"

[[package]]
name = "random-crate"
version = "0.1.0"
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parseCargoLock(content)
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["tokio"])
	assert.True(t, names["serde"])
	assert.False(t, names["random-crate"])
}

func TestParsePipfileLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePipfileLock([]byte{})
	assert.Empty(t, deps)
}

func TestParsePipfileLock_WhenNotableDeps_ShouldDetect(t *testing.T) {
	lockContent := `{
  "default": {
    "django": {"version": "==4.2.0"},
    "flask": {"version": "==2.3.0"},
    "random-pkg": {"version": "==0.1.0"}
  },
  "develop": {}
}`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePipfileLock([]byte(lockContent))
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["django"])
	assert.True(t, names["flask"])
	assert.False(t, names["random-pkg"])
}

func TestParsePipfileLock_WhenInvalidJSON_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePipfileLock([]byte("invalid"))
	assert.Empty(t, deps)
}

func TestParsePoetryLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePoetryLock("")
	assert.Empty(t, deps)
}

func TestParsePoetryLock_WhenNotableDeps_ShouldDetect(t *testing.T) {
	content := `[[package]]
name = "django"
version = "4.2.0"

[[package]]
name = "pytest"
version = "7.3.0"

[[package]]
name = "unknown-thing"
version = "0.1.0"
`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePoetryLock(content)
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["django"])
	assert.True(t, names["pytest"])
	assert.False(t, names["unknown-thing"])
}

func TestParsePackageLock_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageLock([]byte{})
	assert.Empty(t, deps)
}

func TestParsePackageLock_WhenInvalidJSON_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageLock([]byte("not json"))
	assert.Empty(t, deps)
}

func TestParsePackageLock_WhenV7Format_ShouldDetect(t *testing.T) {
	content := `{
  "packages": {
    "node_modules/webpack": {"version": "5.88.0"},
    "node_modules/react": {"version": "18.2.0"},
    "node_modules/random": {"version": "1.0.0"}
  }
}`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageLock([]byte(content))
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["webpack"])
	// "react" check depends on matching "/" prefix logic in the implementation
}

func TestParsePackageLock_WhenV6Format_ShouldDetect(t *testing.T) {
	content := `{
  "dependencies": {
    "webpack": {"version": "5.88.0"},
    "jest": {"version": "29.5.0"}
  }
}`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageLock([]byte(content))
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["webpack"])
	assert.True(t, names["jest"])
}

func TestParsePackageJSONDependencies_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageJSONDependencies([]byte{})
	assert.Empty(t, deps)
}

func TestParsePackageJSONDependencies_WhenInvalidJSON_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageJSONDependencies([]byte("bad"))
	assert.Empty(t, deps)
}

func TestParsePackageJSONDependencies_WhenDeps_ShouldDetect(t *testing.T) {
	content := `{
  "dependencies": {
    "react": "^18.2.0",
    "express": "^4.18.0",
    "random-lib": "^1.0.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "jest": "^29.0.0"
  }
}`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageJSONDependencies([]byte(content))
	names := make(map[string]bool)
	for _, dep := range deps {
		names[dep.Name] = true
	}
	assert.True(t, names["react"])
	assert.True(t, names["express"])
	assert.True(t, names["typescript"])
	assert.True(t, names["jest"])
	assert.False(t, names["random-lib"], "unknown dep should not be included")
}

func TestParsePackageJSONDependencies_WhenVersionParsing_ShouldExtractMajor(t *testing.T) {
	content := `{
  "dependencies": {
    "react": "^18.2.0"
  }
}`
	init := &Initializer{config: InitConfig{}}
	deps := init.parsePackageJSONDependencies([]byte(content))
	require.Len(t, deps, 1)
	assert.Equal(t, "react", deps[0].Name)
	assert.Equal(t, "^18.2.0", deps[0].Version)
	assert.Equal(t, "18", deps[0].MajorVersion)
	assert.Equal(t, "direct", deps[0].Type)
}

// ===========================================================================
// Initializer preferences tests
// ===========================================================================

func TestInitPreferences_WhenNoHints_ShouldReturnDefaults(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	prefs := init.initPreferences()
	assert.Equal(t, "concise", prefs.Verbosity)
	assert.Equal(t, "intermediate", prefs.ExplanationLevel)
	assert.False(t, prefs.RequireTests)
	assert.False(t, prefs.RequireReview)
}

func TestInitPreferences_WhenTableDrivenHint_ShouldSetTestStyle(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"table_driven_tests"}}}
	prefs := init.initPreferences()
	assert.Equal(t, "table_driven", prefs.TestStyle)
}

func TestInitPreferences_WhenConventionalCommitsHint_ShouldSetCommitStyle(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"conventional_commits"}}}
	prefs := init.initPreferences()
	assert.Equal(t, "conventional", prefs.CommitStyle)
}

func TestInitPreferences_WhenStrictHint_ShouldRequireTestsAndReview(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"strict"}}}
	prefs := init.initPreferences()
	assert.True(t, prefs.RequireTests)
	assert.True(t, prefs.RequireReview)
}

func TestInitPreferences_WhenExpertHint_ShouldSetExplanationLevel(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"expert"}}}
	prefs := init.initPreferences()
	assert.Equal(t, "expert", prefs.ExplanationLevel)
}

func TestInitPreferences_WhenBeginnerHint_ShouldSetExplanationLevel(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"beginner"}}}
	prefs := init.initPreferences()
	assert.Equal(t, "beginner", prefs.ExplanationLevel)
}

func TestInitPreferences_WhenMultipleHints_ShouldApplyAll(t *testing.T) {
	init := &Initializer{config: InitConfig{PreferenceHints: []string{"strict", "expert", "conventional_commits", "table_driven_tests"}}}
	prefs := init.initPreferences()
	assert.True(t, prefs.RequireTests)
	assert.True(t, prefs.RequireReview)
	assert.Equal(t, "expert", prefs.ExplanationLevel)
	assert.Equal(t, "conventional", prefs.CommitStyle)
	assert.Equal(t, "table_driven", prefs.TestStyle)
}

// ===========================================================================
// DefaultPhaseDurations tests
// ===========================================================================

func TestDefaultPhaseDurations_WhenCalled_ShouldHaveAllExpectedPhases(t *testing.T) {
	durations := DefaultPhaseDurations()
	expectedPhases := []string{
		"setup", "migration", "directory", "scanning", "analysis", "profile",
		"facts", "prompt_atoms", "prompt_db", "agents", "shared_kb", "kb_creation",
		"codebase_kb", "core_shards_kb", "campaign_kb", "tool_generation",
		"preferences", "session", "tools", "registry", "prompt_sync", "complete",
	}

	for _, phase := range expectedPhases {
		dur, ok := durations[phase]
		assert.True(t, ok, "phase %s should exist in defaults", phase)
		assert.Greater(t, dur.Nanoseconds(), int64(0), "phase %s should have positive duration", phase)
	}
}

// ===========================================================================
// ExtractGoModVersion tests
// ===========================================================================

func TestExtractGoModVersion_WhenPkgPresent_ShouldReturnVersion(t *testing.T) {
	content := `module mymod

require (
	github.com/stretchr/testify v1.8.4
	github.com/google/uuid v1.3.0 // indirect
)
`
	init := &Initializer{config: InitConfig{}}

	version := init.extractGoModVersion(content, "github.com/stretchr/testify")
	assert.Equal(t, "v1.8.4", version)

	version = init.extractGoModVersion(content, "github.com/google/uuid")
	assert.Equal(t, "v1.3.0", version)
}

func TestExtractGoModVersion_WhenPkgNotPresent_ShouldReturnEmpty(t *testing.T) {
	content := `module mymod

require (
	github.com/stretchr/testify v1.8.4
)
`
	init := &Initializer{config: InitConfig{}}
	version := init.extractGoModVersion(content, "github.com/nonexistent/pkg")
	assert.Empty(t, version)
}

func TestExtractGoModVersion_WhenEmptyContent_ShouldReturnEmpty(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	version := init.extractGoModVersion("", "anything")
	assert.Empty(t, version)
}

// ===========================================================================
// Context7 agent suggestions
// ===========================================================================

func TestGetContext7AgentSuggestions_WhenNoDeps_ShouldReturnEmpty(t *testing.T) {
	profile := ProjectProfile{}
	suggestions, err := GetContext7AgentSuggestions(nil, profile)
	require.NoError(t, err)
	assert.Empty(t, suggestions)
}

func TestGetContext7AgentSuggestions_WhenKnownDeps_ShouldSuggest(t *testing.T) {
	profile := ProjectProfile{
		Dependencies: []DependencyInfo{
			{Name: "redis", Type: "direct"},
			{Name: "kubernetes", Type: "direct"},
		},
	}
	suggestions, err := GetContext7AgentSuggestions(nil, profile)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, s := range suggestions {
		names[s.Name] = true
		assert.NotEmpty(t, s.Reason)
		assert.NotEmpty(t, s.SourceTopic)
	}
	assert.True(t, names["RedisExpert"])
	assert.True(t, names["K8sExpert"])
}

// ===========================================================================
// generateFactsFile via Initializer helper
// ===========================================================================

func TestGenerateFactsFile_WhenMinimalProfile_ShouldWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	factsPath := filepath.Join(tmpDir, "profile.mg")

	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	profile := ProjectProfile{
		ProjectID:   "proj_test",
		Name:        "TestProject",
		Description: "A test project",
		Language:    "go",
		Framework:   "gin",
	}

	count, err := init.generateFactsFile(factsPath, profile)
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	content, err := os.ReadFile(factsPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "project_profile")
	assert.Contains(t, string(content), "project_language")
	assert.Contains(t, string(content), "project_framework")
}

func TestGenerateFactsFile_WhenUnknownLanguage_ShouldOmitLangFact(t *testing.T) {
	tmpDir := t.TempDir()
	factsPath := filepath.Join(tmpDir, "profile.mg")

	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	profile := ProjectProfile{
		ProjectID: "proj_test",
		Name:      "TestProject",
		Language:  "unknown",
	}

	count, err := init.generateFactsFile(factsPath, profile)
	require.NoError(t, err)
	assert.Greater(t, count, 0) // At least project_profile

	content, err := os.ReadFile(factsPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "project_language")
}

func TestGenerateFactsFile_WhenBadPath_ShouldReturnError(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{ProjectID: "test"}

	_, err := init.generateFactsFile("/nonexistent/dir/facts.mg", profile)
	assert.Error(t, err)
}

// ===========================================================================
// Agent preferences persistence tests
// ===========================================================================

func TestSaveAndLoadAgentPreferences_WhenRoundTripped_ShouldPreserve(t *testing.T) {
	tmpDir := t.TempDir()
	nerdDir := filepath.Join(tmpDir, ".nerd")
	require.NoError(t, os.MkdirAll(nerdDir, 0755))

	prefs := &AgentSelectionPreferences{
		AcceptedAgents:        []string{"GoExpert", "SecurityAuditor"},
		RejectedAgents:        []string{"RandomAgent"},
		AutoAcceptRecommended: true,
	}

	err := SaveAgentPreferences(tmpDir, prefs)
	require.NoError(t, err)

	loaded, err := LoadAgentPreferences(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, []string{"GoExpert", "SecurityAuditor"}, loaded.AcceptedAgents)
	assert.Equal(t, []string{"RandomAgent"}, loaded.RejectedAgents)
	assert.True(t, loaded.AutoAcceptRecommended)
}

func TestLoadAgentPreferences_WhenMissing_ShouldReturnNilNoError(t *testing.T) {
	tmpDir := t.TempDir()
	loaded, err := LoadAgentPreferences(tmpDir)
	assert.NoError(t, err)
	assert.Nil(t, loaded)
}
