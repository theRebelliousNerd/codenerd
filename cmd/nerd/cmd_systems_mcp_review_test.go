package main

import (
	"testing"

	"codenerd/internal/mcp"
)

// A matching configured review is shown as configured, not as live
// admission: `nerd mcp tools` reads the recorded catalog offline, and whether
// a call is admitted is decided per call (Docs/audits/mcp-capability-
// admission.md, lifecycle qualification).
func TestMCPReviewState(t *testing.T) {
	tool := &mcp.MCPTool{ToolID: "srv/read_note", SchemaHash: "h1"}
	for _, tc := range []struct {
		name    string
		reviews map[string]mcp.ReviewedToolEffect
		state   string
		matched bool
	}{
		{"none", nil, "NOT REVIEWED", false},
		{"stale", map[string]mcp.ReviewedToolEffect{"srv/read_note": {SchemaHash: "h0", Effect: mcp.RemoteRead}}, "REVIEW STALE (schema changed)", false},
		{"configured", map[string]mcp.ReviewedToolEffect{"srv/read_note": {SchemaHash: "h1", Effect: mcp.RemoteRead}}, "configured review: read", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, matched := mcpReviewState(tool, tc.reviews)
			if state != tc.state || matched != tc.matched {
				t.Fatalf("mcpReviewState = %q, %t; want %q, %t", state, matched, tc.state, tc.matched)
			}
		})
	}
}
