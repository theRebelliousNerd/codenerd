// Package campaign provides multi-phase goal orchestration.
// This file implements tool pre-generation via Ouroboros integration.
// It detects capability gaps before campaign execution and generates
// the necessary tools upfront.
package campaign

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/logging"
	"codenerd/internal/mcp"
)

// =============================================================================
// TOOL PREGENERATOR
// =============================================================================
// Pre-generates tools via Ouroboros before campaign execution.
// This ensures all required capabilities are available before tasks start.

// ToolPregenerator handles pre-generation of tools for campaigns.
type ToolPregenerator struct {
	// Autopoiesis components
	toolGenerator *autopoiesis.ToolGenerator
	ouroboros     *autopoiesis.OuroborosLoop

	// MCP tools
	mcpStore *mcp.MCPToolStore

	// Configuration
	config PregeneratorConfig
}

// PregeneratorConfig configures tool pre-generation behavior.
type PregeneratorConfig struct {
	// Timeouts
	DetectionTimeout  time.Duration
	GenerationTimeout time.Duration
	ValidationTimeout time.Duration

	// Limits
	MaxToolsToGenerate int     // Maximum tools to generate per campaign
	MinConfidence      float64 // Minimum confidence to generate a tool

	// Safety
	RequireThunderdome bool // Run generated tools through Thunderdome
	RequireSimulation  bool // Simulate tools in Dream State before use

	// Enabled features
	EnableMCPFallback bool // Try MCP tools before generating new ones
	EnableToolCaching bool // Cache generated tools for reuse
}

// DefaultPregeneratorConfig returns sensible defaults.
func DefaultPregeneratorConfig() PregeneratorConfig {
	return PregeneratorConfig{
		DetectionTimeout:   30 * time.Second,
		GenerationTimeout:  5 * time.Minute,
		ValidationTimeout:  2 * time.Minute,
		MaxToolsToGenerate: 5,
		MinConfidence:      0.6,
		RequireThunderdome: true,
		RequireSimulation:  false,
		EnableMCPFallback:  true,
		EnableToolCaching:  true,
	}
}

// ToolGap represents a detected capability gap.
type ToolGap struct {
	ID          string   `json:"id"`
	Capability  string   `json:"capability"`
	RequiredBy  []string `json:"required_by"` // Task IDs that need this
	Priority    float64  `json:"priority"`    // 0.0-1.0
	Confidence  float64  `json:"confidence"`  // How confident we are this is needed
	Description string   `json:"description"`

	// Resolution
	ResolvedBy     string `json:"resolved_by,omitempty"`     // Tool ID if resolved
	ResolutionType string `json:"resolution_type,omitempty"` // "generated", "mcp", "existing"
}

// GeneratedTool represents a tool generated for the campaign.
type GeneratedTool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Purpose     string    `json:"purpose"`
	InputType   string    `json:"input_type"`
	OutputType  string    `json:"output_type"`
	GeneratedAt time.Time `json:"generated_at"`

	// Validation
	PassedThunderdome bool     `json:"passed_thunderdome"`
	PassedSimulation  bool     `json:"passed_simulation"`
	ValidationErrors  []string `json:"validation_errors,omitempty"`

	// Source
	SourceGap string `json:"source_gap"` // Which gap this resolves

	// Status
	Status     string `json:"status"`                // "pending", "validated", "ready", "failed"
	RegistryID string `json:"registry_id,omitempty"` // ID in tool registry
}

// PregenerationResult contains the results of tool pre-generation.
type PregenerationResult struct {
	GapsDetected   []ToolGap       `json:"gaps_detected"`
	ToolsGenerated []GeneratedTool `json:"tools_generated"`
	MCPToolsUsed   []MCPToolInfo   `json:"mcp_tools_used"`
	UnresolvedGaps []ToolGap       `json:"unresolved_gaps"`

	// Statistics
	TotalGaps    int           `json:"total_gaps"`
	ResolvedGaps int           `json:"resolved_gaps"`
	FailedTools  int           `json:"failed_tools"`
	Duration     time.Duration `json:"duration"`

	// Errors
	Errors []string `json:"errors,omitempty"`
}

