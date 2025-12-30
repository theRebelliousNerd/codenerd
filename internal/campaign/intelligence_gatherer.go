// Package campaign provides multi-phase goal orchestration.
// This file implements intelligence gathering from all available systems
// to provide comprehensive context before campaign planning.
package campaign

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mcp"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/world"

	"golang.org/x/sync/errgroup"
)

// =============================================================================
// CONSULTATION TYPES (local copies to avoid import cycle with shards package)
// =============================================================================

// ConsultationProvider abstracts shard consultation capabilities.
// Implemented by shards.ConsultationManager.
type ConsultationProvider interface {
	RequestBatchConsultation(ctx context.Context, request BatchConsultRequest) ([]ConsultationResponse, error)
}

// BatchConsultRequest represents a request for batch consultation.
type BatchConsultRequest struct {
	Topic      string
	Question   string
	Context    string
	TargetSpec []string
}

// ConsultationResponse represents advice from a shard.
type ConsultationResponse struct {
	RequestID    string
	FromSpec     string
	ToSpec       string
	Advice       string
	Confidence   float64
	References   []string
	Caveats      []string
	Metadata     map[string]string
	ResponseTime time.Time
	Duration     time.Duration
}

// =============================================================================
// INTELLIGENCE GATHERER
// =============================================================================
// Orchestrates pre-planning intelligence gathering from all 12 dormant systems.
// This is the foundation of the "golden jewel" campaign planning system.

// IntelligenceGatherer coordinates intelligence collection from multiple sources.
type IntelligenceGatherer struct {
	// Core dependencies
	kernel *core.RealKernel

	// World Model (codebase awareness)
	worldScanner *world.Scanner
	holographic  *world.HolographicProvider

	// Memory tiers
	learningStore *store.LearningStore
	localStore    *store.LocalStore

	// Self-modification systems
	toolGenerator *autopoiesis.ToolGenerator

	// MCP tools
	mcpStore *mcp.MCPToolStore

	// Shard consultation
	consultation ConsultationProvider

	// Configuration
	config IntelligenceConfig
}

// IntelligenceConfig configures the intelligence gathering process.
type IntelligenceConfig struct {
	// Timeouts
	GatherTimeout     time.Duration
	PerSystemTimeout  time.Duration
	ConsultTimeout    time.Duration

	// Limits
	MaxChurnHotspots  int
	MaxLearnings      int
	MaxMCPTools       int
	MaxPreviousCampaigns int
	GitHistoryDepth   int

	// Feature flags
	EnableWorldModel      bool
	EnableGitHistory      bool
	EnableLearningStore   bool
	EnableKnowledgeGraph  bool
	EnableColdStorage     bool
	EnableSafetyCheck     bool
	EnableAutopoiesis     bool
	EnableMCPTools        bool
	EnablePreviousCampaigns bool
	EnableShardConsult    bool
	EnableTestCoverage    bool
	EnableCodePatterns    bool
}

// DefaultIntelligenceConfig returns sensible defaults.
func DefaultIntelligenceConfig() IntelligenceConfig {
	return IntelligenceConfig{
		GatherTimeout:        5 * time.Minute,
		PerSystemTimeout:     30 * time.Second,
		ConsultTimeout:       2 * time.Minute,
		MaxChurnHotspots:     50,
		MaxLearnings:         100,
		MaxMCPTools:          30,
		MaxPreviousCampaigns: 10,
		GitHistoryDepth:      100,
		EnableWorldModel:     true,
		EnableGitHistory:     true,
		EnableLearningStore:  true,
		EnableKnowledgeGraph: true,
		EnableColdStorage:    true,
		EnableSafetyCheck:    true,
		EnableAutopoiesis:    true,
		EnableMCPTools:       true,
		EnablePreviousCampaigns: true,
		EnableShardConsult:   true,
		EnableTestCoverage:   true,
		EnableCodePatterns:   true,
	}
}

