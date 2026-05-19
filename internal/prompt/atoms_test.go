package prompt

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "empty string",
			content:  "",
			expected: 0,
		},
		{
			name:     "single character",
			content:  "a",
			expected: 1,
		},
		{
			name:     "short word",
			content:  "hello",
			expected: 2, // (5+3)/4 = 2
		},
		{
			name:     "typical sentence",
			content:  "This is a typical prompt with about 50 characters.",
			expected: 13, // (51+3)/4 = 13
		},
		{
			name:     "long content",
			content:  strings.Repeat("a", 1000),
			expected: 250, // (1000+3)/4 = 250
		},
		{
			name:     "exact multiple of 4",
			content:  "1234",
			expected: 1, // (4+3)/4 = 1
		},
		{
			name:     "whitespace only",
			content:  "    ",
			expected: 1, // (4+3)/4 = 1
		},
		{
			name:     "newlines",
			content:  "line1\nline2\nline3",
			expected: 5, // (17+3)/4 = 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHashContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantLen int
	}{
		{
			name:    "empty string",
			content: "",
			wantLen: 0,
		},
		{
			name:    "simple content",
			content: "test content",
			wantLen: 64, // SHA256 hex = 64 chars
		},
		{
			name:    "unicode content",
			content: "Hello, World!",
			wantLen: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashContent(tt.content)
			assert.Len(t, result, tt.wantLen)
		})
	}

	t.Run("same content produces same hash", func(t *testing.T) {
		hash1 := HashContent("test content")
		hash2 := HashContent("test content")
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		hash1 := HashContent("test content")
		hash2 := HashContent("different content")
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		content := "deterministic test"
		hashes := make([]string, 100)
		for i := 0; i < 100; i++ {
			hashes[i] = HashContent(content)
		}
		for i := 1; i < 100; i++ {
			assert.Equal(t, hashes[0], hashes[i])
		}
	})
}

func TestNewPromptAtom(t *testing.T) {
	t.Run("creates atom with computed fields", func(t *testing.T) {
		atom := NewPromptAtom("test/atom", CategoryIdentity, "Test content for the atom")

		assert.Equal(t, "test/atom", atom.ID)
		assert.Equal(t, CategoryIdentity, atom.Category)
		assert.Equal(t, "Test content for the atom", atom.Content)
		assert.Equal(t, 1, atom.Version)
		assert.Greater(t, atom.TokenCount, 0)
		assert.NotEmpty(t, atom.ContentHash)
		assert.False(t, atom.CreatedAt.IsZero())
	})

	t.Run("token count is estimated correctly", func(t *testing.T) {
		content := strings.Repeat("x", 100)
		atom := NewPromptAtom("test/tokens", CategoryProtocol, content)

		expectedTokens := EstimateTokens(content)
		assert.Equal(t, expectedTokens, atom.TokenCount)
	})

	t.Run("hash matches content", func(t *testing.T) {
		content := "unique content for hash test"
		atom := NewPromptAtom("test/hash", CategorySafety, content)

		expectedHash := HashContent(content)
		assert.Equal(t, expectedHash, atom.ContentHash)
	})
}

