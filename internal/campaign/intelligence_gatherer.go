// Package campaign provides multi-phase goal orchestration.
// This file implements intelligence gathering from all available systems
// to provide comprehensive context before campaign planning.
package campaign

import (
	"context"
	"fmt"
	"math"
	"strconv"
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
	GatherTimeout    time.Duration
	PerSystemTimeout time.Duration
	ConsultTimeout   time.Duration

	// Limits
	MaxChurnHotspots     int
	MaxLearnings         int
	MaxMCPTools          int
	MaxPreviousCampaigns int
	GitHistoryDepth      int

	// Feature flags
	EnableWorldModel        bool
	EnableGitHistory        bool
	EnableLearningStore     bool
	EnableKnowledgeGraph    bool
	EnableColdStorage       bool
	EnableSafetyCheck       bool
	EnableAutopoiesis       bool
	EnableMCPTools          bool
	EnablePreviousCampaigns bool
	EnableShardConsult      bool
	EnableTestCoverage      bool
	EnableCodePatterns      bool
}

// DefaultIntelligenceConfig returns sensible defaults.
func DefaultIntelligenceConfig() IntelligenceConfig {
	return IntelligenceConfig{
		GatherTimeout:           5 * time.Minute,
		PerSystemTimeout:        30 * time.Second,
		ConsultTimeout:          2 * time.Minute,
		MaxChurnHotspots:        50,
		MaxLearnings:            100,
		MaxMCPTools:             30,
		MaxPreviousCampaigns:    10,
		GitHistoryDepth:         100,
		EnableWorldModel:        true,
		EnableGitHistory:        true,
		EnableLearningStore:     true,
		EnableKnowledgeGraph:    true,
		EnableColdStorage:       true,
		EnableSafetyCheck:       true,
		EnableAutopoiesis:       true,
		EnableMCPTools:          true,
		EnablePreviousCampaigns: true,
		EnableShardConsult:      true,
		EnableTestCoverage:      true,
		EnableCodePatterns:      true,
	}
}

// IntelligenceReport contains all gathered intelligence for campaign planning.
type IntelligenceReport struct {
	// Timestamp
	GatheredAt time.Time     `json:"gathered_at"`
	Duration   time.Duration `json:"duration"`

	// World Model: Codebase structure
	WorldFacts        []core.Fact         `json:"world_facts"`
	FileTopology      map[string]FileInfo `json:"file_topology"`
	SymbolGraph       []SymbolInfo        `json:"symbol_graph"`
	LanguageBreakdown map[string]int      `json:"language_breakdown"`

	// Git History: Churn analysis (Chesterton's Fence)
	GitChurnHotspots []ChurnHotspot `json:"git_churn_hotspots"`
	RecentCommits    []CommitInfo   `json:"recent_commits"`
	HighChurnFiles   []string       `json:"high_churn_files"`

	// Learning Store: Historical patterns
	HistoricalPatterns []LearningPattern  `json:"historical_patterns"`
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
	ToolGaps            []autopoiesis.ToolNeed `json:"tool_gaps"`
	MissingCapabilities []string               `json:"missing_capabilities"`

	// MCP: Available external tools
	MCPToolsAvailable []MCPToolInfo     `json:"mcp_tools_available"`
	MCPServerStatus   map[string]string `json:"mcp_server_status"`

	// Previous Campaigns: Reusable artifacts
	PreviousCampaigns []CampaignArtifact `json:"previous_campaigns"`
	ReusablePatterns  []string           `json:"reusable_patterns"`

	// Shard Consultation: Expert advice
	ShardAdvice     []ConsultationResponse `json:"shard_advice"`
	AdvisorySummary string                 `json:"advisory_summary"`

	// Test Coverage: Current state
	TestCoverage   map[string]float64 `json:"test_coverage"`
	UncoveredPaths []string           `json:"uncovered_paths"`

	// Code Patterns: Detected patterns
	CodePatterns      []CodePattern `json:"code_patterns"`
	ArchitectureHints []string      `json:"architecture_hints"`

	// Pinned snapshot used by deterministic campaign risk scoring.
	RiskInputs RiskInputSnapshot `json:"risk_inputs"`

	// Errors during gathering (non-fatal)
	GatheringErrors []string `json:"gathering_errors"`
}