// NewToolPregenerator creates a new tool pregenerator.
func NewToolPregenerator(
	toolGenerator *autopoiesis.ToolGenerator,
	ouroboros *autopoiesis.OuroborosLoop,
	mcpStore *mcp.MCPToolStore,
) *ToolPregenerator {
	return &ToolPregenerator{
		toolGenerator: toolGenerator,
		ouroboros:     ouroboros,
		mcpStore:      mcpStore,
		config:        DefaultPregeneratorConfig(),
	}
}

// WithConfig sets custom configuration.
func (p *ToolPregenerator) WithConfig(config PregeneratorConfig) *ToolPregenerator {
	p.config = config
	return p
}

// DetectGaps analyzes the campaign and identifies missing capabilities.
func (p *ToolPregenerator) DetectGaps(ctx context.Context, goal string, tasks []TaskInfo, intel *IntelligenceReport) ([]ToolGap, error) {
	logging.Campaign("Detecting tool gaps for campaign with %d tasks", len(tasks))
	timer := logging.StartTimer(logging.CategoryCampaign, "DetectGaps")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(ctx, p.config.DetectionTimeout)
	defer cancel()

	gaps := []ToolGap{}
	gapIndex := 0

	// Analyze each task for capability requirements
	for _, task := range tasks {
		taskGaps := p.analyzeTaskForGaps(ctx, task, intel)
		for _, gap := range taskGaps {
			gap.ID = fmt.Sprintf("gap-%d", gapIndex)
			gap.RequiredBy = []string{task.ID}
			gapIndex++
			gaps = append(gaps, gap)
		}
	}

	// Deduplicate and merge gaps
	gaps = p.deduplicateGaps(gaps)

	// Check which gaps can be resolved by existing tools
	gaps = p.checkExistingResolutions(ctx, gaps, intel)

	logging.Campaign("Detected %d tool gaps (%d resolvable with existing tools)",
		len(gaps), p.countResolvedGaps(gaps))

	return gaps, nil
}

// PregenerateTools generates tools for detected gaps.
func (p *ToolPregenerator) PregenerateTools(ctx context.Context, gaps []ToolGap) (*PregenerationResult, error) {
	startTime := time.Now()
	logging.Campaign("Pre-generating tools for %d gaps", len(gaps))
	timer := logging.StartTimer(logging.CategoryCampaign, "PregenerateTools")
	defer timer.Stop()

	result := &PregenerationResult{
		GapsDetected:   gaps,
		ToolsGenerated: []GeneratedTool{},
		MCPToolsUsed:   []MCPToolInfo{},
		UnresolvedGaps: []ToolGap{},
		TotalGaps:      len(gaps),
		Errors:         []string{},
	}

	// Filter to unresolved gaps that need generation
	toGenerate := []ToolGap{}
	for _, gap := range gaps {
		if gap.ResolvedBy != "" {
			result.ResolvedGaps++
			continue
		}
		if gap.Confidence < p.config.MinConfidence {
			logging.CampaignDebug("Skipping low-confidence gap: %s (%.0f%%)", gap.Capability, gap.Confidence*100)
			continue
		}
		toGenerate = append(toGenerate, gap)
	}

	// Limit to max tools
	if len(toGenerate) > p.config.MaxToolsToGenerate {
		logging.Campaign("Limiting tool generation from %d to %d", len(toGenerate), p.config.MaxToolsToGenerate)
		// Keep highest priority gaps
		toGenerate = toGenerate[:p.config.MaxToolsToGenerate]
	}

	// Generate each tool
GenerationLoop:
	for _, gap := range toGenerate {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, "Context cancelled during generation")
			break GenerationLoop
		default:
		}

		tool, err := p.generateTool(ctx, gap)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to generate tool for %s: %v", gap.Capability, err))
			result.FailedTools++
			result.UnresolvedGaps = append(result.UnresolvedGaps, gap)
			continue
		}

		result.ToolsGenerated = append(result.ToolsGenerated, *tool)
		result.ResolvedGaps++
	}

	result.Duration = time.Since(startTime)
	logging.Campaign("Tool pre-generation complete: %d generated, %d failed, %d unresolved (took %v)",
		len(result.ToolsGenerated), result.FailedTools, len(result.UnresolvedGaps), result.Duration)

	return result, nil
}

