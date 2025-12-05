// planner.go implements the Session Planner system shard.
//
// The Session Planner manages long-running sessions and campaigns:
// - Decomposes high-level goals into actionable agenda items
// - Tracks progress through multi-phase campaigns
// - Manages checkpoints and retry budgets
// - Suggests milestones and escalation points
//
// This shard is ON-DEMAND (starts for campaigns/complex goals) and LLM-PRIMARY,
// using the model for creative goal decomposition with Mangle for orchestration.
package system

import (
	"codenerd/internal/core"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AgendaItem represents a task in the session agenda.
type AgendaItem struct {
	ID           string    `json:"id"`
	Description  string    `json:"description"`
	Priority     int       `json:"priority"`
	Status       string    `json:"status"` // pending, in_progress, completed, blocked
	Dependencies []string  `json:"dependencies"`
	EstimatedMin int       `json:"estimated_minutes"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
}

// Checkpoint represents a session checkpoint.
type Checkpoint struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	FactsCount  int       `json:"facts_count"`
	ItemsRemaining int    `json:"items_remaining"`
}

// PlannerConfig holds configuration for the session planner.
type PlannerConfig struct {
	// Behavior
	MaxAgendaItems       int           // Max items in agenda
	AutoCheckpointEvery  time.Duration // Create checkpoint every N duration
	MaxRetriesPerTask    int           // Max retries before escalating
	IdleTimeout          time.Duration // Auto-stop after idle

	// Performance
	TickInterval time.Duration // How often to update status
}

// DefaultPlannerConfig returns sensible defaults.
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		MaxAgendaItems:      50,
		AutoCheckpointEvery: 10 * time.Minute,
		MaxRetriesPerTask:   3,
		IdleTimeout:         10 * time.Minute,
		TickInterval:        5 * time.Second,
	}
}

// SessionPlannerShard manages session agenda and campaigns.
type SessionPlannerShard struct {
	*BaseSystemShard
	mu sync.RWMutex

	// Configuration
	config PlannerConfig

	// State
	agenda       []AgendaItem
	checkpoints  []Checkpoint
	retryCount   map[string]int
	activeCampaign string

	// Tracking
	lastCheckpoint time.Time
	lastActivity   time.Time
	tasksCompleted int
	tasksBlocked   int

	// Running state
	running bool
}

// NewSessionPlannerShard creates a new Session Planner shard.
func NewSessionPlannerShard() *SessionPlannerShard {
	return NewSessionPlannerShardWithConfig(DefaultPlannerConfig())
}

// NewSessionPlannerShardWithConfig creates a planner with custom config.
func NewSessionPlannerShardWithConfig(cfg PlannerConfig) *SessionPlannerShard {
	base := NewBaseSystemShard("session_planner", StartupOnDemand)

	// Configure permissions
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionAskUser,
		core.PermissionReadFile,
	}
	base.Config.Model = core.ModelConfig{
		Capability: core.CapabilityHighReasoning, // Need good planning
	}

	// Configure idle timeout
	base.CostGuard.IdleTimeout = cfg.IdleTimeout

	return &SessionPlannerShard{
		BaseSystemShard: base,
		config:          cfg,
		agenda:          make([]AgendaItem, 0),
		checkpoints:     make([]Checkpoint, 0),
		retryCount:      make(map[string]int),
		lastCheckpoint:  time.Now(),
		lastActivity:    time.Now(),
	}
}

// Execute runs the Session Planner's orchestration loop.
func (s *SessionPlannerShard) Execute(ctx context.Context, task string) (string, error) {
	s.SetState(core.ShardStateRunning)
	s.mu.Lock()
	s.running = true
	s.StartTime = time.Now()
	s.lastActivity = time.Now()
	s.mu.Unlock()

	defer func() {
		s.SetState(core.ShardStateCompleted)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	// Initialize kernel if not set
	if s.Kernel == nil {
		s.Kernel = core.NewRealKernel()
	}

	// Parse task for initial goal or campaign
	if task != "" {
		if err := s.initializeFromTask(ctx, task); err != nil {
			return "", fmt.Errorf("failed to initialize: %w", err)
		}
	}

	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return s.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-s.StopCh:
			return s.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Check idle timeout
			if s.CostGuard.IsIdle() {
				return s.generateShutdownSummary("idle timeout"), nil
			}

			// Update agenda based on kernel state
			s.updateAgendaFromKernel()

			// Check for auto-checkpoint
			if time.Since(s.lastCheckpoint) >= s.config.AutoCheckpointEvery {
				s.createCheckpoint("auto")
			}

			// Check for blocked tasks
			s.checkBlockedTasks()

			// Emit status
			s.emitStatusFacts()

			// Emit heartbeat
			_ = s.EmitHeartbeat()
		}
	}
}

// initializeFromTask decomposes a high-level goal into agenda items.
func (s *SessionPlannerShard) initializeFromTask(ctx context.Context, task string) error {
	// Check if it's a campaign reference
	if strings.HasPrefix(task, "campaign:") {
		s.activeCampaign = strings.TrimPrefix(task, "campaign:")
		return s.loadCampaignAgenda()
	}

	// Decompose goal using LLM
	return s.decomposeGoal(ctx, task)
}

// decomposeGoal uses LLM to break down a high-level goal.
func (s *SessionPlannerShard) decomposeGoal(ctx context.Context, goal string) error {
	if s.LLMClient == nil {
		return fmt.Errorf("no LLM client for decomposition")
	}

	can, reason := s.CostGuard.CanCall()
	if !can {
		return fmt.Errorf("LLM blocked: %s", reason)
	}

	prompt := fmt.Sprintf(decompositionPrompt, goal)

	result, err := s.GuardedLLMCall(ctx, plannerSystemPrompt, prompt)
	if err != nil {
		return err
	}

	// Parse agenda items from response
	items := s.parseAgendaItems(result)
	if len(items) == 0 {
		return fmt.Errorf("failed to decompose goal")
	}

	// Limit to max items
	if len(items) > s.config.MaxAgendaItems {
		items = items[:s.config.MaxAgendaItems]
	}

	s.mu.Lock()
	s.agenda = items
	s.lastActivity = time.Now()
	s.mu.Unlock()

	// Emit agenda facts
	for _, item := range items {
		_ = s.Kernel.Assert(core.Fact{
			Predicate: "agenda_item",
			Args: []interface{}{
				item.ID,
				item.Description,
				item.Priority,
				item.Status,
				time.Now().Unix(),
			},
		})
	}

	return nil
}

// parseAgendaItems extracts agenda items from LLM output.
func (s *SessionPlannerShard) parseAgendaItems(output string) []AgendaItem {
	items := make([]AgendaItem, 0)

	// Try to parse as JSON array
	var parsed []struct {
		Description  string   `json:"description"`
		Priority     int      `json:"priority"`
		Dependencies []string `json:"dependencies"`
		EstimatedMin int      `json:"estimated_minutes"`
	}

	// Find JSON array in output
	start := strings.Index(output, "[")
	end := strings.LastIndex(output, "]")
	if start >= 0 && end > start {
		jsonStr := output[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			for i, p := range parsed {
				item := AgendaItem{
					ID:           fmt.Sprintf("task-%d-%d", time.Now().Unix(), i),
					Description:  p.Description,
					Priority:     p.Priority,
					Status:       "pending",
					Dependencies: p.Dependencies,
					EstimatedMin: p.EstimatedMin,
					CreatedAt:    time.Now(),
				}
				if item.Priority == 0 {
					item.Priority = i + 1
				}
				items = append(items, item)
			}
		}
	}

	// Fallback: parse numbered list
	if len(items) == 0 {
		lines := strings.Split(output, "\n")
		priority := 1
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Match numbered items: "1. Do something" or "- Do something"
			if len(line) > 2 && (line[0] >= '1' && line[0] <= '9' || line[0] == '-') {
				desc := strings.TrimLeft(line[1:], ". ")
				if desc != "" {
					items = append(items, AgendaItem{
						ID:          fmt.Sprintf("task-%d-%d", time.Now().Unix(), priority),
						Description: desc,
						Priority:    priority,
						Status:      "pending",
						CreatedAt:   time.Now(),
					})
					priority++
				}
			}
		}
	}

	return items
}

// loadCampaignAgenda loads agenda from campaign facts.
func (s *SessionPlannerShard) loadCampaignAgenda() error {
	// Query campaign_task facts
	results, err := s.Kernel.Query("campaign_task")
	if err != nil {
		return err
	}

	items := make([]AgendaItem, 0)
	for _, fact := range results {
		if len(fact.Args) < 4 {
			continue
		}
		item := AgendaItem{
			ID:        fact.Args[0].(string),
			CreatedAt: time.Now(),
		}
		if desc, ok := fact.Args[2].(string); ok {
			item.Description = desc
		}
		if status, ok := fact.Args[3].(string); ok {
			item.Status = status
		}
		items = append(items, item)
	}

	s.mu.Lock()
	s.agenda = items
	s.mu.Unlock()

	return nil
}

// updateAgendaFromKernel syncs agenda with kernel state.
func (s *SessionPlannerShard) updateAgendaFromKernel() {
	// Query completed tasks
	completed, _ := s.Kernel.Query("task_completed")
	completedIDs := make(map[string]bool)
	for _, fact := range completed {
		if len(fact.Args) > 0 {
			if id, ok := fact.Args[0].(string); ok {
				completedIDs[id] = true
			}
		}
	}

	// Query blocked tasks
	blocked, _ := s.Kernel.Query("task_blocked")
	blockedIDs := make(map[string]bool)
	for _, fact := range blocked {
		if len(fact.Args) > 0 {
			if id, ok := fact.Args[0].(string); ok {
				blockedIDs[id] = true
			}
		}
	}

	s.mu.Lock()
	for i := range s.agenda {
		if completedIDs[s.agenda[i].ID] {
			if s.agenda[i].Status != "completed" {
				s.agenda[i].Status = "completed"
				s.agenda[i].CompletedAt = time.Now()
				s.tasksCompleted++
				s.lastActivity = time.Now()
			}
		} else if blockedIDs[s.agenda[i].ID] {
			if s.agenda[i].Status != "blocked" {
				s.agenda[i].Status = "blocked"
				s.tasksBlocked++
			}
		}
	}
	s.mu.Unlock()
}

// checkBlockedTasks handles blocked tasks and escalation.
func (s *SessionPlannerShard) checkBlockedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.agenda {
		if item.Status == "blocked" {
			s.retryCount[item.ID]++
			if s.retryCount[item.ID] >= s.config.MaxRetriesPerTask {
				// Escalate to user
				_ = s.Kernel.Assert(core.Fact{
					Predicate: "escalation_needed",
					Args: []interface{}{
						"session_planner",
						"task_blocked",
						item.ID,
						item.Description,
						time.Now().Unix(),
					},
				})
			}
		}
	}
}

// createCheckpoint saves current state.
func (s *SessionPlannerShard) createCheckpoint(trigger string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	remaining := 0
	for _, item := range s.agenda {
		if item.Status == "pending" || item.Status == "in_progress" {
			remaining++
		}
	}

	checkpoint := Checkpoint{
		ID:             fmt.Sprintf("checkpoint-%d", time.Now().Unix()),
		Description:    fmt.Sprintf("Auto-checkpoint: %s", trigger),
		Timestamp:      time.Now(),
		ItemsRemaining: remaining,
	}

	s.checkpoints = append(s.checkpoints, checkpoint)
	s.lastCheckpoint = time.Now()

	// Emit checkpoint fact
	_ = s.Kernel.Assert(core.Fact{
		Predicate: "session_checkpoint",
		Args: []interface{}{
			checkpoint.ID,
			checkpoint.ItemsRemaining,
			checkpoint.Timestamp.Unix(),
		},
	})
}

// emitStatusFacts emits current planning status.
func (s *SessionPlannerShard) emitStatusFacts() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pending := 0
	inProgress := 0
	completed := 0
	blocked := 0

	for _, item := range s.agenda {
		switch item.Status {
		case "pending":
			pending++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		case "blocked":
			blocked++
		}
	}

	_ = s.Kernel.Assert(core.Fact{
		Predicate: "session_planner_status",
		Args: []interface{}{
			len(s.agenda),
			pending,
			inProgress,
			completed,
			blocked,
			time.Now().Unix(),
		},
	})
}

// generateShutdownSummary creates a summary of the shard's activity.
func (s *SessionPlannerShard) generateShutdownSummary(reason string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return fmt.Sprintf(
		"Session Planner shutdown (%s). Tasks: %d, Completed: %d, Blocked: %d, Checkpoints: %d, Runtime: %s",
		reason,
		len(s.agenda),
		s.tasksCompleted,
		s.tasksBlocked,
		len(s.checkpoints),
		time.Since(s.StartTime).String(),
	)
}

// GetAgenda returns the current agenda.
func (s *SessionPlannerShard) GetAgenda() []AgendaItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AgendaItem, len(s.agenda))
	copy(result, s.agenda)
	return result
}

// AddTask adds a task to the agenda.
func (s *SessionPlannerShard) AddTask(description string, priority int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := AgendaItem{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Description: description,
		Priority:    priority,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	s.agenda = append(s.agenda, item)
	s.lastActivity = time.Now()

	_ = s.Kernel.Assert(core.Fact{
		Predicate: "agenda_item",
		Args: []interface{}{
			item.ID,
			item.Description,
			item.Priority,
			item.Status,
			time.Now().Unix(),
		},
	})

	return item.ID
}

// plannerSystemPrompt is the system prompt for goal decomposition.
const plannerSystemPrompt = `You are the Session Planner of the codeNERD agent.
Your role is to decompose high-level goals into actionable tasks.

When decomposing a goal:
1. Break it into concrete, measurable tasks
2. Identify dependencies between tasks
3. Estimate time for each task
4. Prioritize based on dependencies and impact

Output a JSON array of tasks:
[
  {
    "description": "Clear description of task",
    "priority": 1,
    "dependencies": ["task-id"],
    "estimated_minutes": 30
  }
]

Guidelines:
- Keep tasks atomic and achievable
- Order by dependency first, then priority
- Include validation/testing tasks
- Note any risks or blockers`

// decompositionPrompt is the template for goal decomposition.
const decompositionPrompt = `Decompose this goal into actionable tasks:

"%s"

Provide a JSON array of tasks with descriptions, priorities, dependencies, and time estimates.`
