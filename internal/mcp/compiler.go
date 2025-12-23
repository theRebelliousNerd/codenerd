package mcp

import (
	"context"
	"fmt"
	"sort"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// KernelInterface defines the interface for Mangle kernel operations.
type KernelInterface interface {
	Assert(fact string) error
	Retract(fact string) error
	Query(query string) ([]map[string]interface{}, error)
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

	if tcc.TokenBudget == 0 {
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

	// Phase 2: Assert vector scores to Mangle kernel
	if c.kernel != nil && len(vectorScores) > 0 {
		for toolID, score := range vectorScores {
			scoreInt := int(score * 100)
			if err := c.kernel.Assert(fmt.Sprintf("mcp_tool_vector_score(%q, %d)", toolID, scoreInt)); err != nil {
				logging.Get(logging.CategoryTools).Debug("Failed to assert vector score: %v", err)
			}
		}
	}

	// Phase 3: Query Mangle for tool selection (or use fallback)
	mangleStart := time.Now()
	selected := c.selectTools(ctx, tcc, allTools, vectorScores)
	stats.MangleQueryMs = time.Since(mangleStart).Milliseconds()

	// Phase 4: Build compiled tool set
	result := c.buildToolSet(allTools, selected, &stats)

	// Phase 5: Fit to budget
	c.fitBudget(result, tcc.TokenBudget, &stats)

	// Cleanup: Retract temporary vector scores
	if c.kernel != nil {
		for toolID := range vectorScores {
			_ = c.kernel.Retract(fmt.Sprintf("mcp_tool_vector_score(%q, _)", toolID))
		}
	}

	stats.Duration = time.Since(start)
	result.Stats = stats

	logging.Get(logging.CategoryTools).Info(
		"JIT Tool Compiler: %dms | tools=%d (full=%d, condensed=%d, minimal=%d) | vec=%dms | budget=%d/%d",
		stats.Duration.Milliseconds(),
		stats.SelectedTools,
		len(result.FullTools),
		len(result.CondensedTools),
		len(result.MinimalTools),
		stats.VectorQueryMs,
		stats.TokensUsed,
		stats.TokenBudget,
	)

	return result, nil
}

// vectorSearch performs semantic search over tool embeddings.
func (c *JITToolCompiler) vectorSearch(ctx context.Context, query string, tools []*MCPTool) (map[string]float64, error) {
	// Generate query embedding
	queryEmbed, err := c.embedder.Embed(ctx, query)
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
func (c *JITToolCompiler) selectTools(ctx context.Context, tcc ToolCompilationContext, tools []*MCPTool, vectorScores map[string]float64) []SelectedTool {
	// Try Mangle-based selection first
	if c.kernel != nil {
		selected, err := c.mangleSelect(ctx, tcc)
		if err == nil && len(selected) > 0 {
			return selected
		}
		logging.Get(logging.CategoryTools).Debug("Mangle selection failed, using fallback: %v", err)
	}

	// Fallback: Simple affinity-based selection
	return c.fallbackSelect(tcc, tools, vectorScores)
}

// mangleSelect uses Mangle kernel for tool selection.
func (c *JITToolCompiler) mangleSelect(ctx context.Context, tcc ToolCompilationContext) ([]SelectedTool, error) {
	// Query for selected tools
	query := fmt.Sprintf("mcp_tool_selected(%q, ToolID, RenderMode)", tcc.ShardType)
	results, err := c.kernel.Query(query)
	if err != nil {
		return nil, err
	}

	var selected []SelectedTool
	for _, r := range results {
		toolID, _ := r["ToolID"].(string)
		renderModeRaw, _ := r["RenderMode"].(string)

		var renderMode RenderMode
		switch renderModeRaw {
		case "/full", "full":
			renderMode = RenderModeFull
		case "/condensed", "condensed":
			renderMode = RenderModeCondensed
		case "/minimal", "minimal":
			renderMode = RenderModeMinimal
		default:
			renderMode = RenderModeCondensed
		}

		selected = append(selected, SelectedTool{
			ToolID:     toolID,
			RenderMode: renderMode,
		})
	}

	return selected, nil
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
		var mode RenderMode
		switch {
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
		})
	}

	return selected
}

// buildToolSet builds the compiled tool set from selected tools.
func (c *JITToolCompiler) buildToolSet(allTools []*MCPTool, selected []SelectedTool, stats *ToolCompilationStats) *CompiledToolSet {
	// Build tool ID to tool map
	toolMap := make(map[string]*MCPTool)
	for _, t := range allTools {
		toolMap[t.ToolID] = t
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
			stats.SkeletonTools++
		case RenderModeCondensed:
			result.CondensedTools = append(result.CondensedTools, ToolSummary{
				Name:      tool.Name,
				Condensed: tool.Condensed,
				ServerID:  tool.ServerID,
			})
			stats.FleshTools++
		case RenderModeMinimal:
			result.MinimalTools = append(result.MinimalTools, tool.Name)
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