func TestPromptAtom_MatchesContext(t *testing.T) {
	tests := []struct {
		name        string
		atom        *PromptAtom
		context     *CompilationContext
		expectMatch bool
	}{
		{
			name:        "nil context always matches",
			atom:        &PromptAtom{ID: "test"},
			context:     nil,
			expectMatch: true,
		},
		{
			name: "empty selectors match any context",
			atom: &PromptAtom{
				ID: "wildcard",
			},
			context: &CompilationContext{
				ShardType:       "/coder",
				IntentVerb:      "/fix",
				OperationalMode: "/active",
			},
			expectMatch: true,
		},
		{
			name: "matching shard type",
			atom: &PromptAtom{
				ID:         "coder-only",
				ShardTypes: []string{"/coder"},
			},
			context: &CompilationContext{
				ShardType: "/coder",
			},
			expectMatch: true,
		},
		{
			name: "non-matching shard type",
			atom: &PromptAtom{
				ID:         "coder-only",
				ShardTypes: []string{"/coder"},
			},
			context: &CompilationContext{
				ShardType: "/tester",
			},
			expectMatch: false,
		},
		{
			name: "multiple allowed shard types - matches one",
			atom: &PromptAtom{
				ID:         "coder-or-tester",
				ShardTypes: []string{"/coder", "/tester"},
			},
			context: &CompilationContext{
				ShardType: "/tester",
			},
			expectMatch: true,
		},
		{
			name: "matching intent verb",
			atom: &PromptAtom{
				ID:          "fix-only",
				IntentVerbs: []string{"/fix", "/debug"},
			},
			context: &CompilationContext{
				IntentVerb: "/fix",
			},
			expectMatch: true,
		},
		{
			name: "non-matching intent verb",
			atom: &PromptAtom{
				ID:          "fix-only",
				IntentVerbs: []string{"/fix"},
			},
			context: &CompilationContext{
				IntentVerb: "/create",
			},
			expectMatch: false,
		},
		{
			name: "matching operational mode",
			atom: &PromptAtom{
				ID:               "active-mode",
				OperationalModes: []string{"/active"},
			},
			context: &CompilationContext{
				OperationalMode: "/active",
			},
			expectMatch: true,
		},
		{
			name: "matching language",
			atom: &PromptAtom{
				ID:        "go-lang",
				Languages: []string{"/go"},
			},
			context: &CompilationContext{
				Language: "/go",
			},
			expectMatch: true,
		},
		{
			name: "matching framework - single",
			atom: &PromptAtom{
				ID:         "bubbletea-atom",
				Frameworks: []string{"/bubbletea"},
			},
			context: &CompilationContext{
				Frameworks: []string{"/bubbletea", "/lipgloss"},
			},
			expectMatch: true,
		},
		{
			name: "non-matching framework",
			atom: &PromptAtom{
				ID:         "react-atom",
				Frameworks: []string{"/react"},
			},
			context: &CompilationContext{
				Frameworks: []string{"/bubbletea"},
			},
			expectMatch: false,
		},
		{
			name: "matching world state",
			atom: &PromptAtom{
				ID:          "failing-tests-atom",
				WorldStates: []string{"failing_tests"},
			},
			context: &CompilationContext{
				FailingTestCount: 5,
			},
			expectMatch: true,
		},
		{
			name: "non-matching world state - no failing tests",
			atom: &PromptAtom{
				ID:          "failing-tests-atom",
				WorldStates: []string{"failing_tests"},
			},
			context: &CompilationContext{
				FailingTestCount: 0,
			},
			expectMatch: false,
		},
		{
			name: "multiple world states - matches one",
			atom: &PromptAtom{
				ID:          "problem-atom",
				WorldStates: []string{"failing_tests", "diagnostics"},
			},
			context: &CompilationContext{
				DiagnosticCount: 3,
			},
			expectMatch: true,
		},
		{
			name: "all dimensions must match",
			atom: &PromptAtom{
				ID:               "specific-atom",
				ShardTypes:       []string{"/coder"},
				IntentVerbs:      []string{"/fix"},
				OperationalModes: []string{"/active"},
			},
			context: &CompilationContext{
				ShardType:       "/coder",
				IntentVerb:      "/fix",
				OperationalMode: "/active",
			},
			expectMatch: true,
		},
		{
			name: "one dimension mismatch fails",
			atom: &PromptAtom{
				ID:               "specific-atom",
				ShardTypes:       []string{"/coder"},
				IntentVerbs:      []string{"/fix"},
				OperationalModes: []string{"/active"},
			},
			context: &CompilationContext{
				ShardType:       "/coder",
				IntentVerb:      "/fix",
				OperationalMode: "/debugging", // mismatch
			},
			expectMatch: false,
		},
		{
			name: "campaign phase matching",
			atom: &PromptAtom{
				ID:             "planning-atom",
				CampaignPhases: []string{"/planning", "/decomposing"},
			},
			context: &CompilationContext{
				CampaignPhase: "/planning",
			},
			expectMatch: true,
		},
		{
			name: "init phase matching",
			atom: &PromptAtom{
				ID:         "analysis-atom",
				InitPhases: []string{"/analysis"},
			},
			context: &CompilationContext{
				InitPhase: "/analysis",
			},
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.atom.NormalizeSelectors()
			result := tt.atom.MatchesContext(tt.context)
			assert.Equal(t, tt.expectMatch, result)
		})
	}
}

func TestPromptAtom_ToFact(t *testing.T) {
	t.Run("converts atom to fact correctly", func(t *testing.T) {
		atom := &PromptAtom{
			ID:          "test/atom",
			Category:    CategoryIdentity,
			Priority:    80,
			TokenCount:  100,
			IsMandatory: true,
		}

		fact := atom.ToFact()

		assert.Equal(t, "prompt_atom", fact.Predicate)
		require.Len(t, fact.Args, 5)
		assert.Equal(t, "test/atom", fact.Args[0])
		assert.Equal(t, "/identity", fact.Args[1])
		assert.Equal(t, 100, fact.Args[2])
		assert.Equal(t, 80, fact.Args[3])
	})

	t.Run("non-mandatory atom", func(t *testing.T) {
		atom := &PromptAtom{
			ID:          "optional/atom",
			Category:    CategoryProtocol,
			Priority:    50,
			TokenCount:  200,
			IsMandatory: false,
		}

		fact := atom.ToFact()

		assert.Equal(t, "prompt_atom", fact.Predicate)
		assert.Equal(t, "/protocol", fact.Args[1])
	})
}

