package prompt_evolution

import (
	"context"

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
