// Package shards provides shared specialist matching for all shard types.
// This file contains the generalized specialist matching system that extends
// beyond just /review to support /fix, /refactor, /create, /test, and other verbs.
package shards

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// SHARED TYPES FOR SPECIALIST MATCHING
// =============================================================================

// AgentRegistry holds registered specialist agents from .nerd/agents.json
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

// SpecialistMatch represents a matched specialist for task orchestration
type SpecialistMatch struct {
	AgentName      string                    // Name of the specialist agent
	KnowledgePath  string                    // Path to the knowledge DB
	Files          []string                  // Files this specialist should work on
	Score          float64                   // Match confidence (0.0-1.0)
	Reason         string                    // Why this specialist was matched
	Classification *SpecialistClassification // Classification info (executor/advisor/observer)
	ShouldExecute  bool                      // Whether this specialist should execute directly
}

// TechnologyPattern maps file patterns and imports to specialist agents
type TechnologyPattern struct {
	ShardName    string   // Technology/specialist identifier
	FilePatterns []string // File path patterns (glob-like)
	ImportHints  []string // Import statements to look for
	ContentHints []string // Code patterns to detect
	Description  string   // Human-readable description
}

// =============================================================================
// TECHNOLOGY PATTERNS
// =============================================================================

// CoreTechnologyPatterns defines the mapping from code patterns to specialists
var CoreTechnologyPatterns = []TechnologyPattern{
	{
		ShardName:    "rod",
		FilePatterns: []string{"browser", "scraper", "crawler", "selenium", "playwright"},
		ImportHints:  []string{"github.com/go-rod/rod", "chromedp", "selenium", "playwright"},
		ContentHints: []string{"Browser()", "MustPage", "MustElement", "CDP", "DevTools"},
		Description:  "Browser automation with Rod/CDP",
	},
	{
		ShardName:    "golang",
		FilePatterns: []string{".go"},
		ImportHints:  []string{},
		ContentHints: []string{"func ", "type ", "interface ", "struct "},
		Description:  "Go language patterns and idioms",
	},
	{
		ShardName:    "react",
		FilePatterns: []string{".tsx", ".jsx", "component"},
		ImportHints:  []string{"react", "useState", "useEffect", "next/"},
		ContentHints: []string{"<", "/>", "useState", "useEffect", "className"},
		Description:  "React/Next.js frontend development",
	},
	{
		ShardName:    "mangle",
		FilePatterns: []string{".mg", "mangle", "policy", "schema"},
		ImportHints:  []string{},
		ContentHints: []string{"Decl ", ":-", "fn:", "let "},
		Description:  "Mangle/Datalog logic programming",
	},
	{
		ShardName:    "sql",
		FilePatterns: []string{"database", "store", "repository", "dao", ".sql"},
		ImportHints:  []string{"database/sql", "sqlx", "gorm", "pgx", "sqlite"},
		ContentHints: []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE"},
		Description:  "SQL database operations",
	},
	{
		ShardName:    "api",
		FilePatterns: []string{"handler", "controller", "endpoint", "route", "api"},
		ImportHints:  []string{"net/http", "gin", "echo", "fiber", "chi"},
		ContentHints: []string{"http.Handler", "c.JSON", "r.GET", "r.POST"},
		Description:  "REST API development",
	},
	{
		ShardName:    "testing",
		FilePatterns: []string{"_test.go", "test_", "spec."},
		ImportHints:  []string{"testing", "testify", "gomock", "jest", "pytest"},
		ContentHints: []string{"func Test", "t.Run", "assert.", "expect("},
		Description:  "Test writing and coverage",
	},
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
	{
		ShardName:    "concurrency",
		FilePatterns: []string{"worker", "pool", "queue", "async", "parallel"},
		ImportHints:  []string{"sync", "golang.org/x/sync"},
		ContentHints: []string{"go func", "chan ", "sync.Mutex", "sync.WaitGroup", "context."},
		Description:  "Concurrency patterns and goroutines",
	},
	{
		ShardName:    "grpc",
		FilePatterns: []string{".proto", "grpc", "pb.go"},
		ImportHints:  []string{"google.golang.org/grpc", "google.golang.org/protobuf"},
		ContentHints: []string{"protobuf", "grpc.Server", "pb.", "proto."},
		Description:  "gRPC service development",
	},
}

