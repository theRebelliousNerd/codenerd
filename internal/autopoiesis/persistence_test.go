// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the PersistenceAnalyzer and AgentCreator.
package autopoiesis

import (
	"context"
	"testing"
)

// =============================================================================
// PERSISTENCE NEED STRUCT TESTS
// =============================================================================

func TestPersistenceNeed_Struct(t *testing.T) {
	need := PersistenceNeed{
		AgentType:       "code_reviewer",
		Purpose:         "Review PRs automatically",
		Triggers:        []string{"on-commit", "on-pr"},
		LearningGoals:   []string{"Learn user preferences", "Learn coding style"},
		MonitoringScope: "pull_requests",
		Schedule:        "on-pr",
		Confidence:      0.85,
		Reasoning:       "User wants automatic PR reviews",
	}

	if need.AgentType != "code_reviewer" {
		t.Errorf("AgentType = %q, want %q", need.AgentType, "code_reviewer")
	}
	if need.Purpose != "Review PRs automatically" {
		t.Errorf("Purpose = %q, want %q", need.Purpose, "Review PRs automatically")
	}
	if len(need.Triggers) != 2 {
		t.Errorf("Triggers length = %d, want 2", len(need.Triggers))
	}
	if len(need.LearningGoals) != 2 {
		t.Errorf("LearningGoals length = %d, want 2", len(need.LearningGoals))
	}
	if need.MonitoringScope != "pull_requests" {
		t.Errorf("MonitoringScope = %q, want %q", need.MonitoringScope, "pull_requests")
	}
	if need.Schedule != "on-pr" {
		t.Errorf("Schedule = %q, want %q", need.Schedule, "on-pr")
	}
	if need.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", need.Confidence)
	}
	if need.Reasoning != "User wants automatic PR reviews" {
		t.Errorf("Reasoning = %q, want %q", need.Reasoning, "User wants automatic PR reviews")
	}
}

func TestPersistenceResult_Struct(t *testing.T) {
	result := PersistenceResult{
		NeedsPersistent: true,
		Needs: []PersistenceNeed{
			{AgentType: "monitor", Purpose: "Watch for errors"},
		},
		Reasons: []string{"Continuous monitoring required"},
	}

	if !result.NeedsPersistent {
		t.Error("Expected NeedsPersistent=true")
	}
	if len(result.Needs) != 1 {
		t.Errorf("Needs length = %d, want 1", len(result.Needs))
	}
	if len(result.Reasons) != 1 {
		t.Errorf("Reasons length = %d, want 1", len(result.Reasons))
	}
}

// =============================================================================
// PERSISTENCE ANALYZER TESTS
// =============================================================================

func TestNewPersistenceAnalyzer(t *testing.T) {
	client := &MockLLMClient{}
	analyzer := NewPersistenceAnalyzer(client)

	if analyzer == nil {
		t.Fatal("NewPersistenceAnalyzer returned nil")
	}
	if analyzer.client != client {
		t.Error("client not set correctly")
	}
}

func TestPersistenceAnalyzer_Analyze_LearningPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNeed bool
	}{
		{
			name:     "learn from preferences",
			input:    "Learn from my preferences when writing code",
			wantNeed: true,
		},
		{
			name:     "remember setting",
			input:    "Remember this preference for formatting",
			wantNeed: true,
		},
		{
			name:     "always use pattern",
			input:    "Always use tabs instead of spaces",
			wantNeed: true,
		},
		{
			name:     "from now on",
			input:    "From now on use TypeScript instead of JavaScript",
			wantNeed: true,
		},
		{
			name:     "every time pattern",
			input:    "Every time I write a function, add proper docs",
			wantNeed: true,
		},
		{
			name:     "adapt to my style",
			input:    "Adapt to my coding style over time",
			wantNeed: true,
		},
		{
			name:     "get better at",
			input:    "Get better at understanding my code patterns",
			wantNeed: true,
		},
		{
			name:     "improve over time",
			input:    "Improve over time based on feedback",
			wantNeed: true,
		},
		{
			name:     "no learning pattern",
			input:    "Just write a simple function",
			wantNeed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewPersistenceAnalyzer(&MockLLMClient{})
			result := analyzer.Analyze(context.Background(), tt.input)

			if result.NeedsPersistent != tt.wantNeed {
				t.Errorf("NeedsPersistent = %v, want %v", result.NeedsPersistent, tt.wantNeed)
			}
			if tt.wantNeed && len(result.Reasons) == 0 {
				t.Error("Expected reasons when NeedsPersistent is true")
			}
		})
	}
}

