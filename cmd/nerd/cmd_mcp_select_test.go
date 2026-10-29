package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/mcp"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	runErr := fn()
	_ = w.Close()
	os.Stdout = saved
	return <-done, runErr
}

func seedMCPCatalog(t *testing.T, dir string) {
	t.Helper()
	store, err := mcp.NewMCPToolStore(filepath.Join(dir, ".nerd", "mcp_tools.db"), nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.SaveServer(ctx, &mcp.MCPServer{
		ID:           "fs",
		Name:         "Filesystem MCP",
		Endpoint:     "http://localhost:9000",
		Protocol:     mcp.ProtocolHTTP,
		Status:       mcp.ServerStatusConnected,
		DiscoveredAt: time.Unix(1700000000, 0),
	}); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	if err := store.SaveTool(ctx, &mcp.MCPTool{
		ToolID:          "fs/read_file",
		ServerID:        "fs",
		Name:            "read_file",
		Description:     "Read a file from disk",
		Condensed:       "Read a file",
		Categories:      []string{"filesystem"},
		Capabilities:    []string{"/read"},
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": 80},
		RegisteredAt:    time.Unix(1700000000, 0),
		AnalyzedAt:      time.Unix(1700000000, 0),
	}); err != nil {
		t.Fatalf("SaveTool: %v", err)
	}
}

func TestMCPSelect_WhenCatalogPersisted_ShouldDecideInTheKernel(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)
	seedMCPCatalog(t, dir)

	savedShard, savedVerb, savedTarget := mcpSelectShard, mcpSelectVerb, mcpSelectTarget
	t.Cleanup(func() { mcpSelectShard, mcpSelectVerb, mcpSelectTarget = savedShard, savedVerb, savedTarget })
	mcpSelectShard, mcpSelectVerb, mcpSelectTarget = "coder", "read", ""
	mcpSelectBudget = 4000

	out, err := captureStdout(t, func() error { return runMCPSelect(nil, nil) })
	if err != nil {
		t.Fatalf("runMCPSelect: %v", err)
	}

	// The point of the command: the decision must come from the kernel, not
	// from the Go fallback. That is only true once policy_mcp.mg is part of the
	// embedded constitution and the catalog is replayed as EDB facts.
	if !strings.Contains(out, "Mangle kernel") {
		t.Errorf("expected a kernel-decided selection, got:\n%s", out)
	}
	if !strings.Contains(out, "read_file") {
		t.Errorf("expected the seeded tool in the output, got:\n%s", out)
	}
	if !strings.Contains(out, "skeleton=1") {
		t.Errorf("filesystem+read tool should count as a policy skeleton tool, got:\n%s", out)
	}
}

func TestMCPSelect_WhenCatalogEmpty_ShouldExplainRatherThanFail(t *testing.T) {
	withWorkspace(t, t.TempDir())
	mcpSelectShard = "coder"

	out, err := captureStdout(t, func() error { return runMCPSelect(nil, nil) })
	if err != nil {
		t.Fatalf("an empty catalog must not be an error: %v", err)
	}
	if !strings.Contains(out, "No MCP tools in the catalog") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestMCPMetrics_WhenToolsRecorded_ShouldRenderPrometheusOnDemand(t *testing.T) {
	dir := t.TempDir()
	withWorkspace(t, dir)
	seedMCPCatalog(t, dir)

	saved := mcpMetricsPrometheus
	t.Cleanup(func() { mcpMetricsPrometheus = saved })
	mcpMetricsPrometheus = true

	out, err := captureStdout(t, func() error { return runMCPMetrics(nil, nil) })
	if err != nil {
		t.Fatalf("runMCPMetrics: %v", err)
	}
	if !strings.Contains(out, `mcp_tool_calls_total{server="fs",tool="read_file"}`) {
		t.Errorf("expected a scrapable counter, got:\n%s", out)
	}
}
