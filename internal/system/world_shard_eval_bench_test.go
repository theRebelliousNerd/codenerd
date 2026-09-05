package system

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// TestWorldShardEvalCost measures what a shared per-turn fact costs the world
// shard on a real cached world model. It is a measurement, not a gate: it
// runs only when CODENERD_WORLD_EVAL_BENCH names a workspace whose
// .nerd/knowledge.db holds a fast scan, and it logs the timings.
//
// CODENERD_WORLD_EVAL_EXCLUDE is a ';'-separated list of configurations, each
// a ','-separated list of policy file basenames to drop from the corpus for
// that configuration ("" = the full corpus), so the cost can be bisected by
// file without touching the embedded corpus.
//
// Observed 2026-09-04 on codeNERD itself (33K world facts): 25 s per
// evaluation on both the full and the differential path once user_intent
// was shared into every shard, while full fixpoints on the 20K-fact shards
// took under half a second.
func TestWorldShardEvalCost(t *testing.T) {
	workspace := os.Getenv("CODENERD_WORLD_EVAL_BENCH")
	if workspace == "" {
		t.Skip("set CODENERD_WORLD_EVAL_BENCH=<workspace> to run")
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

	var worldManifest shards.ShardPredicateManifest
	for _, m := range shards.DefaultShardPredicateManifests() {
		if m.Domain == "world" {
			worldManifest = m
		}
	}
	owned := make(map[string]struct{}, len(worldManifest.OwnedPredicates))
	for _, p := range worldManifest.OwnedPredicates {
		owned[p] = struct{}{}
	}
	var facts []types.Fact
	edbCounts := map[string]int{}
	for _, cf := range cached {
		if _, ok := owned[cf.Predicate]; ok {
			facts = append(facts, types.Fact{Predicate: cf.Predicate, Args: cf.Args})
			edbCounts[cf.Predicate]++
		}
	}
	t.Logf("world facts: %d of %d cached; %v", len(facts), len(cached), edbCounts)

	configs := []string{""}
	if v := os.Getenv("CODENERD_WORLD_EVAL_EXCLUDE"); v != "" {
		configs = strings.Split(v, ";")
	}
	policyFiles := core.DefaultPolicyFiles()

	for _, cfg := range configs {
		exclude := map[string]bool{}
		for _, f := range strings.Split(cfg, ",") {
			if f = strings.TrimSpace(f); f != "" {
				exclude[f] = true
			}
		}
		shard, err := core.NewKernelShard(core.KernelShardConfig{
			Domain:          "world",
			WorkspaceRoot:   workspace,
			OwnedPredicates: worldManifest.OwnedPredicates,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(exclude) > 0 {
			var b strings.Builder
			kept := 0
			for _, pf := range policyFiles {
				if exclude[filepath.Base(pf)] {
					continue
				}
				content, err := core.GetDefaultContent(pf)
				if err != nil {
					t.Fatal(err)
				}
				b.WriteString("\n\n# Policy Module: " + pf + "\n")
				b.WriteString(content)
				kept++
			}
			shard.Kernel().LoadPolicy(b.String())
			t.Logf("config %q: %d of %d policy files kept", cfg, kept, len(policyFiles))
		}
		if err := shard.LoadFacts(facts); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if err := shard.Evaluate(); err != nil {
			t.Fatalf("config %q: %v", cfg, err)
		}
		initial := time.Since(start)
		if err := shard.Assert(types.Fact{Predicate: "user_intent", Args: []any{"/current_intent", types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "internal/core/kernel.go", "none"}}); err != nil {
			t.Fatal(err)
		}
		start = time.Now()
		if err := shard.Evaluate(); err != nil {
			t.Fatal(err)
		}
		perTurn := time.Since(start)
		t.Logf("config %q: initial=%v per-turn=%v", cfg, initial, perTurn)

		if cfg == "" {
			all, err := shard.Kernel().QueryAll()
			if err != nil {
				t.Fatal(err)
			}
			type pc struct {
				pred string
				n    int
			}
			var counts []pc
			total := 0
			for pred, fs := range all {
				if _, isEDB := owned[pred]; isEDB {
					continue
				}
				counts = append(counts, pc{pred, len(fs)})
				total += len(fs)
			}
			sort.Slice(counts, func(i, j int) bool { return counts[i].n > counts[j].n })
			t.Logf("derived facts total=%d; top predicates:", total)
			for i := 0; i < len(counts) && i < 12; i++ {
				t.Logf("  %-40s %d", counts[i].pred, counts[i].n)
			}
		}
	}
}