// IntelligenceReport contains all gathered intelligence for campaign planning.
type IntelligenceReport struct {
	// Timestamp
	GatheredAt time.Time `json:"gathered_at"`
	Duration   time.Duration `json:"duration"`

	// World Model: Codebase structure
	WorldFacts     []core.Fact       `json:"world_facts"`
	FileTopology   map[string]FileInfo `json:"file_topology"`
	SymbolGraph    []SymbolInfo      `json:"symbol_graph"`
	LanguageBreakdown map[string]int `json:"language_breakdown"`

	// Git History: Churn analysis (Chesterton's Fence)
	GitChurnHotspots []ChurnHotspot   `json:"git_churn_hotspots"`
	RecentCommits    []CommitInfo     `json:"recent_commits"`
	HighChurnFiles   []string         `json:"high_churn_files"`

	// Learning Store: Historical patterns
	HistoricalPatterns []LearningPattern `json:"historical_patterns"`
	PreferenceSignals  []PreferenceSignal `json:"preference_signals"`

	// Knowledge Graph: Entity relationships
	KnowledgeLinks []store.KnowledgeLink `json:"knowledge_links"`
	EntityClusters []EntityCluster       `json:"entity_clusters"`

	// Cold Storage: Long-term context
	ColdStorageFacts []store.StoredFact `json:"cold_storage_facts"`

	// Safety: Constitutional pre-check
	SafetyWarnings []SafetyWarning `json:"safety_warnings"`
	BlockedActions []string        `json:"blocked_actions"`

	// Autopoiesis: Tool gaps
	ToolGaps         []autopoiesis.ToolNeed `json:"tool_gaps"`
	MissingCapabilities []string            `json:"missing_capabilities"`

	// MCP: Available external tools
	MCPToolsAvailable []MCPToolInfo `json:"mcp_tools_available"`
	MCPServerStatus   map[string]string `json:"mcp_server_status"`

	// Previous Campaigns: Reusable artifacts
	PreviousCampaigns []CampaignArtifact `json:"previous_campaigns"`
	ReusablePatterns  []string           `json:"reusable_patterns"`

	// Shard Consultation: Expert advice
	ShardAdvice    []ConsultationResponse `json:"shard_advice"`
	AdvisorySummary string                       `json:"advisory_summary"`

	// Test Coverage: Current state
	TestCoverage     map[string]float64 `json:"test_coverage"`
	UncoveredPaths   []string           `json:"uncovered_paths"`

	// Code Patterns: Detected patterns
	CodePatterns     []CodePattern `json:"code_patterns"`
	ArchitectureHints []string     `json:"architecture_hints"`

	// Errors during gathering (non-fatal)
	GatheringErrors []string `json:"gathering_errors"`
}

// Supporting types for IntelligenceReport

// FileInfo represents file topology information.
type FileInfo struct {
	Path        string    `json:"path"`
	Hash        string    `json:"hash"`
	Language    string    `json:"language"`
	LineCount   int       `json:"line_count"`
	IsTestFile  bool      `json:"is_test_file"`
	LastModified time.Time `json:"last_modified"`
}

// SymbolInfo represents a code symbol.
type SymbolInfo struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`     // func, type, const, var
	File     string `json:"file"`
	Line     int    `json:"line"`
	Exported bool   `json:"exported"`
}

// ChurnHotspot represents a file with high churn rate.
type ChurnHotspot struct {
	Path       string `json:"path"`
	ChurnRate  int    `json:"churn_rate"`
	LastChange time.Time `json:"last_change"`
	Reason     string `json:"reason"`
	Warning    string `json:"warning"` // Chesterton's Fence warning
}

// CommitInfo represents a recent git commit.
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
	Files   []string  `json:"files"`
}

// LearningPattern represents a learned pattern from previous sessions.
type LearningPattern struct {
	ShardType   string  `json:"shard_type"`
	Predicate   string  `json:"predicate"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	LastUsed    time.Time `json:"last_used"`
}

// PreferenceSignal represents a user preference.
type PreferenceSignal struct {
	Category string `json:"category"`
	Signal   string `json:"signal"`
	Strength float64 `json:"strength"`
}

// EntityCluster represents a group of related entities.
type EntityCluster struct {
	ClusterID string   `json:"cluster_id"`
	Entities  []string `json:"entities"`
	Relation  string   `json:"relation"`
}

// SafetyWarning represents a constitutional safety warning.
type SafetyWarning struct {
	CampaignID  string `json:"campaign_id"`
	Path        string `json:"path"`
	Action      string `json:"action"`
	RuleViolated string `json:"rule_violated"`
	Severity    string `json:"severity"`
	Remediation string `json:"remediation"`
}

// MCPToolInfo represents an available MCP tool.
type MCPToolInfo struct {
	ToolID      string   `json:"tool_id"`
	ServerID    string   `json:"server_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Affinity    float64  `json:"affinity"` // Relevance to current campaign
}

// CampaignArtifact represents a reusable artifact from previous campaigns.
type CampaignArtifact struct {
	CampaignID  string    `json:"campaign_id"`
	Goal        string    `json:"goal"`
	Phase       string    `json:"phase"`
	TaskCount   int       `json:"task_count"`
	SuccessRate float64   `json:"success_rate"`
	CreatedAt   time.Time `json:"created_at"`
	Patterns    []string  `json:"patterns"`
}

// CodePattern represents a detected code pattern.
type CodePattern struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`     // design, architecture, anti-pattern
	Files       []string `json:"files"`
	Confidence  float64  `json:"confidence"`
	Description string   `json:"description"`
}

