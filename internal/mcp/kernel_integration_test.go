package mcp_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/mcp"
)

// kernelUnderTest mirrors internal/system.mcpKernelAdapter. It is duplicated
// here (test-only) rather than imported because internal/mcp deliberately does
// not depend on internal/core in production code — the dependency direction is
// system -> {core, mcp}. Keeping a faithful copy in the test is what makes the
// end-to-end assertions meaningful: it exercises the same parse/query contract
// the real adapter uses.
type kernelUnderTest struct {
	kernel *core.RealKernel
}

func (k *kernelUnderTest) Assert(fact string) error {
	return k.kernel.AssertString(strings.TrimSuffix(strings.TrimSpace(fact), "."))
}

func (k *kernelUnderTest) Retract(fact string) error {
	parsed, err := core.ParseFactString(strings.TrimSuffix(strings.TrimSpace(fact), "."))
	if err != nil {
		return err
	}
	return k.kernel.RetractExactFactsBatch([]core.Fact{parsed})
}

func (k *kernelUnderTest) Query(query string) ([]map[string]any, error) {
	pattern, err := core.ParseFactString(query)
	if err != nil {
		return nil, err
	}
	variables := map[int]string{}
	for i, arg := range pattern.Args {
		if s, ok := arg.(string); ok && strings.HasPrefix(s, "?") {
			variables[i] = s[1:]
		}
	}
	facts, err := k.kernel.Query(query)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(facts))
	for _, f := range facts {
		binding := map[string]any{}
		for idx, name := range variables {
			if idx < len(f.Args) {
				binding[name] = f.Args[idx]
			}
		}
		results = append(results, binding)
	}
	return results, nil
}

func bootKernel(t *testing.T) *kernelUnderTest {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("boot kernel: %v", err)
	}
	return &kernelUnderTest{kernel: kernel}
}

func mcpTestTool(toolID, serverID string, categories, capabilities []string, affinity int) *mcp.MCPTool {
	return &mcp.MCPTool{
		ToolID:          toolID,
		ServerID:        serverID,
		Name:            toolID[strings.LastIndex(toolID, "/")+1:],
		Description:     "test tool " + toolID,
		Condensed:       "test tool",
		Categories:      categories,
		Capabilities:    capabilities,
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": affinity},
		RegisteredAt:    time.Unix(1700000000, 0),
		AnalyzedAt:      time.Unix(1700000000, 0),
	}
}

// TestKernelPolicy_WhenMCPFactsEmitted_ShouldSelectToolsFromEmbeddedPolicy is
// the end-to-end proof that policy_mcp.mg is part of the embedded constitution
// and that the facts internal/mcp emits are shaped the way it expects. Before
// the policy was relocated under defaults/policy/, this query returned nothing.
func TestKernelPolicy_WhenMCPFactsEmitted_ShouldSelectToolsFromEmbeddedPolicy(t *testing.T) {
	kernel := bootKernel(t)
	emitter := mcp.NewFactEmitter(kernel)

	emitter.EmitServer(&mcp.MCPServer{
		ID:           "fs",
		Name:         "Filesystem MCP",
		Endpoint:     "http://localhost:9000",
		Protocol:     mcp.ProtocolHTTP,
		Status:       mcp.ServerStatusConnected,
		Capabilities: []string{"tools"},
		DiscoveredAt: time.Unix(1700000000, 0),
	})
	emitter.EmitTool(mcpTestTool("fs/read_file", "fs", []string{"filesystem"}, []string{"/read"}, 80))
	emitter.EmitTool(mcpTestTool("fs/run_cmd", "fs", []string{"shell"}, []string{"/execute"}, 60))

	compiler := mcp.NewJITToolCompiler(nil, nil, kernel)
	selected, err := compiler.MangleSelectForTest(context.Background(), "coder")
	if err != nil {
		t.Fatalf("mangleSelect: %v", err)
	}

	modes := map[string]mcp.RenderMode{}
	skeletons := map[string]bool{}
	for _, s := range selected {
		modes[s.ToolID] = s.RenderMode
		skeletons[s.ToolID] = s.Skeleton
	}

	if modes["fs/read_file"] != mcp.RenderModeFull {
		t.Errorf("fs/read_file mode = %q, want full (skeleton tool); got selection %+v", modes["fs/read_file"], selected)
	}
	if !skeletons["fs/read_file"] {
		t.Error("fs/read_file should be reported as a policy skeleton tool")
	}
	// 60 affinity + 10 general-domain boost = 70 -> full render, not skeleton.
	if modes["fs/run_cmd"] != mcp.RenderModeFull {
		t.Errorf("fs/run_cmd mode = %q, want full; got selection %+v", modes["fs/run_cmd"], selected)
	}
	if skeletons["fs/run_cmd"] {
		t.Error("fs/run_cmd is not a filesystem/search tool and must not be a skeleton tool")
	}
}

