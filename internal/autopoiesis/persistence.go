// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains the PersistenceAnalyzer for detecting long-running agent needs.
package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// PERSISTENCE ANALYZER
// =============================================================================
// Detects when user requests suggest a need for persistent (Type 3) agents
// rather than ephemeral (Type 2) agents. Persistent agents survive across
// sessions and learn from interactions.

// PersistenceNeed represents a detected need for a persistent agent
type PersistenceNeed struct {
	AgentType       string   // Suggested agent type
	Purpose         string   // What the agent should do
	Triggers        []string // Patterns that triggered this detection
	LearningGoals   []string // What the agent should learn over time
	MonitoringScope string   // What to monitor (files, PRs, commits, etc.)
	Schedule        string   // When to run (continuous, on-commit, daily, etc.)
	Confidence      float64  // How confident we are this needs persistence
	Reasoning       string   // Why we think persistence is needed
}

// PersistenceResult contains the analysis of persistence needs
type PersistenceResult struct {
	NeedsPersistent bool
	Needs           []PersistenceNeed
	Reasons         []string
}

// PersistenceAnalyzer determines if requests need persistent agents
type PersistenceAnalyzer struct {
	client LLMClient
}

// NewPersistenceAnalyzer creates a new persistence analyzer
func NewPersistenceAnalyzer(client LLMClient) *PersistenceAnalyzer {
	return &PersistenceAnalyzer{client: client}
}

