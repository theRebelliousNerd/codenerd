package context_harness

// GetScenario returns a pre-built test scenario by name.
func GetScenario(name string) *Scenario {
	scenarios := map[string]*Scenario{
		"debugging-marathon":      DebuggingMarathonScenario(),
		"feature-implementation":  FeatureImplementationScenario(),
		"refactoring-campaign":    RefactoringCampaignScenario(),
		"research-and-build":      ResearchAndBuildScenario(),
		"tdd-loop":                TDDLoopScenario(),
		"campaign-execution":      CampaignExecutionScenario(),
		"shard-collaboration":     ShardCollaborationScenario(),
		"mangle-policy-debug":     ManglePolicyDebugScenario(),
	}

	return scenarios[name]
}

// AllScenarios returns all available test scenarios.
func AllScenarios() []*Scenario {
	return []*Scenario{
		DebuggingMarathonScenario(),
		FeatureImplementationScenario(),
		RefactoringCampaignScenario(),
		ResearchAndBuildScenario(),
		TDDLoopScenario(),
		CampaignExecutionScenario(),
		ShardCollaborationScenario(),
		ManglePolicyDebugScenario(),
	}
}

// DebuggingMarathonScenario: 50-turn debugging session.
// Tests: Long-term context retention, error tracking, solution history.
func DebuggingMarathonScenario() *Scenario {
	return &Scenario{
		Name:        "Debugging Marathon",
		Description: "50-turn debugging session testing context retention and solution tracking",
		Turns: []Turn{
			{
				TurnID:  0,
				Speaker: "user",
				Message: "I'm getting a nil pointer dereference in handleRequest at line 142",
				Intent:  "debug",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"server/handler.go"},
					ErrorMessages:   []string{"panic: runtime error: invalid memory address or nil pointer dereference"},
					Topics:          []string{"nil-pointer", "handleRequest"},
				},
			},
			{
				TurnID:  1,
				Speaker: "assistant",
				Message: "Let me examine the handleRequest function. I see the issue - req.Context is nil when called from the middleware chain.",
				Intent:  "analyze",
			},
			{
				TurnID:  2,
				Speaker: "user",
				Message: "How do we fix this? Should we add a nil check?",
				Intent:  "debug",
			},
			{
				TurnID:  3,
				Speaker: "assistant",
				Message: "We should ensure context is always initialized in the middleware. Let me add that.",
				Intent:  "implement",
			},
			// ... more turns simulating hypothesis testing, failed attempts, etc.
			{
				TurnID:  45,
				Speaker: "user",
				Message: "What was the original error we were fixing?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(0),
					Topics:                  []string{"nil-pointer"},
				},
			},
			{
				TurnID:  49,
				Speaker: "user",
				Message: "List all the solutions we tried that didn't work",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					Topics:                  []string{"solutions", "failures"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    45,
				Query:        "What was the original error?",
				MustRetrieve: []string{"turn_0_error", "turn_0_stack_trace", "turn_0_file"},
				ShouldAvoid:  []string{"turn_30_unrelated"},
				MinRecall:    0.9,
				MinPrecision: 0.8,
				Description:  "Should recall original error after 45 turns",
			},
			{
				AfterTurn:    49,
				Query:        "List failed solutions",
				MustRetrieve: []string{"turn_5_failed_solution", "turn_15_failed_solution", "turn_25_failed_solution"},
				MinRecall:    0.8,
				MinPrecision: 0.7,
				Description:  "Should track all failed solution attempts",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      5.0, // Expect 5:1 compression
			AvgRetrievalRecall:    0.85,
			AvgRetrievalPrec:      0.80,
			AvgF1Score:            0.82,
			TokenBudgetViolations: 0,
		},
	}
}

