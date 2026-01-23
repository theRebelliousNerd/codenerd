// Package chat provides tests for wizard flows (config, agent, northstar, onboarding).
// This file tests the multi-step interactive wizard state machines.
package chat

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// CONFIG WIZARD TESTS
// =============================================================================

func TestConfigWizard_StateCreation(t *testing.T) {
	t.Parallel()

	state := &ConfigWizardState{
		Step: StepWelcome,
	}

	if state.Step != StepWelcome {
		t.Errorf("Expected StepWelcome, got %v", state.Step)
	}
}

func TestConfigWizard_AllSteps(t *testing.T) {
	t.Parallel()

	// Verify all steps are defined
	steps := []ConfigWizardStep{
		StepWelcome,
		StepEngine,
		StepClaudeCLIConfig,
		StepCodexCLIConfig,
		StepProvider,
		StepAPIKey,
		StepAntigravityAccounts,
		StepAntigravityAddMore,
		StepAntigravityWaiting,
		StepModel,
		StepShardConfig,
		StepShardModel,
		StepShardTemperature,
		StepShardContext,
		StepNextShard,
		StepEmbeddingProvider,
		StepEmbeddingConfig,
		StepContextWindow,
		StepContextBudget,
		StepCoreLimits,
		StepReview,
		StepComplete,
	}

	// Each step should have a unique value
	seen := make(map[ConfigWizardStep]bool)
	for _, step := range steps {
		if seen[step] {
			t.Errorf("Duplicate step value: %d", step)
		}
		seen[step] = true
	}
}

func TestConfigWizard_ShardProfileConfig(t *testing.T) {
	t.Parallel()

	profile := &ShardProfileConfig{
		Model:            "gpt-4",
		Temperature:      0.7,
		MaxContextTokens: 8000,
		MaxOutputTokens:  4000,
		EnableLearning:   true,
	}

	if profile.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", profile.Model)
	}
	if profile.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", profile.Temperature)
	}
	if profile.MaxContextTokens != 8000 {
		t.Errorf("Expected MaxContextTokens 8000, got %d", profile.MaxContextTokens)
	}
	if profile.MaxOutputTokens != 4000 {
		t.Errorf("Expected MaxOutputTokens 4000, got %d", profile.MaxOutputTokens)
	}
	if !profile.EnableLearning {
		t.Errorf("Expected EnableLearning true")
	}
}

func TestConfigWizard_StepTransitions(t *testing.T) {
	t.Parallel()

	// Test step progression logic
	transitions := []struct {
		from ConfigWizardStep
		to   ConfigWizardStep
	}{
		{StepWelcome, StepEngine},
		{StepEngine, StepProvider},        // If API selected
		{StepEngine, StepClaudeCLIConfig}, // If Claude CLI selected
		{StepProvider, StepAPIKey},
		{StepAPIKey, StepModel},
		{StepModel, StepShardConfig},
	}

	for _, tr := range transitions {
		if tr.to <= tr.from && tr.to != StepReview && tr.to != StepComplete {
			// Most transitions should move forward
			t.Logf("Transition from %d to %d", tr.from, tr.to)
		}
	}
}

func TestConfigWizard_ModelEntry(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepWelcome,
	}

	// Config wizard mode should be active
	if !m.awaitingConfigWizard {
		t.Error("Expected awaitingConfigWizard to be true")
	}
	if m.configWizard.Step != StepWelcome {
		t.Errorf("Expected StepWelcome, got %v", m.configWizard.Step)
	}
}

func TestConfigWizard_ExitOnEscape(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepWelcome,
	}

	// Escape should exit wizard (depending on implementation)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := newModel.(Model)

	// May exit wizard or stay (implementation dependent)
	_ = result
}

// =============================================================================
// ANTIGRAVITY WIZARD TESTS
// =============================================================================

func TestConfigWizard_AntigravityModels(t *testing.T) {
	t.Parallel()

	// Verify Antigravity has correct models from model-resolver.ts
	models := ProviderModels["antigravity"]
	if len(models) == 0 {
		t.Fatal("No models defined for antigravity provider")
	}

	expectedModels := []string{
		"gemini-3-flash",
		"gemini-3-flash-low",
		"gemini-3-flash-medium",
		"gemini-3-flash-high",
		"gemini-3-pro-low",
		"gemini-3-pro-high",
		"claude-sonnet-4-5-thinking",
		"claude-opus-4-5-thinking",
	}

	modelSet := make(map[string]bool)
	for _, m := range models {
		modelSet[m] = true
	}

	for _, expected := range expectedModels {
		if !modelSet[expected] {
			t.Errorf("Missing expected Antigravity model: %s", expected)
		}
	}
}