// NewIntelligenceGatherer creates a new intelligence gatherer.
func NewIntelligenceGatherer(
	kernel *core.RealKernel,
	worldScanner *world.Scanner,
	holographic *world.HolographicProvider,
	learningStore *store.LearningStore,
	localStore *store.LocalStore,
	toolGenerator *autopoiesis.ToolGenerator,
	mcpStore *mcp.MCPToolStore,
	consultation ConsultationProvider,
) *IntelligenceGatherer {
	return &IntelligenceGatherer{
		kernel:        kernel,
		worldScanner:  worldScanner,
		holographic:   holographic,
		learningStore: learningStore,
		localStore:    localStore,
		toolGenerator: toolGenerator,
		mcpStore:      mcpStore,
		consultation:  consultation,
		config:        DefaultIntelligenceConfig(),
	}
}

// WithConfig sets the configuration for the gatherer.
func (g *IntelligenceGatherer) WithConfig(config IntelligenceConfig) *IntelligenceGatherer {
	g.config = config
	return g
}

// Gather collects intelligence from all available systems.
// This is the main entry point for pre-planning intelligence gathering.
func (g *IntelligenceGatherer) Gather(ctx context.Context, goal string, targetPaths []string) (*IntelligenceReport, error) {
	startTime := time.Now()
	logging.Campaign("Intelligence gathering started for goal: %.50s...", goal)

	// Apply overall timeout
	ctx, cancel := context.WithTimeout(ctx, g.config.GatherTimeout)
	defer cancel()

	report := &IntelligenceReport{
		GatheredAt:        startTime,
		FileTopology:      make(map[string]FileInfo),
		LanguageBreakdown: make(map[string]int),
		TestCoverage:      make(map[string]float64),
		MCPServerStatus:   make(map[string]string),
		GatheringErrors:   []string{},
	}

	// Use errgroup for parallel gathering with controlled concurrency
	var mu sync.Mutex
	addError := func(err string) {
		mu.Lock()
		report.GatheringErrors = append(report.GatheringErrors, err)
		mu.Unlock()
	}

	eg, egCtx := errgroup.WithContext(ctx)

	// 1. World Model (codebase structure)
	if g.config.EnableWorldModel && g.worldScanner != nil {
		eg.Go(func() error {
			g.gatherWorldModel(egCtx, report, targetPaths, addError)
			return nil
		})
	}

	// 2. Git History (Chesterton's Fence)
	if g.config.EnableGitHistory {
		eg.Go(func() error {
			g.gatherGitHistory(egCtx, report, targetPaths, addError)
			return nil
		})
	}

	// 3. Learning Store (historical patterns)
	if g.config.EnableLearningStore && g.learningStore != nil {
		eg.Go(func() error {
			g.gatherLearningPatterns(egCtx, report, addError)
			return nil
		})
	}

	// 4. Knowledge Graph (entity relationships)
	if g.config.EnableKnowledgeGraph && g.localStore != nil {
		eg.Go(func() error {
			g.gatherKnowledgeGraph(egCtx, report, targetPaths, addError)
			return nil
		})
	}

	// 5. Cold Storage (long-term context)
	if g.config.EnableColdStorage && g.localStore != nil {
		eg.Go(func() error {
			g.gatherColdStorage(egCtx, report, addError)
			return nil
		})
	}

	// 6. Safety Check (Constitutional Gate pre-check)
	if g.config.EnableSafetyCheck && g.kernel != nil {
		eg.Go(func() error {
			g.gatherSafetyWarnings(egCtx, report, goal, targetPaths, addError)
			return nil
		})
	}

	// 7. MCP Tools (external capabilities)
	if g.config.EnableMCPTools && g.mcpStore != nil {
		eg.Go(func() error {
			g.gatherMCPTools(egCtx, report, goal, addError)
			return nil
		})
	}

	// 8. Previous Campaigns (reusable artifacts)
	if g.config.EnablePreviousCampaigns {
		eg.Go(func() error {
			g.gatherPreviousCampaigns(egCtx, report, goal, addError)
			return nil
		})
	}

	// 9. Test Coverage
	if g.config.EnableTestCoverage && g.kernel != nil {
		eg.Go(func() error {
			g.gatherTestCoverage(egCtx, report, targetPaths, addError)
			return nil
		})
	}

	// 10. Code Patterns
	if g.config.EnableCodePatterns && g.kernel != nil {
		eg.Go(func() error {
			g.gatherCodePatterns(egCtx, report, targetPaths, addError)
			return nil
		})
	}

	// Wait for parallel gathering to complete
	if err := eg.Wait(); err != nil {
		logging.Campaign("Intelligence gathering had errors: %v", err)
	}

	// 11. Autopoiesis Tool Gaps (depends on world model)
	if g.config.EnableAutopoiesis && g.toolGenerator != nil {
		g.gatherToolGaps(ctx, report, goal, addError)
	}

	// 12. Shard Consultation (sequential, depends on gathered context)
	if g.config.EnableShardConsult && g.consultation != nil {
		g.gatherShardAdvice(ctx, report, goal, addError)
	}

	report.Duration = time.Since(startTime)
	logging.Campaign("Intelligence gathering completed: %d world facts, %d churn hotspots, %d learnings, %d errors (took %v)",
		len(report.WorldFacts), len(report.GitChurnHotspots), len(report.HistoricalPatterns),
		len(report.GatheringErrors), report.Duration)

	return report, nil
}

