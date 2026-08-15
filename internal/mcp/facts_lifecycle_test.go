package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

// countingAnalyzer records how many times a tool schema was analyzed, which is
// the observable that distinguishes "cache hit" from "re-analyzed".
type countingAnalyzer struct {
	mu    sync.Mutex
	calls int
}

func (a *countingAnalyzer) Analyze(ctx context.Context, schema MCPToolSchema) (*ToolAnalysis, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	return &ToolAnalysis{
		ToolID:          schema.Name,
		Categories:      []string{"filesystem"},
		Capabilities:    []string{"/read"},
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": 70},
		Condensed:       schema.Description,
	}, nil
}

func (a *countingAnalyzer) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

func newDiscoveryManager(t *testing.T, transport *mockTransport, analyzer ToolAnalyzerInterface) (*MCPClientManager, *MCPToolStore) {
	t.Helper()
	store, err := NewMCPToolStore("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	manager := NewMCPClientManager(store, analyzer, map[string]MCPServerConfig{})
	manager.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv", Status: ServerStatusConnected},
		Transport: transport,
	}
	return manager, store
}

func TestDiscoverTools_WhenSchemaUnchanged_ShouldReuseCachedAnalysis(t *testing.T) {
	transport := &mockTransport{
		connected: true,
		tools: []MCPToolSchema{{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}
	analyzer := &countingAnalyzer{}
	manager, _ := newDiscoveryManager(t, transport, analyzer)

	ctx := context.Background()
	for range 3 {
		if err := manager.DiscoverTools(ctx, "srv"); err != nil {
			t.Fatalf("DiscoverTools: %v", err)
		}
	}

	if got := analyzer.count(); got != 1 {
		t.Errorf("analyzer ran %d times for an unchanged schema, want 1", got)
	}
}

func TestDiscoverTools_WhenSchemaChanged_ShouldReanalyzeAndReplaceFacts(t *testing.T) {
	transport := &mockTransport{
		connected: true,
		tools: []MCPToolSchema{{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}
	analyzer := &countingAnalyzer{}
	manager, store := newDiscoveryManager(t, transport, analyzer)
	kernel := &recordingKernel{}
	manager.SetFactEmitter(NewFactEmitter(kernel))

	ctx := context.Background()
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}
	first, err := store.GetTool(ctx, "srv/read_file")
	if err != nil || first == nil {
		t.Fatalf("GetTool: %v (%v)", err, first)
	}

	// The server changes the tool's input schema without renaming it — the
	// exact case that used to leave stale analysis in place forever.
	transport.tools[0].InputSchema = json.RawMessage(`{"type":"object","properties":{"encoding":{"type":"string"}}}`)
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("DiscoverTools (2): %v", err)
	}

	if got := analyzer.count(); got != 2 {
		t.Errorf("analyzer ran %d times, want 2 (re-analysis on schema change)", got)
	}
	second, err := store.GetTool(ctx, "srv/read_file")
	if err != nil || second == nil {
		t.Fatalf("GetTool (2): %v (%v)", err, second)
	}
	if second.SchemaHash == first.SchemaHash {
		t.Error("schema hash did not change after the schema changed")
	}
	if !containsPrefix(kernel.retractedFacts(), `mcp_tool_registered("srv/read_file"`) {
		t.Errorf("stale tool facts were not retracted before re-analysis: %v", kernel.retractedFacts())
	}
}

func TestDiscoverTools_WhenToolDisappears_ShouldRetractItsFacts(t *testing.T) {
	transport := &mockTransport{
		connected: true,
		tools: []MCPToolSchema{
			{Name: "a", Description: "tool a", InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Description: "tool b", InputSchema: json.RawMessage(`{}`)},
		},
	}
	manager, _ := newDiscoveryManager(t, transport, &countingAnalyzer{})
	kernel := &recordingKernel{}
	manager.SetFactEmitter(NewFactEmitter(kernel))

	ctx := context.Background()
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("DiscoverTools: %v", err)
	}

	// Server stops advertising "b". Leaving its facts would keep the kernel
	// recommending a call that fails at the transport.
	transport.tools = transport.tools[:1]
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("DiscoverTools (2): %v", err)
	}

	if !containsPrefix(kernel.retractedFacts(), `mcp_tool_registered("srv/b"`) {
		t.Errorf("vanished tool facts were not retracted: %v", kernel.retractedFacts())
	}

	live := manager.factEmitter().EmittedFacts()
	if containsSubstring(live, `"srv/b"`) {
		t.Errorf("facts for the vanished tool are still live: %v", live)
	}
	if !containsSubstring(live, `"srv/a"`) {
		t.Errorf("facts for the still-advertised tool were dropped: %v", live)
	}
}