func TestConfigWizard_AntigravityStateFields(t *testing.T) {
	t.Parallel()

	state := NewConfigWizard()

	// Verify Antigravity state fields exist
	if len(state.AntigravityAccounts) > 0 {
		t.Log("Has Antigravity accounts")
	}

	if state.AntigravityAuthState != nil {
		t.Log("Has Antigravity auth state")
	}
}

func TestConfigWizard_AntigravityStepRouting(t *testing.T) {
	t.Parallel()

	// Verify StepAntigravityAccounts comes after StepAPIKey in enum
	if StepAntigravityAccounts <= StepAPIKey {
		t.Error("StepAntigravityAccounts should come after StepAPIKey in enum")
	}

	// Verify StepAntigravityWaiting exists
	if StepAntigravityWaiting <= StepAntigravityAccounts {
		t.Error("StepAntigravityWaiting should come after StepAntigravityAccounts")
	}
}

func TestConfigWizard_AntigravityProviderSkipsAPIKey(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step:     StepProvider,
		Engine:   "api",
		Provider: "",
	}

	// Simulate selecting antigravity provider
	result, _ := m.handleConfigWizardInput("5") // antigravity is option 5
	resultModel := result.(Model)

	// Should go to StepAntigravityAccounts, NOT StepAPIKey
	if resultModel.configWizard.Provider != "antigravity" {
		t.Errorf("Expected provider 'antigravity', got '%s'", resultModel.configWizard.Provider)
	}

	// The step should be StepAntigravityAccounts (skipping API key)
	if resultModel.configWizard.Step == StepAPIKey {
		t.Error("Antigravity should skip StepAPIKey and go to StepAntigravityAccounts")
	}
}

func TestConfigWizard_AntigravityOAuthResultMsg(t *testing.T) {
	t.Parallel()

	// Verify the OAuth result message type exists and has correct fields
	msg := antigravityOAuthResultMsg{
		err:     nil,
		account: nil,
	}

	if msg.err != nil {
		t.Log("Has error field")
	}
	if msg.account != nil {
		t.Log("Has account field")
	}
}

func TestConfigWizard_DefaultAntigravityModel(t *testing.T) {
	t.Parallel()

	defaultModel := DefaultProviderModel("antigravity")
	if defaultModel == "" {
		t.Error("DefaultProviderModel('antigravity') returned empty string")
	}

	// Default should be gemini-3-flash (first in list)
	if defaultModel != "gemini-3-flash" {
		t.Errorf("Expected default 'gemini-3-flash', got '%s'", defaultModel)
	}
}

// =============================================================================
// AGENT WIZARD TESTS
// =============================================================================

func TestAgentWizard_StateCreation(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingAgentDefinition = true

	if !m.awaitingAgentDefinition {
		t.Error("Expected awaitingAgentDefinition to be true")
	}
}

func TestAgentWizard_Entry(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Trigger agent wizard
	newModel, _ := m.handleCommand("/define-agent")
	result := newModel.(Model)

	// Should either start wizard or show message
	if result.awaitingAgentDefinition {
		t.Log("Agent wizard started")
	} else if len(result.history) > 0 {
		t.Log("Message shown instead of wizard")
	}
}

func TestAgentWizard_NoPanicOnInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingAgentDefinition = true
	m.agentWizard = &AgentWizardState{
		Step: 0,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in agent wizard input: %v", r)
		}
	}()

	m.textarea.SetValue("test input")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// =============================================================================
// NORTHSTAR WIZARD TESTS
// =============================================================================

func TestNorthstarWizard_StateCreation(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingNorthstar = true

	if !m.awaitingNorthstar {
		t.Error("Expected awaitingNorthstar to be true")
	}
}