// =============================================================================
// INDIVIDUAL GATHERING METHODS
// =============================================================================

func (g *IntelligenceGatherer) gatherWorldModel(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherWorldModel")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Determine root path for scanning
	root := "."
	if len(paths) > 0 {
		root = paths[0] // Use first path as root
	}

	facts, err := g.worldScanner.ScanWorkspaceCtx(ctx, root)
	if err != nil {
		addError(fmt.Sprintf("World model scan failed: %v", err))
		return
	}

	report.WorldFacts = facts

	// Parse facts into structured data
	for _, fact := range facts {
		switch fact.Predicate {
		case "file_topology":
			if len(fact.Args) >= 5 {
				path, _ := fact.Args[0].(string)
				hash, _ := fact.Args[1].(string)
				lang := g.parseAtom(fact.Args[2])
				modTime, _ := fact.Args[3].(int64)
				isTest := g.parseAtom(fact.Args[4]) == "/true"

				report.FileTopology[path] = FileInfo{
					Path:         path,
					Hash:         hash,
					Language:     strings.TrimPrefix(lang, "/"),
					IsTestFile:   isTest,
					LastModified: time.Unix(modTime, 0),
				}
				report.LanguageBreakdown[strings.TrimPrefix(lang, "/")]++
			}
		case "symbol_graph", "code_defines":
			if len(fact.Args) >= 4 {
				symbol := SymbolInfo{
					File:     g.parseArg(fact.Args[0]),
					Name:     g.parseArg(fact.Args[1]),
					Kind:     g.parseArg(fact.Args[2]),
					Exported: g.parseArg(fact.Args[3]) == "exported",
				}
				if len(fact.Args) >= 5 {
					if line, ok := fact.Args[4].(int); ok {
						symbol.Line = line
					}
				}
				report.SymbolGraph = append(report.SymbolGraph, symbol)
			}
		}
	}

	logging.CampaignDebug("World model gathered: %d files, %d symbols", len(report.FileTopology), len(report.SymbolGraph))
}

func (g *IntelligenceGatherer) gatherGitHistory(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherGitHistory")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	root := "."
	if len(paths) > 0 {
		root = paths[0]
	}

	facts, err := world.ScanGitHistory(ctx, root, g.config.GitHistoryDepth)
	if err != nil {
		addError(fmt.Sprintf("Git history scan failed: %v", err))
		return
	}

	// Parse git facts
	churnMap := make(map[string]int)
	for _, fact := range facts {
		switch fact.Predicate {
		case "churn_rate":
			if len(fact.Args) >= 2 {
				path, _ := fact.Args[0].(string)
				rate, _ := fact.Args[1].(int)
				churnMap[path] = rate
			}
		case "git_history":
			if len(fact.Args) >= 5 {
				commit := CommitInfo{
					Files:   []string{g.parseArg(fact.Args[0])},
					Hash:    g.parseArg(fact.Args[1]),
					Author:  g.parseArg(fact.Args[2]),
					Message: g.parseArg(fact.Args[4]),
				}
				if ts, ok := fact.Args[3].(int64); ok {
					commit.Time = time.Unix(ts, 0)
				}
				report.RecentCommits = append(report.RecentCommits, commit)
			}
		}
	}

	// Convert churn map to hotspots with Chesterton's Fence warnings
	for path, rate := range churnMap {
		hotspot := ChurnHotspot{
			Path:      path,
			ChurnRate: rate,
		}
		if rate > 10 {
			hotspot.Reason = "High churn rate"
			hotspot.Warning = fmt.Sprintf("⚠️ CHESTERTON'S FENCE: This file has been modified %d times. Understand WHY before changing it.", rate)
			report.HighChurnFiles = append(report.HighChurnFiles, path)
		} else if rate > 5 {
			hotspot.Reason = "Moderate churn rate"
			hotspot.Warning = "Consider reviewing recent changes before modification."
		}
		report.GitChurnHotspots = append(report.GitChurnHotspots, hotspot)
	}

	// Sort by churn rate descending
	sort.Slice(report.GitChurnHotspots, func(i, j int) bool {
		return report.GitChurnHotspots[i].ChurnRate > report.GitChurnHotspots[j].ChurnRate
	})

	// Limit to configured max
	if len(report.GitChurnHotspots) > g.config.MaxChurnHotspots {
		report.GitChurnHotspots = report.GitChurnHotspots[:g.config.MaxChurnHotspots]
	}

	logging.CampaignDebug("Git history gathered: %d churn hotspots, %d high-churn files",
		len(report.GitChurnHotspots), len(report.HighChurnFiles))
}

