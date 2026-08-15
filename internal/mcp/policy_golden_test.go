package mcp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

// mcpPolicyPath is the kernel-loaded location of the MCP selection rules.
// Anything under internal/core/defaults/policy/ is swept up by
// RealKernel.loadMangleFiles; the file used to live in this package, where the
// loader never saw it.
const (
	mcpSchemaDir = "../core/defaults"
	mcpPolicyRel = "policy/policy_mcp.mg"
)

// mcpPolicySchemas are the Decl modules the MCP rules join against.
var mcpPolicySchemas = []string{
	"schemas_mcp.mg",   // MCP predicates
	"schemas_tools.mg", // intent_requires_capability, shard_capability_affinity
	"schemas_intent.mg",
	"schemas_world.mg", // file_topology
}

func newMCPPolicyEngine(t *testing.T) *mangle.Engine {
	t.Helper()

	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	// Evaluate once over the complete EDB, the way RealKernel does: its full
	// path rebuilds a fresh fact store from the EDB on every evaluation. This
	// engine accumulates derived facts instead, so evaluating after each
	// inserted fact would leave stale rows from non-monotone rules (fn:max
	// aggregation, safe negation) and misreport what the kernel derives.
	eng.ToggleAutoEval(false)

	for _, name := range mcpPolicySchemas {
		path := filepath.Join(mcpSchemaDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read schema %s: %v", path, err)
		}
		if err := eng.LoadSchemaString(string(data)); err != nil {
			t.Fatalf("load schema %s: %v", path, err)
		}
	}

	policyPath := filepath.Join(mcpSchemaDir, mcpPolicyRel)
	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("read policy %s: %v", policyPath, err)
	}
	if err := eng.LoadSchemaString(string(data)); err != nil {
		t.Fatalf("load policy %s: %v", policyPath, err)
	}
	return eng
}

func loadEDBFile(t *testing.T, eng *mangle.Engine, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read edb %s: %v", path, err)
	}
	loadEDBLines(t, eng, string(data))
}

func loadEDBLines(t *testing.T, eng *mangle.Engine, content string) {
	t.Helper()
	facts := make([]mangle.Fact, 0, 64)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		atom, err := mangle.ParseAtom(strings.TrimSuffix(line, "."))
		if err != nil {
			t.Fatalf("parse fact %q: %v", line, err)
		}
		args := make([]any, len(atom.Args))
		for i, arg := range atom.Args {
			args[i] = arg
		}
		facts = append(facts, mangle.Fact{Predicate: atom.Predicate.Symbol, Args: args})
	}
	if err := eng.AddFacts(facts); err != nil {
		t.Fatalf("add facts: %v", err)
	}
	if err := eng.RecomputeRules(); err != nil {
		t.Fatalf("recompute: %v", err)
	}
}

// selectedForShard returns the rendered mcp_tool_selected facts for one shard,
// sorted so comparisons are order-independent.
func selectedForShard(t *testing.T, eng *mangle.Engine, shardAtom string) []string {
	t.Helper()
	facts, err := eng.GetFacts("mcp_tool_selected")
	if err != nil {
		t.Fatalf("GetFacts(mcp_tool_selected): %v", err)
	}
	var out []string
	for _, f := range facts {
		if len(f.Args) != 3 {
			t.Fatalf("mcp_tool_selected arity changed: %v", f)
		}
		if got, _ := f.Args[0].(string); got != shardAtom {
			continue
		}
		out = append(out, f.String())
	}
	sort.Strings(out)
	return out
}

func goldenLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

