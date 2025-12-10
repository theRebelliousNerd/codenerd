package reviewer

import (
	"testing"
)

func TestEnhancementResultHelpers(t *testing.T) {
	// Test TotalSuggestions
	result := &EnhancementResult{
		FileSuggestions:   []FileSuggestion{{File: "test.go", Title: "Test"}},
		ModuleSuggestions: []ModuleSuggestion{{Package: "pkg", Title: "Test"}},
		SystemInsights:    []SystemInsight{{Category: "arch", Title: "Test"}},
		FeatureIdeas:      []FeatureIdea{{Title: "Test Feature"}},
	}

	if result.TotalSuggestions() != 4 {
		t.Errorf("TotalSuggestions() = %d, want 4", result.TotalSuggestions())
	}

	if !result.HasSuggestions() {
		t.Error("HasSuggestions() = false, want true")
	}

	// Test empty result
	emptyResult := &EnhancementResult{}
	if emptyResult.TotalSuggestions() != 0 {
		t.Errorf("Empty TotalSuggestions() = %d, want 0", emptyResult.TotalSuggestions())
	}
	if emptyResult.HasSuggestions() {
		t.Error("Empty HasSuggestions() = true, want false")
	}

	// Test nil result
	var nilResult *EnhancementResult
	if nilResult.TotalSuggestions() != 0 {
		t.Errorf("Nil TotalSuggestions() = %d, want 0", nilResult.TotalSuggestions())
	}
	if nilResult.HasSuggestions() {
		t.Error("Nil HasSuggestions() = true, want false")
	}
}

func TestCreativeFirstPassHelpers(t *testing.T) {
	fp := &CreativeFirstPass{
		FileSuggestions:   []FileSuggestion{{Category: "refactor", Title: "Simplify"}},
		ModuleSuggestions: []ModuleSuggestion{{Category: "api_design", Title: "Better API"}},
	}

	if fp.TotalSuggestions() != 2 {
		t.Errorf("TotalSuggestions() = %d, want 2", fp.TotalSuggestions())
	}

	query := fp.BuildSearchQuery()
	if query == "" {
		t.Error("BuildSearchQuery() returned empty string")
	}
	if len(query) > 500 {
		t.Errorf("BuildSearchQuery() returned string > 500 chars: %d", len(query))
	}

	// Test ToResult
	result := fp.ToResult()
	if result == nil {
		t.Fatal("ToResult() returned nil")
	}
	if result.FirstPassCount != 2 {
		t.Errorf("ToResult().FirstPassCount = %d, want 2", result.FirstPassCount)
	}
	if result.EnhancementRatio != 1.0 {
		t.Errorf("ToResult().EnhancementRatio = %f, want 1.0", result.EnhancementRatio)
	}
}

func TestParseEnhanceFlag(t *testing.T) {
	tests := []struct {
		name          string
		task          string
		wantEnhance   bool
		wantFileCount int
	}{
		{
			name:          "no flag",
			task:          "review file:test.go",
			wantEnhance:   false,
			wantFileCount: 1,
		},
		{
			name:          "with --andEnhance",
			task:          "review file:test.go --andEnhance",
			wantEnhance:   true,
			wantFileCount: 1,
		},
		{
			name:          "with --enhance",
			task:          "review file:test.go --enhance",
			wantEnhance:   true,
			wantFileCount: 1,
		},
		{
			name:          "flag before file",
			task:          "review --andEnhance file:foo.go",
			wantEnhance:   true,
			wantFileCount: 1,
		},
		{
			name:          "multiple files",
			task:          "review file:a.go file:b.go --andEnhance",
			wantEnhance:   true,
			wantFileCount: 2,
		},
	}

	r := &ReviewerShard{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := r.parseTask(tt.task)
			if err != nil {
				t.Fatalf("parseTask() error = %v", err)
			}
			if parsed.EnableEnhancement != tt.wantEnhance {
				t.Errorf("EnableEnhancement = %v, want %v", parsed.EnableEnhancement, tt.wantEnhance)
			}
			if len(parsed.Files) != tt.wantFileCount {
				t.Errorf("len(Files) = %d, want %d", len(parsed.Files), tt.wantFileCount)
			}
		})
	}
}

func TestNeuroSymbolicConfigEnhancement(t *testing.T) {
	config := DefaultNeuroSymbolicConfig()

	// Default should have enhancement disabled
	if config.EnableCreativeEnhancement {
		t.Error("Default EnableCreativeEnhancement should be false")
	}
	if !config.EnableSelfInterrogation {
		t.Error("Default EnableSelfInterrogation should be true")
	}
	if config.MaxSuggestionsPerLevel != 5 {
		t.Errorf("MaxSuggestionsPerLevel = %d, want 5", config.MaxSuggestionsPerLevel)
	}
	if config.VectorSearchLimit != 10 {
		t.Errorf("VectorSearchLimit = %d, want 10", config.VectorSearchLimit)
	}
}