func (g *IntelligenceGatherer) gatherLearningPatterns(ctx context.Context, report *IntelligenceReport, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherLearningPatterns")
	defer timer.Stop()

	shardTypes := []string{"coder", "tester", "reviewer", "researcher"}

	for _, shardType := range shardTypes {
		learnings, err := g.learningStore.Load(shardType)
		if err != nil {
			addError(fmt.Sprintf("Learning store load failed for %s: %v", shardType, err))
			continue
		}

		for _, learning := range learnings {
			pattern := LearningPattern{
				ShardType:   shardType,
				Predicate:   learning.FactPredicate,
				Confidence:  learning.Confidence,
				LastUsed:    time.Unix(learning.Timestamp, 0),
				Description: g.formatLearningDescription(learning),
			}
			report.HistoricalPatterns = append(report.HistoricalPatterns, pattern)
		}
	}

	// Sort by confidence descending
	sort.Slice(report.HistoricalPatterns, func(i, j int) bool {
		return report.HistoricalPatterns[i].Confidence > report.HistoricalPatterns[j].Confidence
	})

	// Limit
	if len(report.HistoricalPatterns) > g.config.MaxLearnings {
		report.HistoricalPatterns = report.HistoricalPatterns[:g.config.MaxLearnings]
	}

	logging.CampaignDebug("Learning patterns gathered: %d patterns", len(report.HistoricalPatterns))
}

func (g *IntelligenceGatherer) gatherKnowledgeGraph(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherKnowledgeGraph")
	defer timer.Stop()

	// Query for entities related to target paths
	for _, path := range paths {
		links, err := g.localStore.QueryLinks(path, "both")
		if err != nil {
			addError(fmt.Sprintf("Knowledge graph query failed for %s: %v", path, err))
			continue
		}
		report.KnowledgeLinks = append(report.KnowledgeLinks, links...)
	}

	// Cluster entities by relation
	clusterMap := make(map[string][]string)
	for _, link := range report.KnowledgeLinks {
		key := link.Relation
		clusterMap[key] = append(clusterMap[key], link.EntityB)
	}

	for relation, entities := range clusterMap {
		report.EntityClusters = append(report.EntityClusters, EntityCluster{
			ClusterID: relation,
			Entities:  entities,
			Relation:  relation,
		})
	}

	logging.CampaignDebug("Knowledge graph gathered: %d links, %d clusters",
		len(report.KnowledgeLinks), len(report.EntityClusters))
}

func (g *IntelligenceGatherer) gatherColdStorage(ctx context.Context, report *IntelligenceReport, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherColdStorage")
	defer timer.Stop()

	// Load campaign-relevant predicates from cold storage
	predicates := []string{
		"preference_signal",
		"style_preference",
		"avoid_pattern",
		"learned_pattern",
	}

	for _, pred := range predicates {
		facts, err := g.localStore.LoadFacts(pred)
		if err != nil {
			addError(fmt.Sprintf("Cold storage load failed for %s: %v", pred, err))
			continue
		}
		report.ColdStorageFacts = append(report.ColdStorageFacts, facts...)
	}

	logging.CampaignDebug("Cold storage gathered: %d facts", len(report.ColdStorageFacts))
}

func (g *IntelligenceGatherer) gatherSafetyWarnings(ctx context.Context, report *IntelligenceReport, goal string, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherSafetyWarnings")
	defer timer.Stop()

	// Query kernel for any pre-existing safety constraints
	facts, err := g.kernel.Query("blocked_action")
	if err != nil {
		logging.CampaignDebug("No blocked_action facts: %v", err)
		// Not an error, just means no pre-existing blocks
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			report.BlockedActions = append(report.BlockedActions, g.parseArg(fact.Args[0]))
		}
	}

	// Query for safety_warning predicate
	safetyFacts, err := g.kernel.Query("safety_warning")
	if err == nil {
		for _, fact := range safetyFacts {
			if len(fact.Args) >= 4 {
				warning := SafetyWarning{
					Path:         g.parseArg(fact.Args[0]),
					Action:       g.parseArg(fact.Args[1]),
					RuleViolated: g.parseArg(fact.Args[2]),
					Severity:     g.parseArg(fact.Args[3]),
				}
				report.SafetyWarnings = append(report.SafetyWarnings, warning)
			}
		}
	}

	// Check for dangerous patterns in goal
	dangerousPatterns := []string{"rm -rf", "drop database", "delete *", "format c:"}
	goalLower := strings.ToLower(goal)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(goalLower, pattern) {
			report.SafetyWarnings = append(report.SafetyWarnings, SafetyWarning{
				Action:       pattern,
				RuleViolated: "dangerous_pattern",
				Severity:     "critical",
				Remediation:  "Review and confirm this action is intentional",
			})
		}
	}

	logging.CampaignDebug("Safety check: %d warnings, %d blocked actions",
		len(report.SafetyWarnings), len(report.BlockedActions))
}

