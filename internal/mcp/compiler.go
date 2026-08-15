package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// KernelInterface defines the interface for Mangle kernel operations.
type KernelInterface interface {
	Assert(fact string) error
	Retract(fact string) error
	Query(query string) ([]map[string]any, error)
}

// JITToolCompiler compiles a context-aware tool set for LLM consumption.
// It mirrors the JIT Prompt Compiler architecture with skeleton/flesh selection.
type JITToolCompiler struct {
	store    *MCPToolStore
	embedder embedding.EmbeddingEngine
	kernel   KernelInterface
	config   ToolSelectionConfig
}

// NewJITToolCompiler creates a new JIT tool compiler.
func NewJITToolCompiler(store *MCPToolStore, embedder embedding.EmbeddingEngine, kernel KernelInterface) *JITToolCompiler {
	return &JITToolCompiler{
		store:    store,
		embedder: embedder,
		kernel:   kernel,
		config:   DefaultToolSelectionConfig(),
	}
}

// SetConfig sets the tool selection configuration.
func (c *JITToolCompiler) SetConfig(config ToolSelectionConfig) {
	c.config = config
}

// Compile generates a context-aware tool set.
func (c *JITToolCompiler) Compile(ctx context.Context, tcc ToolCompilationContext) (*CompiledToolSet, error) {
	start := time.Now()
	stats := ToolCompilationStats{
		TokenBudget: tcc.TokenBudget,
	}

	if tcc.TokenBudget <= 0 {
		tcc.TokenBudget = c.config.TokenBudget
		stats.TokenBudget = tcc.TokenBudget
	}

	// Get all available tools
	allTools, err := c.store.GetAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}
	stats.TotalTools = len(allTools)

	if len(allTools) == 0 {
		return &CompiledToolSet{Stats: stats}, nil
	}

	// Phase 1: Vector search for relevant tools
	var vectorScores map[string]float64
	vectorStart := time.Now()
	if c.embedder != nil && tcc.TaskDescription != "" {
		vectorScores, err = c.vectorSearch(ctx, tcc.TaskDescription, allTools)
		if err != nil {
			logging.Get(logging.CategoryTools).Debug("Vector search failed: %v", err)
		}
	}
	stats.VectorQueryMs = time.Since(vectorStart).Milliseconds()

	// Phase 2: Assert vector scores to Mangle kernel. Track what was actually
	// asserted: the kernel adapter retracts by exact fact, so cleanup needs the
	// score value, not a wildcard.
	assertedScores := make(map[string]int, len(vectorScores))
	if c.kernel != nil && len(vectorScores) > 0 {
		for toolID, score := range vectorScores {
			scoreInt := int(score * 100)
			if err := c.kernel.Assert(fmt.Sprintf("mcp_tool_vector_score(%q, %d)", toolID, scoreInt)); err != nil {
				logging.Get(logging.CategoryTools).Debug("Failed to assert vector score: %v", err)
				continue
			}
			assertedScores[toolID] = scoreInt
		}
	}

	// Phase 3: Query Mangle for tool selection (or use fallback)
	mangleStart := time.Now()
	selected := c.selectTools(ctx, tcc, allTools, vectorScores, &stats)
	stats.MangleQueryMs = time.Since(mangleStart).Milliseconds()

	// Phase 4: Build compiled tool set
	result := c.buildToolSet(allTools, selected, &stats)

	// Phase 5: Fit to budget
	c.fitBudget(result, tcc.TokenBudget, &stats)

	// Cleanup: Retract temporary vector scores. The previous form passed a "_"
	// wildcard, which the kernel adapter parses as a variable and then fails to
	// match against any stored fact — every compile leaked its scores, and the
	// next compile blended stale similarity into the ranking.
	if c.kernel != nil {
		for toolID, scoreInt := range assertedScores {
			if err := c.kernel.Retract(fmt.Sprintf("mcp_tool_vector_score(%q, %d)", toolID, scoreInt)); err != nil {
				logging.Get(logging.CategoryTools).Debug("Failed to retract vector score for %s: %v", toolID, err)
			}
		}
	}

	stats.Duration = time.Since(start)
	result.Stats = stats

	logging.Get(logging.CategoryTools).Info(
		"JIT Tool Compiler: %dms | path=%s | tools=%d (full=%d, condensed=%d, minimal=%d) | skeleton=%d flesh=%d | vec=%dms | budget=%d/%d",
		stats.Duration.Milliseconds(),
		stats.SelectionPath,
		stats.SelectedTools,
		len(result.FullTools),
		len(result.CondensedTools),
		len(result.MinimalTools),
		stats.SkeletonTools,
		stats.FleshTools,
		stats.VectorQueryMs,
		stats.TokensUsed,
		stats.TokenBudget,
	)

	return result, nil
}

