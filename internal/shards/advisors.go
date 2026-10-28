// Package shards provides specialist agent management.
// This file answers which strategic advisors an executor may consult; the
// consultation engine itself is internal/consultation.
package shards

import "sort"

// GetStrategicAdvisorsFor returns strategic advisors that can assist a given executor.
func GetStrategicAdvisorsFor(executorName string) []string {
	// Based on specialist_assists rule in shards.mg:
	// Strategic advisors can assist any technical executor
	var advisors []string
	for name, class := range DefaultSpecialistClassifications {
		if class.ExecutionMode == SpecialistModeAdvisor &&
			class.KnowledgeTier == TierStrategic {
			advisors = append(advisors, name)
		}
	}
	// Sorted: the list feeds user-visible delegation output, and map order
	// would shuffle the consultation phase from run to run.
	sort.Strings(advisors)
	return advisors
}

// ShouldConsultBeforeExecution determines if an executor should consult advisors first.
func ShouldConsultBeforeExecution(executorName string, taskComplexity string) bool {
	class, ok := GetSpecialistClassification(executorName)
	if !ok {
		return false
	}

	// Only executors consult advisors
	if class.ExecutionMode != SpecialistModeExecutor {
		return false
	}

	// High complexity tasks should consult strategic advisors
	return taskComplexity == "high" || taskComplexity == "complex"
}