func (g *IntelligenceGatherer) gatherMCPTools(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherMCPTools")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Get all available MCP tools
	tools, err := g.mcpStore.GetAllTools(ctx)
	if err != nil {
		addError(fmt.Sprintf("MCP tool fetch failed: %v", err))
		return
	}

	for _, tool := range tools {
		if tool == nil {
			continue
		}
		toolInfo := MCPToolInfo{
			ToolID:      tool.ToolID,
			ServerID:    tool.ServerID,
			Name:        tool.Name,
			Description: tool.Description,
			Categories:  tool.Categories,
			Affinity:    g.calculateToolAffinity(tool, goal),
		}
		report.MCPToolsAvailable = append(report.MCPToolsAvailable, toolInfo)
	}

	// Sort by affinity
	sort.Slice(report.MCPToolsAvailable, func(i, j int) bool {
		return report.MCPToolsAvailable[i].Affinity > report.MCPToolsAvailable[j].Affinity
	})

	// Limit
	if len(report.MCPToolsAvailable) > g.config.MaxMCPTools {
		report.MCPToolsAvailable = report.MCPToolsAvailable[:g.config.MaxMCPTools]
	}

	// Get server status
	servers, err := g.mcpStore.GetAllServers(ctx)
	if err == nil {
		for _, server := range servers {
			report.MCPServerStatus[server.ID] = string(server.Status)
		}
	}

	logging.CampaignDebug("MCP tools gathered: %d tools, %d servers",
		len(report.MCPToolsAvailable), len(report.MCPServerStatus))
}

func (g *IntelligenceGatherer) gatherPreviousCampaigns(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherPreviousCampaigns")
	defer timer.Stop()

	// Query kernel for campaign artifacts
	facts, err := g.kernel.Query("campaign_completed")
	if err != nil {
		logging.CampaignDebug("No previous campaigns: %v", err)
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 4 {
			artifact := CampaignArtifact{
				CampaignID:  g.parseArg(fact.Args[0]),
				Goal:        g.parseArg(fact.Args[1]),
				TaskCount:   g.parseIntArg(fact.Args[2]),
				SuccessRate: g.parseFloatArg(fact.Args[3]),
			}
			report.PreviousCampaigns = append(report.PreviousCampaigns, artifact)
		}
	}

	// Limit to most recent
	if len(report.PreviousCampaigns) > g.config.MaxPreviousCampaigns {
		report.PreviousCampaigns = report.PreviousCampaigns[:g.config.MaxPreviousCampaigns]
	}

	logging.CampaignDebug("Previous campaigns gathered: %d campaigns", len(report.PreviousCampaigns))
}

func (g *IntelligenceGatherer) gatherTestCoverage(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherTestCoverage")
	defer timer.Stop()

	// Query kernel for test_coverage facts
	facts, err := g.kernel.Query("test_coverage")
	if err != nil {
		logging.CampaignDebug("No test coverage facts: %v", err)
		return
	}

	for _, fact := range facts {
		if len(fact.Args) >= 2 {
			path := g.parseArg(fact.Args[0])
			coverage := g.parseFloatArg(fact.Args[1])
			report.TestCoverage[path] = coverage
			if coverage < 0.5 {
				report.UncoveredPaths = append(report.UncoveredPaths, path)
			}
		}
	}

	// Also check for test files corresponding to target paths
	for _, path := range paths {
		testPath := strings.TrimSuffix(path, ".go") + "_test.go"
		if _, ok := report.FileTopology[testPath]; !ok {
			report.UncoveredPaths = append(report.UncoveredPaths, path+" (no test file)")
		}
	}

	logging.CampaignDebug("Test coverage gathered: %d entries, %d uncovered",
		len(report.TestCoverage), len(report.UncoveredPaths))
}

func (g *IntelligenceGatherer) gatherCodePatterns(ctx context.Context, report *IntelligenceReport, paths []string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherCodePatterns")
	defer timer.Stop()

	// Query kernel for detected patterns
	patternPredicates := []string{"design_pattern", "anti_pattern", "architecture_pattern"}
	for _, pred := range patternPredicates {
		facts, err := g.kernel.Query(pred)
		if err != nil {
			continue
		}
		for _, fact := range facts {
			if len(fact.Args) >= 3 {
				pattern := CodePattern{
					Name:        g.parseArg(fact.Args[0]),
					Type:        pred,
					Description: g.parseArg(fact.Args[2]),
				}
				if len(fact.Args) >= 4 {
					pattern.Confidence = g.parseFloatArg(fact.Args[3])
				}
				report.CodePatterns = append(report.CodePatterns, pattern)
			}
		}
	}

	// Detect architecture hints from file structure
	if len(report.FileTopology) > 0 {
		hints := g.detectArchitectureHints(report.FileTopology)
		report.ArchitectureHints = hints
	}

	logging.CampaignDebug("Code patterns gathered: %d patterns, %d architecture hints",
		len(report.CodePatterns), len(report.ArchitectureHints))
}

