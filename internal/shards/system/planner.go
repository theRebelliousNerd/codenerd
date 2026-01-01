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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
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
	ID             string    `json:"id"`
	Description    string    `json:"description"`
	Timestamp      time.Time `json:"timestamp"`
	FactsCount     int       `json:"facts_count"`
	ItemsRemaining int       `json:"items_remaining"`
}

// PlanView provides a structured view of the current plan.
type PlanView struct {
	CampaignID   string       `json:"campaign_id,omitempty"`
	TotalTasks   int          `json:"total_tasks"`
	Pending      int          `json:"pending"`
	InProgress   int          `json:"in_progress"`
	Completed    int          `json:"completed"`
	Blocked      int          `json:"blocked"`
	ProgressPct  float64      `json:"progress_pct"`
	Tasks        []AgendaItem `json:"tasks"`
	Checkpoints  []Checkpoint `json:"checkpoints"`
	StartedAt    time.Time    `json:"started_at"`
	LastActivity time.Time    `json:"last_activity"`
	RuntimeSec   int          `json:"runtime_sec"`
}

// PlannerConfig holds configuration for the session planner.
type PlannerConfig struct {
	// Behavior
	MaxAgendaItems      int           // Max items in agenda
	AutoCheckpointEvery time.Duration // Create checkpoint every N duration
	MaxRetriesPerTask   int           // Max retries before escalating
	IdleTimeout         time.Duration // Auto-stop after idle

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
	agenda         []AgendaItem
	checkpoints    []Checkpoint
	retryCount     map[string]int
	activeCampaign string

	// Tracking
	lastCheckpoint time.Time
	lastActivity   time.Time
	tasksCompleted int
	tasksBlocked   int

	// Running state
	running bool

	// JIT Prompt Compilation (Phase 5)
	// Stored as interface{} to avoid import cycles - should be *articulation.PromptAssembler.
	// Set via SetPromptAssembler() which accepts interface{}.
	promptAssembler interface{}
}

// NewSessionPlannerShard creates a new Session Planner shard.
func NewSessionPlannerShard() *SessionPlannerShard {
	return NewSessionPlannerShardWithConfig(DefaultPlannerConfig())
}

// NewSessionPlannerShardWithConfig creates a planner with custom config.
func NewSessionPlannerShardWithConfig(cfg PlannerConfig) *SessionPlannerShard {
	logging.SystemShards("[SessionPlanner] Initializing session planner shard")
	base := NewBaseSystemShard("session_planner", StartupOnDemand)

	// Configure permissions
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionAskUser,
		types.PermissionReadFile,
	}
	base.Config.Model = types.ModelConfig{
		Capability: types.CapabilityHighReasoning, // Need good planning
	}

	// Configure idle timeout
	base.CostGuard.IdleTimeout = cfg.IdleTimeout

	logging.SystemShardsDebug("[SessionPlanner] Config: max_items=%d, auto_checkpoint=%v, max_retries=%d, idle_timeout=%v",
		cfg.MaxAgendaItems, cfg.AutoCheckpointEvery, cfg.MaxRetriesPerTask, cfg.IdleTimeout)
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
	logging.SystemShards("[SessionPlanner] Starting orchestration loop")
	s.SetState(types.ShardStateRunning)
	s.mu.Lock()
	s.running = true
	s.StartTime = time.Now()
	s.lastActivity = time.Now()
	s.mu.Unlock()

	defer func() {
		s.SetState(types.ShardStateCompleted)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		logging.SystemShards("[SessionPlanner] Orchestration loop terminated")
	}()

	// Initialize kernel if not set
	if s.Kernel == nil {
		logging.SystemShardsDebug("[SessionPlanner] Creating new kernel (none attached)")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		s.Kernel = kernel
	}

	// Parse task for initial goal or campaign
	// Skip decomposition for system_start - it's just a startup signal, not a goal
	if task != "" && task != "system_start" {
		logging.SystemShards("[SessionPlanner] Initializing from task: %s", truncateForLog(task, 100))
		if err := s.initializeFromTask(ctx, task); err != nil {
			logging.Get(logging.CategorySystemShards).Error("[SessionPlanner] Failed to initialize: %v", err)
			return "", fmt.Errorf("failed to initialize: %w", err)
		}
	} else if task == "system_start" {
		logging.SystemShards("[SessionPlanner] System startup - awaiting goals")
	}

	ticker := time.NewTicker(s.config.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.SystemShards("[SessionPlanner] Context cancelled, shutting down")
			return s.generateShutdownSummary("context cancelled"), ctx.Err()
		case <-s.StopCh:
			logging.SystemShards("[SessionPlanner] Stop signal received")
			return s.generateShutdownSummary("stopped"), nil
		case <-ticker.C:
			// Check idle timeout
			if s.CostGuard.IsIdle() {
				logging.SystemShards("[SessionPlanner] Idle timeout reached, shutting down")
				return s.generateShutdownSummary("idle timeout"), nil
			}

			// Update agenda based on kernel state
			s.updateAgendaFromKernel()

			// Check for auto-checkpoint
			if time.Since(s.lastCheckpoint) >= s.config.AutoCheckpointEvery {
				logging.SystemShardsDebug("[SessionPlanner] Creating auto-checkpoint")
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
		logging.SystemShards("[SessionPlanner] Loading campaign: %s", s.activeCampaign)
		return s.loadCampaignAgenda()
	}

	// Decompose goal using LLM
	logging.SystemShardsDebug("[SessionPlanner] Decomposing goal via LLM")
	return s.decomposeGoal(ctx, task)
}

