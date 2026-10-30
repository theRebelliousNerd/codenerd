package prompt_evolution

import (
	"context"
	"encoding/json"
	"testing"

	"codenerd/internal/types"
)

// Test conveniences; production constructs these through the evolver.

// NewAtomGenerator creates a new atom generator pinning at the default scope,
// whose role prompts are a fixed stand-in (internal/system's
// role_prompts_autopoiesis_test.go checks the real compile).
func NewAtomGenerator(llmClient LLMClient, strategyStore *StrategyStore) *AtomGenerator {
	return NewAtomGeneratorWithPinScope(llmClient, fixedRolePrompts{}, strategyStore, DefaultPinScope)
}

// fixedRolePrompts answers every role with its name.
type fixedRolePrompts struct{}

func (fixedRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	return "role prompt for " + req.Role, nil
}

// AllProblemTypes returns all defined problem types.
func AllProblemTypes() []ProblemType {
	return []ProblemType{
		ProblemDebugging,
		ProblemFeatureCreation,
		ProblemRefactoring,
		ProblemTesting,
		ProblemDocumentation,
		ProblemPerformance,
		ProblemSecurity,
		ProblemAPIIntegration,
		ProblemDataMigration,
		ProblemConfigSetup,
		ProblemErrorHandling,
		ProblemConcurrency,
		ProblemTypeSystem,
		ProblemDependencyMgmt,
		ProblemCodeReview,
		ProblemResearch,
	}
}

// testGenerationCohort uses the production selection and projection path;
// generator fixtures cannot supply an independently rendered evidence list.
func testGenerationCohort(t *testing.T, verdicts []*JudgeVerdict) *evidenceCohort {
	t.Helper()
	if len(verdicts) == 0 {
		return nil
	}
	group := failureGroup{key: FailureGroupKey{ProblemType: "debugging", ShardType: "/coder"}}
	for _, v := range verdicts {
		if v == nil {
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		group.failures = append(group.failures, &ExecutionRecord{TaskID: v.TaskID, VerdictVersion: versionOf(b), Verdict: v})
	}
	c, err := selectEvidenceCohort(context.Background(), group, *DefaultEvolverConfig())
	if err != nil {
		t.Fatal(err)
	}
	return c
}
