package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ToolMetric is the per-tool call record the store already accumulates.
type ToolMetric struct {
	ToolID       string  `json:"tool_id"`
	ServerID     string  `json:"server_id"`
	Name         string  `json:"name"`
	Calls        int64   `json:"calls"`
	Successes    int64   `json:"successes"`
	Failures     int64   `json:"failures"`
	SuccessRate  float64 `json:"success_rate"` // 0-1; -1 when there is no history
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

// MCPMetrics is a point-in-time snapshot across all known MCP tools.
type MCPMetrics struct {
	Servers          map[string]ServerStatus `json:"servers"`
	Tools            []ToolMetric            `json:"tools"`
	TotalCalls       int64                   `json:"total_calls"`
	TotalFailures    int64                   `json:"total_failures"`
	ToolsWithHistory int                     `json:"tools_with_history"`
}

// CollectMetrics builds a metrics snapshot from persisted usage counters plus
// live connection state.
//
// Usage counters are already durable (they drive the kernel's success-rate
// boost), so exporting them needs no new bookkeeping — the gap was purely that
// nothing could read them back out. manager may be nil, in which case server
// status is omitted.
func CollectMetrics(ctx context.Context, store *MCPToolStore, manager *MCPClientManager) (*MCPMetrics, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required to collect MCP metrics")
	}

	tools, err := store.GetAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load tools: %w", err)
	}

	metrics := &MCPMetrics{
		Servers: make(map[string]ServerStatus),
		Tools:   make([]ToolMetric, 0, len(tools)),
	}

	if manager != nil {
		connected := make(map[string]struct{})
		for _, id := range manager.GetConnectedServers() {
			connected[id] = struct{}{}
		}
		if servers, err := store.GetAllServers(ctx); err == nil {
			for _, server := range servers {
				if _, ok := connected[server.ID]; ok {
					metrics.Servers[server.ID] = ServerStatusConnected
					continue
				}
				status := server.Status
				if status == "" || status == ServerStatusConnected {
					// The store's row can lag a crash; live state wins.
					status = ServerStatusDisconnected
				}
				metrics.Servers[server.ID] = status
			}
		}
		for id := range connected {
			metrics.Servers[id] = ServerStatusConnected
		}
	}

	for _, tool := range tools {
		if tool == nil {
			continue
		}
		metric := ToolMetric{
			ToolID:       tool.ToolID,
			ServerID:     tool.ServerID,
			Name:         tool.Name,
			Calls:        tool.UsageCount,
			Successes:    tool.SuccessCount,
			Failures:     tool.UsageCount - tool.SuccessCount,
			AvgLatencyMs: tool.AvgLatencyMs,
			SuccessRate:  -1,
		}
		if metric.Failures < 0 {
			metric.Failures = 0
		}
		if tool.UsageCount > 0 {
			metric.SuccessRate = float64(tool.SuccessCount) / float64(tool.UsageCount)
			metrics.ToolsWithHistory++
		}
		metrics.TotalCalls += metric.Calls
		metrics.TotalFailures += metric.Failures
		metrics.Tools = append(metrics.Tools, metric)
	}

	sort.Slice(metrics.Tools, func(i, j int) bool {
		return metrics.Tools[i].ToolID < metrics.Tools[j].ToolID
	})
	return metrics, nil
}

// RenderPrometheus renders the snapshot in Prometheus text exposition format so
// it can be scraped or diffed without a client library.
func (m *MCPMetrics) RenderPrometheus() string {
	if m == nil {
		return ""
	}
	var b strings.Builder

	b.WriteString("# HELP mcp_server_up Whether an MCP server is currently connected.\n")
	b.WriteString("# TYPE mcp_server_up gauge\n")
	serverIDs := make([]string, 0, len(m.Servers))
	for id := range m.Servers {
		serverIDs = append(serverIDs, id)
	}
	sort.Strings(serverIDs)
	for _, id := range serverIDs {
		up := 0
		if m.Servers[id] == ServerStatusConnected {
			up = 1
		}
		fmt.Fprintf(&b, "mcp_server_up{server=%q} %d\n", promLabel(id), up)
	}

	b.WriteString("# HELP mcp_tool_calls_total Total MCP tool invocations.\n")
	b.WriteString("# TYPE mcp_tool_calls_total counter\n")
	for _, t := range m.Tools {
		fmt.Fprintf(&b, "mcp_tool_calls_total{server=%q,tool=%q} %d\n", promLabel(t.ServerID), promLabel(t.Name), t.Calls)
	}

	b.WriteString("# HELP mcp_tool_call_failures_total MCP tool invocations that reported failure.\n")
	b.WriteString("# TYPE mcp_tool_call_failures_total counter\n")
	for _, t := range m.Tools {
		fmt.Fprintf(&b, "mcp_tool_call_failures_total{server=%q,tool=%q} %d\n", promLabel(t.ServerID), promLabel(t.Name), t.Failures)
	}

	b.WriteString("# HELP mcp_tool_latency_ms_avg Rolling average MCP tool latency in milliseconds.\n")
	b.WriteString("# TYPE mcp_tool_latency_ms_avg gauge\n")
	for _, t := range m.Tools {
		fmt.Fprintf(&b, "mcp_tool_latency_ms_avg{server=%q,tool=%q} %d\n", promLabel(t.ServerID), promLabel(t.Name), t.AvgLatencyMs)
	}

	return b.String()
}

// promLabel escapes a Prometheus label value.
func promLabel(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return replacer.Replace(v)
}