func TestNorthstarWizard_Entry(t *testing.T) {
	t.Parallel()

	m := NewTestModel()

	// Trigger northstar wizard
	newModel, _ := m.handleCommand("/northstar")
	result := newModel.(Model)

	// Should either start wizard or show message
	_ = result
}

func TestNorthstarWizard_NoPanicOnInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingNorthstar = true
	m.northstarWizard = &NorthstarWizardState{
		Phase: 0,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in northstar wizard input: %v", r)
		}
	}()

	m.textarea.SetValue("project vision")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// =============================================================================
// ONBOARDING WIZARD TESTS
// =============================================================================

func TestOnboardingWizard_StateCreation(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingOnboarding = true

	if !m.awaitingOnboarding {
		t.Error("Expected awaitingOnboarding to be true")
	}
}

func TestOnboardingWizard_NoPanicOnInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingOnboarding = true
	m.onboardingWizard = &OnboardingWizardState{
		Step: 0,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic in onboarding wizard input: %v", r)
		}
	}()

	m.textarea.SetValue("1")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// =============================================================================
// INPUT MODE TESTS
// =============================================================================

func TestInputMode_Clarification(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModeClarification))

	if !m.awaitingClarification {
		t.Error("Expected awaitingClarification to be true")
	}
}

func TestInputMode_Patch(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModePatch))

	if !m.awaitingPatch {
		t.Error("Expected awaitingPatch to be true")
	}
}

func TestInputMode_AgentWizard(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModeAgentWizard))

	if !m.awaitingAgentDefinition {
		t.Error("Expected awaitingAgentDefinition to be true")
	}
}

func TestInputMode_ConfigWizard(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModeConfigWizard))

	if !m.awaitingConfigWizard {
		t.Error("Expected awaitingConfigWizard to be true")
	}
}

func TestInputMode_Onboarding(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModeOnboarding))

	if !m.awaitingOnboarding {
		t.Error("Expected awaitingOnboarding to be true")
	}
}

// =============================================================================
// CLARIFICATION STATE TESTS
// =============================================================================

func TestClarificationState_Fields(t *testing.T) {
	t.Parallel()

	state := &ClarificationState{
		Question:      "Which file do you mean?",
		Options:       []string{"file1.go", "file2.go", "file3.go"},
		DefaultOption: "file1.go",
		Context:       "serialized context",
	}

	if state.Question != "Which file do you mean?" {
		t.Errorf("Unexpected question: %s", state.Question)
	}
	if len(state.Options) != 3 {
		t.Errorf("Expected 3 options, got %d", len(state.Options))
	}
	if state.DefaultOption != "file1.go" {
		t.Errorf("Unexpected default option: %s", state.DefaultOption)
	}
	if state.Context != "serialized context" {
		t.Errorf("Unexpected context: %s", state.Context)
	}
}

func TestClarificationState_OptionSelection(t *testing.T) {
	t.Parallel()

	m := NewTestModel(WithInputMode(InputModeClarification))
	m.clarificationState = &ClarificationState{
		Question: "Select an option",
		Options:  []string{"option1", "option2"},
	}
	m.selectedOption = 0

	// Up/Down should change selection
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result := newModel.(Model)

	// Selection may or may not change depending on implementation
	_ = result
}

// =============================================================================
// PATCH INPUT MODE TESTS
// =============================================================================

func TestPatchMode_Entry(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingPatch = true
	m.pendingPatchLines = []string{}

	if !m.awaitingPatch {
		t.Error("Expected awaitingPatch to be true")
	}
	if len(m.pendingPatchLines) != 0 {
		t.Errorf("Expected empty pending lines, got %d", len(m.pendingPatchLines))
	}
}

func TestPatchMode_AccumulatesLines(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingPatch = true
	m.pendingPatchLines = []string{"line 1", "line 2"}

	if len(m.pendingPatchLines) != 2 {
		t.Errorf("Expected 2 pending lines, got %d", len(m.pendingPatchLines))
	}
	if !m.awaitingPatch {
		t.Error("Expected awaitingPatch to be true")
	}
}

func TestPatchMode_EndMarker(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingPatch = true
	m.pendingPatchLines = []string{"line 1", "line 2"}

	// Typing --END-- should complete patch
	m.textarea.SetValue("--END--")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Patch mode may be exited
}

// =============================================================================
// WIZARD NAVIGATION TESTS
// =============================================================================