// decomposeGoal uses LLM to break down a high-level goal.
// Uses JIT prompt compilation if available, otherwise falls back to legacy prompts.
func (s *SessionPlannerShard) decomposeGoal(ctx context.Context, goal string) error {
	timer := logging.StartTimer(logging.CategorySystemShards, "[SessionPlanner] Goal decomposition")
	defer timer.Stop()

	if s.LLMClient == nil {
		logging.Get(logging.CategorySystemShards).Error("[SessionPlanner] No LLM client for decomposition")
		return fmt.Errorf("no LLM client for decomposition")
	}

	can, reason := s.CostGuard.CanCall()
	if !can {
		logging.Get(logging.CategorySystemShards).Warn("[SessionPlanner] LLM call blocked: %s", reason)
		return fmt.Errorf("LLM blocked: %s", reason)
	}

	// Build system prompt (JIT required - no fallback)
	systemPrompt := s.buildSystemPrompt(ctx)
	if systemPrompt == "" {
		return fmt.Errorf("JIT prompt compilation failed - ensure campaign/planner atoms exist in internal/prompt/atoms/campaign/planner.yaml")
	}

	// Build user prompt for the goal
	userPrompt := s.buildDecompositionPrompt(ctx, goal)

	rawResponse, err := s.GuardedLLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return err
	}

	// Process through Piggyback Protocol - extract surface, route control to kernel
	processed := articulation.ProcessLLMResponse(rawResponse)
	logging.SystemShardsDebug("[SessionPlanner] Piggyback: method=%s, confidence=%.2f",
		processed.ParseMethod, processed.Confidence)

	// Route control packet to kernel if present
	if processed.Control != nil {
		s.routeControlPacketToKernel(processed.Control)
	}

	// Parse agenda items from surface response
	items := s.parseAgendaItems(processed.Surface)
	if len(items) == 0 {
		logging.Get(logging.CategorySystemShards).Warn("[SessionPlanner] Failed to parse agenda items from LLM response")
		return fmt.Errorf("failed to decompose goal")
	}

	// Limit to max items
	if len(items) > s.config.MaxAgendaItems {
		logging.SystemShardsDebug("[SessionPlanner] Limiting agenda from %d to %d items", len(items), s.config.MaxAgendaItems)
		items = items[:s.config.MaxAgendaItems]
	}

	s.mu.Lock()
	s.agenda = items
	s.lastActivity = time.Now()
	s.mu.Unlock()

	logging.SystemShards("[SessionPlanner] Goal decomposed into %d agenda items", len(items))

	// Emit agenda facts
	for _, item := range items {
		_ = s.Kernel.Assert(types.Fact{
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

// buildSystemPrompt constructs the system prompt for goal decomposition.
// Uses JIT prompt compilation - returns empty string if JIT is unavailable.
// Planner system prompts are JIT-compiled from internal/prompt/atoms/campaign/planner.yaml
func (s *SessionPlannerShard) buildSystemPrompt(ctx context.Context) string {
	s.mu.RLock()
	pa := s.promptAssembler
	shardID := s.ID
	s.mu.RUnlock()

	// If PromptAssembler is not available, fail (no fallback)
	if pa == nil {
		logging.SystemShards("[SessionPlanner] [ERROR] No PromptAssembler configured - cannot compile prompt")
		return ""
	}

	// Type assert to actual PromptAssembler type
	assembler, ok := pa.(*articulation.PromptAssembler)
	if !ok {
		logging.SystemShards("[SessionPlanner] [ERROR] PromptAssembler type mismatch - cannot compile prompt")
		return ""
	}

	// Check if JIT is ready
	if !assembler.JITReady() {
		logging.SystemShards("[SessionPlanner] [ERROR] JIT not ready - cannot compile prompt (ensure campaign/planner atoms exist)")
		return ""
	}

	// Build proper PromptContext for JIT compilation
	promptCtx := &articulation.PromptContext{
		ShardID:   shardID,
		ShardType: "planner",
	}

	jitPrompt, err := assembler.AssembleSystemPrompt(ctx, promptCtx)
	if err != nil {
		logging.SystemShards("[SessionPlanner] [ERROR] JIT compilation failed: %v", err)
		return ""
	}

	logging.SystemShards("[SessionPlanner] [JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
	return jitPrompt
}

// buildDecompositionPrompt constructs the user prompt for goal decomposition.
// User prompts are kept simple and inline since JIT primarily handles system prompts.
func (s *SessionPlannerShard) buildDecompositionPrompt(ctx context.Context, goal string) string {
	return fmt.Sprintf(`Decompose this goal into actionable tasks:

"%s"

Follow the Goal Decomposition Protocol:
1. Analyze the goal to understand the TRUE objective
2. Identify all components that need to exist
3. Map dependencies between components
4. Create atomic, specific tasks with clear target files
5. Assign priorities respecting the build order (types → data → service → interface)

Output a JSON array. Each task should be completable in a single agent turn.`, goal)
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
				// Fix 15.8: Sync status to campaign_task fact
				s.syncTaskStatusToKernel(s.agenda[i].ID, "/completed")
			}
		} else if blockedIDs[s.agenda[i].ID] {
			if s.agenda[i].Status != "blocked" {
				s.agenda[i].Status = "blocked"
				s.tasksBlocked++
				// Fix 15.8: Sync status to campaign_task fact
				s.syncTaskStatusToKernel(s.agenda[i].ID, "/blocked")
			}
		}
	}
	s.mu.Unlock()
}

// syncTaskStatusToKernel updates the campaign_task fact status in the kernel.
func (s *SessionPlannerShard) syncTaskStatusToKernel(taskID, newStatus string) {
	if s.Kernel == nil {
		return
	}

	// Find original fact to preserve immutable fields
	facts, err := s.Kernel.Query("campaign_task")
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[SessionPlanner] Failed to query campaign_task: %v", err)
		return
	}

	for _, f := range facts {
		if len(f.Args) < 5 {
			continue
		}
		id, ok := f.Args[0].(string)
		if !ok || id != taskID {
			continue
		}

		// Found the task fact. Retract it.
		if err := s.Kernel.RetractExactFact(f); err != nil {
			logging.Get(logging.CategorySystemShards).Error("[SessionPlanner] Failed to retract stale campaign_task: %v", err)
			continue
		}

		// Assert new fact with updated status
		// campaign_task(TaskID, PhaseID, Description, Status, TaskType)
		newFact := types.Fact{
			Predicate: "campaign_task",
			Args: []interface{}{
				f.Args[0], // TaskID
				f.Args[1], // PhaseID
				f.Args[2], // Description
				newStatus, // Updated Status
				f.Args[4], // TaskType
			},
		}
		if err := s.Kernel.Assert(newFact); err != nil {
			logging.Get(logging.CategorySystemShards).Error("[SessionPlanner] Failed to assert updated campaign_task: %v", err)
		} else {
			logging.SystemShardsDebug("[SessionPlanner] Synced task %s status to %s", taskID, newStatus)
		}
		return // Done
	}
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
				_ = s.Kernel.Assert(types.Fact{
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
	_ = s.Kernel.Assert(types.Fact{
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

	// Emit summary status fact
	_ = s.Kernel.Assert(types.Fact{
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

	// Emit individual task facts for Mangle reasoning
	for _, item := range s.agenda {
		// Calculate progress percentage for this task
		progressPct := 0
		if item.Status == "completed" {
			progressPct = 100
		} else if item.Status == "in_progress" {
			progressPct = 50 // Assume 50% for in-progress tasks
		}

		// Emit plan_task fact
		_ = s.Kernel.Assert(types.Fact{
			Predicate: "plan_task",
			Args: []interface{}{
				item.ID,
				item.Description,
				item.Status,
				progressPct,
			},
		})
	}

	// Calculate overall progress
	var progressPct float64
	if len(s.agenda) > 0 {
		progressPct = float64(completed) / float64(len(s.agenda)) * 100
	}

	// Emit plan_progress fact
	_ = s.Kernel.Assert(types.Fact{
		Predicate: "plan_progress",
		Args: []interface{}{
			s.activeCampaign,
			len(s.agenda),
			completed,
			int(progressPct),
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

	_ = s.Kernel.Assert(types.Fact{
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

// GetCurrentPlan returns a structured view of the current plan.
func (s *SessionPlannerShard) GetCurrentPlan() *PlanView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	view := &PlanView{
		CampaignID:   s.activeCampaign,
		TotalTasks:   len(s.agenda),
		Tasks:        make([]AgendaItem, len(s.agenda)),
		Checkpoints:  make([]Checkpoint, len(s.checkpoints)),
		StartedAt:    s.StartTime,
		LastActivity: s.lastActivity,
	}

	// Copy tasks and count by status
	copy(view.Tasks, s.agenda)
	for _, item := range s.agenda {
		switch item.Status {
		case "pending":
			view.Pending++
		case "in_progress":
			view.InProgress++
		case "completed":
			view.Completed++
		case "blocked":
			view.Blocked++
		}
	}

	// Calculate progress percentage
	if view.TotalTasks > 0 {
		view.ProgressPct = float64(view.Completed) / float64(view.TotalTasks) * 100
	}

	// Calculate runtime
	view.RuntimeSec = int(time.Since(s.StartTime).Seconds())

	// Copy checkpoints
	copy(view.Checkpoints, s.checkpoints)

	return view
}

// SetPromptAssembler sets the prompt assembler for JIT prompt compilation.
// The assembler should be a *articulation.PromptAssembler but is stored as
// interface{} to avoid import cycles.
func (s *SessionPlannerShard) SetPromptAssembler(assembler interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promptAssembler = assembler
	if assembler != nil {
		logging.SystemShards("[SessionPlanner] PromptAssembler attached")
	}
}

// GetPromptAssembler returns the prompt assembler for JIT prompt compilation.
func (s *SessionPlannerShard) GetPromptAssembler() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.promptAssembler
}

// FormatPlanAsMarkdown formats the current plan as markdown for terminal display.
func (s *SessionPlannerShard) FormatPlanAsMarkdown() string {
	plan := s.GetCurrentPlan()
	var sb strings.Builder

	sb.WriteString("# Session Plan\n\n")

	// Campaign info if applicable
	if plan.CampaignID != "" {
		sb.WriteString(fmt.Sprintf("**Campaign:** %s\n\n", plan.CampaignID))
	}

	// Progress summary
	sb.WriteString("## Progress Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Tasks:** %d\n", plan.TotalTasks))
	sb.WriteString(fmt.Sprintf("- **Completed:** %d (%.1f%%)\n", plan.Completed, plan.ProgressPct))
	sb.WriteString(fmt.Sprintf("- **In Progress:** %d\n", plan.InProgress))
	sb.WriteString(fmt.Sprintf("- **Pending:** %d\n", plan.Pending))
	sb.WriteString(fmt.Sprintf("- **Blocked:** %d\n", plan.Blocked))
	sb.WriteString(fmt.Sprintf("- **Runtime:** %s\n", formatDuration(plan.RuntimeSec)))
	sb.WriteString(fmt.Sprintf("- **Last Activity:** %s\n\n", formatRelativeTime(plan.LastActivity)))

	// Progress bar
	sb.WriteString("```\n")
	sb.WriteString(generateProgressBar(plan.ProgressPct, 50))
	sb.WriteString("\n```\n\n")

	// Tasks by status
	if len(plan.Tasks) > 0 {
		// In Progress tasks
		inProgressTasks := filterTasksByStatus(plan.Tasks, "in_progress")
		if len(inProgressTasks) > 0 {
			sb.WriteString("## In Progress\n\n")
			for _, task := range inProgressTasks {
				sb.WriteString(fmt.Sprintf("- **[%d]** %s\n", task.Priority, task.Description))
				if len(task.Dependencies) > 0 {
					sb.WriteString(fmt.Sprintf("  - Dependencies: %s\n", strings.Join(task.Dependencies, ", ")))
				}
			}
			sb.WriteString("\n")
		}

		// Pending tasks
		pendingTasks := filterTasksByStatus(plan.Tasks, "pending")
		if len(pendingTasks) > 0 {
			sb.WriteString("## Pending\n\n")
			for _, task := range pendingTasks {
				sb.WriteString(fmt.Sprintf("- **[%d]** %s", task.Priority, task.Description))
				if task.EstimatedMin > 0 {
					sb.WriteString(fmt.Sprintf(" (~%dm)", task.EstimatedMin))
				}
				sb.WriteString("\n")
				if len(task.Dependencies) > 0 {
					sb.WriteString(fmt.Sprintf("  - Dependencies: %s\n", strings.Join(task.Dependencies, ", ")))
				}
			}
			sb.WriteString("\n")
		}

		// Blocked tasks
		blockedTasks := filterTasksByStatus(plan.Tasks, "blocked")
		if len(blockedTasks) > 0 {
			sb.WriteString("## Blocked\n\n")
			for _, task := range blockedTasks {
				sb.WriteString(fmt.Sprintf("- **[%d]** %s\n", task.Priority, task.Description))
			}
			sb.WriteString("\n")
		}

		// Completed tasks (limited to last 5)
		completedTasks := filterTasksByStatus(plan.Tasks, "completed")
		if len(completedTasks) > 0 {
			sb.WriteString("## Recently Completed\n\n")
			displayCount := len(completedTasks)
			if displayCount > 5 {
				displayCount = 5
			}
			for i := len(completedTasks) - displayCount; i < len(completedTasks); i++ {
				task := completedTasks[i]
				sb.WriteString(fmt.Sprintf("- ✓ %s", task.Description))
				if !task.CompletedAt.IsZero() {
					sb.WriteString(fmt.Sprintf(" (%s)", formatRelativeTime(task.CompletedAt)))
				}
				sb.WriteString("\n")
			}
			if len(completedTasks) > 5 {
				sb.WriteString(fmt.Sprintf("\n_...and %d more_\n", len(completedTasks)-5))
			}
			sb.WriteString("\n")
		}
	}

	// Checkpoints
	if len(plan.Checkpoints) > 0 {
		sb.WriteString("## Checkpoints\n\n")
		displayCount := len(plan.Checkpoints)
		if displayCount > 3 {
			displayCount = 3
		}
		for i := len(plan.Checkpoints) - displayCount; i < len(plan.Checkpoints); i++ {
			cp := plan.Checkpoints[i]
			sb.WriteString(fmt.Sprintf("- **%s** - %s (%d tasks remaining)\n",
				cp.ID, formatRelativeTime(cp.Timestamp), cp.ItemsRemaining))
		}
		if len(plan.Checkpoints) > 3 {
			sb.WriteString(fmt.Sprintf("\n_...and %d more_\n", len(plan.Checkpoints)-3))
		}
	}

	return sb.String()
}

// FormatPlanAsJSON formats the current plan as JSON for programmatic access.
func (s *SessionPlannerShard) FormatPlanAsJSON() string {
	plan := s.GetCurrentPlan()
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal plan: %s"}`, err.Error())
	}
	return string(data)
}

// Helper functions for formatting

func filterTasksByStatus(tasks []AgendaItem, status string) []AgendaItem {
	var result []AgendaItem
	for _, task := range tasks {
		if task.Status == status {
			result = append(result, task)
		}
	}
	return result
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds%60)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dh %dm", hours, minutes%60)
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	duration := time.Since(t)
	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
}

func generateProgressBar(percent float64, width int) string {
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %.1f%%", bar, percent)
}

// NOTE: Legacy plannerSystemPrompt and decompositionPrompt constants have been DELETED.
// Session planner system prompts are now JIT-compiled from:
//   internal/prompt/atoms/campaign/planner.yaml
// User prompts are kept simple and inline in buildDecompositionPrompt().

// =============================================================================
// PIGGYBACK PROTOCOL ROUTING
// =============================================================================

// routeControlPacketToKernel processes the control_packet and routes data to the kernel.
// This handles mangle_updates, memory_operations, and self_correction signals.
func (s *SessionPlannerShard) routeControlPacketToKernel(control *articulation.ControlPacket) {
	if control == nil {
		return
	}

	kernel := s.Kernel
	if kernel == nil {
		logging.SystemShardsDebug("[SessionPlanner] No kernel available for control packet routing")
		return
	}

	// 1. Assert mangle_updates as facts
	if len(control.MangleUpdates) > 0 {
		logging.SystemShardsDebug("[SessionPlanner] Routing %d mangle_updates to kernel", len(control.MangleUpdates))
		policy := core.MangleUpdatePolicy{
			AllowedPredicates: map[string]struct{}{
				"missing_tool_for": {},
				"observation":      {},
				"task_status":      {},
				"task_completed":   {},
				"campaign_completed": {},
			},
			AllowedPrefixes: []string{
				"campaign_",
				"phase_",
				"task_",
				"context_",
				"plan_",
				"replan_",
				"build_",
				"architectural_",
				"suspicious_",
				"eligible_",
			},
			MaxUpdates: 200,
		}
		facts, blocked := core.FilterMangleUpdates(kernel, control.MangleUpdates, policy)
		for _, b := range blocked {
			logging.SystemShardsDebug("[SessionPlanner] Blocked mangle_update %q: %s", b.Update, b.Reason)
		}
		if len(facts) > 0 {
			if err := kernel.AssertBatch(facts); err != nil {
				logging.Get(logging.CategorySystemShards).Warn("[SessionPlanner] Failed to assert mangle_updates batch: %v", err)
			}
		}
	}

	// 2. Track self-correction for autopoiesis
	if control.SelfCorrection != nil && control.SelfCorrection.Triggered {
		logging.SystemShards("[SessionPlanner] Self-correction triggered: %s", control.SelfCorrection.Hypothesis)
		selfCorrFact := core.Fact{
			Predicate: "self_correction_triggered",
			Args:      []interface{}{s.ID, control.SelfCorrection.Hypothesis, time.Now().Unix()},
		}
		_ = kernel.Assert(selfCorrFact)
	}

	// 3. Log reasoning trace for debugging/learning
	if control.ReasoningTrace != "" {
		logging.SystemShardsDebug("[SessionPlanner] Reasoning trace: %.200s...", control.ReasoningTrace)
	}
}

// parseMangleAtomPlanner removed in favor of core.FilterMangleUpdates.