// vectorSearch performs semantic search over tool embeddings.
func (c *JITToolCompiler) vectorSearch(ctx context.Context, query string, tools []*MCPTool) (map[string]float64, error) {
	// Generate query embedding
	queryTask := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	var queryEmbed []float32
	var err error
	if taskAware, ok := c.embedder.(embedding.TaskTypeAwareEngine); ok && queryTask != "" {
		queryEmbed, err = taskAware.EmbedWithTask(ctx, query, queryTask)
	} else {
		queryEmbed, err = c.embedder.Embed(ctx, query)
	}
	if err != nil {
		return nil, err
	}

	// Search in store
	results, err := c.store.SemanticSearch(ctx, queryEmbed, len(tools))
	if err != nil {
		return nil, err
	}

	// Convert to map
	scores := make(map[string]float64)
	for _, r := range results {
		scores[r.ToolID] = r.Score
	}

	return scores, nil
}

// selectTools selects tools using Mangle or fallback logic.
func (c *JITToolCompiler) selectTools(ctx context.Context, tcc ToolCompilationContext, tools []*MCPTool, vectorScores map[string]float64, stats *ToolCompilationStats) []SelectedTool {
	// Try Mangle-based selection first
	if c.kernel != nil {
		selected, err := c.mangleSelect(ctx, tcc)
		switch {
		case err != nil:
			// Warn, not Debug: a broken kernel query silently downgrades the
			// system from logic-governed selection to a Go heuristic, and that
			// downgrade was previously invisible in default logs.
			logging.Get(logging.CategoryTools).Warn(
				"MCP tool selection: Mangle query failed for shard %q, using Go fallback: %v", tcc.ShardType, err)
		case len(selected) == 0:
			logging.Get(logging.CategoryTools).Info(
				"MCP tool selection: Mangle derived no tools for shard %q, using Go fallback", tcc.ShardType)
		default:
			if stats != nil {
				stats.SelectionPath = SelectionPathMangle
			}
			return selected
		}
	}

	// Fallback: Simple affinity-based selection
	if stats != nil {
		stats.SelectionPath = SelectionPathFallback
	}
	return c.fallbackSelect(tcc, tools, vectorScores)
}

// mangleSelect uses Mangle kernel for tool selection.
func (c *JITToolCompiler) mangleSelect(ctx context.Context, tcc ToolCompilationContext) ([]SelectedTool, error) {
	// ShardType is declared as a /name in schemas_mcp.mg. Quoting it as a
	// string produced a pattern that could never match a stored fact, so this
	// query always came back empty and the compiler always fell back.
	shardAtom := mangleAtom(tcc.ShardType)
	query := fmt.Sprintf("mcp_tool_selected(%s, ToolID, RenderMode)", shardAtom)
	results, err := c.kernel.Query(query)
	if err != nil {
		return nil, err
	}

	skeletons := c.mangleSkeletonSet()

	var selected []SelectedTool
	for _, r := range results {
		toolID, _ := r["ToolID"].(string)
		if toolID == "" {
			continue
		}
		renderModeRaw, _ := r["RenderMode"].(string)

		var renderMode RenderMode
		switch strings.ToLower(renderModeRaw) {
		case "/full", "full":
			renderMode = RenderModeFull
		case "/condensed", "condensed":
			renderMode = RenderModeCondensed
		case "/minimal", "minimal":
			renderMode = RenderModeMinimal
		default:
			renderMode = RenderModeCondensed
		}

		_, isSkeleton := skeletons[toolID]
		selected = append(selected, SelectedTool{
			ToolID:     toolID,
			RenderMode: renderMode,
			Skeleton:   isSkeleton,
		})
	}

	return selected, nil
}