// FeatureImplementationScenario: 75-turn feature implementation.
// Tests: Multi-phase context paging (plan → implement → test), cross-file tracking.
func FeatureImplementationScenario() *Scenario {
	return &Scenario{
		Name:        "Feature Implementation",
		Description: "75-turn feature implementation testing multi-phase context paging",
		Turns: []Turn{
			// Planning phase (turns 0-14)
			{
				TurnID:  0,
				Speaker: "user",
				Message: "I need to add user authentication to the API",
				Intent:  "plan",
				Metadata: TurnMetadata{
					Topics: []string{"authentication", "planning"},
				},
			},
			// Implementation phase (turns 15-54)
			{
				TurnID:  15,
				Speaker: "user",
				Message: "Let's start implementing. Create the User model first",
				Intent:  "implement",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"models/user.go"},
					Topics:          []string{"user-model", "implementation"},
				},
			},
			// Testing phase (turns 55-74)
			{
				TurnID:  55,
				Speaker: "user",
				Message: "Run the authentication tests",
				Intent:  "test",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"models/user_test.go"},
					Topics:          []string{"testing"},
				},
			},
			{
				TurnID:  60,
				Speaker: "user",
				Message: "What was our original plan for password hashing?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(5),
					Topics:                  []string{"planning", "password-hashing"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    60,
				Query:        "Retrieve original plan for password hashing",
				MustRetrieve: []string{"turn_5_password_plan", "turn_5_bcrypt_decision"},
				MinRecall:    0.9,
				MinPrecision: 0.85,
				Description:  "Should retrieve planning details from earlier phase",
			},
			{
				AfterTurn:    74,
				Query:        "List all test failures",
				MustRetrieve: []string{"turn_56_test_fail", "turn_62_test_fail"},
				MinRecall:    0.85,
				MinPrecision: 0.80,
				Description:  "Should track test failures across testing phase",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      6.0,
			AvgRetrievalRecall:    0.87,
			AvgRetrievalPrec:      0.83,
			TokenBudgetViolations: 0,
		},
	}
}

// RefactoringCampaignScenario: 100-turn refactoring across multiple files.
// Tests: Long-term stability, cross-file context, campaign paging.
func RefactoringCampaignScenario() *Scenario {
	return &Scenario{
		Name:        "Refactoring Campaign",
		Description: "100-turn refactoring campaign testing long-term stability",
		Turns: []Turn{
			{
				TurnID:  0,
				Speaker: "user",
				Message: "We need to refactor the entire auth system to use interfaces",
				Intent:  "refactor",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"auth/service.go", "auth/handler.go", "auth/middleware.go"},
					Topics:          []string{"refactoring", "interfaces"},
				},
			},
			// ... 98 more turns of refactoring
			{
				TurnID:  95,
				Speaker: "user",
				Message: "Why did we decide to use interfaces in the first place?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(0),
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    50,
				Query:        "Which files have we modified?",
				MustRetrieve: []string{"turn_10_modified_service", "turn_25_modified_handler", "turn_40_modified_middleware"},
				MinRecall:    0.90,
				MinPrecision: 0.85,
				Description:  "Should track all file modifications",
			},
			{
				AfterTurn:    95,
				Query:        "Original refactoring rationale",
				MustRetrieve: []string{"turn_0_rationale", "turn_0_interface_decision"},
				MinRecall:    0.95,
				MinPrecision: 0.90,
				Description:  "Should recall original decision after 95 turns",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      8.0, // Higher compression for long sessions
			AvgRetrievalRecall:    0.85,
			AvgRetrievalPrec:      0.82,
			TokenBudgetViolations: 0,
		},
	}
}

