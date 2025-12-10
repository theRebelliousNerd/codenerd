package prompt

import (
	"strings"
	"testing"
	"time"

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
			CategoryHallucination,
			CategoryLanguage,
			CategoryFramework,
			CategoryDomain,
			CategoryCampaign,
			CategoryInit,
			CategoryNorthstar,
			CategoryOuroboros,
			CategoryContext,
			CategoryExemplar,
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
			result := matchSelector(tt.selector, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
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
