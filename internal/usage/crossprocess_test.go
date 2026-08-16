package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readUsageData(t *testing.T, path string) UsageData {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var ud UsageData
	if err := json.Unmarshal(raw, &ud); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return ud
}

func writeUsageData(t *testing.T, path string, ud UsageData) {
	t.Helper()
	payload, err := json.MarshalIndent(ud, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, payload, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func seedAggregate(a *AggregatedStats) {
	cost, _ := EstimateCost("glm-4.6", 100, 50)
	a.TotalProject.AddCost(100, 50, cost)
	tc := a.ByProvider["zai"]
	tc.AddCost(100, 50, cost)
	a.ByProvider["zai"] = tc
	tc = a.ByModel["glm-4.6"]
	tc.AddCost(100, 50, cost)
	a.ByModel["glm-4.6"] = tc
	tc = a.ByShardType["coder"]
	tc.AddCost(100, 50, cost)
	a.ByShardType["coder"] = tc
	tc = a.ByShardName["coder-1"]
	tc.AddCost(100, 50, cost)
	a.ByShardName["coder-1"] = tc
	tc = a.ByOperation["chat"]
	tc.AddCost(100, 50, cost)
	a.ByOperation["chat"] = tc
	tc = a.BySession["sess-1"]
	tc.AddCost(100, 50, cost)
	a.BySession["sess-1"] = tc
}

func addOtherWriterAggregate(a *AggregatedStats) {
	cost, _ := EstimateCost("claude-sonnet-4", 100, 50)
	a.TotalProject.AddCost(100, 50, cost)
	tc := a.ByProvider["other-provider"]
	tc.AddCost(100, 50, cost)
	a.ByProvider["other-provider"] = tc
	tc = a.ByModel["claude-sonnet-4"]
	tc.AddCost(100, 50, cost)
	a.ByModel["claude-sonnet-4"] = tc
	tc = a.ByShardType["other-type"]
	tc.AddCost(100, 50, cost)
	a.ByShardType["other-type"] = tc
	tc = a.ByShardName["other-shard"]
	tc.AddCost(100, 50, cost)
	a.ByShardName["other-shard"] = tc
	tc = a.ByOperation["other-op"]
	tc.AddCost(100, 50, cost)
	a.ByOperation["other-op"] = tc
	tc = a.BySession["other-session"]
	tc.AddCost(100, 50, cost)
	a.BySession["other-session"] = tc
}

func TestSave_WhenAnotherWriterAdvancedTheFile_ShouldMergeNotOverwrite(t *testing.T) {
	ws := t.TempDir()
	tr, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tr.Close()

	ctx := WithShardContext(context.Background(), "shard-a", "type-a", "sess-a")
	tr.Track(ctx, "glm-4.6", "zai", 10, 5, "chat")
	if err := tr.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	path := filepath.Join(ws, ".nerd", "usage.json")
	ud := readUsageData(t, path)
	addOtherWriterAggregate(&ud.Aggregate)
	writeUsageData(t, path, ud)

	tr.Track(ctx, "glm-4.6", "zai", 7, 3, "chat")
	if err := tr.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	final := readUsageData(t, path)
	if final.Aggregate.TotalProject.Total != 175 {
		t.Fatalf("TotalProject.Total=%d, want 175", final.Aggregate.TotalProject.Total)
	}
	if final.Aggregate.TotalProject.Input != 117 {
		t.Fatalf("TotalProject.Input=%d, want 117", final.Aggregate.TotalProject.Input)
	}
	if got := final.Aggregate.ByProvider["zai"]; got.Total != 25 {
		t.Fatalf("ByProvider[zai]=%d, want 25", got.Total)
	}
	if got := final.Aggregate.ByProvider["other-provider"]; got.Total != 150 {
		t.Fatalf("ByProvider[other-provider]=%d, want 150", got.Total)
	}
	if got := final.Aggregate.ByModel["glm-4.6"]; got.Total != 25 {
		t.Fatalf("ByModel[glm-4.6]=%d, want 25", got.Total)
	}
	if got := final.Aggregate.ByModel["claude-sonnet-4"]; got.Total != 150 {
		t.Fatalf("ByModel[claude-sonnet-4]=%d, want 150", got.Total)
	}
}

func TestSave_WhenFileIsUnparseable_ShouldStillWriteOwnData(t *testing.T) {
	ws := t.TempDir()
	tr, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tr.Close()

	ctx := WithShardContext(context.Background(), "shard-a", "type-a", "sess-a")
	tr.Track(ctx, "glm-4.6", "zai", 10, 5, "chat")
	if err := tr.Save(); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	path := filepath.Join(ws, ".nerd", "usage.json")
	if err := os.WriteFile(path, []byte("garbage { not json"), 0644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	tr.Track(ctx, "glm-4.6", "zai", 2, 3, "chat")
	if err := tr.Save(); err != nil {
		t.Fatalf("Save with garbage should succeed: %v", err)
	}

	final := readUsageData(t, path)
	if final.Aggregate.TotalProject.Total != 20 {
		t.Fatalf("TotalProject.Total=%d, want 20", final.Aggregate.TotalProject.Total)
	}
	if got := final.Aggregate.ByProvider["zai"]; got.Total != 20 {
		t.Fatalf("ByProvider[zai]=%d, want 20", got.Total)
	}
}

func seedFile(t *testing.T, path string) {
	t.Helper()
	seed := newUsageData()
	seedAggregate(&seed.Aggregate)
	writeUsageData(t, path, seed)
}

func TestLoad_ShouldSetBaselineSoASubsequentSaveDoesNotDoubleCount(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(nerdDir, "usage.json")
	seedFile(t, path)

	tr, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tr.Close()

	if err := tr.Save(); err != nil {
		t.Fatalf("Save after Load: %v", err)
	}

	final := readUsageData(t, path)
	if final.Aggregate.TotalProject.Total != 150 {
		t.Fatalf("TotalProject.Total=%d, want 150 (shallow would double to 300)", final.Aggregate.TotalProject.Total)
	}
	if got := final.Aggregate.ByProvider["zai"]; got.Total != 150 {
		t.Fatalf("ByProvider[zai]=%d, want 150", got.Total)
	}

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tr2, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("second NewTracker: %v", err)
	}
	defer tr2.Close()
	tr2.Track(ctx, "glm-4.6", "zai", 10, 5, "chat")
	if err := tr2.Save(); err != nil {
		t.Fatalf("second save: %v", err)
	}
	final2 := readUsageData(t, path)
	if final2.Aggregate.TotalProject.Total != 165 {
		t.Fatalf("after track, TotalProject.Total=%d, want 165", final2.Aggregate.TotalProject.Total)
	}
}
