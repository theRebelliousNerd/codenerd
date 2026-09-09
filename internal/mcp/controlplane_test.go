package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fatMCPServer is a deliberately large MCP server: enough tools, with realistic
// schemas, that the difference between "render the catalog" and "render an
// atlas" is a measurement rather than an opinion.
type fatMCPServer struct {
	*httptest.Server

	mu        sync.Mutex
	callCount int
	listCount int
	rows      int
}

func newFatMCPServer(t *testing.T, rows int) *fatMCPServer {
	t.Helper()
	f := &fatMCPServer{rows: rows}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Close)
	return f
}

// fatToolSchemas builds a catalog shaped like a real integration server:
// paired read/write/search verbs over several nouns, each with a handful of
// documented parameters.
func fatToolSchemas() []map[string]any {
	nouns := []string{"issue", "repo", "branch", "user", "comment", "release", "label", "milestone"}
	var tools []map[string]any
	for _, noun := range nouns {
		tools = append(tools,
			map[string]any{
				"name":        "get_" + noun,
				"description": "Fetch a single " + noun + " by its identifier, including all of its metadata fields and current state.",
				"inputSchema": map[string]any{
					"type":     "object",
					"required": []string{"id"},
					"properties": map[string]any{
						"id":       map[string]any{"type": "string", "description": "The " + noun + " identifier"},
						"expand":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Related fields to include"},
						"as_of":    map[string]any{"type": "string", "description": "ISO-8601 timestamp for a historical read"},
						"redacted": map[string]any{"type": "boolean", "description": "Omit fields the caller cannot see"},
					},
				},
				"annotations": map[string]any{"readOnlyHint": true},
			},
			map[string]any{
				"name":        "search_" + noun + "s",
				"description": "Search " + noun + "s with a query string, optional filters, sorting and pagination.",
				"inputSchema": map[string]any{
					"type":     "object",
					"required": []string{"query"},
					"properties": map[string]any{
						"query":  map[string]any{"type": "string", "description": "Search expression"},
						"limit":  map[string]any{"type": "integer", "description": "Maximum results"},
						"cursor": map[string]any{"type": "string", "description": "Pagination cursor"},
						"sort":   map[string]any{"type": "string", "enum": []any{"created", "updated", "relevance"}},
					},
				},
				"annotations": map[string]any{"readOnlyHint": true},
			},
			map[string]any{
				"name":        "update_" + noun,
				"description": "Update the mutable fields of an existing " + noun + ".",
				"inputSchema": map[string]any{
					"type":     "object",
					"required": []string{"id", "fields"},
					"properties": map[string]any{
						"id":     map[string]any{"type": "string", "description": "The " + noun + " identifier"},
						"fields": map[string]any{"type": "object", "description": "Field names to new values"},
					},
				},
			},
		)
	}
	tools = append(tools,
		map[string]any{
			"name":        "delete_repo",
			"description": "Permanently delete a repository and everything in it.",
			"inputSchema": map[string]any{
				"type": "object", "required": []string{"id"},
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
			},
		},
		map[string]any{
			"name":        "run_workflow",
			"description": "Run a workflow with a caller-supplied script body.",
			"inputSchema": map[string]any{
				"type": "object", "required": []string{"script"},
				"properties": map[string]any{"script": map[string]any{"type": "string"}},
			},
		},
	)
	return tools
}

func (f *fatMCPServer) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	write := func(result any) {
		payload, _ := json.Marshal(result)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID, "result": json.RawMessage(payload),
		})
	}

	switch req.Method {
	case "initialize":
		write(map[string]any{
			"capabilities": map[string]bool{"tools": true, "resources": true, "prompts": true},
			"serverInfo":   map[string]string{"name": "fat", "version": "1.0"},
		})
	case "tools/list":
		f.mu.Lock()
		f.listCount++
		f.mu.Unlock()
		write(map[string]any{"tools": fatToolSchemas()})
	case "tools/call":
		f.mu.Lock()
		f.callCount++
		rows := f.rows
		f.mu.Unlock()
		items := make([]map[string]any, 0, rows)
		for i := 0; i < rows; i++ {
			items = append(items, map[string]any{
				"id":     fmt.Sprintf("ISSUE-%04d", i),
				"title":  fmt.Sprintf("Something went wrong in subsystem %d", i),
				"body":   strings.Repeat("detail ", 40),
				"status": "open",
			})
		}
		write(map[string]any{"items": items, "next_cursor": "page2"})
	case "resources/list":
		write(map[string]any{"resources": []map[string]any{
			{"uri": "docs://query-dialect", "name": "Query dialect", "description": "How to write search queries", "mimeType": "text/markdown"},
			{"uri": "docs://webhooks", "name": "Webhooks", "description": "Event payload reference", "mimeType": "text/markdown"},
		}})
	case "resources/read":
		write(map[string]any{"contents": []map[string]any{
			{"uri": "docs://query-dialect", "mimeType": "text/markdown", "text": strings.Repeat("query dialect reference. ", 200)},
		}})
	case "prompts/list":
		write(map[string]any{"prompts": []map[string]any{
			{"name": "triage", "description": "Triage an issue", "arguments": []map[string]any{
				{"name": "id", "required": true},
			}},
		}})
	case "ping":
		write(map[string]any{})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": -32601, "message": "method not found"},
		})
	}
}