// ResearchAndBuildScenario: 80-turn research → implementation flow.
// Tests: Cross-phase context, knowledge retrieval from research phase.
func ResearchAndBuildScenario() *Scenario {
	return &Scenario{
		Name:        "Research and Build",
		Description: "80-turn research and implementation testing cross-phase knowledge retrieval",
		Turns: []Turn{
			// Research phase (turns 0-39)
			{
				TurnID:  0,
				Speaker: "user",
				Message: "Research how to implement WebSocket authentication",
				Intent:  "research",
				Metadata: TurnMetadata{
					Topics: []string{"websocket", "authentication", "research"},
				},
			},
			{
				TurnID:  10,
				Speaker: "assistant",
				Message: "Found that gorilla/websocket recommends token-based auth during handshake",
				Intent:  "research",
				Metadata: TurnMetadata{
					Topics: []string{"gorilla-websocket", "token-auth"},
				},
			},
			// Implementation phase (turns 40-79)
			{
				TurnID:  40,
				Speaker: "user",
				Message: "Let's implement WebSocket auth based on what we researched",
				Intent:  "implement",
			},
			{
				TurnID:  50,
				Speaker: "user",
				Message: "What did we learn about gorilla/websocket's auth recommendations?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(10),
					Topics:                  []string{"gorilla-websocket", "research"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    50,
				Query:        "Gorilla WebSocket auth recommendations",
				MustRetrieve: []string{"turn_10_gorilla_recommendation", "turn_10_token_auth"},
				MinRecall:    0.90,
				MinPrecision: 0.85,
				Description:  "Should retrieve research findings from earlier phase",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      7.0,
			AvgRetrievalRecall:    0.88,
			AvgRetrievalPrec:      0.84,
			TokenBudgetViolations: 0,
		},
	}
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

// =============================================================================
// codeNERD-Specific Scenarios
// =============================================================================

// TDDLoopScenario: 40-turn TDD repair loop.
// Tests: Test failure → analyze → fix → retest cycle compression.
func TDDLoopScenario() *Scenario {
	return &Scenario{
		Name:        "TDD Loop",
		Description: "40-turn TDD repair loop testing compression across test-fix cycles",
		Turns: []Turn{
			{
				TurnID:  0,
				Speaker: "user",
				Message: "Run tests on internal/core/kernel.go",
				Intent:  "test",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/core/kernel_test.go"},
					Topics:          []string{"testing"},
				},
			},
			{
				TurnID:  1,
				Speaker: "assistant",
				Message: "Tests failed: AssertionError in TestKernelQuery - expected 5 facts, got 3",
				Intent:  "test-result",
				Metadata: TurnMetadata{
					ErrorMessages:   []string{"AssertionError: expected 5 facts, got 3"},
					FilesReferenced: []string{"internal/core/kernel_test.go"},
					Topics:          []string{"test-failure"},
				},
			},
			{
				TurnID:  2,
				Speaker: "user",
				Message: "Analyze the test failure",
				Intent:  "debug",
			},
			{
				TurnID:  3,
				Speaker: "assistant",
				Message: "The kernel is missing 2 facts because LoadFacts is not persisting transitive dependencies",
				Intent:  "analysis",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/core/kernel.go"},
					Topics:          []string{"dependencies", "facts"},
				},
			},
			// ... cycles continue
			{
				TurnID:  35,
				Speaker: "user",
				Message: "What was the original test failure we were fixing?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(1),
					Topics:                  []string{"test-failure"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    35,
				Query:        "Original test failure",
				MustRetrieve: []string{"turn_1_test_failure", "turn_1_assertion_error"},
				MinRecall:    0.95,
				MinPrecision: 0.90,
				Description:  "Should recall original test failure after multiple fix cycles",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      6.0,
			AvgRetrievalRecall:    0.90,
			AvgRetrievalPrec:      0.85,
			TokenBudgetViolations: 0,
		},
	}
}

// CampaignExecutionScenario: 60-turn multi-phase campaign.
// Tests: Context paging across campaign phases, phase-aware activation.
func CampaignExecutionScenario() *Scenario {
	return &Scenario{
		Name:        "Campaign Execution",
		Description: "60-turn multi-phase campaign testing context paging and phase transitions",
		Turns: []Turn{
			// Phase 1: Planning (turns 0-14)
			{
				TurnID:  0,
				Speaker: "user",
				Message: "Start a campaign to add user authentication to the API",
				Intent:  "campaign-start",
				Metadata: TurnMetadata{
					Topics: []string{"campaign", "authentication", "planning"},
				},
			},
			{
				TurnID:  5,
				Speaker: "assistant",
				Message: "Campaign plan: Phase 1 (Database schema), Phase 2 (API endpoints), Phase 3 (Middleware)",
				Intent:  "plan",
				Metadata: TurnMetadata{
					Topics: []string{"campaign-plan", "phases"},
				},
			},
			// Phase 2: Implementation (turns 15-44)
			{
				TurnID:  15,
				Speaker: "user",
				Message: "Start Phase 1: Database schema",
				Intent:  "phase-start",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"models/user.go"},
					Topics:          []string{"database", "schema"},
				},
			},
			{
				TurnID:  30,
				Speaker: "user",
				Message: "Start Phase 2: API endpoints",
				Intent:  "phase-start",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"api/auth.go"},
					Topics:          []string{"api", "endpoints"},
				},
			},
			// Phase 3: Testing (turns 45-59)
			{
				TurnID:  45,
				Speaker: "user",
				Message: "Start Phase 3: Testing the authentication flow",
				Intent:  "phase-start",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"api/auth_test.go"},
					Topics:          []string{"testing"},
				},
			},
			{
				TurnID:  50,
				Speaker: "user",
				Message: "What was the original campaign plan from Phase 1?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(5),
					Topics:                  []string{"campaign-plan"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    50,
				Query:        "Original campaign plan",
				MustRetrieve: []string{"turn_5_campaign_plan", "turn_5_phases"},
				MinRecall:    0.90,
				MinPrecision: 0.85,
				Description:  "Should retrieve planning from earlier phase despite context paging",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      7.0,
			AvgRetrievalRecall:    0.88,
			AvgRetrievalPrec:      0.83,
			TokenBudgetViolations: 0,
		},
	}
}

