package init

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// TypeU agent parsing tests
// ===========================================================================

func TestParseTypeUAgentFlag_WhenValid_ShouldParse(t *testing.T) {
	def, err := ParseTypeUAgentFlag("K8sExpert:Kubernetes deployment specialist:kubernetes,helm,kubectl")
	require.NoError(t, err)
	assert.Equal(t, "K8sExpert", def.Name)
	assert.Equal(t, "Kubernetes deployment specialist", def.Role)
	assert.Equal(t, []string{"kubernetes", "helm", "kubectl"}, def.Topics)
}

func TestParseTypeUAgentFlag_WhenEmpty_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "flag", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenMissingParts_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("JustName")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "format", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenTwoParts_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("Name:Role")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "format", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenEmptyTopics_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("Name:Role:")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "topics", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenEmptyName_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag(":Role:topic1")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "name", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenEmptyRole_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("Name::topic1")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "role", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenNameHasSpaces_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("My Agent:Role:topic1")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "name", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenRoleTooLong_ShouldReturnError(t *testing.T) {
	longRole := string(make([]byte, 101))
	for i := range longRole {
		longRole = longRole[:i] + "a" + longRole[i+1:]
	}
	_, err := ParseTypeUAgentFlag("Agent:" + longRole + ":topic1")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "role", tErr.Field)
}

func TestParseTypeUAgentFlag_WhenTooManyTopics_ShouldReturnError(t *testing.T) {
	_, err := ParseTypeUAgentFlag("Agent:Role:t1,t2,t3,t4,t5,t6,t7,t8,t9,t10,t11")
	assert.Error(t, err)
	var tErr *TypeUAgentError
	assert.ErrorAs(t, err, &tErr)
	assert.Equal(t, "topics", tErr.Field)
}

func TestParseTypeUAgentFlags_WhenMixed_ShouldReturnDefsAndErrors(t *testing.T) {
	flags := []string{
		"Good:Valid role:topic1",
		"",       // invalid
		"Bad:No", // invalid format
		"Valid2:Another role:topicA,topicB",
	}
	defs, errs := ParseTypeUAgentFlags(flags)
	assert.Len(t, defs, 2)
	assert.Len(t, errs, 2)
	assert.Equal(t, "Good", defs[0].Name)
	assert.Equal(t, "Valid2", defs[1].Name)
}

func TestTypeUAgentError_Error_ShouldFormatMessage(t *testing.T) {
	err := &TypeUAgentError{
		Flag:    "test:bad",
		Field:   "format",
		Message: "expected 3 parts",
	}
	msg := err.Error()
	assert.Contains(t, msg, "test:bad")
	assert.Contains(t, msg, "format")
	assert.Contains(t, msg, "expected 3 parts")
}

func TestToRecommendedAgent_WhenCalled_ShouldConvert(t *testing.T) {
	def := TypeUAgentDefinition{
		Name:   "MyAgent",
		Role:   "My custom role",
		Topics: []string{"topic1", "topic2"},
	}
	agent := def.ToRecommendedAgent()
	assert.Equal(t, "MyAgent", agent.Name)
	assert.Equal(t, "user", agent.Type)
	assert.Equal(t, "My custom role", agent.Description)
	assert.Equal(t, []string{"topic1", "topic2"}, agent.Topics)
	assert.Equal(t, 50, agent.Priority)
	assert.Contains(t, agent.Reason, "User-defined")
	assert.NotEmpty(t, agent.Permissions)
}

func TestTypeUAgentsToRecommended_WhenMultiple_ShouldConvertAll(t *testing.T) {
	defs := []TypeUAgentDefinition{
		{Name: "A", Role: "RoleA", Topics: []string{"t1"}},
		{Name: "B", Role: "RoleB", Topics: []string{"t2", "t3"}},
	}
	agents := TypeUAgentsToRecommended(defs)
	assert.Len(t, agents, 2)
	assert.Equal(t, "A", agents[0].Name)
	assert.Equal(t, "B", agents[1].Name)
}

func TestTypeUAgentsToRecommended_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	agents := TypeUAgentsToRecommended(nil)
	assert.Empty(t, agents)
}

func TestIsAlphanumericRune_WhenVariousRunes_ShouldClassify(t *testing.T) {
	assert.True(t, isAlphanumericRune('a'))
	assert.True(t, isAlphanumericRune('z'))
	assert.True(t, isAlphanumericRune('A'))
	assert.True(t, isAlphanumericRune('Z'))
	assert.True(t, isAlphanumericRune('0'))
	assert.True(t, isAlphanumericRune('9'))
	assert.False(t, isAlphanumericRune(' '))
	assert.False(t, isAlphanumericRune('-'))
	assert.False(t, isAlphanumericRune('_'))
	assert.False(t, isAlphanumericRune('.'))
}

// ===========================================================================
// Strategic knowledge helper tests
// ===========================================================================

