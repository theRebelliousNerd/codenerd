package prompt_evolution

// Test conveniences; production constructs these through the evolver.

// NewAtomGenerator creates a new atom generator pinning at the default scope.
func NewAtomGenerator(llmClient LLMClient, strategyStore *StrategyStore) *AtomGenerator {
	return NewAtomGeneratorWithPinScope(llmClient, strategyStore, DefaultPinScope)
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