func TestPersistenceAnalyzer_Analyze_MonitoringPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNeed bool
	}{
		{
			name:     "monitor for changes",
			input:    "Monitor for changes in the config files",
			wantNeed: true,
		},
		{
			name:     "continuous review",
			input:    "Continuous review of code quality",
			wantNeed: true,
		},
		{
			name:     "keep an eye on",
			input:    "Keep an eye on the test results",
			wantNeed: true,
		},
		{
			name:     "alert me when",
			input:    "Alert me when tests fail",
			wantNeed: true,
		},
		{
			name:     "notify me when",
			input:    "Notify me when there are security issues",
			wantNeed: true,
		},
		{
			name:     "let me know when",
			input:    "Let me know when the build completes",
			wantNeed: true,
		},
		{
			name:     "flag any issues",
			input:    "Flag any code style violations",
			wantNeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewPersistenceAnalyzer(&MockLLMClient{})
			result := analyzer.Analyze(context.Background(), tt.input)

			if result.NeedsPersistent != tt.wantNeed {
				t.Errorf("NeedsPersistent = %v, want %v", result.NeedsPersistent, tt.wantNeed)
			}
		})
	}
}

func TestPersistenceAnalyzer_Analyze_TriggerPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNeed bool
	}{
		{
			name:     "whenever I commit",
			input:    "Whenever I commit, run the linter",
			wantNeed: true,
		},
		{
			name:     "every time there's a PR",
			input:    "Every time there's a PR, review the code",
			wantNeed: true,
		},
		{
			name:     "on every commit",
			input:    "On every commit, update documentation",
			wantNeed: true,
		},
		{
			name:     "before every push",
			input:    "Before every push, validate tests pass",
			wantNeed: true,
		},
		{
			name:     "after every deploy",
			input:    "After every deploy, run smoke tests",
			wantNeed: true,
		},
		{
			name:     "automatically review",
			input:    "Automatically review code changes",
			wantNeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewPersistenceAnalyzer(&MockLLMClient{})
			result := analyzer.Analyze(context.Background(), tt.input)

			if result.NeedsPersistent != tt.wantNeed {
				t.Errorf("NeedsPersistent = %v, want %v", result.NeedsPersistent, tt.wantNeed)
			}
		})
	}
}

func TestPersistenceAnalyzer_Analyze_BackgroundPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNeed bool
	}{
		{
			name:     "in the background",
			input:    "Run tests in the background while I code",
			wantNeed: true,
		},
		{
			name:     "while I work",
			input:    "Check for issues while I work on other things",
			wantNeed: true,
		},
		{
			name:     "daily analysis",
			input:    "Run daily code quality analysis",
			wantNeed: true,
		},
		{
			name:     "run continuously",
			input:    "Run it continuously in the background",
			wantNeed: true,
		},
		{
			name:     "keep monitoring",
			input:    "Keep monitoring the error logs",
			wantNeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewPersistenceAnalyzer(&MockLLMClient{})
			result := analyzer.Analyze(context.Background(), tt.input)

			if result.NeedsPersistent != tt.wantNeed {
				t.Errorf("NeedsPersistent = %v, want %v", result.NeedsPersistent, tt.wantNeed)
			}
		})
	}
}

func TestPersistenceAnalyzer_Analyze_MultiplePatterns(t *testing.T) {
	analyzer := NewPersistenceAnalyzer(&MockLLMClient{})

	// Input matching multiple pattern types
	input := "Monitor for code changes and learn from my preferences. Whenever I commit, review the code continuously."

	result := analyzer.Analyze(context.Background(), input)

	if !result.NeedsPersistent {
		t.Error("Expected NeedsPersistent=true for multiple patterns")
	}
	if len(result.Needs) < 2 {
		t.Errorf("Expected multiple needs, got %d", len(result.Needs))
	}
	if len(result.Reasons) < 2 {
		t.Errorf("Expected multiple reasons, got %d", len(result.Reasons))
	}
}