func TestFactEmitter_WhenStatusChanges_ShouldRetractPreviousStatusExactly(t *testing.T) {
	kernel := &recordingKernel{}
	emitter := NewFactEmitter(kernel)

	emitter.EmitServerStatus("srv", ServerStatusConnected)
	emitter.EmitServerStatus("srv", ServerStatusDisconnected)

	// The kernel adapter retracts by exact fact; a wildcard would be a no-op
	// and both statuses would coexist.
	want := `mcp_server_status("srv", /connected)`
	if !containsPrefix(kernel.retractedFacts(), want) {
		t.Errorf("expected an exact retraction of %q, got %v", want, kernel.retractedFacts())
	}
	if got := emitter.EmittedFactCount(); got != 1 {
		t.Errorf("emitter tracks %d status facts, want 1", got)
	}
}

func TestFactEmitter_WhenValueNeedsEscaping_ShouldProduceParseableFacts(t *testing.T) {
	tool := &MCPTool{
		ToolID:       "srv/weird",
		ServerID:     "srv",
		Name:         "weird",
		Description:  "quotes \" backslash \\ newline \n emoji ✅ " + strings.Repeat("x", 600),
		Categories:   []string{"Code Analysis", "3d-render"},
		Capabilities: []string{"/read"},
		Domain:       "/general",
		RegisteredAt: time.Unix(1700000000, 0),
	}

	for _, fact := range toolFacts(tool) {
		if _, err := mangle.ParseAtom(fact); err != nil {
			t.Errorf("emitted fact is not parseable: %s (%v)", fact, err)
		}
	}
}