// TestKernelPolicy_WhenServerDisconnects_ShouldFlipAvailabilityFact covers the
// retract/update half: a disconnect must move mcp_server_status, and a full
// deregistration must remove the tool facts entirely.
func TestKernelPolicy_WhenServerDisconnects_ShouldFlipAvailabilityFact(t *testing.T) {
	kernel := bootKernel(t)
	emitter := mcp.NewFactEmitter(kernel)

	emitter.EmitServer(&mcp.MCPServer{
		ID:           "fs",
		Endpoint:     "http://localhost:9000",
		Protocol:     mcp.ProtocolHTTP,
		Status:       mcp.ServerStatusConnected,
		DiscoveredAt: time.Unix(1700000000, 0),
	})
	emitter.EmitTool(mcpTestTool("fs/read_file", "fs", []string{"filesystem"}, []string{"/read"}, 80))

	statuses := func() []string {
		facts, err := kernel.kernel.Query("mcp_server_status")
		if err != nil {
			t.Fatalf("query status: %v", err)
		}
		out := make([]string, 0, len(facts))
		for _, f := range facts {
			out = append(out, f.String())
		}
		return out
	}

	before := statuses()
	if len(before) != 1 || !strings.Contains(before[0], "/connected") {
		t.Fatalf("expected exactly one /connected status, got %v", before)
	}

	emitter.EmitServerStatus("fs", mcp.ServerStatusDisconnected)
	after := statuses()
	if len(after) != 1 || !strings.Contains(after[0], "/disconnected") {
		t.Fatalf("status must be replaced, not appended; got %v", after)
	}

	// Cached tools stay available while the server is merely disconnected.
	available, err := kernel.kernel.Query("mcp_tool_available")
	if err != nil {
		t.Fatalf("query available: %v", err)
	}
	if len(available) != 1 {
		t.Fatalf("disconnected server should keep its cached tool available, got %v", available)
	}

	// Full deregistration drops everything.
	emitter.RetractServer("fs")
	registered, err := kernel.kernel.Query("mcp_tool_registered")
	if err != nil {
		t.Fatalf("query registered: %v", err)
	}
	if len(registered) != 0 {
		t.Errorf("RetractServer must remove tool facts, got %v", registered)
	}
	if got := emitter.EmittedFactCount(); got != 0 {
		t.Errorf("emitter still tracks %d facts after RetractServer", got)
	}
}

// TestKernelPolicy_WhenToolReanalyzed_ShouldReplaceStaleMetadata guards the
// re-analyze path: stale categories must not survive alongside new ones.
func TestKernelPolicy_WhenToolReanalyzed_ShouldReplaceStaleMetadata(t *testing.T) {
	kernel := bootKernel(t)
	emitter := mcp.NewFactEmitter(kernel)

	emitter.EmitServer(&mcp.MCPServer{
		ID:           "fs",
		Endpoint:     "http://localhost:9000",
		Protocol:     mcp.ProtocolHTTP,
		Status:       mcp.ServerStatusConnected,
		DiscoveredAt: time.Unix(1700000000, 0),
	})
	emitter.EmitTool(mcpTestTool("fs/thing", "fs", []string{"filesystem"}, []string{"/read"}, 80))
	emitter.EmitTool(mcpTestTool("fs/thing", "fs", []string{"shell"}, []string{"/execute"}, 80))

	categories, err := kernel.kernel.Query("mcp_tool_category")
	if err != nil {
		t.Fatalf("query category: %v", err)
	}
	if len(categories) != 1 {
		t.Fatalf("re-analysis must replace categories, got %v", categories)
	}
	if !strings.Contains(categories[0].String(), "/shell") {
		t.Errorf("expected the new /shell category, got %v", categories[0].String())
	}

	// The tool is no longer filesystem+read, so it must stop being a skeleton.
	skeletons, err := kernel.kernel.Query("mcp_tool_skeleton")
	if err != nil {
		t.Fatalf("query skeleton: %v", err)
	}
	if len(skeletons) != 0 {
		t.Errorf("stale skeleton derivation survived re-analysis: %v", skeletons)
	}
}