func (g *IntelligenceGatherer) gatherToolGaps(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherToolGaps")
	defer timer.Stop()

	if g.toolGenerator == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, g.config.PerSystemTimeout)
	defer cancel()

	// Detect tool needs based on goal
	need, err := g.toolGenerator.DetectToolNeed(ctx, goal, "")
	if err != nil {
		addError(fmt.Sprintf("Tool gap detection failed: %v", err))
		return
	}

	if need != nil {
		report.ToolGaps = append(report.ToolGaps, *need)
		report.MissingCapabilities = append(report.MissingCapabilities, need.Purpose)
	}

	logging.CampaignDebug("Tool gaps gathered: %d gaps", len(report.ToolGaps))
}

func (g *IntelligenceGatherer) gatherShardAdvice(ctx context.Context, report *IntelligenceReport, goal string, addError func(string)) {
	timer := logging.StartTimer(logging.CategoryCampaign, "gatherShardAdvice")
	defer timer.Stop()

	if g.consultation == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, g.config.ConsultTimeout)
	defer cancel()

	// Build context from already-gathered intelligence
	contextBuilder := strings.Builder{}
	contextBuilder.WriteString(fmt.Sprintf("Campaign Goal: %s\n\n", goal))

	if len(report.HighChurnFiles) > 0 {
		contextBuilder.WriteString("High Churn Files:\n")
		for _, f := range report.HighChurnFiles[:min(5, len(report.HighChurnFiles))] {
			contextBuilder.WriteString(fmt.Sprintf("- %s\n", f))
		}
		contextBuilder.WriteString("\n")
	}

	if len(report.SafetyWarnings) > 0 {
		contextBuilder.WriteString("Safety Warnings:\n")
		for _, w := range report.SafetyWarnings[:min(3, len(report.SafetyWarnings))] {
			contextBuilder.WriteString(fmt.Sprintf("- %s: %s\n", w.Action, w.RuleViolated))
		}
		contextBuilder.WriteString("\n")
	}

	// Consult domain experts
	request := BatchConsultRequest{
		Topic:      "Campaign Planning Review",
		Question:   fmt.Sprintf("Review this campaign plan and provide your expert advice. What should we be careful about? What patterns should we follow? What are the risks?\n\nGoal: %s", goal),
		Context:    contextBuilder.String(),
		TargetSpec: []string{"coder", "tester", "reviewer", "researcher"},
	}

	responses, err := g.consultation.RequestBatchConsultation(ctx, request)
	if err != nil {
		addError(fmt.Sprintf("Shard consultation failed: %v", err))
		return
	}

	report.ShardAdvice = responses

	// Synthesize advisory summary
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("## Advisory Summary\n\n")
	for _, resp := range responses {
		if resp.Confidence > 0.5 {
			summaryBuilder.WriteString(fmt.Sprintf("**%s** (%.0f%% confidence): %s\n\n",
				resp.FromSpec, resp.Confidence*100, g.truncateAdvice(resp.Advice, 200)))
		}
	}
	report.AdvisorySummary = summaryBuilder.String()

	logging.CampaignDebug("Shard advice gathered: %d responses", len(report.ShardAdvice))
}

// =============================================================================
// HELPER METHODS
// =============================================================================

func (g *IntelligenceGatherer) parseAtom(arg interface{}) string {
	if ma, ok := arg.(core.MangleAtom); ok {
		return string(ma)
	}
	if s, ok := arg.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", arg)
}

func (g *IntelligenceGatherer) parseArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (g *IntelligenceGatherer) parseIntArg(arg interface{}) int {
	switch v := arg.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func (g *IntelligenceGatherer) parseFloatArg(arg interface{}) float64 {
	switch v := arg.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0.0
	}
}

func (g *IntelligenceGatherer) formatLearningDescription(learning types.ShardLearning) string {
	// Format learning args into human-readable description
	if len(learning.FactArgs) == 0 {
		return learning.FactPredicate
	}
	args := make([]string, len(learning.FactArgs))
	for i, arg := range learning.FactArgs {
		args[i] = fmt.Sprintf("%v", arg)
	}
	return fmt.Sprintf("%s(%s)", learning.FactPredicate, strings.Join(args, ", "))
}

func (g *IntelligenceGatherer) calculateToolAffinity(tool *mcp.MCPTool, goal string) float64 {
	// Simple keyword matching for affinity scoring
	goalLower := strings.ToLower(goal)
	descLower := strings.ToLower(tool.Description)
	nameLower := strings.ToLower(tool.Name)

	affinity := 0.0

	// Check for keyword matches
	keywords := strings.Fields(goalLower)
	for _, kw := range keywords {
		if len(kw) < 3 {
			continue
		}
		if strings.Contains(descLower, kw) {
			affinity += 0.1
		}
		if strings.Contains(nameLower, kw) {
			affinity += 0.2
		}
	}

	// Cap at 1.0
	if affinity > 1.0 {
		affinity = 1.0
	}

	return affinity
}