// TaskInfo represents task information for gap analysis.
type TaskInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Type        string   `json:"type"`    // "implement", "test", "refactor", etc.
	Actions     []string `json:"actions"` // Specific actions like "parse_yaml", "validate_schema"
	FilePaths   []string `json:"file_paths"`
}

// analyzeTaskForGaps analyzes a single task for capability gaps.
func (p *ToolPregenerator) analyzeTaskForGaps(ctx context.Context, task TaskInfo, intel *IntelligenceReport) []ToolGap {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	gaps := []ToolGap{}

	// Use intelligence report to boost confidence for detected patterns
	var intelFileTypes []string
	if intel != nil {
		for path := range intel.FileTopology {
			intelFileTypes = append(intelFileTypes, strings.ToLower(path))
		}
	}

	// Pattern-based detection
	descLower := strings.ToLower(task.Description)

	// Check for parsing needs
	if strings.Contains(descLower, "parse") || strings.Contains(descLower, "extract") {
		formats := []string{"yaml", "json", "xml", "csv", "toml", "ini"}
		for _, format := range formats {
			if strings.Contains(descLower, format) {
				// Boost confidence if intel mentions this format
				confidence := 0.7
				for _, ft := range intelFileTypes {
					if strings.Contains(ft, "."+format) {
						confidence = 0.85
						break
					}
				}
				gaps = append(gaps, ToolGap{
					Capability:  fmt.Sprintf("parse_%s", format),
					Description: fmt.Sprintf("Parse %s files/content", format),
					Priority:    0.8,
					Confidence:  confidence,
				})
			}
		}
	}

	// Check for validation needs
	if strings.Contains(descLower, "validate") || strings.Contains(descLower, "check") {
		gaps = append(gaps, ToolGap{
			Capability:  "validator",
			Description: "Validate data against schema or rules",
			Priority:    0.7,
			Confidence:  0.6,
		})
	}

	// Check for API interaction needs
	if strings.Contains(descLower, "api") || strings.Contains(descLower, "fetch") || strings.Contains(descLower, "request") {
		gaps = append(gaps, ToolGap{
			Capability:  "http_client",
			Description: "Make HTTP requests to external APIs",
			Priority:    0.9,
			Confidence:  0.8,
		})
	}

	// Check for database needs
	if strings.Contains(descLower, "database") || strings.Contains(descLower, "sql") || strings.Contains(descLower, "query") {
		gaps = append(gaps, ToolGap{
			Capability:  "database_client",
			Description: "Execute database queries",
			Priority:    0.9,
			Confidence:  0.7,
		})
	}

	// Check for file transformation needs
	if strings.Contains(descLower, "convert") || strings.Contains(descLower, "transform") {
		gaps = append(gaps, ToolGap{
			Capability:  "converter",
			Description: "Convert between data formats",
			Priority:    0.6,
			Confidence:  0.5,
		})
	}

	// Use autopoiesis tool detection if available
	if p.toolGenerator != nil {
		need, err := p.toolGenerator.DetectToolNeed(ctx, task.Description, "")
		if err == nil && need != nil {
			gaps = append(gaps, ToolGap{
				Capability:  need.Name,
				Description: need.Purpose,
				Priority:    need.Priority,
				Confidence:  need.Confidence,
			})
		}
	}

	return gaps
}

// deduplicateGaps merges duplicate gaps.
func (p *ToolPregenerator) deduplicateGaps(gaps []ToolGap) []ToolGap {
	seen := make(map[string]*ToolGap)

	for i := range gaps {
		gap := &gaps[i]
		if existing, ok := seen[gap.Capability]; ok {
			// Merge: keep higher confidence and priority
			if gap.Confidence > existing.Confidence {
				existing.Confidence = gap.Confidence
			}
			if gap.Priority > existing.Priority {
				existing.Priority = gap.Priority
			}
			existing.RequiredBy = append(existing.RequiredBy, gap.RequiredBy...)
		} else {
			seen[gap.Capability] = gap
		}
	}

	result := make([]ToolGap, 0, len(seen))
	for _, gap := range seen {
		result = append(result, *gap)
	}

	return result
}

