package shards

import (
	"strings"
)

// Task complexity levels, the value set of task_complexity/2.
const (
	TaskComplexityHigh   = "/high"
	TaskComplexityNormal = "/normal"
)

// TaskComplexity classifies a delegated task for strategic_advisor_required
// (policy/shards.mg): a task that names complexity, security, architecture,
// criticality or refactoring, or that spans more than three files, is /high.
// internal/experts asserts the level as task_complexity(Task, Level) with the
// task it selects shard agents for.
func TaskComplexity(task string, fileCount int) string {
	lower := strings.ToLower(task)
	for _, marker := range []string{"complex", "security", "architecture", "critical", "refactor"} {
		if strings.Contains(lower, marker) {
			return TaskComplexityHigh
		}
	}
	if fileCount > 3 {
		return TaskComplexityHigh
	}
	return TaskComplexityNormal
}