func TestMangleAtom_WhenLabelIsMessy_ShouldProduceValidNameConstant(t *testing.T) {
	cases := map[string]string{
		"filesystem":    "/filesystem",
		"/read":         "/read",
		"Code Analysis": "/code_analysis",
		"3d-render":     "/n3d_render",
		"":              "/unknown",
		"   ":           "/unknown",
		"///":           "/unknown",
	}
	for in, want := range cases {
		if got := mangleAtom(in); got != want {
			t.Errorf("mangleAtom(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUsageAdjustment_WhenHistoryVaries_ShouldMatchPolicyThresholds(t *testing.T) {
	cases := []struct {
		name string
		tool *MCPTool
		want int
	}{
		{"no history", &MCPTool{}, 0},
		{"too few samples", &MCPTool{UsageCount: 2, SuccessCount: 2}, 0},
		{"proven", &MCPTool{UsageCount: 10, SuccessCount: 9}, 15},
		{"unreliable", &MCPTool{UsageCount: 10, SuccessCount: 2}, -20},
		{"slow but reliable", &MCPTool{UsageCount: 10, SuccessCount: 10, AvgLatencyMs: 6000}, 5},
		{"middling", &MCPTool{UsageCount: 10, SuccessCount: 6}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usageAdjustment(tc.tool); got != tc.want {
				t.Errorf("usageAdjustment = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFallbackSelect_WhenToolIsPolicySkeleton_ShouldForceFullRenderAndCount(t *testing.T) {
	c := NewJITToolCompiler(nil, nil, nil)
	tools := []*MCPTool{
		// Low affinity, but filesystem+read makes it mandatory in policy.
		{ToolID: "fs/read", Name: "read", Categories: []string{"filesystem"}, Capabilities: []string{"/read"},
			ShardAffinities: map[string]int{"coder": 10}},
		{ToolID: "fs/other", Name: "other", Categories: []string{"shell"}, Capabilities: []string{"/execute"},
			ShardAffinities: map[string]int{"coder": 100}},
	}
	selected := c.fallbackSelect(ToolCompilationContext{ShardType: "coder"}, tools, nil)

	modes := map[string]SelectedTool{}
	for _, s := range selected {
		modes[s.ToolID] = s
	}
	if modes["fs/read"].RenderMode != RenderModeFull || !modes["fs/read"].Skeleton {
		t.Errorf("skeleton tool = %+v, want full render and Skeleton=true", modes["fs/read"])
	}
	if modes["fs/other"].Skeleton {
		t.Errorf("non-skeleton tool marked skeleton: %+v", modes["fs/other"])
	}

	stats := &ToolCompilationStats{}
	c.buildToolSet(tools, selected, stats)
	if stats.SkeletonTools != 1 || stats.FleshTools != 1 {
		t.Errorf("skeleton/flesh split = %d/%d, want 1/1", stats.SkeletonTools, stats.FleshTools)
	}
}

func TestSelectTools_WhenKernelDerivesNothing_ShouldReportFallbackPath(t *testing.T) {
	c := NewJITToolCompiler(nil, nil, &recordingKernel{}) // Query returns no rows
	stats := &ToolCompilationStats{}
	tools := []*MCPTool{{ToolID: "t", ShardAffinities: map[string]int{"coder": 100}}}

	c.selectTools(context.Background(), ToolCompilationContext{ShardType: "coder"}, tools, nil, stats)
	if stats.SelectionPath != SelectionPathFallback {
		t.Errorf("SelectionPath = %q, want %q", stats.SelectionPath, SelectionPathFallback)
	}

	c2 := NewJITToolCompiler(nil, nil, &mockKernelWithMangle{results: []map[string]any{
		{"ToolID": "t", "RenderMode": "/full"},
	}})
	stats2 := &ToolCompilationStats{}
	c2.selectTools(context.Background(), ToolCompilationContext{ShardType: "coder"}, tools, nil, stats2)
	if stats2.SelectionPath != SelectionPathMangle {
		t.Errorf("SelectionPath = %q, want %q", stats2.SelectionPath, SelectionPathMangle)
	}
}

func TestMangleSelect_WhenShardTypeGiven_ShouldQueryWithNameConstant(t *testing.T) {
	spy := &queryRecordingKernel{}
	c := NewJITToolCompiler(nil, nil, spy)

	if _, err := c.mangleSelect(context.Background(), ToolCompilationContext{ShardType: "coder"}); err != nil {
		t.Fatalf("mangleSelect: %v", err)
	}

	// ShardType is declared /name. Quoting it as a string produced a pattern
	// that never matched, so the kernel path silently never fired.
	want := `mcp_tool_selected(/coder, ToolID, RenderMode)`
	if len(spy.queries) == 0 || spy.queries[0] != want {
		t.Errorf("query = %q, want %q", spy.queries, want)
	}
}

// stubEmbedder returns a fixed vector so SemanticSearch produces deterministic
// scores without a model.
type stubEmbedder struct{}

func (stubEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{1, 0, 0, 0}, nil
}

func (stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0, 0, 0}
	}
	return out, nil
}

func (stubEmbedder) Dimensions() int { return 4 }
func (stubEmbedder) Name() string    { return "stub" }

func TestCompile_WhenVectorScoresAsserted_ShouldRetractThemExactly(t *testing.T) {
	store, err := NewMCPToolStore("file::memory:?cache=shared", stubEmbedder{})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	tool := &MCPTool{
		ToolID:          "srv/search",
		ServerID:        "srv",
		Name:            "search",
		Description:     "search things",
		Condensed:       "search things",
		Categories:      []string{"search"},
		Capabilities:    []string{"/search"},
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": 90},
		Embedding:       []float32{1, 0, 0, 0},
		RegisteredAt:    time.Unix(1700000000, 0),
		AnalyzedAt:      time.Unix(1700000000, 0),
	}
	if err := store.SaveTool(ctx, tool); err != nil {
		t.Fatalf("SaveTool: %v", err)
	}

	kernel := &recordingKernel{}
	compiler := NewJITToolCompiler(store, stubEmbedder{}, kernel)
	if _, err := compiler.Compile(ctx, ToolCompilationContext{
		ShardType:       "coder",
		TaskDescription: "find a file",
		TokenBudget:     4000,
	}); err != nil {
		t.Fatalf("Compile: %v", err)
	}

	var asserted []string
	for _, f := range kernel.assertedFacts() {
		if strings.HasPrefix(f, "mcp_tool_vector_score(") {
			asserted = append(asserted, f)
		}
	}
	if len(asserted) == 0 {
		t.Fatal("expected at least one vector score assertion")
	}

	// Every asserted score must be retracted with the identical literal: the
	// kernel adapter retracts by exact fact, so the previous "_" wildcard form
	// retracted nothing and leaked stale similarity into the next compile.
	retracted := map[string]bool{}
	for _, f := range kernel.retractedFacts() {
		retracted[f] = true
		if strings.Contains(f, ", _)") {
			t.Errorf("wildcard retraction is a no-op against the kernel adapter: %q", f)
		}
	}
	for _, f := range asserted {
		if !retracted[f] {
			t.Errorf("asserted %q was never retracted; retractions: %v", f, kernel.retractedFacts())
		}
	}
}

func containsSubstring(values []string, want string) bool {
	for _, v := range values {
		if strings.Contains(v, want) {
			return true
		}
	}
	return false
}

type queryRecordingKernel struct {
	queries []string
}

func (k *queryRecordingKernel) Assert(string) error  { return nil }
func (k *queryRecordingKernel) Retract(string) error { return nil }
func (k *queryRecordingKernel) Query(q string) ([]map[string]any, error) {
	k.queries = append(k.queries, q)
	return nil, nil
}
