package system

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

// TestWorldCortexProvenanceCost measures what proof recording costs the
// production cortex on a real cached world model: a per-turn write with
// recording off, the same write with recording on, and one explanation. It is
// a measurement, not a gate: it runs only when CODENERD_WORLD_PROVENANCE_BENCH
// names a workspace whose .nerd/knowledge.db holds a fast scan, and it logs the
// timings and the live heap.
//
// Observed 2026-11-12 in a live session on codeNERD itself (707k EDB facts in
// the world shard): every evaluation during a deliberation was a full recorded
// fixpoint of 10-14 s, because recording made the incremental paths
// ineligible.
func TestWorldCortexProvenanceCost(t *testing.T) {
	workspace := os.Getenv("CODENERD_WORLD_PROVENANCE_BENCH")
	if workspace == "" {
		t.Skip("set CODENERD_WORLD_PROVENANCE_BENCH=<workspace> to run")
	}
	db, err := store.NewLocalStore(filepath.Join(workspace, ".nerd", "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cached, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) == 0 {
		t.Skip("no cached world facts")
	}
	facts := make([]core.Fact, 0, len(cached))
	for _, cf := range cached {
		facts = append(facts, core.Fact{Predicate: cf.Predicate, Args: cf.Args})
	}
	facts = append(facts, world.ImportCycleFacts(facts)...)

	cortex, err := NewDomainCortex(workspace)
	if err != nil {
		t.Fatal(err)
	}
	// CODENERD_WORLD_PROVENANCE_CPU names a file for a CPU profile of the
	// world load and its first evaluation (the boot cost).
	if path := os.Getenv("CODENERD_WORLD_PROVENANCE_CPU"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
	}
	start := time.Now()
	if err := cortex.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	loaded := time.Since(start)
	start = time.Now()
	if err := cortex.Evaluate(); err != nil {
		t.Fatal(err)
	}
	pprof.StopCPUProfile()
	t.Logf("world facts loaded: %d in %v; first evaluation %v; heap %s", len(facts), loaded, time.Since(start), heapInUse())

	// A turn writes a per-turn fact the deliberation rules read and one the
	// world shard owns: in the live session periodic writes reached the world
	// shard, so every recorded evaluation there was a full fixpoint.
	var stopped []core.Fact
	if err := cortex.Assert(types.Fact{Predicate: "config_param", Args: []any{types.MangleAtom("/deliberation_round_cap"), int64(3)}}); err != nil {
		t.Fatal(err)
	}
	turn := func(label string, round int64) {
		t.Helper()
		start := time.Now()
		if err := cortex.Assert(types.Fact{Predicate: "deliberation_tick", Args: []any{"bench-thread", round, int64(0)}}); err != nil {
			t.Fatal(err)
		}
		if err := cortex.Assert(types.Fact{Predicate: "is_test_file", Args: []any{fmt.Sprintf("bench_%d_test.go", round)}}); err != nil {
			t.Fatal(err)
		}
		var err error
		if stopped, err = cortex.Query("deliberation_stopped"); err != nil {
			t.Fatal(err)
		}
		if _, err := cortex.Query("is_test_file"); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: assert+query %v; heap %s", label, time.Since(start), heapInUse())
	}
	turn("recording off", 1)
	turn("recording off", 2)

	cortex.EnableProvenance()
	turn("recording on", 100)
	turn("recording on", 101)
	if len(stopped) == 0 {
		t.Fatal("deliberation_stopped derived nothing past the round cap")
	}
	goal, err := stopped[0].ToAtom()
	if err != nil {
		t.Fatal(err)
	}

	start = time.Now()
	env, err := cortex.ExplainEnvelope(goal.String(), core.ExplainOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("explain %s: %v recorded=%v proofs=%d; heap %s", goal, time.Since(start), env.RecordedThisEpoch, len(env.Proofs), heapInUse())
	if !env.RecordedThisEpoch || len(env.Proofs) == 0 {
		t.Fatalf("explanation of a derived goal has no recorded proof: %+v", env)
	}
	if path := os.Getenv("CODENERD_WORLD_PROVENANCE_HEAP"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	runtime.KeepAlive(cortex)
}

func heapInUse() string {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("%d MiB", m.HeapInuse>>20)
}