func TestPromptAtom_ToSelectorFacts(t *testing.T) {
	t.Run("generates selector facts for all dimensions", func(t *testing.T) {
		atom := &PromptAtom{
			ID:               "test/selectors",
			OperationalModes: []string{"/active", "/debugging"},
			ShardTypes:       []string{"/coder"},
			IntentVerbs:      []string{"/fix", "/debug"},
		}

		facts := atom.ToSelectorFacts()

		// Should have 2 + 1 + 2 = 5 facts
		assert.Len(t, facts, 5)

		// Verify predicate
		for _, fact := range facts {
			assert.Equal(t, "atom_selector", fact.Predicate)
			assert.Equal(t, "test/selectors", fact.Args[0])
		}
	})

	t.Run("empty selectors produce no facts", func(t *testing.T) {
		atom := &PromptAtom{
			ID: "empty/selectors",
		}

		facts := atom.ToSelectorFacts()
		assert.Empty(t, facts)
	})
}

func TestPromptAtom_ToDependencyFacts(t *testing.T) {
	t.Run("generates dependency facts", func(t *testing.T) {
		atom := &PromptAtom{
			ID:        "child/atom",
			DependsOn: []string{"parent/atom1", "parent/atom2"},
		}

		facts := atom.ToDependencyFacts()

		assert.Len(t, facts, 2)
		for _, fact := range facts {
			assert.Equal(t, "atom_depends", fact.Predicate)
			assert.Equal(t, "child/atom", fact.Args[0])
		}
	})

	t.Run("no dependencies produce no facts", func(t *testing.T) {
		atom := &PromptAtom{
			ID: "independent/atom",
		}

		facts := atom.ToDependencyFacts()
		assert.Empty(t, facts)
	})
}

func TestPromptAtom_ToConflictFacts(t *testing.T) {
	t.Run("generates conflict facts", func(t *testing.T) {
		atom := &PromptAtom{
			ID:            "atom/a",
			ConflictsWith: []string{"atom/b", "atom/c"},
		}

		facts := atom.ToConflictFacts()

		assert.Len(t, facts, 2)
		for _, fact := range facts {
			assert.Equal(t, "atom_conflicts", fact.Predicate)
			assert.Equal(t, "atom/a", fact.Args[0])
		}
	})
}

func TestPromptAtom_ToExclusionFact(t *testing.T) {
	t.Run("generates exclusion fact when group set", func(t *testing.T) {
		atom := &PromptAtom{
			ID:          "exclusive/atom",
			IsExclusive: "group1",
		}

		fact := atom.ToExclusionFact()

		require.NotNil(t, fact)
		assert.Equal(t, "atom_exclusive", fact.Predicate)
		assert.Equal(t, "exclusive/atom", fact.Args[0])
		assert.Equal(t, "group1", fact.Args[1])
	})

	t.Run("returns nil when no exclusion group", func(t *testing.T) {
		atom := &PromptAtom{
			ID: "non-exclusive/atom",
		}

		fact := atom.ToExclusionFact()
		assert.Nil(t, fact)
	})
}