func (f *fatMCPServer) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callCount
}

// newTestControlPlane connects a plane to a fat server with a per-test store.
func newTestControlPlane(t *testing.T, rows int) (*ControlPlane, *fatMCPServer) {
	t.Helper()
	server := newFatMCPServer(t, rows)

	store, err := NewMCPToolStore(filepath.Join(t.TempDir(), "mcp.db"), nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	manager := NewMCPClientManager(store, NewToolAnalyzer(nil, nil), map[string]MCPServerConfig{
		"fat": {
			ID: "fat", Enabled: true, Protocol: "http", BaseURL: server.URL,
			Timeout: "10s", AutoConnect: true, AutoDiscoverTools: true,
		},
	})

	ctx := context.Background()
	if err := manager.ConnectAll(ctx); err != nil {
		t.Fatalf("ConnectAll: %v", err)
	}
	if err := manager.WaitForDiscovery(ctx); err != nil {
		t.Fatalf("WaitForDiscovery: %v", err)
	}

	return NewControlPlane(manager, store, nil), server
}

// naiveCatalogBytes is the baseline: what a client that injects every
// advertised tool's name, description and input schema would spend.
func naiveCatalogBytes(t *testing.T) int {
	t.Helper()
	raw, err := json.Marshal(fatToolSchemas())
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	return len(raw)
}

func TestControlPlane_AtlasShouldCostFarLessThanTheRawCatalog(t *testing.T) {
	t.Parallel()

	plane, _ := newTestControlPlane(t, 5)
	atlas := plane.Atlas(context.Background(), AtlasOptions{View: ViewCompact})

	if atlas.TotalTools != len(fatToolSchemas()) {
		t.Fatalf("atlas covers %d tools, server advertises %d", atlas.TotalTools, len(fatToolSchemas()))
	}

	rendered, err := json.Marshal(atlas)
	if err != nil {
		t.Fatalf("marshal atlas: %v", err)
	}
	baseline := naiveCatalogBytes(t)
	ratio := float64(baseline) / float64(len(rendered))

	t.Logf("catalog dump = %d bytes; compact atlas = %d bytes; %.1fx reduction over %d tools",
		baseline, len(rendered), ratio, atlas.TotalTools)

	// The whole premise is that the standing per-turn cost stops scaling with
	// the catalog. A modest floor is asserted rather than the measured figure,
	// so the test pins the property and does not break every time a tool
	// description is reworded.
	if ratio < 8 {
		t.Errorf("atlas is only %.1fx smaller than the raw catalog; the standing cost has not actually moved", ratio)
	}

	summary := plane.Atlas(context.Background(), AtlasOptions{View: ViewSummary})
	summaryBytes, _ := json.Marshal(summary)
	if len(summaryBytes) >= len(rendered) {
		t.Errorf("summary atlas (%d) is not smaller than compact (%d)", len(summaryBytes), len(rendered))
	}
}

func TestControlPlane_AtlasShouldGroupIntoFacetsWithRisk(t *testing.T) {
	t.Parallel()

	plane, _ := newTestControlPlane(t, 5)
	atlas := plane.Atlas(context.Background(), AtlasOptions{View: ViewCompact})

	if len(atlas.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(atlas.Servers))
	}
	byFacet := map[Facet]FacetCensus{}
	for _, row := range atlas.Servers[0].Facets {
		byFacet[row.Facet] = row
	}

	// The server declared readOnlyHint on its get_* and search_* tools, so
	// those facets must come back safe rather than merely guessed.
	if row, ok := byFacet[FacetRead]; !ok || row.Risk != RiskSafe {
		t.Errorf("read facet = %+v; want present and safe", row)
	}
	if row, ok := byFacet[FacetSearch]; !ok || row.Count != 8 {
		t.Errorf("search facet = %+v; want 8 tools", row)
	}
	// delete_repo is in /write and drags the facet's reported risk up.
	if row, ok := byFacet[FacetWrite]; !ok || row.Risk != RiskDestructive {
		t.Errorf("write facet = %+v; want present and destructive", row)
	}
	if row, ok := byFacet[FacetExecute]; !ok || row.Risk != RiskArbitrary {
		t.Errorf("execute facet = %+v; want present and arbitrary", row)
	}
}