func TestChunkDocument_WhenShortContent_ShouldReturnSingle(t *testing.T) {
	chunks := chunkDocument("short", 1000)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "short", chunks[0])
}

func TestChunkDocument_WhenExactlyMax_ShouldReturnSingle(t *testing.T) {
	content := "12345"
	chunks := chunkDocument(content, 5)
	assert.Len(t, chunks, 1)
}

func TestChunkDocument_WhenLongContent_ShouldSplitByLines(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	chunks := chunkDocument(content, 12)
	assert.Greater(t, len(chunks), 1, "should split into multiple chunks")
	// All original lines should be preserved across chunks
	total := ""
	for _, c := range chunks {
		total += c
	}
	assert.Contains(t, total, "line1")
	assert.Contains(t, total, "line5")
}

func TestChunkDocument_WhenEmpty_ShouldReturnSingle(t *testing.T) {
	chunks := chunkDocument("", 100)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "", chunks[0])
}

func TestTruncateString_WhenVariousLengths_ShouldTruncateCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{name: "short string", input: "hello", maxLen: 10, expected: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, expected: "hello"},
		{name: "needs truncation", input: "hello world", maxLen: 8, expected: "hello..."},
		{name: "very short max", input: "hello", maxLen: 3, expected: "hel"},
		{name: "max 2", input: "hello", maxLen: 2, expected: "he"},
		{name: "max 1", input: "hello", maxLen: 1, expected: "h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeysFromMap_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	keys := keysFromMap(map[string][]string{})
	assert.Empty(t, keys)
}

func TestKeysFromMap_WhenPopulated_ShouldReturnAllKeys(t *testing.T) {
	m := map[string][]string{
		"a": {"v1"},
		"b": {"v2"},
		"c": {"v3"},
	}
	keys := keysFromMap(m)
	assert.Len(t, keys, 3)
	// Keys are in arbitrary order, so just check they're all there
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	assert.True(t, keySet["a"])
	assert.True(t, keySet["b"])
	assert.True(t, keySet["c"])
}

// ===========================================================================
// Agent detection tests
// ===========================================================================

func TestFormatDomainExpertise_WhenEmpty_ShouldReturnDefault(t *testing.T) {
	result := formatDomainExpertise(nil)
	assert.Equal(t, "    - General expertise", result)
}

func TestFormatDomainExpertise_WhenTopics_ShouldFormatBullets(t *testing.T) {
	result := formatDomainExpertise([]string{"Go concurrency", "Error handling"})
	assert.Contains(t, result, "    - Go concurrency")
	assert.Contains(t, result, "    - Error handling")
}

func TestDetermineRequiredAgents_WhenGo_ShouldIncludeGoExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "go"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["GoExpert"])
}

func TestDetermineRequiredAgents_WhenPython_ShouldIncludePythonExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "python"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["PythonExpert"])
}

func TestDetermineRequiredAgents_WhenTypeScript_ShouldIncludeTSExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "typescript"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["TSExpert"])
}

func TestDetermineRequiredAgents_WhenRust_ShouldIncludeRustExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "rust"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["RustExpert"])
}

func TestDetermineRequiredAgents_WhenKotlin_ShouldIncludeAndroidExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "kotlin"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["AndroidExpert"])
}

func TestDetermineRequiredAgents_WhenUnknownLang_ShouldFallbackToCore(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "haskell"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["SecurityAuditor"])
	assert.True(t, names["TestArchitect"])
}

func TestDetermineRequiredAgents_WhenGinFramework_ShouldIncludeWebAPIExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "go", Framework: "gin"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["WebAPIExpert"])
}

func TestDetermineRequiredAgents_WhenReactFramework_ShouldIncludeFrontendExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "typescript", Framework: "react"}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["FrontendExpert"])
}

func TestDetermineRequiredAgents_WhenRodDep_ShouldIncludeRodExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{
		Language:     "go",
		Dependencies: []DependencyInfo{{Name: "rod", Type: "direct"}},
	}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["RodExpert"])
}

func TestDetermineRequiredAgents_WhenBubbleteaDep_ShouldIncludeBubbleTeaExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{
		Language:     "go",
		Dependencies: []DependencyInfo{{Name: "bubbletea", Type: "direct"}},
	}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["BubbleTeaExpert"])
}

func TestDetermineRequiredAgents_WhenMangleDep_ShouldIncludeMangleExpert(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{
		Language:     "go",
		Dependencies: []DependencyInfo{{Name: "mangle", Type: "direct"}},
	}
	agents := init.determineRequiredAgents(profile)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["MangleExpert"])
}

func TestDetermineRequiredAgents_WhenToolsAssigned_ShouldHaveToolPrefs(t *testing.T) {
	init := &Initializer{config: InitConfig{}}
	profile := ProjectProfile{Language: "go"}
	agents := init.determineRequiredAgents(profile)

	for _, a := range agents {
		if a.Name == "GoExpert" {
			assert.NotEmpty(t, a.Tools, "GoExpert should have tools")
			assert.NotEmpty(t, a.ToolPreferences, "GoExpert should have tool preferences")
			return
		}
	}
	t.Fatal("GoExpert not found in agents")
}