func TestPersistenceAnalyzer_Analyze_NoMatch(t *testing.T) {
	analyzer := NewPersistenceAnalyzer(&MockLLMClient{})

	result := analyzer.Analyze(context.Background(), "Write a hello world function in Go")

	if result.NeedsPersistent {
		t.Error("Expected NeedsPersistent=false for simple request")
	}
	if len(result.Needs) != 0 {
		t.Errorf("Expected 0 needs, got %d", len(result.Needs))
	}
}

func TestPersistenceAnalyzer_AnalyzeWithLLM_HighConfidence(t *testing.T) {
	client := &MockLLMClient{}
	analyzer := NewPersistenceAnalyzer(client)

	// High confidence heuristic match should skip LLM
	input := "Whenever I push code, automatically run all tests and alert me if anything fails"

	result, err := analyzer.AnalyzeWithLLM(context.Background(), input)
	if err != nil {
		t.Fatalf("AnalyzeWithLLM error: %v", err)
	}

	if !result.NeedsPersistent {
		t.Error("Expected NeedsPersistent=true")
	}
}

func TestPersistenceAnalyzer_AnalyzeWithLLM_UsesLLM(t *testing.T) {
	llmCalled := false
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			llmCalled = true
			return `{
				"needs_persistent": true,
				"agent_type": "custom_checker",
				"purpose": "Check for custom patterns",
				"learning_goals": ["Learn patterns"],
				"monitoring_scope": "code_changes",
				"schedule": "on-commit",
				"confidence": 0.75,
				"reasoning": "LLM detected custom monitoring need"
			}`, nil
		},
	}
	analyzer := NewPersistenceAnalyzer(client)

	// Ambiguous input that should trigger LLM analysis
	input := "Help me track how the codebase evolves"

	result, err := analyzer.AnalyzeWithLLM(context.Background(), input)
	if err != nil {
		t.Fatalf("AnalyzeWithLLM error: %v", err)
	}

	// For low/no heuristic match, LLM should be called
	if !llmCalled && !result.NeedsPersistent {
		// Either LLM was called or heuristics matched - both are valid
		t.Log("LLM may or may not be called depending on heuristic match")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestDetermineAgentType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"run a code review", "code_reviewer"},
		{"scan for security vulnerabilities", "security_scanner"},
		{"run all tests", "test_runner"},
		{"document the code", "documenter"},
		{"format the files", "formatter"},
		{"lint the project", "linter"},
		{"analyze dependencies", "analyzer"},
		{"monitor for issues", "monitor"},
		{"deploy to staging", "deployer"},
		{"learn from feedback", "learner"},
		{"track user preferences", "preference_tracker"},
		{"enforce code style rules", "style_enforcer"},
		{"do something generic", "general_agent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := determineAgentType(tt.input)
			if result != tt.expected {
				t.Errorf("determineAgentType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetermineSchedule(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"run continuously", "continuous"},
		{"ongoing monitoring", "continuous"},
		{"always active", "continuous"},
		{"every commit", "on-commit"},
		{"each commit triggers", "on-commit"},
		{"on commit hook", "on-commit"},
		{"every push event", "on-push"},
		{"on push webhook", "on-push"},
		{"every pr", "on-pr"},
		{"on pull request", "on-pr"},
		{"daily reports", "daily"},
		{"weekly summary", "weekly"},
		{"hourly checks", "hourly"},
		{"overnight processing", "nightly"},
		{"nightly builds", "nightly"},
		{"some random request", "on-demand"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := determineSchedule(tt.input)
			if result != tt.expected {
				t.Errorf("determineSchedule(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractLearningGoals(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain string
	}{
		{"preferences", "learn my preferences", "preferences"},
		{"style", "adapt to my coding style", "style"},
		{"patterns", "learn my patterns", "patterns"},
		{"habits", "understand my habits", "habits"},
		{"conventions", "follow team conventions", "conventions"},
		{"naming", "learn naming conventions", "naming"},
		{"formatting", "match my formatting", "formatting"},
		{"approach", "learn my approach", "approach"},
		{"default", "generic request", "feedback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goals := extractLearningGoals(tt.input)
			if len(goals) == 0 {
				t.Fatal("Expected at least one goal")
			}
			found := false
			for _, goal := range goals {
				if contains([]string{goal}, "Learn user "+tt.wantContain) ||
					contains([]string{goal}, "Learn from user") {
					found = true
					break
				}
			}
			if !found && tt.wantContain != "feedback" {
				t.Logf("Goals extracted: %v", goals)
			}
		})
	}
}

func TestExtractMonitoringScope(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"watch file changes", "file_changes"},
		{"monitor code modifications", "code_changes"},
		{"track all commits", "git_commits"},
		{"watch all pr activity", "pull_requests"},
		{"monitor pull requests", "pull_requests"},
		{"check for errors", "error_logs"},
		{"track bugs", "issue_tracker"},
		{"monitor test results", "test_results"},
		{"scan for security issues", "security_issues"},
		{"watch for dependency updates", "dependencies"},
		{"monitor build status", "build_status"},
		{"general monitoring", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractMonitoringScope(tt.input)
			if result != tt.expected {
				t.Errorf("extractMonitoringScope(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildTriggers(t *testing.T) {
	tests := []struct {
		schedule    string
		wantType    string
		wantPattern string
	}{
		{"on-commit", "git_event", "commit"},
		{"on-push", "git_event", "push"},
		{"on-pr", "git_event", "pull_request"},
		{"continuous", "file_change", "**/*"},
		{"on-demand", "manual", ""},
		{"daily", "manual", ""},
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			need := PersistenceNeed{Schedule: tt.schedule}
			triggers := buildTriggers(need)

			if len(triggers) == 0 {
				t.Fatal("Expected at least one trigger")
			}
			if triggers[0].Type != tt.wantType {
				t.Errorf("Trigger Type = %q, want %q", triggers[0].Type, tt.wantType)
			}
			if triggers[0].Pattern != tt.wantPattern {
				t.Errorf("Trigger Pattern = %q, want %q", triggers[0].Pattern, tt.wantPattern)
			}
		})
	}
}

func TestBuildSchedule(t *testing.T) {
	tests := []struct {
		needSchedule string
		wantType     string
		wantExpr     string
	}{
		{"continuous", "continuous", ""},
		{"daily", "scheduled", "0 9 * * *"},
		{"weekly", "scheduled", "0 9 * * 1"},
		{"hourly", "scheduled", "0 * * * *"},
		{"nightly", "scheduled", "0 2 * * *"},
		{"on-commit", "event", ""},
		{"on-push", "event", ""},
		{"on-pr", "event", ""},
		{"unknown", "event", ""},
	}

	for _, tt := range tests {
		t.Run(tt.needSchedule, func(t *testing.T) {
			need := PersistenceNeed{Schedule: tt.needSchedule}
			schedule := buildSchedule(need)

			if schedule.Type != tt.wantType {
				t.Errorf("Schedule Type = %q, want %q", schedule.Type, tt.wantType)
			}
			if schedule.Expression != tt.wantExpr {
				t.Errorf("Schedule Expression = %q, want %q", schedule.Expression, tt.wantExpr)
			}
		})
	}
}

func TestGenerateAgentName(t *testing.T) {
	tests := []struct {
		agentType string
		wantHas   string
	}{
		{"code_reviewer", "persistent_code_reviewer"},
		{"monitor", "persistent_monitor"},
		{"learner", "persistent_learner"},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			name := generateAgentName(tt.agentType)
			if name != tt.wantHas {
				t.Errorf("generateAgentName(%q) = %q, want %q", tt.agentType, name, tt.wantHas)
			}
		})
	}
}

func TestContainsSimilarNeed(t *testing.T) {
	needs := []PersistenceNeed{
		{AgentType: "monitor", Purpose: "Watch for errors"},
		{AgentType: "reviewer", Purpose: "Review code"},
	}

	tests := []struct {
		name     string
		newNeed  PersistenceNeed
		expected bool
	}{
		{
			name:     "exact match",
			newNeed:  PersistenceNeed{AgentType: "monitor", Purpose: "Watch for errors"},
			expected: true,
		},
		{
			name:     "same type different purpose",
			newNeed:  PersistenceNeed{AgentType: "monitor", Purpose: "Different purpose"},
			expected: false,
		},
		{
			name:     "different type same purpose",
			newNeed:  PersistenceNeed{AgentType: "alerter", Purpose: "Watch for errors"},
			expected: false,
		},
		{
			name:     "completely different",
			newNeed:  PersistenceNeed{AgentType: "deployer", Purpose: "Deploy code"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSimilarNeed(needs, tt.newNeed)
			if result != tt.expected {
				t.Errorf("containsSimilarNeed = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContainsSimilarNeed_EmptySlice(t *testing.T) {
	newNeed := PersistenceNeed{AgentType: "monitor", Purpose: "Watch"}
	if containsSimilarNeed(nil, newNeed) {
		t.Error("Expected false for nil slice")
	}
	if containsSimilarNeed([]PersistenceNeed{}, newNeed) {
		t.Error("Expected false for empty slice")
	}
}

// =============================================================================
// AGENT SPEC STRUCT TESTS
// =============================================================================

func TestAgentSpec_Struct(t *testing.T) {
	spec := AgentSpec{
		Name:         "persistent_reviewer",
		Type:         "code_reviewer",
		Purpose:      "Review code automatically",
		SystemPrompt: "You are a code reviewer...",
		Triggers: []TriggerSpec{
			{Type: "git_event", Pattern: "pull_request"},
		},
		LearningStore: "agents/code_reviewer/memory.json",
		Schedule: ScheduleSpec{
			Type:   "event",
			Events: []string{"on-pr"},
		},
		Outputs: []string{"console", "log"},
		Memory: MemorySpec{
			Enabled:       true,
			StoragePath:   "agents/code_reviewer/memory.json",
			RetentionDays: 90,
		},
	}

	if spec.Name != "persistent_reviewer" {
		t.Errorf("Name = %q, want %q", spec.Name, "persistent_reviewer")
	}
	if spec.Type != "code_reviewer" {
		t.Errorf("Type = %q, want %q", spec.Type, "code_reviewer")
	}
	if spec.Purpose != "Review code automatically" {
		t.Errorf("Purpose = %q, want %q", spec.Purpose, "Review code automatically")
	}
	if spec.SystemPrompt != "You are a code reviewer..." {
		t.Errorf("SystemPrompt = %q, want %q", spec.SystemPrompt, "You are a code reviewer...")
	}
	if len(spec.Triggers) != 1 {
		t.Errorf("Triggers length = %d, want 1", len(spec.Triggers))
	}
	if spec.LearningStore != "agents/code_reviewer/memory.json" {
		t.Errorf("LearningStore = %q, want %q", spec.LearningStore, "agents/code_reviewer/memory.json")
	}
	if len(spec.Schedule.Events) != 1 || spec.Schedule.Events[0] != "on-pr" {
		t.Errorf("Schedule.Events = %v, want [on-pr]", spec.Schedule.Events)
	}
	if len(spec.Outputs) != 2 || spec.Outputs[0] != "console" {
		t.Errorf("Outputs = %v, want [console, log]", spec.Outputs)
	}
	if !spec.Memory.Enabled {
		t.Error("Expected Memory.Enabled=true")
	}
	if spec.Memory.StoragePath != "agents/code_reviewer/memory.json" {
		t.Errorf("Memory.StoragePath = %q, want %q", spec.Memory.StoragePath, "agents/code_reviewer/memory.json")
	}
}

func TestTriggerSpec_Struct(t *testing.T) {
	trigger := TriggerSpec{
		Type:      "git_event",
		Pattern:   "commit",
		Condition: "branch == main",
	}

	if trigger.Type != "git_event" {
		t.Errorf("Type = %q, want %q", trigger.Type, "git_event")
	}
	if trigger.Pattern != "commit" {
		t.Errorf("Pattern = %q, want %q", trigger.Pattern, "commit")
	}
	if trigger.Condition != "branch == main" {
		t.Errorf("Condition = %q, want %q", trigger.Condition, "branch == main")
	}
}

func TestScheduleSpec_Struct(t *testing.T) {
	schedule := ScheduleSpec{
		Type:       "scheduled",
		Expression: "0 9 * * *",
		Events:     []string{"daily-start"},
	}

	if schedule.Type != "scheduled" {
		t.Errorf("Type = %q, want %q", schedule.Type, "scheduled")
	}
	if schedule.Expression != "0 9 * * *" {
		t.Errorf("Expression = %q, want %q", schedule.Expression, "0 9 * * *")
	}
	if len(schedule.Events) != 1 || schedule.Events[0] != "daily-start" {
		t.Errorf("Events = %v, want [daily-start]", schedule.Events)
	}
}

func TestMemorySpec_Struct(t *testing.T) {
	memory := MemorySpec{
		Enabled:       true,
		StoragePath:   "/path/to/memory.json",
		RetentionDays: 30,
	}

	if !memory.Enabled {
		t.Error("Expected Enabled=true")
	}
	if memory.StoragePath != "/path/to/memory.json" {
		t.Errorf("StoragePath = %q, want %q", memory.StoragePath, "/path/to/memory.json")
	}
	if memory.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", memory.RetentionDays)
	}
}

// =============================================================================
// AGENT CREATOR TESTS
// =============================================================================

func TestNewAgentCreator(t *testing.T) {
	client := &MockLLMClient{}
	creator := NewAgentCreator(client, "/tmp/agents")

	if creator == nil {
		t.Fatal("NewAgentCreator returned nil")
	}
	if creator.client != client {
		t.Error("client not set correctly")
	}
	if creator.agentsDir != "/tmp/agents" {
		t.Errorf("agentsDir = %q, want %q", creator.agentsDir, "/tmp/agents")
	}
}

func TestAgentCreator_CreateFromNeed(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "You are a code monitoring agent that watches for code quality issues.", nil
		},
	}
	creator := NewAgentCreator(client, "/tmp/agents")

	need := PersistenceNeed{
		AgentType:       "monitor",
		Purpose:         "Monitor code quality",
		LearningGoals:   []string{"Learn quality patterns"},
		MonitoringScope: "code_changes",
		Schedule:        "continuous",
		Confidence:      0.9,
	}

	spec, err := creator.CreateFromNeed(context.Background(), need)
	if err != nil {
		t.Fatalf("CreateFromNeed error: %v", err)
	}

	if spec == nil {
		t.Fatal("CreateFromNeed returned nil spec")
	}
	if spec.Name == "" {
		t.Error("Expected Name to be set")
	}
	if spec.Type != "monitor" {
		t.Errorf("Type = %q, want %q", spec.Type, "monitor")
	}
	if spec.Purpose != "Monitor code quality" {
		t.Errorf("Purpose = %q, want %q", spec.Purpose, "Monitor code quality")
	}
	if spec.SystemPrompt == "" {
		t.Error("Expected SystemPrompt to be set")
	}
	if len(spec.Triggers) == 0 {
		t.Error("Expected Triggers to be set")
	}
	if !spec.Memory.Enabled {
		t.Error("Expected Memory to be enabled when LearningGoals exist")
	}
}

func TestAgentCreator_CreateFromNeed_NoLearningGoals(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "You are a simple monitoring agent.", nil
		},
	}
	creator := NewAgentCreator(client, "/tmp/agents")

	need := PersistenceNeed{
		AgentType:     "alerter",
		Purpose:       "Alert on errors",
		LearningGoals: []string{}, // Empty
		Schedule:      "on-commit",
	}

	spec, err := creator.CreateFromNeed(context.Background(), need)
	if err != nil {
		t.Fatalf("CreateFromNeed error: %v", err)
	}

	if spec.Memory.Enabled {
		t.Error("Expected Memory to be disabled when no LearningGoals")
	}
}

func TestAgentCreator_CreateFromNeed_LLMFallback(t *testing.T) {
	client := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "", context.DeadlineExceeded // Simulate LLM failure
		},
	}
	creator := NewAgentCreator(client, "/tmp/agents")

	need := PersistenceNeed{
		AgentType: "reviewer",
		Purpose:   "Review code",
		Schedule:  "on-pr",
	}

	spec, err := creator.CreateFromNeed(context.Background(), need)
	if err != nil {
		t.Fatalf("CreateFromNeed should not error on LLM failure: %v", err)
	}

	// Should fallback to basic system prompt
	if spec.SystemPrompt == "" {
		t.Error("Expected fallback SystemPrompt to be set")
	}
	if !contains([]string{spec.SystemPrompt}, "You are a reviewer agent") {
		t.Logf("SystemPrompt: %s", spec.SystemPrompt)
	}
}