// mangleSkeletonSet reads the policy's mandatory tool set. Failure is not fatal
// — it only costs the skeleton/flesh split in the stats.
func (c *JITToolCompiler) mangleSkeletonSet() map[string]struct{} {
	results, err := c.kernel.Query("mcp_tool_skeleton(ToolID)")
	if err != nil {
		logging.Get(logging.CategoryTools).Debug("Failed to query mcp_tool_skeleton: %v", err)
		return nil
	}
	skeletons := make(map[string]struct{}, len(results))
	for _, r := range results {
		if toolID, _ := r["ToolID"].(string); toolID != "" {
			skeletons[toolID] = struct{}{}
		}
	}
	return skeletons
}

// fallbackSelect provides simple selection when Mangle is unavailable.
func (c *JITToolCompiler) fallbackSelect(tcc ToolCompilationContext, tools []*MCPTool, vectorScores map[string]float64) []SelectedTool {
	type scoredTool struct {
		tool       *MCPTool
		logicScore int
		vecScore   int
		finalScore int
	}

	var scored []scoredTool
	for _, tool := range tools {
		st := scoredTool{tool: tool}

		// Logic score from shard affinity
		if tool.ShardAffinities != nil {
			shardKey := tcc.ShardType
			if len(shardKey) > 0 && shardKey[0] == '/' {
				shardKey = shardKey[1:]
			}
			if score, ok := tool.ShardAffinities[shardKey]; ok {
				st.logicScore = score
			}
		}

		// Usage feedback, mirroring policy_mcp.mg section 50.5 so the two
		// selection paths cannot disagree about which tools have earned trust.
		st.logicScore += usageAdjustment(tool)
		if st.logicScore < 0 {
			st.logicScore = 0
		}

		// Vector score
		if score, ok := vectorScores[tool.ToolID]; ok {
			st.vecScore = int(score * 100)
		}

		// Combined score (70% logic, 30% vector)
		st.finalScore = (st.logicScore*7 + st.vecScore*3) / 10

		scored = append(scored, st)
	}

	// Sort by final score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].finalScore > scored[j].finalScore
	})

	// Assign render modes based on score
	var selected []SelectedTool
	for _, st := range scored {
		skeleton := isSkeletonTool(st.tool)

		var mode RenderMode
		switch {
		case skeleton:
			// Policy grants skeleton tools full render unconditionally.
			mode = RenderModeFull
		case st.finalScore >= c.config.FullThreshold:
			mode = RenderModeFull
		case st.finalScore >= c.config.CondensedThreshold:
			mode = RenderModeCondensed
		case st.finalScore >= c.config.MinimalThreshold:
			mode = RenderModeMinimal
		default:
			continue // Excluded
		}

		selected = append(selected, SelectedTool{
			ToolID:      st.tool.ToolID,
			RenderMode:  mode,
			LogicScore:  st.logicScore,
			VectorScore: st.vecScore,
			FinalScore:  st.finalScore,
			Skeleton:    skeleton,
		})
	}

	return selected
}

// usageAdjustment is the Go mirror of policy_mcp.mg 50.5: proven tools are
// promoted, unreliable and consistently slow tools are demoted. Tools with too
// little history (< minUsageSamples calls) are left alone so a single early
// failure cannot bury a tool forever.
func usageAdjustment(tool *MCPTool) int {
	const (
		minUsageSamples  = 3
		slowLatencyMs    = 5000
		successBoost     = 15
		unreliablePenalt = 20
		slowPenalty      = 10
	)

	adjustment := 0
	if tool.UsageCount >= minUsageSamples {
		rate := (tool.SuccessCount * 100) / tool.UsageCount
		switch {
		case rate >= 80:
			adjustment += successBoost
		case rate < 50:
			adjustment -= unreliablePenalt
		}
	}
	if tool.AvgLatencyMs >= slowLatencyMs {
		adjustment -= slowPenalty
	}
	return adjustment
}

// isSkeletonTool is the Go mirror of policy_mcp.mg 50.7. Keeping the two in
// sync matters for the stats split: SkeletonTools used to just count whatever
// landed in the full tier, which had nothing to do with the policy's notion of
// a mandatory tool.
func isSkeletonTool(tool *MCPTool) bool {
	if tool == nil {
		return false
	}
	hasCategory := func(want string) bool {
		for _, c := range tool.Categories {
			if mangleAtom(c) == want {
				return true
			}
		}
		return false
	}
	hasCapability := func(want string) bool {
		for _, c := range tool.Capabilities {
			if mangleAtom(c) == want {
				return true
			}
		}
		return false
	}

	if hasCategory("/filesystem") && hasCapability("/read") {
		return true
	}
	return hasCategory("/search") && hasCapability("/search")
}