// AgentPatternMapping maps technology pattern ShardName to actual agent names
var AgentPatternMapping = map[string]string{
	"rod":         "RodExpert",
	"golang":      "GoExpert",
	"react":       "ReactExpert",
	"mangle":      "MangleExpert",
	"sql":         "DatabaseExpert",
	"api":         "APIExpert",
	"testing":     "TestArchitect",
	"bubbletea":   "BubbleTeaExpert",
	"cobra":       "CobraExpert",
	"security":    "SecurityAuditor",
	"concurrency": "ConcurrencyExpert",
	"grpc":        "GRPCExpert",
}

// =============================================================================
// SPECIALIST CLASSIFICATION SYSTEM
// =============================================================================

// SpecialistExecutionMode defines the execution capability of a specialist
type SpecialistExecutionMode string

const (
	// SpecialistModeExecutor - Can directly write/modify code in their domain
	SpecialistModeExecutor SpecialistExecutionMode = "/executor"

	// SpecialistModeAdvisor - Can only advise, not execute
	SpecialistModeAdvisor SpecialistExecutionMode = "/advisor"

	// SpecialistModeObserver - Background monitoring only
	SpecialistModeObserver SpecialistExecutionMode = "/observer"
)

// SpecialistKnowledgeTier defines the type of knowledge a specialist has
type SpecialistKnowledgeTier string

const (
	// TierTechnical - Implementation expertise (how to code)
	TierTechnical SpecialistKnowledgeTier = "/technical"

	// TierStrategic - Architectural/philosophical guidance (what to code)
	TierStrategic SpecialistKnowledgeTier = "/strategic"

	// TierDomain - Project-specific knowledge (why we code this way)
	TierDomain SpecialistKnowledgeTier = "/domain"
)

// SpecialistClassification holds the classification of a specialist
type SpecialistClassification struct {
	ExecutionMode       SpecialistExecutionMode
	KnowledgeTier       SpecialistKnowledgeTier
	CanExecute          bool
	CanAdvise           bool
	CanObserve          bool
	BackgroundCapable   bool
	CampaignIntegration string // /phase_executor, /plan_reviewer, /background_monitor
}

// DefaultSpecialistClassifications maps specialist names to their classifications
// These are parsed from the classification atoms in .nerd/agents/*/prompts.yaml
var DefaultSpecialistClassifications = map[string]SpecialistClassification{
	"goexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"bubbleteaexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"cobraexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"rodexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"mangleexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"securityauditor": {
		ExecutionMode:       SpecialistModeAdvisor,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/plan_reviewer",
	},
	"testarchitect": {
		ExecutionMode:       SpecialistModeAdvisor,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/plan_reviewer",
	},
	"northstar": {
		ExecutionMode:       SpecialistModeObserver,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/alignment_guardian",
	},
}

// GetSpecialistClassification returns the classification for a specialist
func GetSpecialistClassification(name string) (SpecialistClassification, bool) {
	// Normalize name to lowercase
	lowerName := strings.ToLower(strings.TrimSpace(name))
	class, ok := DefaultSpecialistClassifications[lowerName]
	return class, ok
}

// CanSpecialistExecute returns whether a specialist can execute tasks directly
func CanSpecialistExecute(name string) bool {
	class, ok := GetSpecialistClassification(name)
	if !ok {
		return false // Unknown specialists default to advisory
	}
	return class.CanExecute
}

