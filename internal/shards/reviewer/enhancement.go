// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains type definitions for the creative enhancement pipeline.
package reviewer

// EnhancementResult holds all creative analysis output from the two-pass pipeline.
// This is generated when --andEnhance flag is passed to /review.
type EnhancementResult struct {
	// Suggestions at different levels
	FileSuggestions   []FileSuggestion   `json:"file_suggestions"`
	ModuleSuggestions []ModuleSuggestion `json:"module_suggestions"`
	SystemInsights    []SystemInsight    `json:"system_insights"`
	FeatureIdeas      []FeatureIdea      `json:"feature_ideas"`

	// Self-consultation metadata
	VectorInspiration []PastSuggestion `json:"vector_inspiration,omitempty"`
	SelfQA            []SelfQuestion   `json:"self_qa,omitempty"`

	// Pipeline stats
	FirstPassCount   int     `json:"first_pass_count"`
	SecondPassCount  int     `json:"second_pass_count"`
	EnhancementRatio float64 `json:"enhancement_ratio"` // second/first - measures creative amplification
}

// FileSuggestion represents a file-level improvement suggestion.
type FileSuggestion struct {
	File        string  `json:"file"`
	Category    string  `json:"category"` // refactor, performance, readability, testing, error_handling
	Title       string  `json:"title"`
	Description string  `json:"description"`
	CodeExample string  `json:"code_example,omitempty"`
	Effort      string  `json:"effort"`     // trivial, small, medium, large
	Confidence  float64 `json:"confidence"` // 0.0-1.0
	InspiredBy  string  `json:"inspired_by,omitempty"` // past suggestion ID if applicable
}

// ModuleSuggestion represents a package/module-level improvement suggestion.
type ModuleSuggestion struct {
	Package       string   `json:"package"`
	Category      string   `json:"category"` // api_design, coupling, cohesion, naming, abstraction
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	AffectedFiles []string `json:"affected_files"`
	Effort        string   `json:"effort"`
	Confidence    float64  `json:"confidence"`
}

// SystemInsight represents a system-wide architectural insight.
type SystemInsight struct {
	Category       string   `json:"category"` // architecture, security, scalability, maintainability
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Impact         string   `json:"impact"` // low, medium, high
	RelatedModules []string `json:"related_modules"`
	Confidence     float64  `json:"confidence"`
}

// FeatureIdea represents a potential new feature or capability.
type FeatureIdea struct {
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Rationale     string   `json:"rationale"`
	Complexity    string   `json:"complexity"` // trivial, small, medium, large, epic
	Prerequisites []string `json:"prerequisites,omitempty"`
	Confidence    float64  `json:"confidence"`
}

// PastSuggestion represents a historically similar suggestion from vector search.
type PastSuggestion struct {
	ID             string  `json:"id"`
	Similarity     float64 `json:"similarity"`
	Summary        string  `json:"summary"`
	WasImplemented bool    `json:"was_implemented"`
	ReviewID       string  `json:"review_id,omitempty"`
}

// SelfQuestion represents a question/answer from the self-interrogation phase.
type SelfQuestion struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Insight  string `json:"insight"` // What this Q&A revealed
}

// CreativeFirstPass holds intermediate results from the first creative analysis pass.
type CreativeFirstPass struct {
	FileSuggestions   []FileSuggestion   `json:"file_suggestions"`
	ModuleSuggestions []ModuleSuggestion `json:"module_suggestions"`
	SystemInsights    []SystemInsight    `json:"system_insights"`
	FeatureIdeas      []FeatureIdea      `json:"feature_ideas"`
}

// TotalSuggestions returns the total count of all suggestions.
func (fp *CreativeFirstPass) TotalSuggestions() int {
	return len(fp.FileSuggestions) + len(fp.ModuleSuggestions) +
		len(fp.SystemInsights) + len(fp.FeatureIdeas)
}

// BuildSearchQuery creates a semantic search query from first pass suggestions.
func (fp *CreativeFirstPass) BuildSearchQuery() string {
	var parts []string

	// Extract key themes from suggestions
	for _, fs := range fp.FileSuggestions {
		parts = append(parts, fs.Category+" "+fs.Title)
	}
	for _, ms := range fp.ModuleSuggestions {
		parts = append(parts, ms.Category+" "+ms.Title)
	}
	for _, si := range fp.SystemInsights {
		parts = append(parts, si.Category+" "+si.Title)
	}
	for _, fi := range fp.FeatureIdeas {
		parts = append(parts, fi.Title)
	}

	// Limit query length
	query := ""
	for _, p := range parts {
		if len(query)+len(p) > 500 {
			break
		}
		if query != "" {
			query += " "
		}
		query += p
	}

	return query
}

// ToResult converts first pass to a result (used as fallback if second pass fails).
func (fp *CreativeFirstPass) ToResult() *EnhancementResult {
	return &EnhancementResult{
		FileSuggestions:   fp.FileSuggestions,
		ModuleSuggestions: fp.ModuleSuggestions,
		SystemInsights:    fp.SystemInsights,
		FeatureIdeas:      fp.FeatureIdeas,
		FirstPassCount:    fp.TotalSuggestions(),
		SecondPassCount:   fp.TotalSuggestions(),
		EnhancementRatio:  1.0,
	}
}

// HasSuggestions returns true if any suggestions exist.
func (r *EnhancementResult) HasSuggestions() bool {
	if r == nil {
		return false
	}
	return len(r.FileSuggestions) > 0 || len(r.ModuleSuggestions) > 0 ||
		len(r.SystemInsights) > 0 || len(r.FeatureIdeas) > 0
}

// TotalSuggestions returns the total count of all suggestions.
func (r *EnhancementResult) TotalSuggestions() int {
	if r == nil {
		return 0
	}
	return len(r.FileSuggestions) + len(r.ModuleSuggestions) +
		len(r.SystemInsights) + len(r.FeatureIdeas)
}