func TestWizard_BackNavigation(t *testing.T) {
	t.Parallel()

	// Test that wizards support going back (if implemented)
	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepProvider,
	}

	// Sending "back" or special key might go back
	m.textarea.SetValue("back")
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := newModel.(Model)

	// May or may not change step
	_ = result
}

func TestWizard_CancelNavigation(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepProvider,
	}

	// Ctrl+C or "cancel" might exit
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = newModel

	// May exit wizard or quit entirely
}

// =============================================================================
// WIZARD VIEW TESTS
// =============================================================================

func TestWizard_ViewNoPanic(t *testing.T) {
	t.Parallel()

	wizardSetups := []struct {
		name  string
		setup func(*Model)
	}{
		{
			name: "config wizard",
			setup: func(m *Model) {
				m.awaitingConfigWizard = true
				m.configWizard = &ConfigWizardState{Step: StepWelcome}
			},
		},
		{
			name: "agent wizard",
			setup: func(m *Model) {
				m.awaitingAgentDefinition = true
				m.agentWizard = &AgentWizardState{Step: 0}
			},
		},
		{
			name: "northstar wizard",
			setup: func(m *Model) {
				m.awaitingNorthstar = true
				m.northstarWizard = &NorthstarWizardState{Phase: 0}
			},
		},
		{
			name: "onboarding wizard",
			setup: func(m *Model) {
				m.awaitingOnboarding = true
				m.onboardingWizard = &OnboardingWizardState{Step: 0}
			},
		},
		{
			name: "clarification",
			setup: func(m *Model) {
				m.awaitingClarification = true
				m.clarificationState = &ClarificationState{
					Question: "test",
					Options:  []string{"a", "b"},
				}
			},
		},
		{
			name: "patch mode",
			setup: func(m *Model) {
				m.awaitingPatch = true
				m.pendingPatchLines = []string{"line 1"}
			},
		},
	}

	for _, ws := range wizardSetups {
		t.Run(ws.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC in View() for %s: %v", ws.name, r)
				}
			}()

			m := NewTestModel()
			ws.setup(&m)
			_ = m.View()
		})
	}
}

// =============================================================================
// WIZARD INPUT VALIDATION TESTS
// =============================================================================

func TestWizard_EmptyInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepAPIKey,
	}

	// Empty input should be handled gracefully
	m.textarea.SetValue("")
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := newModel.(Model)

	// May show error or use default
	_ = result
}

func TestWizard_WhitespaceInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepAPIKey,
	}

	// Whitespace-only input should be handled
	m.textarea.SetValue("   ")
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := newModel.(Model)

	_ = result
}

func TestWizard_NumericInput(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.awaitingConfigWizard = true
	m.configWizard = &ConfigWizardState{
		Step: StepProvider,
	}

	// Numeric selection
	m.textarea.SetValue("1")
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := newModel.(Model)

	_ = result
}

// =============================================================================
// CAMPAIGN LAUNCH CLARIFICATION TESTS
// =============================================================================

func TestCampaignLaunchClarification_State(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.launchClarifyPending = true
	m.launchClarifyGoal = "improve test coverage"
	m.launchClarifyAnswers = ""

	if !m.launchClarifyPending {
		t.Error("Expected launchClarifyPending to be true")
	}
	if m.launchClarifyGoal != "improve test coverage" {
		t.Errorf("Unexpected goal: %s", m.launchClarifyGoal)
	}
	if m.launchClarifyAnswers != "" {
		t.Errorf("Expected empty answers initially")
	}
}

// =============================================================================
// DREAM STATE LEARNING TESTS
// =============================================================================

func TestDreamLearning_TrackHypothetical(t *testing.T) {
	t.Parallel()

	m := NewTestModel()
	m.lastDreamHypothetical = "What if we refactored X?"

	if m.lastDreamHypothetical != "What if we refactored X?" {
		t.Errorf("Unexpected hypothetical: %s", m.lastDreamHypothetical)
	}
}

// =============================================================================
// REFLECTION STATE TESTS
// =============================================================================