func (g *IntelligenceGatherer) detectArchitectureHints(topology map[string]FileInfo) []string {
	var hints []string

	// Count files by layer
	layers := make(map[string]int)
	for path := range topology {
		if strings.Contains(path, "/cmd/") || strings.Contains(path, "\\cmd\\") {
			layers["cmd"]++
		}
		if strings.Contains(path, "/internal/") || strings.Contains(path, "\\internal\\") {
			layers["internal"]++
		}
		if strings.Contains(path, "/pkg/") || strings.Contains(path, "\\pkg\\") {
			layers["pkg"]++
		}
		if strings.Contains(path, "/api/") || strings.Contains(path, "\\api\\") {
			layers["api"]++
		}
	}

	if layers["internal"] > 10 {
		hints = append(hints, "Standard Go project structure with internal packages")
	}
	if layers["cmd"] > 0 {
		hints = append(hints, fmt.Sprintf("%d CLI entrypoints detected", layers["cmd"]))
	}
	if layers["api"] > 0 {
		hints = append(hints, "API layer present - consider API stability")
	}

	return hints
}

func (g *IntelligenceGatherer) truncateAdvice(advice string, maxLen int) string {
	if len(advice) <= maxLen {
		return advice
	}
	return advice[:maxLen] + "..."
}

// =============================================================================
// FORMATTING FOR LLM CONTEXT
// =============================================================================

// FormatForContext formats the intelligence report for LLM context injection.
func (r *IntelligenceReport) FormatForContext() string {
	var sb strings.Builder

	sb.WriteString("# INTELLIGENCE REPORT\n\n")
	sb.WriteString(fmt.Sprintf("Gathered: %s (took %v)\n\n", r.GatheredAt.Format(time.RFC3339), r.Duration))

	// Codebase Overview
	sb.WriteString("## Codebase Overview\n")
	sb.WriteString(fmt.Sprintf("- Files scanned: %d\n", len(r.FileTopology)))
	sb.WriteString(fmt.Sprintf("- Symbols indexed: %d\n", len(r.SymbolGraph)))
	if len(r.LanguageBreakdown) > 0 {
		sb.WriteString("- Languages: ")
		langs := make([]string, 0, len(r.LanguageBreakdown))
		for lang, count := range r.LanguageBreakdown {
			langs = append(langs, fmt.Sprintf("%s (%d)", lang, count))
		}
		sb.WriteString(strings.Join(langs, ", ") + "\n")
	}
	sb.WriteString("\n")

	// High Churn Files (Chesterton's Fence)
	if len(r.GitChurnHotspots) > 0 {
		sb.WriteString("## High Churn Files (Chesterton's Fence)\n")
		sb.WriteString("⚠️ These files change frequently. Understand WHY before modifying.\n\n")
		for i, h := range r.GitChurnHotspots {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(r.GitChurnHotspots)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %d changes\n", h.Path, h.ChurnRate))
		}
		sb.WriteString("\n")
	}

	// Historical Patterns
	if len(r.HistoricalPatterns) > 0 {
		sb.WriteString("## Learned Patterns\n")
		for i, p := range r.HistoricalPatterns {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s (%.0f%% confidence)\n", p.Description, p.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Safety Warnings
	if len(r.SafetyWarnings) > 0 {
		sb.WriteString("## ⚠️ Safety Warnings\n")
		for _, w := range r.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (severity: %s)\n", w.Action, w.RuleViolated, w.Severity))
		}
		sb.WriteString("\n")
	}

	// Available Tools
	if len(r.MCPToolsAvailable) > 0 {
		sb.WriteString("## Available MCP Tools\n")
		for i, t := range r.MCPToolsAvailable {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more tools\n", len(r.MCPToolsAvailable)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", t.Name, t.Description))
		}
		sb.WriteString("\n")
	}

	// Tool Gaps
	if len(r.ToolGaps) > 0 {
		sb.WriteString("## Tool Gaps Detected\n")
		for _, g := range r.ToolGaps {
			sb.WriteString(fmt.Sprintf("- %s: %s (confidence: %.0f%%)\n", g.Name, g.Purpose, g.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Expert Advice
	if r.AdvisorySummary != "" {
		sb.WriteString(r.AdvisorySummary)
		sb.WriteString("\n")
	}

	// Test Coverage
	if len(r.UncoveredPaths) > 0 {
		sb.WriteString("## Test Coverage Gaps\n")
		for i, p := range r.UncoveredPaths {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	// Architecture Hints
	if len(r.ArchitectureHints) > 0 {
		sb.WriteString("## Architecture Hints\n")
		for _, h := range r.ArchitectureHints {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