func TestControlPlane_ProbeShouldDiscloseSchemaOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	plane, _ := newTestControlPlane(t, 5)
	ctx := context.Background()

	browse := plane.Probe(ctx, ProbeOptions{Facet: FacetSearch, View: ViewCompact})
	if len(browse.Tools) == 0 {
		t.Fatal("probe returned no tools")
	}
	for _, tool := range browse.Tools {
		if tool.Schema != "" {
			t.Errorf("%s carried a full schema while browsing; that is the cost the ladder exists to defer", tool.ToolID)
		}
		if tool.Signature == "" {
			t.Errorf("%s has no signature; browsing must still be actionable", tool.ToolID)
		}
	}

	one := plane.Probe(ctx, ProbeOptions{Tool: "get_issue"})
	if len(one.Tools) != 1 {
		t.Fatalf("naming a tool returned %d results, want 1", len(one.Tools))
	}
	if one.Tools[0].Schema == "" {
		t.Error("naming a tool must disclose its full schema; that is the point of the rung")
	}
	if !strings.Contains(one.Tools[0].Signature, "id: str") {
		t.Errorf("signature = %q; want the required argument named and typed", one.Tools[0].Signature)
	}
}

func TestControlPlane_ProbeShouldRejectAmbiguousBareNameRatherThanGuess(t *testing.T) {
	t.Parallel()

	tools := []*MCPTool{
		{ToolID: "a/search", Name: "search", ServerID: "a"},
		{ToolID: "b/search", Name: "search", ServerID: "b"},
	}
	if _, err := resolveTool(tools, "search", ""); err == nil {
		t.Fatal("an ambiguous bare name resolved; a call could land on the wrong server")
	}
	if got, err := resolveTool(tools, "search", "b"); err != nil || got.ToolID != "b/search" {
		t.Errorf("server-qualified resolve = %v, %v", got, err)
	}
}

func TestControlPlane_CallShouldShapeLargeResultAndRetainTheRest(t *testing.T) {
	t.Parallel()

	plane, server := newTestControlPlane(t, 200)
	ctx := context.Background()

	result := plane.Call(ctx, CallOptions{
		Tool: "search_issues", Args: map[string]any{"query": "bug"}, View: ViewCompact,
	})
	if !result.Success {
		t.Fatalf("call failed: %s", result.Error)
	}
	if !result.Truncated {
		t.Fatal("a 200-row result was not truncated at compact")
	}
	if result.Bytes >= result.FullBytes {
		t.Errorf("shaped %d bytes from a %d-byte payload; nothing was saved", result.Bytes, result.FullBytes)
	}
	if result.Shape == "" {
		t.Error("no shape sketch; the caller cannot tell what was withheld")
	}
	if result.Handle == "" {
		t.Fatal("no handle minted for a truncated result")
	}
	t.Logf("payload %d bytes -> %d bytes shaped (%.0fx), shape=%s",
		result.FullBytes, result.Bytes, float64(result.FullBytes)/float64(result.Bytes), result.Shape)

	callsBefore := server.calls()
	expanded := plane.Expand(ctx, ExpandOptions{
		Handle: result.Handle, Pointer: "/items/150", View: ViewFull,
	})
	if !expanded.Success {
		t.Fatalf("expand failed: %s", expanded.Error)
	}
	// This is the load-bearing assertion of the whole handle design. The call
	// behind a handle may have had side effects, so expanding must never be
	// able to fire them again.
	if got := server.calls(); got != callsBefore {
		t.Errorf("expand re-invoked the server (%d -> %d calls); a handle must be retained, not recomputed",
			callsBefore, got)
	}
	body, _ := json.Marshal(expanded.Data)
	if !strings.Contains(string(body), "ISSUE-0150") {
		t.Errorf("expanded slice = %s; want row 150", body)
	}
}