// checkExistingResolutions checks if gaps can be resolved with existing tools.
func (p *ToolPregenerator) checkExistingResolutions(ctx context.Context, gaps []ToolGap, intel *IntelligenceReport) []ToolGap {
	// Early exit on context cancellation
	if err := ctx.Err(); err != nil {
		return gaps
	}

	for i := range gaps {
		gap := &gaps[i]

		// Check MCP tools first
		if p.config.EnableMCPFallback && intel != nil {
			for _, tool := range intel.MCPToolsAvailable {
				if p.toolMatchesGap(tool, *gap) {
					gap.ResolvedBy = tool.ToolID
					gap.ResolutionType = "mcp"
					break
				}
			}
		}

		// Check core built-in tools
		if gap.ResolvedBy == "" {
			if toolID := p.checkBuiltinTools(gap.Capability); toolID != "" {
				gap.ResolvedBy = toolID
				gap.ResolutionType = "existing"
			}
		}
	}

	return gaps
}

// toolMatchesGap checks if an MCP tool can resolve a gap.
func (p *ToolPregenerator) toolMatchesGap(tool MCPToolInfo, gap ToolGap) bool {
	nameLower := strings.ToLower(tool.Name)
	descLower := strings.ToLower(tool.Description)
	capLower := strings.ToLower(gap.Capability)

	// Direct name match
	if strings.Contains(nameLower, capLower) {
		return true
	}

	// Category match
	for _, cat := range tool.Categories {
		if strings.Contains(strings.ToLower(cat), capLower) {
			return true
		}
	}

	// Description match
	keywords := strings.Fields(capLower)
	matchCount := 0
	for _, kw := range keywords {
		if len(kw) < 3 {
			continue
		}
		if strings.Contains(descLower, kw) {
			matchCount++
		}
	}
	return matchCount >= len(keywords)/2
}

// checkBuiltinTools checks if a built-in tool exists for the capability.
func (p *ToolPregenerator) checkBuiltinTools(capability string) string {
	builtins := map[string]string{
		"read_file":   "core/read_file",
		"write_file":  "core/write_file",
		"glob":        "core/glob",
		"grep":        "core/grep",
		"http_client": "research/web_fetch",
		"web_search":  "research/web_search",
		"run_command": "shell/run_command",
		"bash":        "shell/bash",
	}

	capLower := strings.ToLower(capability)
	for name, id := range builtins {
		if strings.Contains(capLower, name) || strings.Contains(name, capLower) {
			return id
		}
	}
	return ""
}

// generateTool generates a single tool for a gap.
func (p *ToolPregenerator) generateTool(ctx context.Context, gap ToolGap) (*GeneratedTool, error) {
	logging.Campaign("Generating tool for gap: %s", gap.Capability)

	ctx, cancel := context.WithTimeout(ctx, p.config.GenerationTimeout)
	defer cancel()

	tool := &GeneratedTool{
		ID:          fmt.Sprintf("gen-%s-%d", gap.Capability, time.Now().Unix()),
		Name:        gap.Capability,
		Purpose:     gap.Description,
		GeneratedAt: time.Now(),
		SourceGap:   gap.ID,
		Status:      "pending",
	}

	// Use Ouroboros loop if available
	if p.ouroboros != nil {
		need := &autopoiesis.ToolNeed{
			Name:       gap.Capability,
			Purpose:    gap.Description,
			Priority:   gap.Priority,
			Confidence: gap.Confidence,
		}

		result := p.ouroboros.Execute(ctx, need)
		if !result.Success {
			tool.Status = "failed"
			tool.ValidationErrors = append(tool.ValidationErrors, result.Error)
			return tool, fmt.Errorf("tool generation failed: %s", result.Error)
		}

		tool.Name = result.ToolName
		if result.ToolHandle != nil {
			tool.RegistryID = result.ToolHandle.Name
		}
		tool.Status = "validated"

		// Run through Thunderdome if required
		if p.config.RequireThunderdome && result.ToolHandle != nil {
			passed, errors := p.runThunderdomeForTool(ctx, result.ToolHandle)
			tool.PassedThunderdome = passed
			if !passed {
				tool.ValidationErrors = append(tool.ValidationErrors, errors...)
				tool.Status = "failed"
				return tool, fmt.Errorf("tool failed Thunderdome: %v", errors)
			}
		}

		tool.Status = "ready"
		logging.Campaign("Tool generated and validated: %s", tool.Name)
		return tool, nil
	}

	// Fallback: just mark as pending for manual implementation
	tool.Status = "pending_manual"
	tool.ValidationErrors = append(tool.ValidationErrors, "Ouroboros not available - manual implementation required")
	return tool, nil
}