func TestMCPPolicy_WhenFixtureEDBLoaded_ShouldMatchGoldenSelection(t *testing.T) {
	eng := newMCPPolicyEngine(t)
	loadEDBFile(t, eng, filepath.Join("testdata", "mcp_selection.edb"))

	got := selectedForShard(t, eng, "/coder")
	want := goldenLines(t, filepath.Join("testdata", "mcp_selection_coder.golden"))

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mcp_tool_selected(/coder, ...) mismatch\n got:\n  %s\nwant:\n  %s",
			strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestMCPPolicy_WhenToolIsSkeleton_ShouldSelectForEveryShard(t *testing.T) {
	eng := newMCPPolicyEngine(t)
	loadEDBFile(t, eng, filepath.Join("testdata", "mcp_selection.edb"))

	// fs/read_file is a skeleton tool; it must be full-rendered even for a
	// shard whose affinity (20) is far below the base-relevance floor.
	got := selectedForShard(t, eng, "/tester")
	want := []string{`mcp_tool_selected(/tester, "fs/read_file", /full).`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tester selection = %v, want %v", got, want)
	}
}

func TestMCPPolicy_WhenServerErrored_ShouldNotDeriveAvailability(t *testing.T) {
	eng := newMCPPolicyEngine(t)
	loadEDBFile(t, eng, filepath.Join("testdata", "mcp_selection.edb"))

	facts, err := eng.GetFacts("mcp_tool_available")
	if err != nil {
		t.Fatalf("GetFacts(mcp_tool_available): %v", err)
	}
	for _, f := range facts {
		if id, _ := f.Args[0].(string); id == "dead/thing" {
			t.Fatalf("tool on an /error server must not be available: %v", f)
		}
	}
	if len(facts) != 5 {
		t.Errorf("expected 5 available tools (fs/* + web/fetch), got %d: %v", len(facts), facts)
	}
}

func TestMCPPolicy_WhenUsageRecorded_ShouldDeriveSuccessRate(t *testing.T) {
	eng := newMCPPolicyEngine(t)
	loadEDBFile(t, eng, filepath.Join("testdata", "mcp_selection.edb"))

	// The success-rate formula lives in Mangle, not Go: Go only reports the
	// raw counters.
	rates := map[string]int64{}
	facts, err := eng.GetFacts("mcp_tool_success_rate")
	if err != nil {
		t.Fatalf("GetFacts(mcp_tool_success_rate): %v", err)
	}
	for _, f := range facts {
		id, _ := f.Args[0].(string)
		switch v := f.Args[1].(type) {
		case int64:
			rates[id] = v
		case int:
			rates[id] = int64(v)
		}
	}
	if rates["fs/proven"] != 90 {
		t.Errorf("fs/proven success rate = %d, want 90", rates["fs/proven"])
	}
	if rates["fs/flaky"] != 20 {
		t.Errorf("fs/flaky success rate = %d, want 20", rates["fs/flaky"])
	}
}

// TestMCPPolicy_WhenFactsComeFromEmitter_ShouldDriveSelection closes the loop:
// the strings FactEmitter produces must parse, load, and derive a selection.
// A golden fixture alone would not catch a Go-side type or atom mistake.
func TestMCPPolicy_WhenFactsComeFromEmitter_ShouldDriveSelection(t *testing.T) {
	eng := newMCPPolicyEngine(t)

	server := &MCPServer{
		ID:           "octo",
		Name:         "Octo MCP",
		Endpoint:     "http://localhost:7777",
		Protocol:     ProtocolHTTP,
		Status:       ServerStatusConnected,
		Capabilities: []string{"tools", "resources"},
		DiscoveredAt: fixedTime(),
	}
	tool := &MCPTool{
		ToolID:          "octo/grep",
		ServerID:        "octo",
		Name:            "grep",
		Description:     "Search files for a pattern \"fast\"",
		Condensed:       "Search files",
		Categories:      []string{"search"},
		Capabilities:    []string{"/search"},
		Domain:          "/general",
		ShardAffinities: map[string]int{"coder": 70},
		RegisteredAt:    fixedTime(),
		AnalyzedAt:      fixedTime(),
	}

	var emitted []string
	emitted = append(emitted, serverFacts(server)...)
	emitted = append(emitted, `mcp_server_status("octo", /connected)`)
	emitted = append(emitted, toolFacts(tool)...)

	loadEDBLines(t, eng, strings.Join(emitted, ".\n")+".")

	got := selectedForShard(t, eng, "/coder")
	want := []string{`mcp_tool_selected(/coder, "octo/grep", /full).`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("emitter-driven selection = %v, want %v (emitted: %v)", got, want, emitted)
	}
}

func fixedTime() time.Time {
	return time.Unix(1700000000, 0).UTC()
}