func TestControlPlane_CallShouldRefuseBadArgumentsWithTheSchemaAndNotDispatch(t *testing.T) {
	t.Parallel()

	plane, server := newTestControlPlane(t, 5)

	result := plane.Call(context.Background(), CallOptions{
		Tool: "get_issue", Args: map[string]any{"expand": "not-an-array"},
	})
	if result.Success {
		t.Fatal("a call missing a required argument succeeded")
	}
	if server.calls() != 0 {
		t.Errorf("server was called %d times for an invalid call; validation must happen before dispatch", server.calls())
	}
	if result.Schema == "" {
		t.Error("no schema returned; the one moment the caller has proven it needs the schema is the moment it got an argument wrong")
	}
	if !strings.Contains(result.Error, "missing required") {
		t.Errorf("error = %q; want the missing argument named", result.Error)
	}
}

func TestControlPlane_CallShouldGateDestructiveToolsUntilConfirmed(t *testing.T) {
	t.Parallel()

	plane, server := newTestControlPlane(t, 5)
	ctx := context.Background()

	refused := plane.Call(ctx, CallOptions{Tool: "delete_repo", Args: map[string]any{"id": "x"}})
	if refused.Success {
		t.Fatal("a destructive tool ran without confirmation")
	}
	if server.calls() != 0 {
		t.Errorf("server was called %d times despite the gate", server.calls())
	}
	if refused.Risk != RiskDestructive {
		t.Errorf("risk = %q, want %q", refused.Risk, RiskDestructive)
	}

	confirmed := plane.Call(ctx, CallOptions{
		Tool: "delete_repo", Args: map[string]any{"id": "x"}, ConfirmRisk: true,
	})
	if !confirmed.Success {
		t.Fatalf("confirmed destructive call failed: %s", confirmed.Error)
	}
	if server.calls() != 1 {
		t.Errorf("server calls = %d, want 1 after confirmation", server.calls())
	}
}

func TestControlPlane_CallShouldGateArbitraryExecution(t *testing.T) {
	t.Parallel()

	plane, server := newTestControlPlane(t, 5)
	result := plane.Call(context.Background(), CallOptions{
		Tool: "run_workflow", Args: map[string]any{"script": "rm -rf /"},
	})
	if result.Success {
		t.Fatal("an arbitrary-execution tool ran without confirmation")
	}
	if result.Risk != RiskArbitrary {
		t.Errorf("risk = %q, want %q", result.Risk, RiskArbitrary)
	}
	if server.calls() != 0 {
		t.Errorf("server was called %d times despite the gate", server.calls())
	}
}

func TestControlPlane_ContextShouldRankAndBoundServerResources(t *testing.T) {
	t.Parallel()

	plane, _ := newTestControlPlane(t, 5)
	ctx := context.Background()

	result := plane.Context(ctx, ContextOptions{Query: "query dialect search", View: ViewCompact})
	if !result.Success {
		t.Fatal("context lookup failed")
	}
	if len(result.Resources) == 0 {
		t.Fatal("no resources returned; the non-tool half of the server is invisible")
	}
	if result.Resources[0].URI != "docs://query-dialect" {
		t.Errorf("top resource = %q; ranking did not put the matching document first", result.Resources[0].URI)
	}

	read := plane.Context(ctx, ContextOptions{Query: "query dialect", View: ViewCompact, Read: true})
	top := read.Resources[0]
	if top.Excerpt == "" {
		t.Fatal("read=true returned no excerpt")
	}
	budget := BudgetFor(ViewCompact)
	if len(top.Excerpt) > budget.MaxStringBytes+8 {
		t.Errorf("excerpt is %d bytes, over the %d budget; JIT context must be bounded like everything else",
			len(top.Excerpt), budget.MaxStringBytes)
	}
	if top.Bytes <= len(top.Excerpt) {
		t.Errorf("full size %d not larger than excerpt %d; the caller cannot tell it was clipped", top.Bytes, len(top.Excerpt))
	}
	if top.Handle == "" {
		t.Error("a clipped resource must carry a handle so the rest is reachable")
	}
}

func TestControlPlane_WhenNoServers_ShouldReportHonestlyRatherThanFail(t *testing.T) {
	t.Parallel()

	plane := NewControlPlane(nil, nil, nil)
	atlas := plane.Atlas(context.Background(), AtlasOptions{})
	if !atlas.Success {
		t.Error("an empty atlas should succeed; nothing configured is an answer, not a failure")
	}
	if !strings.Contains(atlas.Summary, "no MCP servers") {
		t.Errorf("summary = %q; want it to say plainly that nothing is configured", atlas.Summary)
	}
}
