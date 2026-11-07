package core

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

// requireEDBConsistent checks the two halves of the EDB against each other:
// every live slot is indexed under its key at its own position, the index
// holds nothing else, and the counts agree.
func requireEDBConsistent(t *testing.T, k *RealKernel, stage string) {
	t.Helper()
	k.mu.RLock()
	defer k.mu.RUnlock()
	live, pos := 0, 0
	for _, c := range k.facts.chunks {
		for _, s := range c.slots {
			if s.key != "" {
				live++
				if got, ok := k.factIndex.slotOf(s.fact.Predicate, s.key); !ok || got != pos {
					t.Fatalf("%s: %s sits at slot %d, the index says %d (held %v)", stage, s.key, pos, got, ok)
				}
				if held, ok := k.factIndex.bucket(s.fact.Predicate)[s.key]; !ok || !reflect.DeepEqual(held, s.fact) {
					t.Fatalf("%s: %s is stored but not indexed as itself", stage, s.key)
				}
			}
			pos++
		}
	}
	if pos != k.facts.n {
		t.Fatalf("%s: chunks hold %d slots, the store counts %d", stage, pos, k.facts.n)
	}
	if live != k.facts.len() || live != k.factIndex.len() {
		t.Fatalf("%s: %d live slots, store len %d, index len %d", stage, live, k.facts.len(), k.factIndex.len())
	}
}

func edgeFact(i int) Fact {
	return Fact{Predicate: "code_calls", Args: []any{fmt.Sprintf("A%d", i), fmt.Sprintf("B%d", i%7)}}
}

