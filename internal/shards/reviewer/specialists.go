package reviewer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// AGENT REGISTRY TYPES
// =============================================================================
// Types for loading and matching against registered specialist agents.

// AgentRegistry holds the registered specialists from .nerd/agents.json
type AgentRegistry struct {
	Version   string            `json:"version"`
	CreatedAt string            `json:"created_at"`
	Agents    []RegisteredAgent `json:"agents"`
}

// RegisteredAgent represents a specialist agent from the registry
type RegisteredAgent struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	KnowledgePath string            `json:"knowledge_path"`
	KBSize        int               `json:"kb_size"`
	CreatedAt     string            `json:"created_at"`
	Status        string            `json:"status"`
	Tools         []string          `json:"tools"`
	ToolPrefs     map[string]string `json:"tool_preferences"`
}

// SpecialistMatch represents a matched specialist for pre-review orchestration
type SpecialistMatch struct {
	AgentName     string   // Name of the specialist agent
	KnowledgePath string   // Path to the knowledge DB
	Files         []string // Files this specialist should review
	Score         float64  // Match confidence (0.0-1.0)
	Reason        string   // Why this specialist was matched
}

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

// =============================================================================
// PRE-REVIEW SPECIALIST MATCHING
// =============================================================================
// Matches registered specialist agents to files BEFORE review for multi-shard orchestration.

// agentPatternMapping maps technology pattern ShardName to actual agent names
var agentPatternMapping = map[string]string{
	"rod":       "RodExpert",
	"golang":    "GoExpert",
	"react":     "ReactExpert",
	"mangle":    "MangleExpert",
	"sql":       "DatabaseExpert",
	"api":       "APIExpert",
	"testing":   "TestArchitect",
	"bubbletea": "BubbleTeaExpert",
	"cobra":     "CobraExpert",
	"security":  "SecurityAuditor",
}

// additionalPatterns extends knownTechnologies with more specific patterns
var additionalPatterns = []TechnologyPattern{
	{
		ShardName:    "bubbletea",
		FilePatterns: []string{"tui", "chat", "model.go", "view.go", "update.go"},
		ImportHints:  []string{"github.com/charmbracelet/bubbletea", "github.com/charmbracelet/lipgloss", "github.com/charmbracelet/bubbles"},
		ContentHints: []string{"tea.Model", "tea.Cmd", "tea.Msg", "lipgloss.Style", "tea.Batch"},
		Description:  "Bubbletea TUI development",
	},
	{
		ShardName:    "cobra",
		FilePatterns: []string{"cmd/", "cli", "root.go"},
		ImportHints:  []string{"github.com/spf13/cobra", "github.com/spf13/viper"},
		ContentHints: []string{"cobra.Command", "RunE:", "PersistentFlags", "AddCommand"},
		Description:  "Cobra CLI framework",
	},
	{
		ShardName:    "security",
		FilePatterns: []string{"auth", "security", "crypto", "password", "token"},
		ImportHints:  []string{"crypto/", "golang.org/x/crypto", "github.com/golang-jwt/jwt"},
		ContentHints: []string{"bcrypt", "jwt", "encrypt", "decrypt", "hash", "secret"},
		Description:  "Security-sensitive code",
	},
}

// MatchSpecialistsForReview analyzes files and matches them to registered specialists.
// This runs BEFORE the review to enable multi-shard parallel reviews.
// Returns a slice of SpecialistMatch sorted by score, deduplicated by agent.
func MatchSpecialistsForReview(ctx context.Context, files []string, registry *AgentRegistry) []SpecialistMatch {
	if registry == nil || len(registry.Agents) == 0 {
		return nil
	}

	// Build set of available agents for quick lookup
	availableAgents := make(map[string]RegisteredAgent)
	for _, agent := range registry.Agents {
		typ := strings.ToLower(strings.TrimSpace(agent.Type))
		status := strings.ToLower(strings.TrimSpace(agent.Status))
		// ShardTypeUser is an alias for persistent specialists in codeNERD.
		if (typ == "persistent" || typ == "user" || typ == "") && status == "ready" {
			availableAgents[strings.ToLower(agent.Name)] = agent
		}
	}

	// Combine standard and additional patterns
	allPatterns := append(knownTechnologies, additionalPatterns...)

	// Track matches per agent
	agentMatches := make(map[string]*SpecialistMatch)

	for _, file := range files {
		// Read file content for pattern matching
		content := ""
		if data, err := os.ReadFile(file); err == nil {
			content = string(data)
		}

		lowerFile := strings.ToLower(file)
		lowerContent := strings.ToLower(content)
		ext := filepath.Ext(file)

		for _, tech := range allPatterns {
			score := 0.0
			matches := false

			// Check file extension specifically for .mg files -> Mangle
			if ext == ".mg" && tech.ShardName == "mangle" {
				score += 0.5
				matches = true
			}

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

			if !matches || score < 0.3 {
				continue
			}

			// Map pattern to actual agent name
			agentName := agentPatternMapping[tech.ShardName]
			if agentName == "" {
				// Capitalize first letter manually to avoid deprecated strings.Title
				name := tech.ShardName
				if len(name) > 0 {
					agentName = strings.ToUpper(name[:1]) + name[1:] + "Expert"
				}
			}

			// Check if this agent is available
			agentLower := strings.ToLower(agentName)
			agent, available := availableAgents[agentLower]
			if !available {
				continue
			}

			// Update or create match
			if existing, ok := agentMatches[agentLower]; ok {
				if score > existing.Score {
					existing.Score = score
				}
				// Add file if not already present
				fileExists := false
				for _, f := range existing.Files {
					if f == file {
						fileExists = true
						break
					}
				}
				if !fileExists {
					existing.Files = append(existing.Files, file)
				}
			} else {
				agentMatches[agentLower] = &SpecialistMatch{
					AgentName:     agent.Name,
					KnowledgePath: agent.KnowledgePath,
					Files:         []string{file},
					Score:         score,
					Reason:        tech.Description,
				}
			}
		}
	}

	// Convert to slice and sort by score
	var matches []SpecialistMatch
	for _, m := range agentMatches {
		if m.Score > 1.0 {
			m.Score = 1.0
		}
		matches = append(matches, *m)
	}

	// Sort by score descending (bubble sort for simplicity)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	return matches
}

// GetAllPatterns returns all technology patterns (standard + additional)
func GetAllPatterns() []TechnologyPattern {
	return append(knownTechnologies, additionalPatterns...)
}