// IsEmpty returns true if the intelligence report has no meaningful data.
func (i *IntelligenceReport) IsEmpty() bool {
	if i == nil {
		return true
	}
	return len(i.FileTopology) == 0 &&
		len(i.SymbolGraph) == 0 &&
		len(i.WorldFacts) == 0 &&
		len(i.GitChurnHotspots) == 0
}

// Supporting types for IntelligenceReport

// FileInfo represents file topology information.
type FileInfo struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	Language     string    `json:"language"`
	LineCount    int       `json:"line_count"`
	IsTestFile   bool      `json:"is_test_file"`
	LastModified time.Time `json:"last_modified"`
}

// SymbolInfo represents a code symbol.
type SymbolInfo struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"` // func, type, const, var
	File     string `json:"file"`
	Line     int    `json:"line"`
	Exported bool   `json:"exported"`
}

// ChurnHotspot represents a file with high churn rate.
type ChurnHotspot struct {
	Path       string    `json:"path"`
	ChurnRate  int       `json:"churn_rate"`
	LastChange time.Time `json:"last_change"`
	Reason     string    `json:"reason"`
	Warning    string    `json:"warning"` // Chesterton's Fence warning
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
	ShardType   string    `json:"shard_type"`
	Predicate   string    `json:"predicate"`
	Description string    `json:"description"`
	Confidence  float64   `json:"confidence"`
	LastUsed    time.Time `json:"last_used"`
}

// PreferenceSignal represents a user preference.
type PreferenceSignal struct {
	Category string  `json:"category"`
	Signal   string  `json:"signal"`
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
	CampaignID   string `json:"campaign_id"`
	Path         string `json:"path"`
	Action       string `json:"action"`
	RuleViolated string `json:"rule_violated"`
	Severity     string `json:"severity"`
	Remediation  string `json:"remediation"`
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
	Type        string   `json:"type"` // design, architecture, anti-pattern
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
	var errorCount int
	const maxGatherErrors = 100
	addError := func(err string) {
		mu.Lock()
		defer mu.Unlock()
		if errorCount < maxGatherErrors {
			report.GatheringErrors = append(report.GatheringErrors, err)
			errorCount++
		} else if errorCount == maxGatherErrors {
			report.GatheringErrors = append(report.GatheringErrors, fmt.Sprintf("... and further errors suppressed (limit %d reached)", maxGatherErrors))
			errorCount++
		}
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
	report.RiskInputs = deriveRiskInputSnapshotFromReport(report)
	logging.Campaign("Intelligence gathering completed: %d world facts, %d churn hotspots, %d learnings, %d errors (took %v)",
		len(report.WorldFacts), len(report.GitChurnHotspots), len(report.HistoricalPatterns),
		len(report.GatheringErrors), report.Duration)

	return report, nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

func (g *IntelligenceGatherer) parseAtom(arg any) string {
	if ma, ok := arg.(core.MangleAtom); ok {
		return string(ma)
	}
	if s, ok := arg.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", arg)
}

func (g *IntelligenceGatherer) parseArg(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (g *IntelligenceGatherer) parseIntArg(arg any) int {
	switch v := arg.(type) {
	case int:
		return v
	case int64:
		// Bounds check to prevent platform-dependent overflow on 32-bit
		if v > math.MaxInt32 {
			return math.MaxInt32
		}
		if v < math.MinInt32 {
			return math.MinInt32
		}
		return int(v)
	case float64:
		if v > float64(math.MaxInt32) {
			return math.MaxInt32
		}
		if v < float64(math.MinInt32) {
			return math.MinInt32
		}
		return int(v)
	default:
		return 0
	}
}

func (g *IntelligenceGatherer) parseFloatArg(arg any) float64 {
	switch v := arg.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		// Handle string-encoded floats from Mangle (e.g., "0.95")
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		return 0.0
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
	keywords := strings.FieldsSeq(goalLower)
	for kw := range keywords {
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

// truncateField truncates a string to maxLen for safe context injection.
func truncateField(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
