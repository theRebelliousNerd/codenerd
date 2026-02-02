package prompt

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCompilationContext(t *testing.T) {
	t.Run("creates context with defaults", func(t *testing.T) {
		cc := NewCompilationContext()

		assert.Equal(t, "/active", cc.OperationalMode)
		assert.Equal(t, 200000, cc.TokenBudget) // Default updated to 200k
		assert.Equal(t, 8000, cc.ReservedTokens)
		assert.Equal(t, 20, cc.SemanticTopK)
	})
}

func TestCompilationContext_WorldStates(t *testing.T) {
	tests := []struct {
		name     string
		context  *CompilationContext
		expected []string
	}{
		{
			name:     "no world states when all zero/false",
			context:  &CompilationContext{},
			expected: nil,
		},
		{
			name: "failing tests",
			context: &CompilationContext{
				FailingTestCount: 5,
			},
			expected: []string{"failing_tests"},
		},
		{
			name: "diagnostics",
			context: &CompilationContext{
				DiagnosticCount: 3,
			},
			expected: []string{"diagnostics"},
		},
		{
			name: "large refactor",
			context: &CompilationContext{
				IsLargeRefactor: true,
			},
			expected: []string{"large_refactor"},
		},
		{
			name: "security issues",
			context: &CompilationContext{
				HasSecurityIssues: true,
			},
			expected: []string{"security_issues"},
		},
		{
			name: "new files",
			context: &CompilationContext{
				HasNewFiles: true,
			},
			expected: []string{"new_files"},
		},
		{
			name: "high churn",
			context: &CompilationContext{
				IsHighChurn: true,
			},
			expected: []string{"high_churn"},
		},
		{
			name: "multiple states",
			context: &CompilationContext{
				FailingTestCount:  5,
				DiagnosticCount:   3,
				IsLargeRefactor:   true,
				HasSecurityIssues: false,
			},
			expected: []string{"failing_tests", "diagnostics", "large_refactor"},
		},
		{
			name: "all states active",
			context: &CompilationContext{
				FailingTestCount:  1,
				DiagnosticCount:   1,
				IsLargeRefactor:   true,
				HasSecurityIssues: true,
				HasNewFiles:       true,
				IsHighChurn:       true,
			},
			expected: []string{"failing_tests", "diagnostics", "large_refactor", "security_issues", "new_files", "high_churn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			states := tt.context.WorldStates()
			if diff := cmp.Diff(tt.expected, states); diff != "" {
				t.Errorf("WorldStates() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompilationContext_AvailableTokens(t *testing.T) {
	tests := []struct {
		name     string
		budget   int
		reserved int
		expected int
	}{
		{
			name:     "standard calculation",
			budget:   10000,
			reserved: 2000,
			expected: 8000,
		},
		{
			name:     "zero reserved",
			budget:   10000,
			reserved: 0,
			expected: 10000,
		},
		{
			name:     "reserved equals budget",
			budget:   10000,
			reserved: 10000,
			expected: 0,
		},
		{
			name:     "reserved exceeds budget returns zero",
			budget:   5000,
			reserved: 10000,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &CompilationContext{
				TokenBudget:    tt.budget,
				ReservedTokens: tt.reserved,
			}

			result := cc.AvailableTokens()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompilationContext_Validate(t *testing.T) {
	tests := []struct {
		name      string
		context   *CompilationContext
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid context",
			context: &CompilationContext{
				TokenBudget:    10000,
				ReservedTokens: 2000,
			},
			wantError: false,
		},
		{
			name: "zero token budget",
			context: &CompilationContext{
				TokenBudget:    0,
				ReservedTokens: 2000,
			},
			wantError: true,
			errorMsg:  "budget must be positive",
		},
		{
			name: "negative token budget",
			context: &CompilationContext{
				TokenBudget:    -1000,
				ReservedTokens: 2000,
			},
			wantError: true,
			errorMsg:  "budget must be positive",
		},
		{
			name: "negative reserved tokens",
			context: &CompilationContext{
				TokenBudget:    10000,
				ReservedTokens: -100,
			},
			wantError: true,
			errorMsg:  "reserved tokens cannot be negative",
		},
		{
			name: "reserved equals budget",
			context: &CompilationContext{
				TokenBudget:    10000,
				ReservedTokens: 10000,
			},
			wantError: true,
			errorMsg:  "reserved tokens",
		},
		{
			name: "reserved exceeds budget",
			context: &CompilationContext{
				TokenBudget:    5000,
				ReservedTokens: 10000,
			},
			wantError: true,
			errorMsg:  "reserved tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.context.Validate()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompilationContext_Clone(t *testing.T) {
	t.Run("creates deep copy", func(t *testing.T) {
		original := &CompilationContext{
			OperationalMode:   "/active",
			CampaignPhase:     "/planning",
			CampaignID:        "campaign-123",
			CampaignName:      "Test Campaign",
			BuildLayer:        "/service",
			InitPhase:         "/analysis",
			NorthstarPhase:    "/requirements",
			OuroborosStage:    "/detection",
			IntentVerb:        "/fix",
			IntentTarget:      "main.go",
			ShardType:         "/coder",
			ShardID:           "shard-456",
			ShardName:         "Test Shard",
			FailingTestCount:  5,
			DiagnosticCount:   3,
			IsLargeRefactor:   true,
			HasSecurityIssues: false,
			HasNewFiles:       true,
			IsHighChurn:       false,
			Language:          "/go",
			Frameworks:        []string{"/bubbletea", "/lipgloss"},
			TokenBudget:       50000,
			ReservedTokens:    5000,
			SemanticQuery:     "test query",
			SemanticTopK:      30,
		}

		clone := original.Clone()

		// Verify values are equal
		assert.Equal(t, original.OperationalMode, clone.OperationalMode)
		assert.Equal(t, original.CampaignPhase, clone.CampaignPhase)
		assert.Equal(t, original.ShardType, clone.ShardType)
		assert.Equal(t, original.Frameworks, clone.Frameworks)
		assert.Equal(t, original.TokenBudget, clone.TokenBudget)

		// Verify independence - modifying clone doesn't affect original
		clone.Frameworks[0] = "/modified"
		assert.Equal(t, "/bubbletea", original.Frameworks[0])

		clone.OperationalMode = "/debugging"
		assert.Equal(t, "/active", original.OperationalMode)
	})

	t.Run("handles nil frameworks", func(t *testing.T) {
		original := &CompilationContext{
			TokenBudget:    10000,
			ReservedTokens: 1000,
		}

		clone := original.Clone()
		assert.Nil(t, clone.Frameworks)
	})
}

func TestCompilationContext_FluentAPI(t *testing.T) {
	t.Run("WithOperationalMode", func(t *testing.T) {
		cc := NewCompilationContext().WithOperationalMode("/debugging")
		assert.Equal(t, "/debugging", cc.OperationalMode)
	})

	t.Run("WithCampaign", func(t *testing.T) {
		cc := NewCompilationContext().WithCampaign("camp-123", "Test Campaign", "/planning")
		assert.Equal(t, "camp-123", cc.CampaignID)
		assert.Equal(t, "Test Campaign", cc.CampaignName)
		assert.Equal(t, "/planning", cc.CampaignPhase)
	})

	t.Run("WithShard", func(t *testing.T) {
		cc := NewCompilationContext().WithShard("/coder", "shard-456", "My Coder")
		assert.Equal(t, "/coder", cc.ShardType)
		assert.Equal(t, "shard-456", cc.ShardID)
		assert.Equal(t, "My Coder", cc.ShardName)
	})

	t.Run("WithLanguage", func(t *testing.T) {
		cc := NewCompilationContext().WithLanguage("/go", "/bubbletea", "/lipgloss")
		assert.Equal(t, "/go", cc.Language)
		assert.Equal(t, []string{"/bubbletea", "/lipgloss"}, cc.Frameworks)
	})

	t.Run("WithIntent", func(t *testing.T) {
		cc := NewCompilationContext().WithIntent("/fix", "main.go")
		assert.Equal(t, "/fix", cc.IntentVerb)
		assert.Equal(t, "main.go", cc.IntentTarget)
	})

	t.Run("WithTokenBudget", func(t *testing.T) {
		cc := NewCompilationContext().WithTokenBudget(50000, 5000)
		assert.Equal(t, 50000, cc.TokenBudget)
		assert.Equal(t, 5000, cc.ReservedTokens)
	})

	t.Run("WithSemanticQuery", func(t *testing.T) {
		cc := NewCompilationContext().WithSemanticQuery("test query", 50)
		assert.Equal(t, "test query", cc.SemanticQuery)
		assert.Equal(t, 50, cc.SemanticTopK)
	})

	t.Run("WithSemanticQuery zero topK preserves default", func(t *testing.T) {
		cc := NewCompilationContext().WithSemanticQuery("test query", 0)
		assert.Equal(t, "test query", cc.SemanticQuery)
		assert.Equal(t, 20, cc.SemanticTopK) // Default value preserved
	})

	t.Run("chained fluent calls", func(t *testing.T) {
		cc := NewCompilationContext().
			WithOperationalMode("/debugging").
			WithShard("/coder", "shard-1", "Coder").
			WithLanguage("/go", "/bubbletea").
			WithIntent("/fix", "bug.go").
			WithTokenBudget(80000, 8000)

		assert.Equal(t, "/debugging", cc.OperationalMode)
		assert.Equal(t, "/coder", cc.ShardType)
		assert.Equal(t, "/go", cc.Language)
		assert.Equal(t, "/fix", cc.IntentVerb)
		assert.Equal(t, 80000, cc.TokenBudget)
	})
}

func TestCompilationContext_String(t *testing.T) {
	t.Run("returns human-readable summary", func(t *testing.T) {
		cc := NewCompilationContext().
			WithOperationalMode("/active").
			WithCampaign("", "", "/planning").
			WithShard("/coder", "", "").
			WithLanguage("/go").
			WithIntent("/fix", "").
			WithTokenBudget(100000, 8000)

		str := cc.String()

		assert.Contains(t, str, "mode=/active")
		assert.Contains(t, str, "campaign=/planning")
		assert.Contains(t, str, "shard=/coder")
		assert.Contains(t, str, "lang=/go")
		assert.Contains(t, str, "intent=/fix")
		assert.Contains(t, str, "budget=92000")
	})
}

func TestCompilationContext_ToContextFacts(t *testing.T) {
	t.Run("generates facts for non-empty fields", func(t *testing.T) {
		cc := &CompilationContext{
			OperationalMode:  "/active",
			CampaignPhase:    "/planning",
			ShardType:        "/coder",
			Language:         "/go",
			Frameworks:       []string{"/bubbletea", "/gin"},
			FailingTestCount: 5,
			DiagnosticCount:  3,
		}

		facts := cc.ToContextFacts()

		// Should have facts for: mode, campaign_phase, shard_type, language,
		// 2 frameworks, 2 world states (failing_tests, diagnostics)
		// Note: Empty string fields are not included
		assert.GreaterOrEqual(t, len(facts), 6)

		// Verify facts are properly formatted compile_context predicates
		for _, fact := range facts {
			assert.Contains(t, fact, "compile_context(")
			assert.Contains(t, fact, ").")
		}
	})

	t.Run("includes all frameworks", func(t *testing.T) {
		cc := &CompilationContext{
			Frameworks: []string{"/bubbletea", "/gin", "/lipgloss"},
		}

		facts := cc.ToContextFacts()

		frameworkCount := 0
		for _, fact := range facts {
			if strings.Contains(fact.(string), "/framework") {
				frameworkCount++
			}
		}
		assert.Equal(t, 3, frameworkCount)
	})

	t.Run("includes world states", func(t *testing.T) {
		cc := &CompilationContext{
			FailingTestCount: 5,
			IsLargeRefactor:  true,
		}

		facts := cc.ToContextFacts()

		worldStateCount := 0
		for _, fact := range facts {
			if strings.Contains(fact.(string), "/world_state") {
				worldStateCount++
			}
		}
		assert.Equal(t, 2, worldStateCount)
	})

	t.Run("generates valid Mangle syntax", func(t *testing.T) {
		cc := &CompilationContext{
			OperationalMode: "/dream",
			Language:        "/go",
			IntentVerb:      "/debug",
		}

		facts := cc.ToContextFacts()

		// Check format: compile_context(/dimension, /value).
		for _, fact := range facts {
			s := fact.(string)
			assert.True(t, strings.HasPrefix(s, "compile_context("))
			assert.True(t, strings.HasSuffix(s, ")."))
		}
	})
}

func TestAllContextDimensions(t *testing.T) {
	dimensions := AllContextDimensions()

	t.Run("returns all expected dimensions", func(t *testing.T) {
		expectedNames := []string{
			"operational_mode",
			"campaign_phase",
			"build_layer",
			"init_phase",
			"northstar_phase",
			"ouroboros_stage",
			"intent_verb",
			"shard_type",
			"language",
			"world_state",
		}

		actualNames := make([]string, len(dimensions))
		for i, d := range dimensions {
			actualNames[i] = d.Name
		}

		assert.ElementsMatch(t, expectedNames, actualNames)
	})

	t.Run("each dimension has values", func(t *testing.T) {
		for _, dim := range dimensions {
			assert.NotEmpty(t, dim.Values, "dimension %s should have values", dim.Name)
			assert.NotEmpty(t, dim.Description, "dimension %s should have description", dim.Name)
		}
	})

	t.Run("operational mode has expected values", func(t *testing.T) {
		var opMode *ContextDimension
		for i := range dimensions {
			if dimensions[i].Name == "operational_mode" {
				opMode = &dimensions[i]
				break
			}
		}

		require.NotNil(t, opMode)
		assert.Contains(t, opMode.Values, "/active")
		assert.Contains(t, opMode.Values, "/dream")
		assert.Contains(t, opMode.Values, "/debugging")
	})

	t.Run("shard type has expected values", func(t *testing.T) {
		var shardType *ContextDimension
		for i := range dimensions {
			if dimensions[i].Name == "shard_type" {
				shardType = &dimensions[i]
				break
			}
		}

		require.NotNil(t, shardType)
		assert.Contains(t, shardType.Values, "/coder")
		assert.Contains(t, shardType.Values, "/tester")
		assert.Contains(t, shardType.Values, "/reviewer")
	})
}

// Benchmark tests

func BenchmarkWorldStates(b *testing.B) {
	cc := &CompilationContext{
		FailingTestCount:  5,
		DiagnosticCount:   3,
		IsLargeRefactor:   true,
		HasSecurityIssues: true,
		HasNewFiles:       true,
		IsHighChurn:       true,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cc.WorldStates()
	}
}

func BenchmarkCompilationContextClone(b *testing.B) {
	cc := &CompilationContext{
		OperationalMode:   "/active",
		CampaignPhase:     "/planning",
		CampaignID:        "campaign-123",
		ShardType:         "/coder",
		Language:          "/go",
		Frameworks:        []string{"/bubbletea", "/lipgloss", "/gin"},
		FailingTestCount:  5,
		DiagnosticCount:   3,
		IsLargeRefactor:   true,
		TokenBudget:       100000,
		ReservedTokens:    8000,
		SemanticQuery:     "test query for benchmarking",
		SemanticTopK:      30,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cc.Clone()
	}
}

func BenchmarkValidate(b *testing.B) {
	cc := &CompilationContext{
		TokenBudget:    100000,
		ReservedTokens: 8000,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cc.Validate()
	}
}

func BenchmarkToContextFacts(b *testing.B) {
	cc := &CompilationContext{
		OperationalMode:  "/active",
		CampaignPhase:    "/planning",
		ShardType:        "/coder",
		Language:         "/go",
		Frameworks:       []string{"/bubbletea", "/lipgloss"},
		FailingTestCount: 5,
		DiagnosticCount:  3,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cc.ToContextFacts()
	}
}