// Persistence indicators categorized by type
var (
	// Learning/Memory patterns - agent should remember things
	learningPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)learn\s+(from|about)\s+(my|our|the)\s+(preferences?|style|patterns?|habits?)`),
		regexp.MustCompile(`(?i)(remember|recall|store)\s+(this|that|my)\s+(preference|setting|choice|decision)`),
		regexp.MustCompile(`(?i)always\s+(use|prefer|apply|remember|do)`),
		regexp.MustCompile(`(?i)from\s+now\s+on\s+`),
		regexp.MustCompile(`(?i)every\s+time\s+(i|we)\s+`),
		regexp.MustCompile(`(?i)adapt\s+to\s+(my|our)\s+`),
		regexp.MustCompile(`(?i)get\s+better\s+at\s+`),
		regexp.MustCompile(`(?i)improve\s+over\s+time`),
	}

	// Monitoring patterns - agent should watch for things
	monitoringPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(monitor|watch|track|observe)\s+(for\s+)?(changes?|updates?|errors?|issues?|bugs?)`),
		regexp.MustCompile(`(?i)(continuous|ongoing|regular|periodic)\s+(review|analysis|monitoring|checking|scanning)`),
		regexp.MustCompile(`(?i)keep\s+(an?\s+)?(eye|track|watch)\s+(on|of)`),
		regexp.MustCompile(`(?i)alert\s+(me|us)\s+(when|if|on)`),
		regexp.MustCompile(`(?i)notify\s+(me|us)\s+(when|if|about)`),
		regexp.MustCompile(`(?i)let\s+(me|us)\s+know\s+(when|if)`),
		regexp.MustCompile(`(?i)flag\s+(any|all)\s+`),
	}

	// Trigger-based patterns - agent should react to events
	triggerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)whenever\s+(i|we|someone)\s+(commit|push|deploy|merge|create|open|close)`),
		regexp.MustCompile(`(?i)every\s+time\s+(i|we|there'?s?\s+a)`),
		regexp.MustCompile(`(?i)on\s+(every|each|all)\s+(commit|push|pr|pull\s+request|merge|deploy)`),
		regexp.MustCompile(`(?i)before\s+(every|each)\s+(commit|push|deploy|merge)`),
		regexp.MustCompile(`(?i)after\s+(every|each)\s+(commit|push|deploy|merge)`),
		regexp.MustCompile(`(?i)automatically\s+(review|check|scan|analyze|test)`),
	}

	// Background work patterns - agent should work independently
	backgroundPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)in\s+the\s+background`),
		regexp.MustCompile(`(?i)while\s+(i|we)\s+(work|code|develop)`),
		regexp.MustCompile(`(?i)(overnight|daily|weekly|hourly)`),
		regexp.MustCompile(`(?i)run\s+(this|it|the\s+\w+)\s+(continuously|regularly|periodically)`),
		regexp.MustCompile(`(?i)keep\s+(running|checking|monitoring|analyzing)`),
	}

	// Schedule indicators
	scheduleIndicators = map[string]string{
		"continuously": "continuous",
		"ongoing":      "continuous",
		"always":       "continuous",
		"every commit": "on-commit",
		"each commit":  "on-commit",
		"on commit":    "on-commit",
		"every push":   "on-push",
		"on push":      "on-push",
		"every pr":     "on-pr",
		"pull request": "on-pr",
		"daily":        "daily",
		"weekly":       "weekly",
		"hourly":       "hourly",
		"overnight":    "nightly",
		"nightly":      "nightly",
	}

	// Agent type mappings based on context
	agentTypeMappings = map[string]string{
		"review":     "code_reviewer",
		"security":   "security_scanner",
		"test":       "test_runner",
		"document":   "documenter",
		"format":     "formatter",
		"lint":       "linter",
		"analyze":    "analyzer",
		"monitor":    "monitor",
		"deploy":     "deployer",
		"learn":      "learner",
		"preference": "preference_tracker",
		"style":      "style_enforcer",
	}
)

// Analyze determines if a request needs persistent agents
func (pa *PersistenceAnalyzer) Analyze(ctx context.Context, input string) PersistenceResult {
	result := PersistenceResult{
		NeedsPersistent: false,
		Needs:           []PersistenceNeed{},
		Reasons:         []string{},
	}

	lower := strings.ToLower(input)

	// Check learning patterns
	for _, pattern := range learningPatterns {
		if pattern.MatchString(lower) {
			result.NeedsPersistent = true
			result.Reasons = append(result.Reasons, "Learning/memory capability required")

			need := PersistenceNeed{
				AgentType:     determineAgentType(lower),
				Purpose:       "Learn and adapt to user preferences",
				Triggers:      []string{pattern.String()},
				LearningGoals: extractLearningGoals(lower),
				Schedule:      "continuous",
				Confidence:    0.8,
				Reasoning:     "User wants agent to remember/learn preferences",
			}
			result.Needs = append(result.Needs, need)
			break
		}
	}

	// Check monitoring patterns
	for _, pattern := range monitoringPatterns {
		if pattern.MatchString(lower) {
			result.NeedsPersistent = true
			result.Reasons = append(result.Reasons, "Continuous monitoring required")

			need := PersistenceNeed{
				AgentType:       determineAgentType(lower),
				Purpose:         "Monitor and alert on conditions",
				Triggers:        []string{pattern.String()},
				MonitoringScope: extractMonitoringScope(lower),
				Schedule:        "continuous",
				Confidence:      0.85,
				Reasoning:       "User wants ongoing monitoring/alerting",
			}
			result.Needs = append(result.Needs, need)
			break
		}
	}

	// Check trigger patterns
	for _, pattern := range triggerPatterns {
		if pattern.MatchString(lower) {
			result.NeedsPersistent = true
			result.Reasons = append(result.Reasons, "Event-triggered automation required")

			need := PersistenceNeed{
				AgentType:  determineAgentType(lower),
				Purpose:    "React to development events",
				Triggers:   []string{pattern.String()},
				Schedule:   determineSchedule(lower),
				Confidence: 0.9,
				Reasoning:  "User wants automatic reaction to events",
			}
			result.Needs = append(result.Needs, need)
			break
		}
	}

	// Check background patterns
	for _, pattern := range backgroundPatterns {
		if pattern.MatchString(lower) {
			result.NeedsPersistent = true
			result.Reasons = append(result.Reasons, "Background processing required")

			need := PersistenceNeed{
				AgentType:  determineAgentType(lower),
				Purpose:    "Process in background",
				Triggers:   []string{pattern.String()},
				Schedule:   determineSchedule(lower),
				Confidence: 0.75,
				Reasoning:  "User wants background/async processing",
			}
			result.Needs = append(result.Needs, need)
			break
		}
	}

	return result
}

// AnalyzeWithLLM uses LLM for deeper persistence analysis
func (pa *PersistenceAnalyzer) AnalyzeWithLLM(ctx context.Context, input string) (PersistenceResult, error) {
	// First do heuristic analysis
	result := pa.Analyze(ctx, input)

	// If heuristics are confident, skip LLM
	if result.NeedsPersistent && len(result.Needs) > 0 && result.Needs[0].Confidence > 0.85 {
		return result, nil
	}

	// Use LLM for ambiguous cases
	prompt := fmt.Sprintf(`Analyze if this user request requires a PERSISTENT agent.

Persistent agents:
- Survive across sessions
- Learn from interactions over time
- Monitor for events/conditions
- React to triggers (commits, PRs, etc.)

User Request: %q

Heuristic Analysis:
- Needs Persistent: %v
- Reasons: %v

