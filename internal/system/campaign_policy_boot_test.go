package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The campaign management rules (campaign_rules.mg) read three measurements
// nothing asserted: the context pager's use of its budget, what a completed
// task's files carry, and the campaign's progress. The orchestrator now
// asserts them and acts on the rules' answers. This runs the rules on the real
// sharded kernel: the quality chain joins the plan's artifacts to the world
// scan in the world shard, and the pressure and milestone rules run where the
// campaign's facts live.
func TestCampaignPolicyRulesFireOnTheShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
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
	write("svc/svc.go", "package svc\n\n// TODO: finish this\nfunc Serve() {}\n")
	write("clean/clean.go", "package clean\n\nfunc Add(a, b int) int { return a + b }\n")
	write("clean/clean_test.go", "package clean\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { _ = Add(1, 2) }\n")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	artifact := func(task, path string) types.Fact {
		return types.Fact{Predicate: "task_artifact", Args: []any{task, types.MangleAtom("/source_file"), path, "h"}}
	}
	if err := cortex.Kernel.AssertBatch([]types.Fact{artifact("t_dirty", "svc/svc.go"), artifact("t_clean", "clean/clean.go")}); err != nil {
		t.Fatal(err)
	}

	owed := func() map[string]bool {
		rows, err := cortex.Kernel.Query("remediation_task_needed")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, f := range rows {
			got[types.ExtractString(f.Args[0])+types.ExtractString(f.Args[1])] = true
		}
		return got
	}
	got := owed()
	if !got["t_dirty/placeholder"] || !got["t_dirty/no_tests"] {
		t.Errorf("a task whose file says TODO and has no test owes both fixes: %v", got)
	}
	for k := range got {
		if len(k) >= 7 && k[:7] == "t_clean" {
			t.Errorf("a clean, tested file owes nothing: %v", got)
		}
	}

	// The orchestrator records what a fix targets; that retires the debt, and the
	// fix itself is never judged.
	if err := cortex.Kernel.Assert(types.Fact{Predicate: "task_remediation_target",
		Args: []any{"t_dirty_fix", "t_dirty", types.MangleAtom("/placeholder")}}); err != nil {
		t.Fatal(err)
	}
	if err := cortex.Kernel.Assert(artifact("t_dirty_fix", "svc/svc.go")); err != nil {
		t.Fatal(err)
	}
	got = owed()
	if got["t_dirty/placeholder"] || !got["t_dirty/no_tests"] {
		t.Errorf("a recorded fix retires only the violation it targets: %v", got)
	}
	for k := range got {
		if len(k) >= 11 && k[:11] == "t_dirty_fix" {
			t.Errorf("a remediation task is not judged again: %v", got)
		}
	}

	// Context pressure and the prefetch decision it drives.
	limit := func(used int64) string {
		_ = cortex.Kernel.Retract("campaign_context_use")
		if err := cortex.Kernel.Assert(types.Fact{Predicate: "campaign_context_use", Args: []any{"camp1", used, int64(100)}}); err != nil {
			t.Fatal(err)
		}
		rows, err := cortex.Kernel.Query("campaign_prefetch_limit")
		if err != nil {
			t.Fatal(err)
		}
		var out string
		for _, f := range rows {
			if types.ExtractString(f.Args[0]) == "camp1" {
				out += types.ExtractString(f.Args[1]) + ","
			}
		}
		return out
	}
	if g := limit(10); g != "3," {
		t.Errorf("10%% used prefetches 3, got %q", g)
	}
	if g := limit(85); g != "1," {
		t.Errorf("85%% used prefetches 1, got %q", g)
	}
	if g := limit(96); g != "0," {
		t.Errorf("96%% used prefetches nothing, got %q", g)
	}

	// Milestones: half the phases done, announced until recorded.
	pending := func() bool {
		rows, err := cortex.Kernel.Query("milestone_pending")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range rows {
			if types.ExtractString(f.Args[0]) == "camp1" && types.ExtractString(f.Args[1]) == "/halfway" {
				return true
			}
		}
		return false
	}
	if err := cortex.Kernel.Assert(types.Fact{Predicate: "campaign_progress", Args: []any{"camp1", int64(1), int64(2), int64(3), int64(6)}}); err != nil {
		t.Fatal(err)
	}
	if !pending() {
		t.Error("one of two phases is halfway and pending")
	}
	if err := cortex.Kernel.Assert(types.Fact{Predicate: "campaign_milestone",
		Args: []any{"camp1", types.MangleAtom("/halfway"), "reached", int64(1)}}); err != nil {
		t.Fatal(err)
	}
	if pending() {
		t.Error("a recorded milestone is no longer pending")
	}
}
