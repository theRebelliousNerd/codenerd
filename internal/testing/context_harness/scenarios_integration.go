package context_harness

// Integration scenarios that test real system behavior.
// These require --mode=real to run with actual ActivationEngine, Compressor, and Kernel.

// CampaignPhaseTransitionScenario tests phase reset and context paging.
func CampaignPhaseTransitionScenario() *Scenario {
	turns := make([]Turn, 60)

	// Phase 1: Planning (turns 0-19)
	for i := 0; i < 20; i++ {
		turns[i] = Turn{
			TurnID:        i,
			Speaker:       "user",
			Message:       planningMessages[i%len(planningMessages)],
			Intent:        "plan",
			CampaignPhase: "planning",
			Metadata: TurnMetadata{
				Topics: []string{"planning", "strategy", "architecture"},
			},
		}
	}

	// Phase 2: Implementation (turns 20-59)
	for i := 20; i < 60; i++ {
		isTransition := i == 20
		turns[i] = Turn{
			TurnID:          i,
			Speaker:         "user",
			Message:         implementationMessages[(i-20)%len(implementationMessages)],
			Intent:          "implement",
			CampaignPhase:   "implementation",
			PhaseTransition: isTransition,
			Metadata: TurnMetadata{
				Topics:          []string{"implementation", "coding"},
				FilesReferenced: []string{"auth/handler.go", "auth/middleware.go"},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "campaign-phase-transition",
		Name:        "Campaign Phase Transition",
		Description: "Tests phase reset, context paging, and campaign-aware activation",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		InitialFacts: []string{
			`current_campaign("auth-migration")`,
			`campaign_phase("auth-migration", "planning", 1)`,
			`campaign_phase("auth-migration", "implementation", 2)`,
			`phase_objective("planning", "Define migration strategy")`,
		},
		Turns: turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    25,
				Query:        "What was the migration strategy from planning?",
				MustRetrieve: []string{"turn_5_topic", "turn_10_topic"},
				MinRecall:    0.6,
				MinPrecision: 0.5,
				Description:  "Verify Phase 1 context still retrievable after phase transition",
				ValidateActivation: &ActivationValidation{
					FactPattern:    "campaign_phase.*implementation",
					MinCampaignBoost: 20,
				},
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:   0.4,
			AvgRetrievalRecall: 0.6,
			AvgRetrievalPrec:   0.5,
		},
	}
}

// SWEBenchIssueResolutionScenario tests issue context with tiered file boosting.
func SWEBenchIssueResolutionScenario() *Scenario {
	turns := make([]Turn, 50)

	for i := 0; i < 50; i++ {
		turns[i] = Turn{
			TurnID:  i,
			Speaker: "user",
			Message: issueResolutionMessages[i%len(issueResolutionMessages)],
			Intent:  "debug",
			Metadata: TurnMetadata{
				Topics:          []string{"debugging", "issue", "fix"},
				FilesReferenced: []string{"django/db/models/query.py"},
				ErrorMessages:   []string{"TypeError: unsupported operand type(s)"},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "swebench-issue-resolution",
		Name:        "SWE-Bench Issue Resolution",
		Description: "Tests issue context with tiered file boosting and keyword weights",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		InitialFacts: []string{
			`active_issue("django__django-12345")`,
			`issue_mentioned_file("django__django-12345", "django/db/models/query.py", 1)`,
			`issue_error_type("django__django-12345", "TypeError")`,
		},
		Turns: turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    30,
				Query:        "What error was occurring in query.py?",
				MustRetrieve: []string{"turn_0_error_message", "turn_5_topic"},
				MinRecall:    0.7,
				MinPrecision: 0.5,
				Description:  "Verify tier 1 files get +50 boost",
				ValidateActivation: &ActivationValidation{
					FactPattern: "turn_references_file.*query.py",
					MinIssueBoost: 30,
				},
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:   0.35,
			AvgRetrievalRecall: 0.7,
			AvgRetrievalPrec:   0.5,
		},
	}
}

// TokenBudgetOverflowScenario tests compression triggering at 60% utilization.
func TokenBudgetOverflowScenario() *Scenario {
	turns := make([]Turn, 100)

	for i := 0; i < 100; i++ {
		// Generate increasingly verbose messages to trigger compression
		msg := verboseMessages[i%len(verboseMessages)]
		turns[i] = Turn{
			TurnID:  i,
			Speaker: "user",
			Message: msg,
			Intent:  "implement",
			Metadata: TurnMetadata{
				Topics:          []string{"coding", "implementation"},
				FilesReferenced: []string{"main.go", "handler.go", "service.go"},
				SymbolsReferenced: []string{"HandleRequest", "ProcessData", "ValidateInput"},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "token-budget-overflow",
		Name:        "Token Budget Overflow",
		Description: "Tests compression triggering at 60% token utilization",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		Turns:       turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    60,
				Query:        "What files have we been working on?",
				MustRetrieve: []string{"turn_50_references_file"},
				MinRecall:    0.5,
				MinPrecision: 0.4,
				Description:  "Verify compression triggered and context maintained",
				ValidateCompression: &CompressionCheckpoint{
					ExpectTriggered:      true,
					MinRatio:             2.0,
					MaxBudgetUtilization: 0.8,
				},
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      0.3,
			AvgRetrievalRecall:    0.5,
			AvgRetrievalPrec:      0.4,
			TokenBudgetViolations: 0,
		},
	}
}

// DependencySpreadingScenario tests symbol graph spreading with depth decay.
func DependencySpreadingScenario() *Scenario {
	turns := make([]Turn, 40)

	for i := 0; i < 40; i++ {
		turns[i] = Turn{
			TurnID:  i,
			Speaker: "user",
			Message: dependencyMessages[i%len(dependencyMessages)],
			Intent:  "refactor",
			Metadata: TurnMetadata{
				Topics:            []string{"refactoring", "dependencies"},
				SymbolsReferenced: []string{"ProcessData", "ValidateInput", "HandleRequest"},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "dependency-spreading",
		Name:        "Dependency Spreading",
		Description: "Tests symbol graph spreading with 50% depth decay per level",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		InitialFacts: []string{
			`symbol_graph("HandleRequest", "ProcessData", "calls")`,
			`symbol_graph("ProcessData", "ValidateInput", "calls")`,
			`symbol_graph("ValidateInput", "ParseJSON", "calls")`,
		},
		Turns: turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    25,
				Query:        "What functions call ProcessData?",
				MustRetrieve: []string{"turn_0_references_symbol"},
				MinRecall:    0.6,
				MinPrecision: 0.5,
				Description:  "Verify callers of focused symbols get boost",
				ValidateActivation: &ActivationValidation{
					FactPattern:        "turn_references_symbol.*ProcessData",
					MinDependencyBoost: 15,
				},
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:   0.4,
			AvgRetrievalRecall: 0.6,
			AvgRetrievalPrec:   0.5,
		},
	}
}

// VerbSpecificBoostingScenario tests 8 intent verb boosts.
func VerbSpecificBoostingScenario() *Scenario {
	verbs := []string{"fix", "debug", "refactor", "test", "implement", "review", "research", "explain"}
	turns := make([]Turn, 30)

	for i := 0; i < 30; i++ {
		verb := verbs[i%len(verbs)]
		turns[i] = Turn{
			TurnID:  i,
			Speaker: "user",
			Message: verbMessages[verb],
			Intent:  verb,
			Metadata: TurnMetadata{
				Topics: []string{verb},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "verb-specific-boosting",
		Name:        "Verb-Specific Boosting",
		Description: "Tests that correct predicates are boosted per intent verb",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		Turns:       turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    15,
				Query:        "What diagnostics do we have?",
				MustRetrieve: []string{"turn_1_topic"},
				MinRecall:    0.5,
				MinPrecision: 0.4,
				Description:  "Verify debug intent boosts diagnostic predicates",
				ValidateActivation: &ActivationValidation{
					FactPattern:       "turn_.*_topic.*debug",
					MinRelevanceBoost: 20,
				},
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:   0.45,
			AvgRetrievalRecall: 0.5,
			AvgRetrievalPrec:   0.4,
		},
	}
}

// EphemeralFilteringScenario tests boot guard and fact category filtering.
func EphemeralFilteringScenario() *Scenario {
	turns := make([]Turn, 20)

	for i := 0; i < 20; i++ {
		turns[i] = Turn{
			TurnID:  i,
			Speaker: "user",
			Message: "What are the project patterns?",
			Intent:  "query",
			Metadata: TurnMetadata{
				Topics: []string{"patterns", "conventions"},
			},
		}
	}

	return &Scenario{
		ScenarioID:  "ephemeral-filtering",
		Name:        "Ephemeral Filtering",
		Description: "Tests that ephemeral predicates are filtered at boot",
		Mode:        RealMode,
		Category:    CategoryIntegration,
		InitialFacts: []string{
			// Persistent facts - should be loadable
			`project_pattern("error_handling", "wrap errors with context")`,
			`learned_preference("prefer_explicit_returns", true)`,
			// Ephemeral facts - should be filtered at boot
			// (these would normally be rejected by the kernel's boot filter)
		},
		Turns: turns,
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    10,
				Query:        "What patterns do we use?",
				MustRetrieve: []string{"project_pattern"},
				ShouldAvoid:  []string{"user_intent", "pending_action"},
				MinRecall:    0.7,
				MinPrecision: 0.6,
				Description:  "Verify persistent facts load but ephemeral facts filtered",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:   0.5,
			AvgRetrievalRecall: 0.7,
			AvgRetrievalPrec:   0.6,
		},
	}
}

// IntegrationScenarios returns all integration scenarios.
func IntegrationScenarios() []*Scenario {
	return []*Scenario{
		CampaignPhaseTransitionScenario(),
		SWEBenchIssueResolutionScenario(),
		TokenBudgetOverflowScenario(),
		DependencySpreadingScenario(),
		VerbSpecificBoostingScenario(),
		EphemeralFilteringScenario(),
	}
}

// Message templates for scenarios
var planningMessages = []string{
	"Let's plan the authentication migration",
	"What's the best approach for the auth system?",
	"We need to design the token refresh flow",
	"How should we handle session state?",
	"Let's document the migration strategy",
}

var implementationMessages = []string{
	"Implement the auth handler",
	"Add middleware for token validation",
	"Create the session manager",
	"Write the JWT parsing logic",
	"Add error handling for auth failures",
}

var issueResolutionMessages = []string{
	"I'm seeing a TypeError in query.py",
	"The queryset filter is failing",
	"Need to fix the annotation issue",
	"The aggregation is returning wrong results",
	"Debug the model lookup error",
}

var verboseMessages = []string{
	"This is a verbose message about implementing a complex feature with many details about the architecture, design patterns, and implementation considerations that we need to take into account",
	"Another detailed explanation covering multiple aspects of the codebase including dependencies, testing strategies, and deployment considerations that span across several modules",
	"Comprehensive analysis of the current system state with recommendations for improvements, refactoring opportunities, and technical debt that should be addressed",
}

var dependencyMessages = []string{
	"Refactor ProcessData to improve modularity",
	"Check the callers of ValidateInput",
	"Update HandleRequest to use new validation",
	"Trace the data flow through the pipeline",
	"Find all usages of ParseJSON",
}

var verbMessages = map[string]string{
	"fix":       "Fix the bug in the auth handler",
	"debug":     "Debug the connection timeout issue",
	"refactor":  "Refactor the data processing pipeline",
	"test":      "Write tests for the validation logic",
	"implement": "Implement the new caching layer",
	"review":    "Review the recent changes to auth",
	"research":  "Research best practices for JWT handling",
	"explain":   "Explain how the session manager works",
}