Return JSON only:
{
  "needs_persistent": true/false,
  "agent_type": "type of agent needed",
  "purpose": "what the agent should do",
  "learning_goals": ["what to learn over time"],
  "monitoring_scope": "what to monitor",
  "schedule": "when to run (continuous, on-commit, daily, etc.)",
  "confidence": 0.0-1.0,
  "reasoning": "explanation"
}

JSON only:`, input, result.NeedsPersistent, result.Reasons)

	resp, err := pa.client.Complete(ctx, prompt)
	if err != nil {
		return result, nil // Fall back to heuristic result
	}

	// Parse LLM response
	var llmResult struct {
		NeedsPersistent bool     `json:"needs_persistent"`
		AgentType       string   `json:"agent_type"`
		Purpose         string   `json:"purpose"`
		LearningGoals   []string `json:"learning_goals"`
		MonitoringScope string   `json:"monitoring_scope"`
		Schedule        string   `json:"schedule"`
		Confidence      float64  `json:"confidence"`
		Reasoning       string   `json:"reasoning"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err != nil {
		return result, nil
	}

	// Merge with heuristic results
	if llmResult.NeedsPersistent && llmResult.Confidence > 0.5 {
		result.NeedsPersistent = true
		result.Reasons = append(result.Reasons, "LLM analysis: "+llmResult.Reasoning)

		need := PersistenceNeed{
			AgentType:       llmResult.AgentType,
			Purpose:         llmResult.Purpose,
			LearningGoals:   llmResult.LearningGoals,
			MonitoringScope: llmResult.MonitoringScope,
			Schedule:        llmResult.Schedule,
			Confidence:      llmResult.Confidence,
			Reasoning:       llmResult.Reasoning,
		}

		// Add to needs if not duplicate
		if !containsSimilarNeed(result.Needs, need) {
			result.Needs = append(result.Needs, need)
		}
	}

	return result, nil
}

// =============================================================================
// AGENT CREATOR
// =============================================================================
// Creates and registers persistent agents based on detected needs

// AgentSpec defines the specification for creating a persistent agent
type AgentSpec struct {
	Name          string            // Unique agent name
	Type          string            // Agent type (code_reviewer, monitor, etc.)
	Purpose       string            // Human-readable purpose
	SystemPrompt  string            // System prompt for the agent
	Triggers      []TriggerSpec     // What triggers the agent
	LearningStore string            // Path to learning storage
	Schedule      ScheduleSpec      // When to run
	Outputs       []string          // Where to send outputs
	Memory        MemorySpec        // Memory configuration
}

// TriggerSpec defines what triggers an agent
type TriggerSpec struct {
	Type      string // "file_change", "git_event", "time", "manual"
	Pattern   string // File pattern, event name, cron expression
	Condition string // Additional condition (optional)
}

// ScheduleSpec defines when an agent runs
type ScheduleSpec struct {
	Type       string // "continuous", "event", "scheduled"
	Expression string // Cron expression for scheduled
	Events     []string // Events for event-based
}

// MemorySpec defines agent memory configuration
type MemorySpec struct {
	Enabled      bool   // Whether memory is enabled
	StoragePath  string // Where to store memories
	RetentionDays int   // How long to keep memories
}

// AgentCreator creates persistent agents
type AgentCreator struct {
	client      LLMClient
	agentsDir   string // Directory for agent definitions
}

// NewAgentCreator creates a new agent creator
func NewAgentCreator(client LLMClient, agentsDir string) *AgentCreator {
	return &AgentCreator{
		client:    client,
		agentsDir: agentsDir,
	}
}

// CreateFromNeed creates an agent spec from a persistence need
func (ac *AgentCreator) CreateFromNeed(ctx context.Context, need PersistenceNeed) (*AgentSpec, error) {
	// Generate system prompt for the agent
	systemPrompt, err := ac.generateSystemPrompt(ctx, need)
	if err != nil {
		systemPrompt = fmt.Sprintf("You are a %s agent. Your purpose: %s", need.AgentType, need.Purpose)
	}

	// Build triggers
	triggers := buildTriggers(need)

	// Build schedule
	schedule := buildSchedule(need)

	// Build memory spec
	memory := MemorySpec{
		Enabled:       len(need.LearningGoals) > 0,
		StoragePath:   fmt.Sprintf("agents/%s/memory.json", need.AgentType),
		RetentionDays: 90,
	}

	spec := &AgentSpec{
		Name:         generateAgentName(need.AgentType),
		Type:         need.AgentType,
		Purpose:      need.Purpose,
		SystemPrompt: systemPrompt,
		Triggers:     triggers,
		Schedule:     schedule,
		Memory:       memory,
		Outputs:      []string{"console", "log"},
	}

	return spec, nil
}

