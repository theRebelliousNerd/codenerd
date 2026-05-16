package mcp

import (
	"context"
	"testing"
)

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

// -----------------------------------------------------------------------------
// Marathon 16: MCP Compiler Gap Implementations
// -----------------------------------------------------------------------------

func TestBuildToolSet_NilPointers(t *testing.T) {
	compiler := &JITToolCompiler{config: DefaultToolSelectionConfig()}

	// Gap 1: allTools contains nil pointers
	tools := []*MCPTool{
		{ToolID: "t1", Name: "Tool1", Condensed: "c1", ServerID: "s1"},
		nil,
		{ToolID: "t2", Name: "Tool2", Condensed: "c2", ServerID: "s1"},
		nil,
	}
	selected := []SelectedTool{
		{ToolID: "t1", RenderMode: RenderModeFull},
		{ToolID: "t2", RenderMode: RenderModeMinimal},
	}

	stats := &ToolCompilationStats{}
	set := compiler.buildToolSet(tools, selected, stats)

	if len(set.FullTools) != 1 || len(set.MinimalTools) != 1 {
		t.Fatalf("unexpected tool set sizes: full=%d minimal=%d", len(set.FullTools), len(set.MinimalTools))
	}
}

func TestBuildToolSet_StateConflicts_DuplicateID(t *testing.T) {
	compiler := &JITToolCompiler{config: DefaultToolSelectionConfig()}

	// Gap 2: Duplicate ToolIDs (last-write-wins)
	tools := []*MCPTool{
		{ToolID: "t1", Name: "Tool1-First", Condensed: "c1", ServerID: "s1"},
		{ToolID: "t1", Name: "Tool1-Last", Condensed: "c2", ServerID: "s1"},
	}
	selected := []SelectedTool{
		{ToolID: "t1", RenderMode: RenderModeFull},
	}

	stats := &ToolCompilationStats{}
	set := compiler.buildToolSet(tools, selected, stats)

	if len(set.FullTools) != 1 {
		t.Fatalf("expected 1 full tool")
	}
	if set.FullTools[0].Name != "Tool1-Last" {
		t.Fatalf("Expected last-write-wins (Tool1-Last), got %s", set.FullTools[0].Name)
	}
}

type mockKernelWithMangle struct {
	results []map[string]interface{}
}

func (m *mockKernelWithMangle) Assert(fact string) error { return nil }
func (m *mockKernelWithMangle) Retract(fact string) error { return nil }
func (m *mockKernelWithMangle) Query(query string) ([]map[string]interface{}, error) {
	return m.results, nil
}

func TestMangleSelect_TypeCoercion_CaseInsensitive(t *testing.T) {
	compiler := &JITToolCompiler{
		kernel: &mockKernelWithMangle{
			results: []map[string]interface{}{
				{"ToolID": "t1", "RenderMode": "FULL"},
				{"ToolID": "t2", "RenderMode": "/CONDENSED"},
			},
		},
	}

	// Gap 3: Case insensitive render modes
	selected, err := compiler.mangleSelect(context.Background(), ToolCompilationContext{ShardType: "coder"})
	if err != nil {
		t.Fatalf("mangleSelect failed: %v", err)
	}

	if len(selected) != 2 {
		t.Fatalf("expected 2 selected tools")
	}

	if selected[0].RenderMode != RenderModeFull {
		t.Errorf("expected FULL to be parsed as RenderModeFull, got %v", selected[0].RenderMode)
	}
	if selected[1].RenderMode != RenderModeCondensed {
		t.Errorf("expected /CONDENSED to be parsed as RenderModeCondensed, got %v", selected[1].RenderMode)
	}
}

type mockEmptyStore struct{}
func (m *mockEmptyStore) GetAllTools(ctx context.Context) ([]*MCPTool, error) {
	return []*MCPTool{{ToolID: "t1"}}, nil
}
func (m *mockEmptyStore) SemanticSearch(ctx context.Context, embed []float32, limit int) ([]ToolSearchResult, error) {
	return nil, nil
}

func TestCompile_UserRequestExtremes_NegativeBudget(t *testing.T) {
	store, err := NewMCPToolStore("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()
	
	// Create a client for the store
	// For testing, just inject the tools
	
	compiler := &JITToolCompiler{
		config: ToolSelectionConfig{TokenBudget: 500},
		store:  store,
	}

	// Gap 4: Negative TokenBudget should be coerced to config.TokenBudget
	tcc := ToolCompilationContext{
		TokenBudget: -1000,
	}

	// We don't have tools in store, but the stats should reflect the coerced budget.
	// We'll just verify that the context was coerced in the logic.
	// A trick is to call Compile with empty store and check stats.TokenBudget.
	
	set, err := compiler.Compile(context.Background(), tcc)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if set.Stats.TokenBudget != 500 {
		t.Errorf("Expected token budget to be coerced to 500, got %d", set.Stats.TokenBudget)
	}
}

func BenchmarkFitBudget_10000Tools(b *testing.B) {
	compiler := &JITToolCompiler{
		config: ToolSelectionConfig{
			MaxFullTools:      10,
			MaxCondensedTools: 20,
		},
	}

	// Gap 5: Benchmark performance
	for i := 0; i < b.N; i++ {
		result := &CompiledToolSet{}
		for j := 0; j < 10000; j++ {
			result.FullTools = append(result.FullTools, MCPTool{})
		}
		stats := &ToolCompilationStats{}
		compiler.fitBudget(result, 500, stats)
	}
}