// IsExecutorSpecialist returns whether a specialist is an executor type
func IsExecutorSpecialist(name string) bool {
	class, ok := GetSpecialistClassification(name)
	if !ok {
		return false
	}
	return class.ExecutionMode == SpecialistModeExecutor
}

// IsStrategicAdvisor returns whether a specialist is a strategic advisor
func IsStrategicAdvisor(name string) bool {
	class, ok := GetSpecialistClassification(name)
	if !ok {
		return false
	}
	return class.KnowledgeTier == TierStrategic
}

// ShouldSpecialistExecuteTask determines if a specialist should execute directly
// based on task confidence and specialist classification
func ShouldSpecialistExecuteTask(name string, confidence float64) bool {
	class, ok := GetSpecialistClassification(name)
	if !ok || !class.CanExecute {
		return false
	}
	// High confidence (>0.8) means specialist should execute directly
	// Lower confidence means specialist should advise instead
	return confidence > 0.8
}

// =============================================================================
// VERB-AWARE SPECIALIST MATCHING
// =============================================================================

// ExecutionMode defines how specialists participate in task execution
type ExecutionMode int

const (
	// ModeParallel - All shards execute the same task in parallel (e.g., /review)
	// Good for: reviews, analysis, security scans where multiple perspectives help
	ModeParallel ExecutionMode = iota

	// ModeAdvisory - Specialists advise, then generic shard executes (e.g., /fix)
	// Phase 1: Specialists provide domain-specific advice
	// Phase 2: Generic shard executes with advice as context
	ModeAdvisory

	// ModeAdvisoryWithCritique - Full three-phase pattern for mutations
	// Phase 1: Specialists provide advice BEFORE execution
	// Phase 2: Generic shard executes the task
	// Phase 3: Specialists critique the result AFTER execution
	ModeAdvisoryWithCritique

	// ModeSpecialistDirect - Executor specialist handles task directly
	// Used when a high-confidence executor specialist is matched
	ModeSpecialistDirect
)

// VerbSpecialistConfig defines which specialists are relevant for each verb
type VerbSpecialistConfig struct {
	MinConfidence   float64       // Minimum score to include a specialist
	PreferPatterns  []string      // Prefer these technology patterns for this verb
	ExcludePatterns []string      // Exclude these patterns for this verb
	MaxSpecialists  int           // Maximum number of specialists to match
	IncludeGeneric  bool          // Whether to include the generic shard too
	Mode            ExecutionMode // How specialists participate (parallel, advisory, etc.)
}

// DefaultVerbConfigs defines specialist matching behavior per verb
var DefaultVerbConfigs = map[string]VerbSpecialistConfig{
	"/review": {
		MinConfidence:  0.3,
		MaxSpecialists: 3,
		IncludeGeneric: true,
		Mode:           ModeParallel, // Multiple reviewers in parallel
	},
	"/fix": {
		MinConfidence:   0.4,
		MaxSpecialists:  2,
		IncludeGeneric:  true,
		PreferPatterns:  []string{"golang", "react", "sql", "security"},
		ExcludePatterns: []string{"testing"},
		Mode:            ModeAdvisoryWithCritique, // Advise → Fix → Critique
	},
	"/refactor": {
		MinConfidence:  0.35,
		MaxSpecialists: 2,
		IncludeGeneric: true,
		PreferPatterns: []string{"golang", "react", "concurrency", "api"},
		Mode:           ModeAdvisoryWithCritique, // Advise → Refactor → Critique
	},
	"/create": {
		MinConfidence:  0.35,
		MaxSpecialists: 2,
		IncludeGeneric: true,
		Mode:           ModeAdvisory, // Advise → Create (no critique needed for new code)
	},
	"/test": {
		MinConfidence:  0.4,
		MaxSpecialists: 2,
		IncludeGeneric: true,
		PreferPatterns: []string{"testing", "golang", "react"},
		Mode:           ModeParallel, // Multiple test perspectives help
	},
	"/debug": {
		MinConfidence:  0.4,
		MaxSpecialists: 2,
		IncludeGeneric: true,
		PreferPatterns: []string{"concurrency", "security", "api"},
		Mode:           ModeAdvisory, // Get advice, then debug
	},
	"/security": {
		MinConfidence:  0.3,
		MaxSpecialists: 3,
		IncludeGeneric: true,
		PreferPatterns: []string{"security", "api", "sql"},
		Mode:           ModeParallel, // Multiple security reviewers
	},
}

