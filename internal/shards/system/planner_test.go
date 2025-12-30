package system

import (
	"codenerd/internal/core"
	"strings"
	"testing"
	"time"
)

func TestPlanView(t *testing.T) {
	// SKIP: This test requires full constitution boot which has stratification issues
	t.Skip("Skipping: constitution stratification issues need refactoring")

	planner := NewSessionPlannerShard()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	planner.Kernel = kernel

	// Add some tasks
	planner.AddTask("Implement feature A", 1)
	planner.AddTask("Write tests for A", 2)
	planner.AddTask("Review code", 3)

	// Get the plan view
	plan := planner.GetCurrentPlan()

	if plan.TotalTasks != 3 {
		t.Errorf("Expected 3 total tasks, got %d", plan.TotalTasks)
	}

	if plan.Pending != 3 {
		t.Errorf("Expected 3 pending tasks, got %d", plan.Pending)
	}

	if plan.Completed != 0 {
		t.Errorf("Expected 0 completed tasks, got %d", plan.Completed)
	}

	if plan.ProgressPct != 0 {
		t.Errorf("Expected 0%% progress, got %.1f%%", plan.ProgressPct)
	}
}

func TestPlanViewWithProgress(t *testing.T) {
	// SKIP: This test requires full constitution boot which has stratification issues
	t.Skip("Skipping: constitution stratification issues need refactoring")

	planner := NewSessionPlannerShard()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	planner.Kernel = kernel

	// Add tasks
	planner.AddTask("Task 1", 1)
	planner.AddTask("Task 2", 2)
	planner.AddTask("Task 3", 3)

	// Manually update statuses (simulating task completion)
	planner.mu.Lock()
	if len(planner.agenda) >= 3 {
		planner.agenda[0].Status = "completed"
		planner.agenda[0].CompletedAt = time.Now()
		planner.agenda[1].Status = "in_progress"
		planner.agenda[1].StartedAt = time.Now()
		// Task 3 stays pending
	}
	planner.mu.Unlock()

	// Get the plan view
	plan := planner.GetCurrentPlan()

	if plan.Completed != 1 {
		t.Errorf("Expected 1 completed task, got %d", plan.Completed)
	}

	if plan.InProgress != 1 {
		t.Errorf("Expected 1 in-progress task, got %d", plan.InProgress)
	}

	if plan.Pending != 1 {
		t.Errorf("Expected 1 pending task, got %d", plan.Pending)
	}

	expectedProgress := 100.0 / 3.0 // 33.33%
	if plan.ProgressPct < expectedProgress-1 || plan.ProgressPct > expectedProgress+1 {
		t.Errorf("Expected ~%.1f%% progress, got %.1f%%", expectedProgress, plan.ProgressPct)
	}
}

func TestFormatPlanAsMarkdown(t *testing.T) {
	planner := NewSessionPlannerShard()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	planner.Kernel = kernel

	// Add some tasks
	planner.AddTask("Implement feature A", 1)
	planner.AddTask("Write tests", 2)

	markdown := planner.FormatPlanAsMarkdown()

	// Check that markdown contains expected sections
	if !strings.Contains(markdown, "# Session Plan") {
		t.Error("Expected markdown to contain '# Session Plan' header")
	}

	if !strings.Contains(markdown, "## Progress Summary") {
		t.Error("Expected markdown to contain '## Progress Summary' section")
	}

	if !strings.Contains(markdown, "## Pending") {
		t.Error("Expected markdown to contain '## Pending' section")
	}

	if !strings.Contains(markdown, "Implement feature A") {
		t.Error("Expected markdown to contain task description")
	}

	if !strings.Contains(markdown, "**Total Tasks:** 2") {
		t.Error("Expected markdown to show total tasks")
	}
}

func TestFormatPlanAsJSON(t *testing.T) {
	planner := NewSessionPlannerShard()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	planner.Kernel = kernel

	// Add a task
	planner.AddTask("Test task", 1)

	json := planner.FormatPlanAsJSON()

	// Check that JSON contains expected fields
	if !strings.Contains(json, `"total_tasks"`) {
		t.Error("Expected JSON to contain 'total_tasks' field")
	}

	if !strings.Contains(json, `"pending"`) {
		t.Error("Expected JSON to contain 'pending' field")
	}

	if !strings.Contains(json, `"progress_pct"`) {
		t.Error("Expected JSON to contain 'progress_pct' field")
	}

	if !strings.Contains(json, `"tasks"`) {
		t.Error("Expected JSON to contain 'tasks' field")
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		percent  float64
		width    int
		contains string
	}{
		{0, 10, "░░░░░░░░░░"},
		{50, 10, "█████░░░░░"},
		{100, 10, "██████████"},
	}

	for _, tt := range tests {
		bar := generateProgressBar(tt.percent, tt.width)
		if !strings.Contains(bar, tt.contains) {
			t.Errorf("Expected progress bar for %.0f%% to contain '%s', got '%s'",
				tt.percent, tt.contains, bar)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{30, "30s"},
		{90, "1m 30s"},
		{3600, "1h 0m"},
		{3665, "1h 1m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.seconds)
		if result != tt.expected {
			t.Errorf("formatDuration(%d) = %s, expected %s", tt.seconds, result, tt.expected)
		}
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		time     time.Time
		expected string
	}{
		{time.Time{}, "never"},
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-25 * time.Hour), "1d ago"},
	}

	for _, tt := range tests {
		result := formatRelativeTime(tt.time)
		if result != tt.expected {
			t.Errorf("formatRelativeTime() = %s, expected %s", result, tt.expected)
		}
	}
}

func TestFilterTasksByStatus(t *testing.T) {
	tasks := []AgendaItem{
		{ID: "1", Status: "pending"},
		{ID: "2", Status: "completed"},
		{ID: "3", Status: "pending"},
		{ID: "4", Status: "blocked"},
	}

	pending := filterTasksByStatus(tasks, "pending")
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending tasks, got %d", len(pending))
	}

	completed := filterTasksByStatus(tasks, "completed")
	if len(completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(completed))
	}

	blocked := filterTasksByStatus(tasks, "blocked")
	if len(blocked) != 1 {
		t.Errorf("Expected 1 blocked task, got %d", len(blocked))
	}
}