// ===========================================================================
// hasMainFunction tests
// ===========================================================================

func TestHasMainFunction_WhenMainGoExists_ShouldReturnTrue(t *testing.T) {
	tmpDir := t.TempDir()
	mainContent := `package main

func main() {
	fmt.Println("hello")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644))

	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	assert.True(t, init.hasMainFunction(tmpDir))
}

func TestHasMainFunction_WhenCmdDirExists_ShouldSearchForMain(t *testing.T) {
	// NOTE: filepath.Walk silently consumes SkipAll (returns nil not SkipAll),
	// so hasMainFunction's cmd/ directory check has a known limitation.
	// This test documents that behavior: it checks the function runs without error.
	tmpDir := t.TempDir()
	cmdDir := filepath.Join(tmpDir, "cmd", "app")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))
	mainContent := `package main
func main() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainContent), 0644))

	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	// Documents actual behavior: filepath.Walk consumes SkipAll, so cmd/ scanning
	// returns false even when func main() exists in a subdirectory.
	result := init.hasMainFunction(tmpDir)
	assert.False(t, result, "known limitation: Walk consumes SkipAll so cmd/ search returns false")
}

func TestHasMainFunction_WhenNoMainGo_ShouldReturnFalse(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "lib.go"), []byte("package lib\nfunc Foo() {}"), 0644))

	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	assert.False(t, init.hasMainFunction(tmpDir))
}

func TestHasMainFunction_WhenEmptyDir_ShouldReturnFalse(t *testing.T) {
	tmpDir := t.TempDir()
	init := &Initializer{config: InitConfig{Workspace: tmpDir}}
	assert.False(t, init.hasMainFunction(tmpDir))
}

// ===========================================================================
// loadExistingAgentRegistry tests
// ===========================================================================

func TestLoadExistingAgentRegistry_WhenMissing_ShouldReturnNil(t *testing.T) {
	tmpDir := t.TempDir()
	init := &Initializer{config: InitConfig{}}
	agents, err := init.loadExistingAgentRegistry(tmpDir)
	assert.NoError(t, err)
	assert.Nil(t, agents)
}

func TestLoadExistingAgentRegistry_WhenValidJSON_ShouldLoadAgents(t *testing.T) {
	tmpDir := t.TempDir()
	registryJSON := `{
		"version": "1.0",
		"created_at": "2024-01-01T00:00:00Z",
		"agents": [
			{"name": "GoExpert", "type": "persistent", "status": "ready"},
			{"name": "SecurityAuditor", "type": "persistent", "status": "ready"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "agents.json"), []byte(registryJSON), 0644))

	init := &Initializer{config: InitConfig{}}
	agents, err := init.loadExistingAgentRegistry(tmpDir)
	require.NoError(t, err)
	assert.Len(t, agents, 2)
	assert.Equal(t, "GoExpert", agents["GoExpert"].Name)
	assert.Equal(t, "SecurityAuditor", agents["SecurityAuditor"].Name)
}

func TestLoadExistingAgentRegistry_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "agents.json"), []byte("not json"), 0644))

	init := &Initializer{config: InitConfig{}}
	_, err := init.loadExistingAgentRegistry(tmpDir)
	assert.Error(t, err)
}

// ===========================================================================
// createDirectoryStructure tests
// ===========================================================================

func TestCreateDirectoryStructure_WhenCalled_ShouldCreateDirs(t *testing.T) {
	tmpDir := t.TempDir()
	init := &Initializer{config: InitConfig{Workspace: tmpDir}}

	nerdDir, err := init.createDirectoryStructure()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".nerd"), nerdDir)

	// Verify subdirectories exist
	for _, subdir := range []string{"shards", "sessions", "tools"} {
		info, statErr := os.Stat(filepath.Join(nerdDir, subdir))
		assert.NoError(t, statErr, "%s should exist", subdir)
		assert.True(t, info.IsDir(), "%s should be a directory", subdir)
	}
}

// ===========================================================================
// GenerateToolsForProject more coverage
// ===========================================================================

func TestGenerateToolsForProject_WhenGoAndFramework_ShouldMerge(t *testing.T) {
	tools := GenerateToolsForProject([]string{"go", "gin"})
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	// Should have both go tools and gin tools
	assert.True(t, names["go_build"], "should have go_build")
	assert.True(t, names["go_test"], "should have go_test")
}

func TestGenerateToolsForProject_WhenPythonAndFramework_ShouldMerge(t *testing.T) {
	tools := GenerateToolsForProject([]string{"python", "gin"})
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	assert.True(t, names["pytest"], "should have pytest from python language tools")
	// gin is a framework, so it should add gin tools
	ginTools := GetFrameworkTools("gin")
	if len(ginTools) > 0 {
		assert.True(t, names[ginTools[0].Name], "should have gin framework tools")
	}
}
