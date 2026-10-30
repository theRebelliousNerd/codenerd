package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A real Cortex, its sharded kernel, and the production write/refresh boundary.
// Before retirement a write returned with the old symbol still in the EDB and
// modified/1 activated an independent full-scan agent.
func TestWorldRefreshAfterWriteUsesBootScanner(t *testing.T) {
	ws := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(ws, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/app\n\ngo 1.26\n")
	write("lib/lib.go", "package lib\nfunc Value() int { return 1 }\n")
	write("main.go", "package main\nfunc Before() {}\n")
	write("main_test.go", "package main\nimport \"testing\"\nfunc TestBefore(t *testing.T) {}\n")
	cfg := config.DefaultUserConfig()
	cfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortexWithConfig(ctx, BootConfig{
		Workspace: ws, UserConfigOverride: cfg, LLMClientOverride: &MockLLMClient{},
		DisableSystemShards: []string{"constitution_gate", "perception_firewall", "executive_policy",
			"session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cortex.Close()
	if _, ok := cortex.Kernel.(*core.CortexKernel); !ok {
		t.Fatalf("kernel = %T", cortex.Kernel)
	}
	if _, exists := cortex.ShardManager.GetProfile("world_model_ingestor"); exists {
		t.Fatal("retired ingestor still registered")
	}
	before := cortex.Scanner.RefreshStats()
	if before.FullScans != 1 {
		t.Fatalf("boot full scans = %d", before.FullScans)
	}
	bootLinks, err := cortex.VirtualStore.QueryKnowledgeGraph("main.go", "incoming")
	if err != nil || len(bootLinks) == 0 {
		t.Fatalf("boot must publish its initial symbol projection: links=%v err=%v", bootLinks, err)
	}

	cortex.VirtualStore.DisableBootGuard()
	body := "package main\nimport \"example.com/app/lib\"\nfunc After() int { return lib.Value() }\n"
	payload := map[string]any{"content": body}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ws, "main.go")
	pending := core.Fact{Predicate: "pending_action", Args: []any{"world-write", types.MangleAtom("/write_file"), path, string(encoded), time.Now().Unix()}}
	if err := cortex.Kernel.Assert(pending); err != nil {
		t.Fatal(err)
	}
	result, err := cortex.VirtualStore.RouteActionResult(ctx, core.Fact{Predicate: "next_action",
		Args: []any{"world-write", "write_file", path, payload}})
	if err != nil || !result.Success || strings.Contains(result.Output, "World refresh failed") {
		t.Fatalf("write result=%+v err=%v", result, err)
	}
	assertRows := func(query string, want int) {
		t.Helper()
		facts, err := cortex.Kernel.Query(query)
		if err != nil || len(facts) != want {
			t.Fatalf("%s = %v err=%v, want %d rows", query, facts, err, want)
		}
	}
	assertRows(`file_topology("main.go", H, /go, T, /false)`, 1)
	assertRows(`symbol_graph("func:After", /function, /public, "main.go", S)`, 1)
	assertRows(`symbol_graph("func:Before", K, V, F, S)`, 0)
	assertRows(`dependency_link("main.go", "lib/lib.go", K)`, 1)
	assertRows(`file_in_project("main.go")`, 1)
	assertRows(`symbol_graph("func:TestBefore", /function, /public, "main_test.go", S)`, 1)
	// The consumer joins the freshly resolved edge to the same canonical key.
	if err := cortex.Kernel.Assert(core.Fact{Predicate: "modified", Args: []any{"lib/lib.go"}}); err != nil {
		t.Fatal(err)
	}
	assertRows(`impacted("main.go")`, 1)
	assertRows(`activate_shard(/world_model_ingestor)`, 0)
	for _, name := range cortex.ShardManager.EnsureOnDemandShards(ctx) {
		if name == "world_model_ingestor" {
			t.Fatal("write spawned a second scanner")
		}
	}
	after := cortex.Scanner.RefreshStats()
	if after.FullScans != before.FullScans || after.DeltaScans != before.DeltaScans+1 {
		t.Fatalf("write must use one delta and no second full scan: before=%+v after=%+v", before, after)
	}
	// The model uses the modular registry's completion sink rather than
	// RouteAction. It must reach exactly the same scanner and update test symbols.
	registry := cortex.VirtualStore.GetModularTools()
	if registry == nil {
		t.Fatal("Cortex did not hydrate modular tools")
	}
	toolResult, err := registry.Execute(ctx, "write_file", map[string]any{
		"path":    filepath.Join(ws, "main_test.go"),
		"content": "package main\nimport \"testing\"\nfunc TestAfter(t *testing.T) { _ = After() }\n",
	})
	if err != nil || toolResult == nil || toolResult.Error != nil {
		t.Fatalf("modular write=%+v err=%v", toolResult, err)
	}
	assertRows(`symbol_graph("func:TestAfter", /function, /public, "main_test.go", S)`, 1)
	assertRows(`symbol_graph("func:TestBefore", K, V, F, S)`, 0)
	final := cortex.Scanner.RefreshStats()
	if final.FullScans != before.FullScans || final.DeltaScans != after.DeltaScans+1 {
		t.Fatalf("modular write must use one delta and no full scan: before=%+v after=%+v", after, final)
	}
	t.Logf("boot refresh=%v; action write refresh=%v; modular write refresh=%v", before.LastDuration, after.LastDuration, final.LastDuration)

	exerciseWorldPublicationRetry(t, ctx, cortex, path, body)
}
