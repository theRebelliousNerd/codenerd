package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCollectMetrics_WhenToolsHaveHistory_ShouldReportRatesAndLatency(t *testing.T) {
	store, err := NewMCPToolStore("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	tool := &MCPTool{
		ToolID:       "fs/read_file",
		ServerID:     "fs",
		Name:         "read_file",
		Description:  "read",
		RegisteredAt: time.Unix(1700000000, 0),
	}
	if err := store.SaveTool(ctx, tool); err != nil {
		t.Fatalf("SaveTool: %v", err)
	}
	for i := range 4 {
		if err := store.RecordToolUsage(ctx, tool.ToolID, i != 0, 100); err != nil {
			t.Fatalf("RecordToolUsage: %v", err)
		}
	}

	metrics, err := CollectMetrics(ctx, store, nil)
	if err != nil {
		t.Fatalf("CollectMetrics: %v", err)
	}
	if len(metrics.Tools) != 1 {
		t.Fatalf("expected 1 tool metric, got %d", len(metrics.Tools))
	}
	got := metrics.Tools[0]
	if got.Calls != 4 || got.Successes != 3 || got.Failures != 1 {
		t.Errorf("counters = %d/%d/%d, want 4/3/1", got.Calls, got.Successes, got.Failures)
	}
	if got.SuccessRate != 0.75 {
		t.Errorf("success rate = %v, want 0.75", got.SuccessRate)
	}
	if got.AvgLatencyMs != 100 {
		t.Errorf("avg latency = %d, want 100", got.AvgLatencyMs)
	}
	if metrics.TotalCalls != 4 || metrics.TotalFailures != 1 {
		t.Errorf("totals = %d/%d, want 4/1", metrics.TotalCalls, metrics.TotalFailures)
	}
}

func TestRenderPrometheus_WhenSnapshotTaken_ShouldEmitScrapableText(t *testing.T) {
	metrics := &MCPMetrics{
		Servers: map[string]ServerStatus{"fs": ServerStatusConnected, "web": ServerStatusDisconnected},
		Tools: []ToolMetric{
			{ToolID: "fs/read_file", ServerID: "fs", Name: "read_file", Calls: 4, Successes: 3, Failures: 1, AvgLatencyMs: 100},
		},
	}

	out := metrics.RenderPrometheus()
	for _, want := range []string{
		`mcp_server_up{server="fs"} 1`,
		`mcp_server_up{server="web"} 0`,
		`mcp_tool_calls_total{server="fs",tool="read_file"} 4`,
		`mcp_tool_call_failures_total{server="fs",tool="read_file"} 1`,
		`mcp_tool_latency_ms_avg{server="fs",tool="read_file"} 100`,
		"# TYPE mcp_tool_calls_total counter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCollectMetrics_WhenStoreMissing_ShouldError(t *testing.T) {
	if _, err := CollectMetrics(context.Background(), nil, nil); err == nil {
		t.Error("expected an error when no store is configured")
	}
}