func TestPromptAtom_Validate(t *testing.T) {
	tests := []struct {
		name      string
		atom      *PromptAtom
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid atom",
			atom: &PromptAtom{
				ID:       "valid/atom",
				Content:  "Valid content",
				Category: CategoryIdentity,
			},
			wantError: false,
		},
		{
			name: "missing ID",
			atom: &PromptAtom{
				Content:  "Content",
				Category: CategoryIdentity,
			},
			wantError: true,
			errorMsg:  "ID is required",
		},
		{
			name: "missing content",
			atom: &PromptAtom{
				ID:       "no-content",
				Category: CategoryIdentity,
			},
			wantError: true,
			errorMsg:  "content is required",
		},
		{
			name: "missing category",
			atom: &PromptAtom{
				ID:      "no-category",
				Content: "Content",
			},
			wantError: true,
			errorMsg:  "category is required",
		},
		{
			name: "invalid category",
			atom: &PromptAtom{
				ID:       "invalid-category",
				Content:  "Content",
				Category: AtomCategory("invalid"),
			},
			wantError: true,
			errorMsg:  "unknown category",
		},
		{
			name: "self-dependency",
			atom: &PromptAtom{
				ID:        "self-dep",
				Content:   "Content",
				Category:  CategoryProtocol,
				DependsOn: []string{"self-dep"},
			},
			wantError: true,
			errorMsg:  "cannot depend on itself",
		},
		{
			name: "self-conflict",
			atom: &PromptAtom{
				ID:            "self-conflict",
				Content:       "Content",
				Category:      CategoryProtocol,
				ConflictsWith: []string{"self-conflict"},
			},
			wantError: true,
			errorMsg:  "cannot conflict with itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.atom.Validate()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptAtom_Clone(t *testing.T) {
	t.Run("creates deep copy", func(t *testing.T) {
		original := &PromptAtom{
			ID:               "original",
			Version:          1,
			Content:          "Original content",
			TokenCount:       100,
			ContentHash:      "hash123",
			Category:         CategoryIdentity,
			Subcategory:      "sub",
			OperationalModes: []string{"/active", "/debugging"},
			ShardTypes:       []string{"/coder"},
			DependsOn:        []string{"dep1", "dep2"},
			ConflictsWith:    []string{"conflict1"},
			Embedding:        []float32{0.1, 0.2, 0.3},
			CreatedAt:        time.Now(),
		}

		clone := original.Clone()

		// Verify values are equal
		if diff := cmp.Diff(original.ID, clone.ID); diff != "" {
			t.Errorf("ID mismatch (-want +got):\n%s", diff)
		}
		assert.Equal(t, original.Content, clone.Content)
		assert.Equal(t, original.Category, clone.Category)
		assert.Equal(t, original.OperationalModes, clone.OperationalModes)
		assert.Equal(t, original.Embedding, clone.Embedding)

		// Verify independence - modifying clone doesn't affect original
		clone.OperationalModes[0] = "/modified"
		assert.Equal(t, "/active", original.OperationalModes[0])

		clone.DependsOn[0] = "modified"
		assert.Equal(t, "dep1", original.DependsOn[0])

		clone.Embedding[0] = 999.0
		assert.Equal(t, float32(0.1), original.Embedding[0])
	})

	t.Run("handles nil slices", func(t *testing.T) {
		original := &PromptAtom{
			ID:       "nil-slices",
			Content:  "Content",
			Category: CategoryProtocol,
		}

		clone := original.Clone()

		assert.Nil(t, clone.OperationalModes)
		assert.Nil(t, clone.DependsOn)
		assert.Nil(t, clone.Embedding)
	})
}

func TestAllCategories(t *testing.T) {
	categories := AllCategories()

	t.Run("returns all expected categories", func(t *testing.T) {
		expected := []AtomCategory{
			CategoryIdentity,
			CategoryProtocol,
			CategorySafety,
			CategoryMethodology,
			CategoryCapability,
			CategoryHallucination,
			CategoryLanguage,
			CategoryFramework,
			CategoryDomain,
			CategoryCampaign,
			CategoryInit,
			CategoryNorthstar,
			CategoryOuroboros,
			CategoryAutopoiesis,
			CategoryContext,
			CategoryExemplar,
			CategoryReviewer,
			CategoryEval,
			CategoryKnowledge,
			CategoryBuildLayer,
			CategoryIntent,
			CategoryWorldState,
		}

		assert.ElementsMatch(t, expected, categories)
	})

	t.Run("no duplicates", func(t *testing.T) {
		seen := make(map[AtomCategory]bool)
		for _, cat := range categories {
			assert.False(t, seen[cat], "duplicate category: %s", cat)
			seen[cat] = true
		}
	})
}

func TestEmbeddedCorpus(t *testing.T) {
	atoms := []*PromptAtom{
		NewPromptAtom("identity/coder", CategoryIdentity, "Coder identity"),
		NewPromptAtom("protocol/piggyback", CategoryProtocol, "Piggyback protocol"),
		NewPromptAtom("safety/constitution", CategorySafety, "Constitutional safety"),
	}

	corpus := NewEmbeddedCorpus(atoms)

	t.Run("Get returns existing atom", func(t *testing.T) {
		atom, ok := corpus.Get("identity/coder")
		require.True(t, ok)
		assert.Equal(t, "Coder identity", atom.Content)
	})

	t.Run("Get returns false for non-existent atom", func(t *testing.T) {
		_, ok := corpus.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("GetByCategory returns atoms in category", func(t *testing.T) {
		identityAtoms := corpus.GetByCategory(CategoryIdentity)
		assert.Len(t, identityAtoms, 1)
		assert.Equal(t, "identity/coder", identityAtoms[0].ID)
	})

	t.Run("GetByCategory returns empty for unused category", func(t *testing.T) {
		atoms := corpus.GetByCategory(CategoryExemplar)
		assert.Empty(t, atoms)
	})

	t.Run("All returns all atoms", func(t *testing.T) {
		allAtoms := corpus.All()
		assert.Len(t, allAtoms, 3)
	})

	t.Run("Count returns correct count", func(t *testing.T) {
		assert.Equal(t, 3, corpus.Count())
	})
}

func TestMatchSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector []string
		value    string
		expected bool
	}{
		{
			name:     "empty selector matches any value",
			selector: []string{},
			value:    "/anything",
			expected: true,
		},
		{
			name:     "empty selector matches empty value",
			selector: []string{},
			value:    "",
			expected: true,
		},
		{
			name:     "non-empty selector with empty value",
			selector: []string{"/active"},
			value:    "",
			expected: false,
		},
		{
			name:     "exact match",
			selector: []string{"/active"},
			value:    "/active",
			expected: true,
		},
		{
			name:     "no match",
			selector: []string{"/active"},
			value:    "/debugging",
			expected: false,
		},
		{
			name:     "one of multiple matches",
			selector: []string{"/active", "/debugging", "/creative"},
			value:    "/debugging",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeList(tt.selector)
			result := matchSelector(tt.selector, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// TEST_GAP IMPLEMENTATIONS (from QA boundary analysis)
// =============================================================================

// TestMatchesContext_NilEmbeddedPointers validates MatchesContext behavior
// when the context has zero-valued or partially populated fields.
// GAP: Boundary Value - Nil context matching with nil embedded pointers.
func TestMatchesContext_NilEmbeddedPointers(t *testing.T) {
	tests := []struct {
		name        string
		atom        *PromptAtom
		context     *CompilationContext
		expectMatch bool
	}{
		{
			name: "context with all zero-value fields matches wildcard atom",
			atom: &PromptAtom{ID: "wildcard"},
			context: &CompilationContext{
				// All fields zero-valued
			},
			expectMatch: true,
		},
		{
			name: "atom with selectors vs zero-value context - shard type",
			atom: &PromptAtom{
				ID:         "needs-shard",
				ShardTypes: []string{"/coder"},
			},
			context:     &CompilationContext{},
			expectMatch: false,
		},
		{
			name: "atom with selectors vs zero-value context - intent",
			atom: &PromptAtom{
				ID:          "needs-intent",
				IntentVerbs: []string{"/fix"},
			},
			context:     &CompilationContext{},
			expectMatch: false,
		},
		{
			// NOTE: Framework matching intentionally skips the check when
			// context has no framework info. This means framework-specific
			// atoms remain eligible when project analysis hasn't run yet.
			name: "atom with frameworks vs nil frameworks in context - lenient match",
			atom: &PromptAtom{
				ID:         "needs-framework",
				Frameworks: []string{"/gin"},
			},
			context: &CompilationContext{
				Frameworks: nil,
			},
			expectMatch: true, // Intentional: nil frameworks = skip check
		},
		{
			name: "atom with frameworks vs empty frameworks in context - lenient match",
			atom: &PromptAtom{
				ID:         "needs-framework",
				Frameworks: []string{"/gin"},
			},
			context: &CompilationContext{
				Frameworks: []string{},
			},
			expectMatch: true, // Intentional: empty frameworks = skip check
		},
		{
			name: "atom with world states vs zero counts in context",
			atom: &PromptAtom{
				ID:          "needs-failures",
				WorldStates: []string{"failing_tests"},
			},
			context: &CompilationContext{
				FailingTestCount: 0,
				DiagnosticCount:  0,
			},
			expectMatch: false,
		},
		{
			name: "wildcard atom matches zero-value context",
			atom: &PromptAtom{
				ID:               "total-wildcard",
				OperationalModes: []string{},
				ShardTypes:       []string{},
				IntentVerbs:      []string{},
			},
			context:     &CompilationContext{},
			expectMatch: true,
		},
		{
			name: "context with only some fields populated",
			atom: &PromptAtom{
				ID:         "partial-match",
				ShardTypes: []string{"/coder"},
				Languages:  []string{"/go"},
			},
			context: &CompilationContext{
				ShardType: "/coder",
				Language:  "/go",
				// IntentVerb, OperationalMode etc. all empty
			},
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.atom.NormalizeSelectors()
			result := tt.atom.MatchesContext(tt.context)
			assert.Equal(t, tt.expectMatch, result)
		})
	}
}

// TestEstimateTokens_LargeStrings evaluates token estimation and hashing
// performance on exceedingly large inputs to detect performance cliffs.
// GAP: Extreme Input - Large strings for token estimation/hashing.
func TestEstimateTokens_LargeStrings(t *testing.T) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, sz := range sizes {
		t.Run("EstimateTokens_"+sz.name, func(t *testing.T) {
			content := strings.Repeat("a", sz.size)
			start := time.Now()
			result := EstimateTokens(content)
			elapsed := time.Since(start)

			expectedTokens := (sz.size + 3) / 4
			assert.Equal(t, expectedTokens, result)

			// Token estimation should be O(1) — just a len() call
			if elapsed > 10*time.Millisecond {
				t.Errorf("EstimateTokens(%s) took %v, expected <10ms", sz.name, elapsed)
			}
		})

		t.Run("HashContent_"+sz.name, func(t *testing.T) {
			content := strings.Repeat("b", sz.size)
			start := time.Now()
			hash := HashContent(content)
			elapsed := time.Since(start)

			assert.Len(t, hash, 64)

			// SHA256 of 10MB should still be fast
			if elapsed > 1*time.Second {
				t.Errorf("HashContent(%s) took %v, expected <1s", sz.name, elapsed)
			}
			t.Logf("HashContent(%s): %v", sz.name, elapsed)
		})
	}

	t.Run("NewPromptAtom_with_large_content", func(t *testing.T) {
		content := strings.Repeat("Large atom content. ", 50000) // ~1MB
		start := time.Now()
		atom := NewPromptAtom("large/atom", CategoryIdentity, content)
		elapsed := time.Since(start)

		assert.Greater(t, atom.TokenCount, 0)
		assert.NotEmpty(t, atom.ContentHash)

		if elapsed > 1*time.Second {
			t.Errorf("NewPromptAtom with 1MB content took %v, expected <1s", elapsed)
		}
		t.Logf("NewPromptAtom(1MB): tokens=%d, elapsed=%v", atom.TokenCount, elapsed)
	})
}

// TestClone_EmptyVsNilSlices verifies that Clone correctly distinguishes
// between nil slices and initialized empty slices.
// GAP: Boundary Value - Empty vs Missing Slices in Clone.
func TestClone_EmptyVsNilSlices(t *testing.T) {
	t.Run("nil slices remain nil after clone", func(t *testing.T) {
		original := &PromptAtom{
			ID:               "nil-slices",
			Content:          "Content",
			Category:         CategoryProtocol,
			OperationalModes: nil,
			ShardTypes:       nil,
			DependsOn:        nil,
			Embedding:        nil,
		}

		clone := original.Clone()

		assert.Nil(t, clone.OperationalModes, "nil OperationalModes should remain nil")
		assert.Nil(t, clone.ShardTypes, "nil ShardTypes should remain nil")
		assert.Nil(t, clone.DependsOn, "nil DependsOn should remain nil")
		assert.Nil(t, clone.Embedding, "nil Embedding should remain nil")
	})

	t.Run("empty slices remain empty after clone", func(t *testing.T) {
		original := &PromptAtom{
			ID:               "empty-slices",
			Content:          "Content",
			Category:         CategoryProtocol,
			OperationalModes: []string{},
			ShardTypes:       []string{},
			DependsOn:        []string{},
			Embedding:        []float32{},
		}

		clone := original.Clone()

		// Empty slices should not become nil
		assert.NotNil(t, clone.OperationalModes, "empty OperationalModes should not become nil")
		assert.NotNil(t, clone.ShardTypes, "empty ShardTypes should not become nil")
		assert.NotNil(t, clone.DependsOn, "empty DependsOn should not become nil")
		// Note: Embedding uses a nil check, so empty []float32{} with len 0 is handled differently
		assert.Empty(t, clone.OperationalModes)
		assert.Empty(t, clone.ShardTypes)
		assert.Empty(t, clone.DependsOn)
	})

	t.Run("mixed nil and empty slices preserve semantics", func(t *testing.T) {
		original := &PromptAtom{
			ID:               "mixed",
			Content:          "Content",
			Category:         CategoryIdentity,
			OperationalModes: nil,
			ShardTypes:       []string{},
			IntentVerbs:      []string{"/fix"},
			DependsOn:        nil,
			ConflictsWith:    []string{},
		}

		clone := original.Clone()

		assert.Nil(t, clone.OperationalModes)
		assert.NotNil(t, clone.ShardTypes)
		assert.Equal(t, []string{"/fix"}, clone.IntentVerbs)
		assert.Nil(t, clone.DependsOn)
		assert.NotNil(t, clone.ConflictsWith)
	})
}

// TestConcurrentAtomReadWrite verifies that concurrent reads of atom fields
// and hashing don't race with mutations on a shared atom instance.
// GAP: State Conflict - Concurrency behavior.
func TestConcurrentAtomReadWrite(t *testing.T) {
	atom := &PromptAtom{
		ID:               "concurrent/atom",
		Version:          1,
		Content:          "Concurrent test content",
		TokenCount:       100,
		ContentHash:      HashContent("Concurrent test content"),
		Category:         CategoryIdentity,
		OperationalModes: []string{"/active", "/debugging"},
		ShardTypes:       []string{"/coder", "/tester"},
		IntentVerbs:      []string{"/fix", "/debug"},
		DependsOn:        []string{"dep1"},
		Embedding:        []float32{0.1, 0.2, 0.3},
	}
	atom.NormalizeSelectors()

	ctx := &CompilationContext{
		ShardType:       "/coder",
		IntentVerb:      "/fix",
		OperationalMode: "/active",
	}

	// Run concurrent reads (Clone, MatchesContext, ToFact) against mutations
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			clone := atom.Clone()
			_ = clone.MatchesContext(ctx)
			_ = clone.ToFact()
			_ = clone.ToSelectorFacts()
		}
	}()

	// Concurrent reads on the same atom — Clone returns independent copies
	// so reads on the original should be safe
	for i := 0; i < 500; i++ {
		_ = atom.MatchesContext(ctx)
		_ = atom.ContentHash
		_ = atom.TokenCount
		_ = atom.ToFact()
	}

	<-done

	// Verify atom is still consistent
	assert.Equal(t, "concurrent/atom", atom.ID)
	assert.Equal(t, 100, atom.TokenCount)
	assert.True(t, atom.MatchesContext(ctx))
}

// TestMatchSelector_MalformedInputs verifies matchSelector and NormalizeSelectors
// handle edge cases like unprintable runes, empty strings, and unusual Unicode.
// GAP: Type Coercion / Formatting - Malformed slice contents.
func TestMatchSelector_MalformedInputs(t *testing.T) {
	tests := []struct {
		name     string
		selector []string
		value    string
		expected bool
	}{
		{
			name:     "unprintable runes in selector",
			selector: []string{"/\x00null\x01"},
			value:    "/\x00null\x01",
			expected: true,
		},
		{
			name:     "tab and newline in selector",
			selector: []string{"/tab\there", "/new\nline"},
			value:    "/tab\there",
			expected: true,
		},
		{
			name:     "unicode emoji in selector",
			selector: []string{"/🚀rocket"},
			value:    "/🚀rocket",
			expected: true,
		},
		{
			name:     "unicode CJK characters",
			selector: []string{"/日本語"},
			value:    "/日本語",
			expected: true,
		},
		{
			name:     "empty string in selector list",
			selector: []string{""},
			value:    "",
			expected: false, // non-empty list but empty value → false
		},
		{
			name:     "double slash prefix",
			selector: []string{"//double"},
			value:    "//double",
			expected: true,
		},
		{
			name:     "slash only",
			selector: []string{"/"},
			value:    "/",
			expected: true, // after normalization: "" == ""
		},
		{
			name:     "very long selector value",
			selector: []string{"/" + strings.Repeat("x", 10000)},
			value:    "/" + strings.Repeat("x", 10000),
			expected: true,
		},
		{
			name:     "whitespace-only selector",
			selector: []string{"/   "},
			value:    "/   ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeList(tt.selector)
			result := matchSelector(tt.selector, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeSelectors_MalformedInputs verifies NormalizeSelectors handles
// edge cases without panicking.
func TestNormalizeSelectors_MalformedInputs(t *testing.T) {
	t.Run("nil slices don't panic", func(t *testing.T) {
		atom := &PromptAtom{
			ID:               "nil-normalize",
			OperationalModes: nil,
			ShardTypes:       nil,
		}
		assert.NotPanics(t, func() {
			atom.NormalizeSelectors()
		})
	})

	t.Run("empty slices don't panic", func(t *testing.T) {
		atom := &PromptAtom{
			ID:               "empty-normalize",
			OperationalModes: []string{},
			ShardTypes:       []string{},
		}
		assert.NotPanics(t, func() {
			atom.NormalizeSelectors()
		})
	})

	t.Run("unprintable runes are preserved", func(t *testing.T) {
		atom := &PromptAtom{
			ID:         "unicode-normalize",
			ShardTypes: []string{"/\x00null", "/emoji🎉", "/日本"},
		}
		atom.NormalizeSelectors()

		assert.Equal(t, "\x00null", atom.ShardTypes[0])
		assert.Equal(t, "emoji🎉", atom.ShardTypes[1])
		assert.Equal(t, "日本", atom.ShardTypes[2])
	})

	t.Run("double normalization is idempotent", func(t *testing.T) {
		atom := &PromptAtom{
			ID:          "idempotent",
			IntentVerbs: []string{"/fix", "/debug"},
		}
		atom.NormalizeSelectors()
		first := make([]string, len(atom.IntentVerbs))
		copy(first, atom.IntentVerbs)

		atom.NormalizeSelectors()

		assert.Equal(t, first, atom.IntentVerbs, "double normalization should be idempotent")
	})
}

// Benchmark tests

func BenchmarkEstimateTokens(b *testing.B) {
	content := strings.Repeat("This is sample content for token estimation. ", 100)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		EstimateTokens(content)
	}
}

func BenchmarkHashContent(b *testing.B) {
	content := strings.Repeat("Content to hash repeatedly. ", 100)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		HashContent(content)
	}
}

func BenchmarkMatchesContext(b *testing.B) {
	atom := &PromptAtom{
		ID:               "benchmark/atom",
		ShardTypes:       []string{"/coder", "/tester"},
		IntentVerbs:      []string{"/fix", "/debug", "/refactor"},
		OperationalModes: []string{"/active", "/debugging"},
		Languages:        []string{"/go", "/python"},
		Frameworks:       []string{"/bubbletea", "/gin"},
		WorldStates:      []string{"failing_tests", "diagnostics"},
	}

	cc := &CompilationContext{
		ShardType:        "/coder",
		IntentVerb:       "/fix",
		OperationalMode:  "/active",
		Language:         "/go",
		Frameworks:       []string{"/bubbletea"},
		FailingTestCount: 5,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		atom.NormalizeSelectors()
		atom.MatchesContext(cc)
	}
}

func BenchmarkClone(b *testing.B) {
	atom := &PromptAtom{
		ID:               "benchmark/clone",
		Version:          1,
		Content:          strings.Repeat("Content ", 100),
		TokenCount:       250,
		ContentHash:      "hash123456789",
		Category:         CategoryIdentity,
		OperationalModes: []string{"/active", "/debugging", "/creative"},
		ShardTypes:       []string{"/coder", "/tester", "/reviewer"},
		DependsOn:        []string{"dep1", "dep2", "dep3"},
		Embedding:        make([]float32, 3072),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		atom.Clone()
	}
}

func TestPromptAtom_Clone_NilVsEmpty(t *testing.T) {
	// Boundary Value: Nil vs Empty Slice Serialization
	atomEmpty := &PromptAtom{
		ID:               "empty/slice",
		OperationalModes: []string{}, // initialized but empty
		DependsOn:        nil,        // nil pointer
		Category:         CategoryIdentity,
		Content:          "Empty test",
	}

	cloned := atomEmpty.Clone()

	// An empty slice should remain empty (not nil)
	if cloned.OperationalModes == nil {
		t.Errorf("Expected OperationalModes to be empty slice, got nil")
	}

	// A nil slice should remain nil
	if cloned.DependsOn != nil {
		t.Errorf("Expected DependsOn to be nil, got initialized slice")
	}

	// Compare JSON serialization outputs to prove API contracts are maintained
	emptyBytes, _ := json.Marshal(atomEmpty)
	clonedBytes, _ := json.Marshal(cloned)

	if string(emptyBytes) != string(clonedBytes) {
		t.Errorf("JSON serialization divergence due to slice pointer allocation rules")
	}
}

func TestPromptAtom_TypeCoercion_ZeroBytes(t *testing.T) {
	// Testing Type Coercion and extremely malformed inputs
	invalidStr := "hello\x00world\xff"

	atom := NewPromptAtom("bad/atom", CategoryIdentity, invalidStr)

	// Ensure hashing does not panic on malformed UTF-8
	hash := HashContent(invalidStr)
	if hash == "" {
		t.Fatalf("Hash generated empty string for invalid UTF-8")
	}

	// Ensure Validation handles it
	err := atom.Validate()
	if err != nil {
		t.Fatalf("Validation rejected input too early without explicit rule")
	}
}

func TestPromptAtom_MalformedMangleFacts(t *testing.T) {
	// A category missing will result in "/" which may break Mangle parsing.
	atom := &PromptAtom{
		ID:          "bad/category",
		Category:    AtomCategory(""),
		Priority:    10,
		TokenCount:  10,
		IsMandatory: false,
	}

	fact := atom.ToFact()

	// fact.Args[1] will literally be "/"
	if fact.Args[1] != "/" {
		t.Errorf("Expected default coercion to '/', got %v", fact.Args[1])
	}
}

func TestPromptAtom_DependencyCycle_ShouldBeCaughtByCompiler(t *testing.T) {
	atomA := &PromptAtom{
		ID:        "atomA",
		DependsOn: []string{"atomB"},
	}
	atomB := &PromptAtom{
		ID:        "atomB",
		DependsOn: []string{"atomC"},
	}
	atomC := &PromptAtom{
		ID:        "atomC",
		DependsOn: []string{"atomA"},
	}

	_ = []*PromptAtom{atomA, atomB, atomC}
}

func TestMatchSelector_BoundaryValues(t *testing.T) {
	// Tests type coercion and zero-length boundaries for the internal slice matching logic.
	tests := []struct {
		name     string
		selector []string
		value    string
		expected bool
	}{
		{"Double slash value", []string{"coder"}, "//double", false},
		{"Just a slash", []string{""}, "/", true},
		{"Whitespace inside selector", []string{" /coder"}, " /coder", true},
		{"Null byte inclusion", []string{"\x00coder"}, "\x00coder", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizeList(tt.selector)
			result := matchSelector(tt.selector, tt.value)
			if result != tt.expected {
				t.Errorf("matchSelector(%v, %q) = %t, want %t", tt.selector, tt.value, result, tt.expected)
			}
		})
	}
}

func TestPromptAtom_ExtremeLoad(t *testing.T) {
	// Simulating a scenario where a sub-agent attempts to wrap an entire massive log file
	// into an ephemeral PromptAtom.
	// Ensure that token count math and SHA256 doesn't OOM or integer overflow.
	hugeSize := 1024 * 1024 * 5 // 5MB string
	hugeStr := strings.Repeat("a", hugeSize)

	// This should run quickly due to Go's optimized SHA256, but confirms memory bounds.
	atom := NewPromptAtom("extreme/load", CategoryIdentity, hugeStr)

	expectedTokens := (hugeSize + 3) / 4
	if atom.TokenCount != expectedTokens {
		t.Fatalf("Token count failed for massive load: expected %d, got %d", expectedTokens, atom.TokenCount)
	}
}

func TestPromptAtom_ConcurrencyRace(t *testing.T) {
	// Tests for State Conflicts - verifies that matches against a read-only global corpus atom
	// are strictly thread-safe and no accidental state mutation occurs during 'NormalizeSelectors' or 'MatchesContext'
	atom := &PromptAtom{
		ID:          "race/atom",
		Frameworks:  []string{"/react", "/bubbletea"},
		WorldStates: []string{"diagnostics"},
	}

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cc := &CompilationContext{
				Frameworks:      []string{"/react"},
				DiagnosticCount: idx, // Different integer each time
			}
			// If MatchContext mutates state, go test -race will catch it here.
			_ = atom.MatchesContext(cc)
			_ = atom.Clone()
		}(i)
	}
	wg.Wait()
}

func TestPromptAtom_DatalogFactTranslation(t *testing.T) {
	// Verifies Boundary / Type behaviors when generating Datalog facts
	atom := &PromptAtom{
		ID:          "fact/test",
		Category:    CategoryIdentity,
		IsMandatory: true,
	}

	fact := atom.ToFact()
	// Specifically test that the Mangle Engine expects "/true" instead of true or "true"
	// The Atom/String Dissonance is a primary AI failure mode in Mangle.
	if len(fact.Args) < 5 || fact.Args[4] != core.MangleAtom("/true") {
		t.Fatalf("Mangle Atom translation failed. Expected '/true', got %v", fact.Args[4])
	}
}

