package mcp

import "testing"

func TestFallbackSelectAssignsRenderModes(t *testing.T) {
	compiler := &JITToolCompiler{config: DefaultToolSelectionConfig()}

	tools := []*MCPTool{
		{
			ToolID:          "tool-full",
			Name:            "Full",
			ServerID:        "server",
			Condensed:       "full tool",
			ShardAffinities: map[string]int{"coder": 90},
		},
		{
			ToolID:          "tool-min",
			Name:            "Minimal",
			ServerID:        "server",
			Condensed:       "minimal tool",
			ShardAffinities: map[string]int{"coder": 30},
		},
		{
			ToolID:          "tool-none",
			Name:            "None",
			ServerID:        "server",
			Condensed:       "excluded tool",
			ShardAffinities: map[string]int{"coder": 5},
		},
	}

	vectorScores := map[string]float64{
		"tool-full": 1.0,
		"tool-min":  0.0,
	}

	selected := compiler.fallbackSelect(ToolCompilationContext{ShardType: "/coder"}, tools, vectorScores)
	if len(selected) != 2 {
		t.Fatalf("selected len = %d, want 2", len(selected))
	}

	modes := map[string]RenderMode{}
	for _, sel := range selected {
		modes[sel.ToolID] = sel.RenderMode
	}

	if modes["tool-full"] != RenderModeFull {
		t.Fatalf("tool-full mode = %s, want full", modes["tool-full"])
	}
	if modes["tool-min"] != RenderModeMinimal {
		t.Fatalf("tool-min mode = %s, want minimal", modes["tool-min"])
	}
	if _, ok := modes["tool-none"]; ok {
		t.Fatalf("tool-none should be excluded")
	}
}

func TestBuildToolSet(t *testing.T) {
	compiler := &JITToolCompiler{config: DefaultToolSelectionConfig()}

	tools := []*MCPTool{
		{ToolID: "t1", Name: "Tool1", Condensed: "c1", ServerID: "s1"},
		{ToolID: "t2", Name: "Tool2", Condensed: "c2", ServerID: "s1"},
		{ToolID: "t3", Name: "Tool3", Condensed: "c3", ServerID: "s1"},
	}
	selected := []SelectedTool{
		{ToolID: "t1", RenderMode: RenderModeFull},
		{ToolID: "t2", RenderMode: RenderModeCondensed},
		{ToolID: "t3", RenderMode: RenderModeMinimal},
	}

	stats := &ToolCompilationStats{}
	set := compiler.buildToolSet(tools, selected, stats)

	if len(set.FullTools) != 1 || len(set.CondensedTools) != 1 || len(set.MinimalTools) != 1 {
		t.Fatalf("unexpected tool set sizes: full=%d condensed=%d minimal=%d",
			len(set.FullTools), len(set.CondensedTools), len(set.MinimalTools))
	}
	if stats.SelectedTools != 3 {
		t.Fatalf("SelectedTools = %d, want 3", stats.SelectedTools)
	}
}

func TestFitBudgetDemotesTools(t *testing.T) {
	compiler := &JITToolCompiler{
		config: ToolSelectionConfig{
			MaxFullTools:      0,
			MaxCondensedTools: 0,
		},
	}

	result := &CompiledToolSet{
		FullTools: []MCPTool{
			{Name: "full1", Condensed: "f1", ServerID: "s"},
			{Name: "full2", Condensed: "f2", ServerID: "s"},
		},
		CondensedTools: []ToolSummary{
			{Name: "cond1", Condensed: "c1", ServerID: "s"},
			{Name: "cond2", Condensed: "c2", ServerID: "s"},
		},
		MinimalTools: []string{"min1", "min2"},
	}

	stats := &ToolCompilationStats{}
	compiler.fitBudget(result, 50, stats)

	if len(result.FullTools) != 0 {
		t.Fatalf("expected full tools to be demoted")
	}
	if len(result.CondensedTools) != 0 {
		t.Fatalf("expected condensed tools to be demoted")
	}
	if stats.TokensUsed > 50 {
		t.Fatalf("TokensUsed = %d, want <= 50", stats.TokensUsed)
	}
}