// A retraction on a large EDB reaches its own slot through the index: it
// copies no chunk, compacts nothing and walks no fact, and neither does the
// evaluation that follows it. Counters, not clocks: the cost of a retraction
// must not be a function of how many facts the world holds.
func TestEDBRetractTouchesOnlyItsSlot(t *testing.T) {
	k := setupMockKernel(t)
	worldBatch(t, k, "base", 3000) // ~18K EDB rows
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if rows := k.FactCount(); rows < 15000 {
		t.Fatalf("world holds %d rows, want a large EDB", rows)
	}
	requireEDBConsistent(t, k, "after load")

	k.mu.RLock()
	copies, tombstones, compactions, scans := k.facts.chunkCopies, k.facts.tombstoned, k.facts.compactions, k.facts.scans.Load()
	k.mu.RUnlock()

	victim := worldTopology("base/f1500.go")
	if err := k.RetractExactFact(victim); err != nil {
		t.Fatal(err)
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	if mode := k.LastEvaluation().Mode; mode != "dred" {
		t.Fatalf("evaluated as %q, want dred", mode)
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if got := k.facts.tombstoned - tombstones; got != 1 {
		t.Fatalf("retracting one fact tombstoned %d slots", got)
	}
	if got := k.facts.chunkCopies - copies; got != 0 {
		t.Fatalf("retracting one fact copied %d chunks of an unshared store", got)
	}
	if got := k.facts.compactions - compactions; got != 0 {
		t.Fatalf("retracting one fact compacted the EDB %d times", got)
	}
	if got := k.facts.scans.Load() - scans; got != 0 {
		t.Fatalf("retracting one fact and evaluating walked the EDB %d times", got)
	}
}

// After a clone, a write on either side copies the one chunk it lands in and
// nothing of the other side changes. Concurrent clones under the read lock are
// safe (run with -race).
func TestEDBCloneCopiesOnlyTheChunkItWrites(t *testing.T) {
	parent := setupMockKernel(t)
	var facts []Fact
	for i := 0; i < 20*edbChunkSlots; i++ {
		facts = append(facts, edgeFact(i))
	}
	if err := parent.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	want := parent.facts.slice()

	clones := make([]*RealKernel, 8)
	var wg sync.WaitGroup
	for i := range clones {
		wg.Add(1)
		go func() { defer wg.Done(); clones[i] = parent.Clone() }()
	}
	wg.Wait()
	child := clones[0]
	if child.facts.chunkCopies != 0 {
		t.Fatalf("cloning copied %d chunks", child.facts.chunkCopies)
	}

	if err := child.RetractExactFact(facts[5*edbChunkSlots+3]); err != nil {
		t.Fatal(err)
	}
	if got := child.facts.chunkCopies; got != 1 {
		t.Fatalf("a retraction on a clone copied %d chunks, want the one it touched", got)
	}
	if err := child.AssertWithoutEval(edgeFact(1_000_000)); err != nil {
		t.Fatal(err)
	}
	if got := child.facts.chunkCopies; got != 2 {
		t.Fatalf("an append on a clone copied %d chunks in all, want the last one (2)", got)
	}
	if err := parent.AssertWithoutEval(edgeFact(2_000_000)); err != nil {
		t.Fatal(err)
	}
	if got := parent.facts.chunkCopies; got != 1 {
		t.Fatalf("an append on the parent copied %d chunks, want the last one", got)
	}
	if err := parent.RetractExactFact(facts[7*edbChunkSlots]); err != nil {
		t.Fatal(err)
	}

	if got := clones[1].facts.slice(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a sibling clone changed: %d facts, want %d", len(got), len(want))
	}
	gotChild := child.facts.slice()
	if len(gotChild) != len(want) || gotChild[len(gotChild)-1].Args[0] != "A1000000" {
		t.Fatalf("clone holds %d facts, want %d with its own append last", len(gotChild), len(want))
	}
	for _, f := range gotChild {
		if f.Args[0] == "A2000000" {
			t.Fatal("the parent's write shows on the clone")
		}
	}
	for _, f := range parent.facts.slice() {
		if f.Args[0] == "A1000000" {
			t.Fatal("the clone's write shows on the parent")
		}
	}
	requireEDBConsistent(t, parent, "parent")
	requireEDBConsistent(t, child, "clone")
	requireEDBConsistent(t, clones[1], "sibling")
}

// The ordered EDB is what a model of it says through every kind of write,
// including enough retractions to compact, and the index follows each move.
func TestEDBMatchesModelThroughWritesAndCompaction(t *testing.T) {
	k := setupMockKernel(t)
	rng := rand.New(rand.NewSource(7))
	k.Clear() // the model starts from an empty EDB
	var model []Fact
	has := func(f Fact) int {
		for i, m := range model {
			if m.Predicate == f.Predicate && argsSliceEqual(m.Args, f.Args) {
				return i
			}
		}
		return -1
	}
	universe := 900

	for step := 0; step < 4000; step++ {
		f := edgeFact(rng.Intn(universe))
		switch op := rng.Intn(10); {
		case op < 5:
			if err := k.AssertWithoutEval(f); err != nil {
				t.Fatal(err)
			}
			if has(f) < 0 {
				model = append(model, f)
			}
		case op < 9:
			if err := k.RetractExactFact(f); err != nil {
				t.Fatal(err)
			}
			if i := has(f); i >= 0 {
				model = append(model[:i:i], model[i+1:]...)
			}
		default:
			first := f.Args[0]
			if err := k.RetractFact(Fact{Predicate: "code_calls", Args: []any{first}}); err != nil {
				t.Fatal(err)
			}
			kept := model[:0:0]
			for _, m := range model {
				if !argsEqual(m.Args[0], first) {
					kept = append(kept, m)
				}
			}
			model = kept
		}
		if step%401 == 0 {
			requireEDBConsistent(t, k, fmt.Sprintf("step %d", step))
			if got := k.facts.slice(); !reflect.DeepEqual(got, model) && !(len(got) == 0 && len(model) == 0) {
				t.Fatalf("step %d: EDB order diverged from the model (%d vs %d facts)", step, len(got), len(model))
			}
		}
	}
	compactions := k.facts.compactions
	if compactions == 0 {
		t.Fatal("the run never compacted; widen it")
	}
	requireEDBConsistent(t, k, "end")
	if got := k.facts.slice(); !reflect.DeepEqual(got, model) {
		t.Fatalf("EDB order diverged from the model (%d vs %d facts)", len(got), len(model))
	}
	// The evaluation reads the same world a full rebuild of it would.
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	requireSameStore(t, "model run", dumpStore(t, k), fullOracle(t, k))
}

// A heartbeat refresh finds its row through the index; it does not walk the
// EDB, however large, and keeps the row's place in the order.
func TestHeartbeatRefreshDoesNotWalkTheEDB(t *testing.T) {
	k := setupMockKernel(t)
	var facts []Fact
	for i := 0; i < 5*edbChunkSlots; i++ {
		facts = append(facts, edgeFact(i))
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	held := k.FactCount() // the boot facts and the loaded ones
	beat := func(ts int64) Fact {
		return Fact{Predicate: "system_heartbeat", Args: []any{MangleAtom("/shard_a"), ts}}
	}
	if err := k.Assert(beat(1)); err != nil {
		t.Fatal(err)
	}
	if err := k.AssertWithoutEval(edgeFact(9_999_999)); err != nil {
		t.Fatal(err)
	}
	k.mu.RLock()
	scans := k.facts.scans.Load()
	k.mu.RUnlock()
	for ts := int64(2); ts < 50; ts++ {
		if err := k.Assert(beat(ts)); err != nil {
			t.Fatal(err)
		}
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	if got := k.facts.scans.Load() - scans; got != 0 {
		t.Fatalf("heartbeat refreshes walked the EDB %d times", got)
	}
	rows := k.facts.slice()
	at := held
	if rows[at].Predicate != "system_heartbeat" || rows[at].Args[1] != int64(49) {
		t.Fatalf("heartbeat row is %v at its original place, want the latest timestamp", rows[at])
	}
	if rows[at+1].Args[0] != "A9999999" {
		t.Fatalf("the refresh moved the row: %v follows it", rows[at+1])
	}
}

// Removing the last facts leaves no tombstone behind, so retracting and
// asserting the same fact over and over never grows the store.
func TestEDBTrimsTrailingTombstones(t *testing.T) {
	var f edbFacts
	for i := 0; i < edbChunkSlots+5; i++ {
		f.append(edgeFact(i), fmt.Sprintf("k%d", i))
	}
	for round := 0; round < 100; round++ {
		pos := f.append(edgeFact(10_000), "again")
		if !f.remove(pos) {
			t.Fatal("remove of a live slot reported false")
		}
	}
	if f.n != edbChunkSlots+5 || f.dead != 0 || len(f.chunks) != 2 {
		t.Fatalf("store holds %d slots (%d dead) in %d chunks after 100 append/remove rounds", f.n, f.dead, len(f.chunks))
	}
	// Interior tombstones stay until compaction, the order undisturbed.
	f.remove(3)
	f.remove(edbChunkSlots + 1)
	if f.len() != edbChunkSlots+3 || f.dead != 2 {
		t.Fatalf("len %d dead %d after two interior removals", f.len(), f.dead)
	}
	var moved int
	f.compact(func(Fact, string, int) { moved++ })
	if moved != edbChunkSlots+3 || f.dead != 0 || f.n != edbChunkSlots+3 {
		t.Fatalf("compaction moved %d facts, left %d slots (%d dead)", moved, f.n, f.dead)
	}
}