// buildToolSet builds the compiled tool set from selected tools.
func (c *JITToolCompiler) buildToolSet(allTools []*MCPTool, selected []SelectedTool, stats *ToolCompilationStats) *CompiledToolSet {
	// Build tool ID to tool map
	toolMap := make(map[string]*MCPTool)
	for _, t := range allTools {
		if t != nil {
			toolMap[t.ToolID] = t // Last write wins if there are duplicates
		}
	}

	result := &CompiledToolSet{}

	for _, sel := range selected {
		tool, ok := toolMap[sel.ToolID]
		if !ok {
			continue
		}

		switch sel.RenderMode {
		case RenderModeFull:
			result.FullTools = append(result.FullTools, *tool)
		case RenderModeCondensed:
			result.CondensedTools = append(result.CondensedTools, ToolSummary{
				Name:      tool.Name,
				Condensed: tool.Condensed,
				ServerID:  tool.ServerID,
			})
		case RenderModeMinimal:
			result.MinimalTools = append(result.MinimalTools, tool.Name)
		default:
			continue
		}

		// Skeleton/flesh is a policy distinction (mandatory vs. contextual),
		// not a render tier. Counting full-render tools as skeleton made the
		// stat meaningless whenever a merely high-scoring tool got full render.
		if sel.Skeleton {
			stats.SkeletonTools++
		} else {
			stats.FleshTools++
		}
	}

	stats.SelectedTools = len(result.FullTools) + len(result.CondensedTools) + len(result.MinimalTools)
	return result
}

// fitBudget ensures the tool set fits within the token budget.
func (c *JITToolCompiler) fitBudget(result *CompiledToolSet, budget int, stats *ToolCompilationStats) {
	// Estimate tokens per tool type
	const (
		fullToolTokens      = 200 // Average tokens for full tool schema
		condensedToolTokens = 30  // Average tokens for condensed description
		minimalToolTokens   = 5   // Average tokens for name only
	)

	// Calculate current usage
	tokens := len(result.FullTools)*fullToolTokens +
		len(result.CondensedTools)*condensedToolTokens +
		len(result.MinimalTools)*minimalToolTokens

	// Limit full tools if over budget
	for tokens > budget && len(result.FullTools) > c.config.MaxFullTools {
		// Demote last full tool to condensed
		lastFull := result.FullTools[len(result.FullTools)-1]
		result.FullTools = result.FullTools[:len(result.FullTools)-1]
		result.CondensedTools = append(result.CondensedTools, ToolSummary{
			Name:      lastFull.Name,
			Condensed: lastFull.Condensed,
			ServerID:  lastFull.ServerID,
		})
		tokens = tokens - fullToolTokens + condensedToolTokens
	}

	// Limit condensed tools if still over budget
	for tokens > budget && len(result.CondensedTools) > c.config.MaxCondensedTools {
		// Demote last condensed to minimal
		lastCondensed := result.CondensedTools[len(result.CondensedTools)-1]
		result.CondensedTools = result.CondensedTools[:len(result.CondensedTools)-1]
		result.MinimalTools = append(result.MinimalTools, lastCondensed.Name)
		tokens = tokens - condensedToolTokens + minimalToolTokens
	}

	// Remove minimal tools if still over budget
	for tokens > budget && len(result.MinimalTools) > 0 {
		result.MinimalTools = result.MinimalTools[:len(result.MinimalTools)-1]
		tokens -= minimalToolTokens
	}

	stats.TokensUsed = tokens
}

// CompileForShard is a convenience method to compile tools for a specific shard.
func (c *JITToolCompiler) CompileForShard(ctx context.Context, shardType string, taskDescription string) (*CompiledToolSet, error) {
	return c.Compile(ctx, ToolCompilationContext{
		ShardType:       shardType,
		TaskDescription: taskDescription,
		TokenBudget:     c.config.TokenBudget,
	})
}