// ShardCollaborationScenario: 50-turn multi-shard workflow.
// Tests: Piggyback protocol, cross-shard context, shard result tracking.
func ShardCollaborationScenario() *Scenario {
	return &Scenario{
		Name:        "Shard Collaboration",
		Description: "50-turn multi-shard workflow testing Piggyback protocol and cross-shard context",
		Turns: []Turn{
			{
				TurnID:  0,
				Speaker: "user",
				Message: "Review the security of internal/auth/handler.go",
				Intent:  "review",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/auth/handler.go"},
					Topics:          []string{"security", "review"},
				},
			},
			{
				TurnID:  1,
				Speaker: "assistant",
				Message: "[ReviewerShard] Found SQL injection vulnerability in login handler at line 42",
				Intent:  "review-result",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/auth/handler.go"},
					ErrorMessages:   []string{"SQL injection vulnerability"},
					Topics:          []string{"security-issue", "sql-injection"},
				},
			},
			{
				TurnID:  2,
				Speaker: "user",
				Message: "Fix the SQL injection issue",
				Intent:  "fix",
			},
			{
				TurnID:  3,
				Speaker: "assistant",
				Message: "[CoderShard] Fixed: Replaced string concatenation with parameterized query",
				Intent:  "fix-result",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/auth/handler.go"},
					Topics:          []string{"fix", "parameterized-query"},
				},
			},
			{
				TurnID:  4,
				Speaker: "user",
				Message: "Generate tests for the fix",
				Intent:  "test",
			},
			{
				TurnID:  5,
				Speaker: "assistant",
				Message: "[TesterShard] Generated 5 test cases covering SQL injection scenarios",
				Intent:  "test-result",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/auth/handler_test.go"},
					Topics:          []string{"testing", "sql-injection"},
				},
			},
			{
				TurnID:  40,
				Speaker: "user",
				Message: "What security issue did the ReviewerShard find originally?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(1),
					Topics:                  []string{"security-issue", "reviewer"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    40,
				Query:        "Original security issue from ReviewerShard",
				MustRetrieve: []string{"turn_1_reviewer_result", "turn_1_sql_injection"},
				MinRecall:    0.95,
				MinPrecision: 0.90,
				Description:  "Should recall ReviewerShard findings across shard transitions",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      6.5,
			AvgRetrievalRecall:    0.90,
			AvgRetrievalPrec:      0.87,
			TokenBudgetViolations: 0,
		},
	}
}

// ManglePolicyDebugScenario: 45-turn Mangle policy debugging.
// Tests: Policy rule comprehension, spreading activation for logic queries.
func ManglePolicyDebugScenario() *Scenario {
	return &Scenario{
		Name:        "Mangle Policy Debug",
		Description: "45-turn Mangle policy debugging testing logic-specific context retrieval",
		Turns: []Turn{
			{
				TurnID:  0,
				Speaker: "user",
				Message: "Explain the next_action derivation in internal/mangle/policy.gl",
				Intent:  "explain",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/mangle/policy.gl"},
					Topics:          []string{"mangle", "policy", "next_action"},
				},
			},
			{
				TurnID:  1,
				Speaker: "assistant",
				Message: "The next_action rule derives from delegate_task when a shard type is matched",
				Intent:  "explain-result",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/mangle/policy.gl"},
					Topics:          []string{"mangle-rules", "delegation"},
				},
			},
			{
				TurnID:  10,
				Speaker: "user",
				Message: "Add a new rule to delegate /test intents to the tester shard",
				Intent:  "implement",
				Metadata: TurnMetadata{
					FilesReferenced: []string{"internal/mangle/policy.gl"},
					Topics:          []string{"mangle-rule", "tester-shard"},
				},
			},
			{
				TurnID:  35,
				Speaker: "user",
				Message: "What was the original derivation rule for next_action?",
				Intent:  "recall",
				Metadata: TurnMetadata{
					IsQuestionReferringBack: true,
					ReferencesBackToTurn:    intPtr(1),
					Topics:                  []string{"mangle-rules", "next_action"},
				},
			},
		},
		Checkpoints: []Checkpoint{
			{
				AfterTurn:    35,
				Query:        "Original next_action derivation rule",
				MustRetrieve: []string{"turn_1_next_action_rule", "turn_1_delegate_task"},
				MinRecall:    0.90,
				MinPrecision: 0.85,
				Description:  "Should recall Mangle rule explanations across code changes",
			},
		},
		ExpectedMetrics: Metrics{
			CompressionRatio:      6.0,
			AvgRetrievalRecall:    0.88,
			AvgRetrievalPrec:      0.83,
			TokenBudgetViolations: 0,
		},
	}
}