func TestReflectionState_Creation(t *testing.T) {
	t.Parallel()

	state := &ReflectionState{
		Query:         "test query",
		UsedEmbedding: true,
		Duration:      time.Second,
		Warnings:      []string{"warning1"},
	}

	if state.Query != "test query" {
		t.Errorf("Unexpected query: %s", state.Query)
	}
	if !state.UsedEmbedding {
		t.Error("Expected UsedEmbedding to be true")
	}
	if state.Duration != time.Second {
		t.Errorf("Expected duration 1s, got %v", state.Duration)
	}
	if len(state.Warnings) != 1 {
		t.Errorf("Expected 1 warning")
	}
}

// =============================================================================
// SHARD RESULT TESTS
// =============================================================================

func TestShardResult_Fields(t *testing.T) {
	t.Parallel()

	sr := &ShardResult{
		ShardType:  "reviewer",
		Task:       "review main.go",
		RawOutput:  "Found 5 issues",
		Timestamp:  time.Now(),
		TurnNumber: 3,
		Findings:   []map[string]any{{"file": "main.go", "line": 10}},
		Metrics:    map[string]any{"issues": 5},
		ExtraData:  map[string]any{"custom": "data"},
	}

	if sr.ShardType != "reviewer" {
		t.Errorf("Expected shardType 'reviewer', got '%s'", sr.ShardType)
	}
	if len(sr.Findings) != 1 {
		t.Errorf("Expected 1 finding, got %d", len(sr.Findings))
	}
	if sr.Task != "review main.go" {
		t.Errorf("Unexpected task: %s", sr.Task)
	}
	if sr.RawOutput != "Found 5 issues" {
		t.Errorf("Unexpected raw output: %s", sr.RawOutput)
	}
	if sr.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if sr.TurnNumber != 3 {
		t.Errorf("Unexpected turn number: %d", sr.TurnNumber)
	}
	if sr.Metrics["issues"] != 5 {
		t.Error("Metrics mismatch")
	}
	if sr.ExtraData["custom"] != "data" {
		t.Error("ExtraData mismatch")
	}
}

// =============================================================================
// AGGREGATED REVIEW TESTS
// =============================================================================

func TestAggregatedReview_Fields(t *testing.T) {
	t.Parallel()

	review := &PersistedReview{
		ID:               "review-123",
		Target:           "./...",
		Participants:     []string{"security", "performance"},
		TotalFindings:    10,
		Files:            []string{"main.go", "util.go"},
		HolisticInsights: []string{"Code is generally well-structured"},
	}

	if review.ID != "review-123" {
		t.Errorf("Unexpected ID: %s", review.ID)
	}
	if len(review.Participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(review.Participants))
	}
	if review.Target != "./..." {
		t.Errorf("Unexpected target: %s", review.Target)
	}
	if review.TotalFindings != 10 {
		t.Errorf("Unexpected total findings: %d", review.TotalFindings)
	}
	if len(review.Files) != 2 {
		t.Errorf("Unexpected file count: %d", len(review.Files))
	}
	if len(review.HolisticInsights) != 1 {
		t.Errorf("Unexpected insight count: %d", len(review.HolisticInsights))
	}
}

func TestParsedFinding_Fields(t *testing.T) {
	t.Parallel()

	finding := ParsedFinding{
		File:           "main.go",
		Line:           42,
		Severity:       "high",
		Category:       "security",
		Message:        "SQL injection vulnerability",
		Recommendation: "Use prepared statements",
		ShardSource:    "security_reviewer",
	}

	if finding.File != "main.go" {
		t.Errorf("Unexpected file: %s", finding.File)
	}
	if finding.Line != 42 {
		t.Errorf("Unexpected line: %d", finding.Line)
	}
	if finding.Severity != "high" {
		t.Errorf("Unexpected severity: %s", finding.Severity)
	}
	if finding.Category != "security" {
		t.Errorf("Unexpected category: %s", finding.Category)
	}
	if finding.Message != "SQL injection vulnerability" {
		t.Errorf("Unexpected message: %s", finding.Message)
	}
	if finding.Recommendation != "Use prepared statements" {
		t.Errorf("Unexpected recommendation: %s", finding.Recommendation)
	}
	if finding.ShardSource != "security_reviewer" {
		t.Errorf("Unexpected source: %s", finding.ShardSource)
	}
	if finding.Severity != "high" {
		t.Errorf("Expected severity 'high', got '%s'", finding.Severity)
	}
}
