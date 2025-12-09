package reviewer

import (
	"strings"
)

// =============================================================================
// SPECIALIST RECOMMENDATION SYSTEM
// =============================================================================
// Detects technologies in reviewed code and recommends specialist shards.

// SpecialistRecommendation suggests a specialist shard for working on this code.
type SpecialistRecommendation struct {
	ShardName  string   `json:"shard_name"` // e.g., "rod", "golang", "react"
	Reason     string   `json:"reason"`     // Why this specialist is recommended
	Confidence float64  `json:"confidence"` // 0.0-1.0
	ForFiles   []string `json:"for_files"`  // Which files this applies to
	TaskHints  []string `json:"task_hints"` // Suggested tasks for the specialist
}

// TechnologyPattern maps file patterns and imports to specialist shards.
type TechnologyPattern struct {
	ShardName    string   // Specialist shard name
	FilePatterns []string // File path patterns (glob-like)
	ImportHints  []string // Import statements to look for
	ContentHints []string // Code patterns to detect
	TaskHints    []string // Suggested tasks for this specialist
	Description  string   // Human-readable description
}

// knownTechnologies defines the mapping from code patterns to specialist shards.
var knownTechnologies = []TechnologyPattern{
	{
		ShardName:    "rod",
		FilePatterns: []string{"browser", "scraper", "crawler", "selenium", "playwright"},
		ImportHints:  []string{"github.com/go-rod/rod", "chromedp", "selenium", "playwright"},
		ContentHints: []string{"Browser()", "MustPage", "MustElement", "CDP", "DevTools"},
		TaskHints:    []string{"implement browser automation", "add page scraping", "fix element selection"},
		Description:  "Browser automation with Rod/CDP",
	},
	{
		ShardName:    "golang",
		FilePatterns: []string{".go"},
		ImportHints:  []string{}, // Any Go file qualifies
		ContentHints: []string{"func ", "type ", "interface ", "struct "},
		TaskHints:    []string{"refactor code", "add error handling", "improve concurrency"},
		Description:  "Go language patterns and idioms",
	},
	{
		ShardName:    "react",
		FilePatterns: []string{".tsx", ".jsx", "component"},
		ImportHints:  []string{"react", "useState", "useEffect", "next/"},
		ContentHints: []string{"<", "/>", "useState", "useEffect", "className"},
		TaskHints:    []string{"add component", "fix state management", "improve rendering"},
		Description:  "React/Next.js frontend development",
	},
	{
		ShardName:    "mangle",
		FilePatterns: []string{".mg", "mangle", "policy", "schema"},
		ImportHints:  []string{},
		ContentHints: []string{"Decl ", ":-", "fn:", "let "},
		TaskHints:    []string{"add policy rule", "define predicate", "fix constraint"},
		Description:  "Mangle/Datalog logic programming",
	},
	{
		ShardName:    "sql",
		FilePatterns: []string{"database", "store", "repository", "dao"},
		ImportHints:  []string{"database/sql", "sqlx", "gorm", "pgx", "sqlite"},
		ContentHints: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE"},
		TaskHints:    []string{"optimize query", "add migration", "fix schema"},
		Description:  "SQL database operations",
	},
	{
		ShardName:    "api",
		FilePatterns: []string{"handler", "controller", "endpoint", "route", "api"},
		ImportHints:  []string{"net/http", "gin", "echo", "fiber", "chi"},
		ContentHints: []string{"http.Handler", "c.JSON", "r.GET", "r.POST"},
		TaskHints:    []string{"add endpoint", "fix authentication", "improve validation"},
		Description:  "REST API development",
	},
	{
		ShardName:    "testing",
		FilePatterns: []string{"_test.go", "test_", "spec."},
		ImportHints:  []string{"testing", "testify", "gomock", "jest", "pytest"},
		ContentHints: []string{"func Test", "t.Run", "assert.", "expect("},
		TaskHints:    []string{"add test cases", "improve coverage", "fix flaky test"},
		Description:  "Test writing and coverage",
	},
}

// detectSpecialists analyzes file content and returns specialist recommendations.
func (r *ReviewerShard) detectSpecialists(files []string, contents map[string]string) []SpecialistRecommendation {
	var recommendations []SpecialistRecommendation
	shardScores := make(map[string]*SpecialistRecommendation)

	for _, file := range files {
		content := contents[file]
		lowerFile := strings.ToLower(file)
		lowerContent := strings.ToLower(content)

		for _, tech := range knownTechnologies {
			score := 0.0
			matches := false

			// Check file patterns
			for _, pattern := range tech.FilePatterns {
				if strings.Contains(lowerFile, strings.ToLower(pattern)) {
					score += 0.3
					matches = true
					break
				}
			}

			// Check import hints
			for _, imp := range tech.ImportHints {
				if strings.Contains(content, imp) {
					score += 0.4
					matches = true
					break
				}
			}

			// Check content hints
			hintMatches := 0
			for _, hint := range tech.ContentHints {
				if strings.Contains(lowerContent, strings.ToLower(hint)) {
					hintMatches++
				}
			}
			if len(tech.ContentHints) > 0 && hintMatches > 0 {
				score += float64(hintMatches) / float64(len(tech.ContentHints)) * 0.3
				matches = true
			}

			// Only add if we found matches
			if matches && score > 0.2 {
				if existing, ok := shardScores[tech.ShardName]; ok {
					// Update existing
					if score > existing.Confidence {
						existing.Confidence = score
					}
					existing.ForFiles = append(existing.ForFiles, file)
				} else {
					// Create new
					shardScores[tech.ShardName] = &SpecialistRecommendation{
						ShardName:  tech.ShardName,
						Reason:     tech.Description,
						Confidence: score,
						ForFiles:   []string{file},
						TaskHints:  tech.TaskHints,
					}
				}
			}
		}
	}

	// Convert to slice and sort by confidence
	for _, rec := range shardScores {
		// Cap confidence at 1.0
		if rec.Confidence > 1.0 {
			rec.Confidence = 1.0
		}
		// Only include high-confidence recommendations
		if rec.Confidence >= 0.3 {
			recommendations = append(recommendations, *rec)
		}
	}

	// Sort by confidence (descending)
	for i := 0; i < len(recommendations); i++ {
		for j := i + 1; j < len(recommendations); j++ {
			if recommendations[j].Confidence > recommendations[i].Confidence {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	// Limit to top 3 recommendations
	if len(recommendations) > 3 {
		recommendations = recommendations[:3]
	}

	return recommendations
}
