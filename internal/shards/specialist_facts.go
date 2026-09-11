package shards

import (
	"sort"
	"strings"

	"codenerd/internal/core"
)

// Kernel projection of specialist matching.
//
// policy/shards.mg has always carried the specialist routing rules —
// specialist_should_execute, specialist_should_advise,
// strategic_advisor_required, specialist_assists — but nothing in Go ever
// asserted their inputs (specialist_classification, specialist_match,
// task_complexity), so every one of them starved while the same decisions
// were re-implemented as Go booleans (ShouldSpecialistExecuteTask,
// ShouldConsultBeforeExecution) that the kernel never saw. Two truths, one
// live. These builders make the Go matcher the producer; the chat delegation
// path then asks the kernel and keeps the Go form only as the nil-kernel
// fallback.

// SpecialistAtom is the /name identity a specialist carries in the kernel:
// its registry name, lowercased. Every specialist_* predicate keys on it.
func SpecialistAtom(name string) core.MangleAtom {
	return core.MangleAtom("/" + strings.ToLower(strings.TrimSpace(name)))
}

// ClassificationFacts projects DefaultSpecialistClassifications into
// specialist_classification(Agent, Mode, Tier) and
// specialist_campaign_role(Agent, Role), in name order so the fact set is
// stable across loads.
func ClassificationFacts() []core.Fact {
	names := make([]string, 0, len(DefaultSpecialistClassifications))
	for name := range DefaultSpecialistClassifications {
		names = append(names, name)
	}
	sort.Strings(names)

	facts := make([]core.Fact, 0, 2*len(names))
	for _, name := range names {
		class := DefaultSpecialistClassifications[name]
		facts = append(facts, core.Fact{
			Predicate: "specialist_classification",
			Args:      []any{SpecialistAtom(name), core.MangleAtom(string(class.ExecutionMode)), core.MangleAtom(string(class.KnowledgeTier))},
		})
		if class.CampaignIntegration != "" {
			facts = append(facts, core.Fact{
				Predicate: "specialist_campaign_role",
				Args:      []any{SpecialistAtom(name), core.MangleAtom(class.CampaignIntegration)},
			})
		}
	}
	return facts
}

// Task complexity levels, the value set of task_complexity/2.
const (
	TaskComplexityHigh   = "/high"
	TaskComplexityNormal = "/normal"
)

// TaskComplexity classifies a task for strategic_advisor_required: a task
// that names complexity, security, architecture, criticality or refactoring,
// or that spans more than three files, is /high. This is the heuristic the
// chat delegation path used inline; it lives here so the fact and the
// fallback boolean cannot drift apart.
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

// MatchFacts projects a match set for one task into
// specialist_match(Agent, Task, Confidence) with Confidence on the 0-100
// scale the rules compare against, plus task_complexity(Task, Level).
func MatchFacts(task, complexity string, matches []SpecialistMatch) []core.Fact {
	facts := make([]core.Fact, 0, len(matches)+1)
	for _, match := range matches {
		if strings.TrimSpace(match.AgentName) == "" {
			continue
		}
		facts = append(facts, core.Fact{
			Predicate: "specialist_match",
			Args:      []any{SpecialistAtom(match.AgentName), task, int64(match.Score*100 + 0.5)},
		})
	}
	facts = append(facts, core.Fact{
		Predicate: "task_complexity",
		Args:      []any{task, core.MangleAtom(complexity)},
	})
	return facts
}