// GetExecutionMode returns the execution mode for a verb
func GetExecutionMode(verb string) ExecutionMode {
	config, ok := DefaultVerbConfigs[verb]
	if !ok {
		return ModeParallel // Default to parallel
	}
	return config.Mode
}

// MatchSpecialistsForTask matches registered specialists to files for any verb.
// This is the generalized version that supports all verbs, not just /review.
// TODO: Implement semantic matching logic for capability discovery if basic pattern matching is insufficient.
func MatchSpecialistsForTask(ctx context.Context, verb string, files []string, registry *AgentRegistry) []SpecialistMatch {
	if registry == nil || len(registry.Agents) == 0 {
		return nil
	}

	// Get verb-specific config or use review defaults
	config, ok := DefaultVerbConfigs[verb]
	if !ok {
		config = DefaultVerbConfigs["/review"]
	}

	// Build set of available agents for quick lookup
	availableAgents := make(map[string]RegisteredAgent)
	for _, agent := range registry.Agents {
		typ := strings.ToLower(strings.TrimSpace(agent.Type))
		status := strings.ToLower(strings.TrimSpace(agent.Status))
		// ShardTypeUser is an alias for persistent specialists
		if (typ == "persistent" || typ == "user" || typ == "") && status == "ready" {
			availableAgents[strings.ToLower(agent.Name)] = agent
		}
	}

	// Build exclude set
	excludeSet := make(map[string]bool)
	for _, p := range config.ExcludePatterns {
		excludeSet[p] = true
	}

	// Build prefer set with bonus scoring
	preferSet := make(map[string]bool)
	for _, p := range config.PreferPatterns {
		preferSet[p] = true
	}

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

		for _, tech := range CoreTechnologyPatterns {
			// Skip excluded patterns
			if excludeSet[tech.ShardName] {
				continue
			}

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

			// Apply preference bonus
			if preferSet[tech.ShardName] {
				score *= 1.2 // 20% boost for preferred patterns
			}

			if !matches || score < config.MinConfidence {
				continue
			}

			// Map pattern to actual agent name
			agentName := AgentPatternMapping[tech.ShardName]
			if agentName == "" {
				// Capitalize first letter
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
					// Update ShouldExecute based on new score
					existing.ShouldExecute = ShouldSpecialistExecuteTask(agentLower, score)
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
				// Get classification for this specialist
				var classification *SpecialistClassification
				if class, ok := GetSpecialistClassification(agentLower); ok {
					classification = &class
				}

				agentMatches[agentLower] = &SpecialistMatch{
					AgentName:      agent.Name,
					KnowledgePath:  agent.KnowledgePath,
					Files:          []string{file},
					Score:          score,
					Reason:         tech.Description,
					Classification: classification,
					ShouldExecute:  ShouldSpecialistExecuteTask(agentLower, score),
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

	// Limit to max specialists
	if config.MaxSpecialists > 0 && len(matches) > config.MaxSpecialists {
		matches = matches[:config.MaxSpecialists]
	}

	return matches
}

// ShouldIncludeGenericShard returns whether the generic shard should also run
// alongside specialists for the given verb.
func ShouldIncludeGenericShard(verb string) bool {
	config, ok := DefaultVerbConfigs[verb]
	if !ok {
		return true // Default to including generic shard
	}
	return config.IncludeGeneric
}

// GetAllPatterns returns all technology patterns for external use
func GetAllPatterns() []TechnologyPattern {
	return CoreTechnologyPatterns
}