// generateSystemPrompt creates a system prompt for the agent
func (ac *AgentCreator) generateSystemPrompt(ctx context.Context, need PersistenceNeed) (string, error) {
	prompt := fmt.Sprintf(`Generate a system prompt for a persistent coding agent with these characteristics:

Agent Type: %s
Purpose: %s
Learning Goals: %v
Monitoring Scope: %s
Schedule: %s

The system prompt should:
1. Clearly define the agent's role and responsibilities
2. Specify how to handle different scenarios
3. Include guidelines for learning and adaptation
4. Define output format expectations

Generate a professional system prompt (no code blocks, just the prompt text):`,
		need.AgentType, need.Purpose, need.LearningGoals, need.MonitoringScope, need.Schedule)

	return ac.client.Complete(ctx, prompt)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// determineAgentType determines the appropriate agent type from input
func determineAgentType(input string) string {
	for keyword, agentType := range agentTypeMappings {
		if strings.Contains(input, keyword) {
			return agentType
		}
	}
	return "general_agent"
}

// determineSchedule determines the schedule from input
func determineSchedule(input string) string {
	for indicator, schedule := range scheduleIndicators {
		if strings.Contains(input, indicator) {
			return schedule
		}
	}
	return "on-demand"
}

// extractLearningGoals extracts what the agent should learn
func extractLearningGoals(input string) []string {
	goals := []string{}

	learningKeywords := []string{
		"preferences", "style", "patterns", "habits",
		"conventions", "naming", "formatting", "approach",
	}

	for _, keyword := range learningKeywords {
		if strings.Contains(input, keyword) {
			goals = append(goals, fmt.Sprintf("Learn user %s", keyword))
		}
	}

	if len(goals) == 0 {
		goals = append(goals, "Learn from user feedback")
	}

	return goals
}

// extractMonitoringScope extracts what should be monitored
func extractMonitoringScope(input string) string {
	scopes := map[string]string{
		"file":       "file_changes",
		"code":       "code_changes",
		"commit":     "git_commits",
		"pr":         "pull_requests",
		"pull":       "pull_requests",
		"error":      "error_logs",
		"bug":        "issue_tracker",
		"test":       "test_results",
		"security":   "security_issues",
		"dependency": "dependencies",
		"build":      "build_status",
	}

	for keyword, scope := range scopes {
		if strings.Contains(input, keyword) {
			return scope
		}
	}

	return "general"
}

// buildTriggers creates trigger specs from a need
func buildTriggers(need PersistenceNeed) []TriggerSpec {
	triggers := []TriggerSpec{}

	switch need.Schedule {
	case "on-commit":
		triggers = append(triggers, TriggerSpec{
			Type:    "git_event",
			Pattern: "commit",
		})
	case "on-push":
		triggers = append(triggers, TriggerSpec{
			Type:    "git_event",
			Pattern: "push",
		})
	case "on-pr":
		triggers = append(triggers, TriggerSpec{
			Type:    "git_event",
			Pattern: "pull_request",
		})
	case "continuous":
		triggers = append(triggers, TriggerSpec{
			Type:    "file_change",
			Pattern: "**/*",
		})
	default:
		triggers = append(triggers, TriggerSpec{
			Type: "manual",
		})
	}

	return triggers
}

// buildSchedule creates a schedule spec from a need
func buildSchedule(need PersistenceNeed) ScheduleSpec {
	switch need.Schedule {
	case "continuous":
		return ScheduleSpec{Type: "continuous"}
	case "daily":
		return ScheduleSpec{Type: "scheduled", Expression: "0 9 * * *"}
	case "weekly":
		return ScheduleSpec{Type: "scheduled", Expression: "0 9 * * 1"}
	case "hourly":
		return ScheduleSpec{Type: "scheduled", Expression: "0 * * * *"}
	case "nightly":
		return ScheduleSpec{Type: "scheduled", Expression: "0 2 * * *"}
	case "on-commit", "on-push", "on-pr":
		return ScheduleSpec{Type: "event", Events: []string{need.Schedule}}
	default:
		return ScheduleSpec{Type: "event"}
	}
}

// generateAgentName creates a unique agent name
func generateAgentName(agentType string) string {
	// In production, would include timestamp or UUID
	return fmt.Sprintf("persistent_%s", agentType)
}

// containsSimilarNeed checks if a similar need already exists
func containsSimilarNeed(needs []PersistenceNeed, newNeed PersistenceNeed) bool {
	for _, n := range needs {
		if n.AgentType == newNeed.AgentType && n.Purpose == newNeed.Purpose {
			return true
		}
	}
	return false
}