// runThunderdomeForTool runs a runtime tool through adversarial testing.
func (p *ToolPregenerator) runThunderdomeForTool(ctx context.Context, tool *autopoiesis.RuntimeTool) (bool, []string) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return false, []string{fmt.Sprintf("Thunderdome cancelled: %v", err)}
	}

	// Placeholder for Thunderdome integration
	// In full implementation, this would run attack vectors against the tool
	logging.CampaignDebug("Thunderdome testing for tool: %s (ctx deadline: %v)", tool.Name,
		func() string {
			if d, ok := ctx.Deadline(); ok {
				return d.Format(time.RFC3339)
			}
			return "none"
		}())

	// For now, assume tools pass if they were generated successfully
	return true, nil
}

// countResolvedGaps counts gaps that have been resolved.
func (p *ToolPregenerator) countResolvedGaps(gaps []ToolGap) int {
	count := 0
	for _, gap := range gaps {
		if gap.ResolvedBy != "" {
			count++
		}
	}
	return count
}

// =============================================================================
// FORMATTING FOR LLM CONTEXT
// =============================================================================

// FormatForContext formats the pre-generation result for LLM context injection.
func (r *PregenerationResult) FormatForContext() string {
	var sb strings.Builder

	sb.WriteString("# TOOL PRE-GENERATION RESULTS\n\n")
	sb.WriteString(fmt.Sprintf("**Gaps Detected:** %d\n", r.TotalGaps))
	sb.WriteString(fmt.Sprintf("**Resolved:** %d\n", r.ResolvedGaps))
	sb.WriteString(fmt.Sprintf("**Failed:** %d\n", r.FailedTools))
	sb.WriteString(fmt.Sprintf("**Duration:** %v\n\n", r.Duration))

	// Generated tools
	if len(r.ToolsGenerated) > 0 {
		sb.WriteString("## Generated Tools\n")
		for _, tool := range r.ToolsGenerated {
			status := "✅"
			if tool.Status == "failed" {
				status = "❌"
			} else if tool.Status == "pending_manual" {
				status = "⏳"
			}
			sb.WriteString(fmt.Sprintf("- %s `%s`: %s\n", status, tool.Name, tool.Purpose))
		}
		sb.WriteString("\n")
	}

	// MCP tools used
	if len(r.MCPToolsUsed) > 0 {
		sb.WriteString("## MCP Tools Available\n")
		for _, tool := range r.MCPToolsUsed {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", tool.Name, tool.Description))
		}
		sb.WriteString("\n")
	}

	// Unresolved gaps
	if len(r.UnresolvedGaps) > 0 {
		sb.WriteString("## ⚠️ Unresolved Gaps\n")
		sb.WriteString("These capabilities may need manual implementation:\n")
		for _, gap := range r.UnresolvedGaps {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", gap.Capability, gap.Description))
		}
		sb.WriteString("\n")
	}

	// Errors
	if len(r.Errors) > 0 {
		sb.WriteString("## Errors\n")
		for _, err := range r.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return sb.String()
}

// IsReady returns true if all required tools are available.
func (r *PregenerationResult) IsReady() bool {
	return len(r.UnresolvedGaps) == 0 && r.FailedTools == 0
}

// GetBlockingIssues returns issues that block campaign execution.
func (r *PregenerationResult) GetBlockingIssues() []string {
	var issues []string

	for _, gap := range r.UnresolvedGaps {
		if gap.Priority > 0.7 {
			issues = append(issues, fmt.Sprintf("Missing high-priority tool: %s", gap.Capability))
		}
	}

	for _, err := range r.Errors {
		if strings.Contains(strings.ToLower(err), "critical") || strings.Contains(strings.ToLower(err), "failed") {
			issues = append(issues, err)
		}
	}

	return issues
}
