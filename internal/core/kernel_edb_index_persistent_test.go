package core

import (
	"fmt"
	"maps"
	"testing"
)

// worldRow is a file_topology row, the biggest predicate of a real world.
func worldRow(i int) Fact {
	return Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("f%d.go", i), "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}
}

func evaluatedWorldMock(t *testing.T, rows int) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	for i := 0; i < rows; i++ {
		if err := k.AssertWithoutEval(worldRow(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return k
}

// Parent, clone and grandchild each write the same large predicate, rows the
// others hold and rows they do not; each ends up with exactly its own rows, in
// the index, the storage and the evaluated store, and nobody sees a sibling's
// write. The grandchild is cloned from the clone after both diverged, and the
// older generations keep writing after it exists.
func TestEDBIndexParentCloneAndGrandchildWriteOnePredicate(t *testing.T) {
	const base = 600
	parent := evaluatedWorldMock(t, base)
	child := parent.Clone()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(parent.AssertWithoutEval(worldRow(1000)))
	must(parent.RetractExactFact(worldRow(1)))
	must(child.AssertWithoutEval(worldRow(2000)))
	must(child.RetractExactFact(worldRow(2)))
	must(child.EvaluatePending())
	grand := child.Clone()
	must(grand.AssertWithoutEval(worldRow(3000)))
	must(grand.RetractExactFact(worldRow(2000)))
	must(grand.RetractExactFact(worldRow(3)))
	must(child.AssertWithoutEval(worldRow(2001)))
	must(parent.AssertWithoutEval(worldRow(1001)))
	must(parent.RetractExactFact(worldRow(0)))

	want := func(extra []int, gone ...int) map[string]bool {
		rows := map[string]bool{}
		for i := 0; i < base; i++ {
			rows[fmt.Sprintf("f%d.go", i)] = true
		}
		for _, i := range gone {
			delete(rows, fmt.Sprintf("f%d.go", i))
		}
		for _, i := range extra {
			rows[fmt.Sprintf("f%d.go", i)] = true
		}
		return rows
	}
	for _, c := range []struct {
		name string
		k    *RealKernel
		rows map[string]bool
	}{
		{"parent", parent, want([]int{1000, 1001}, 0, 1)},
		{"clone", child, want([]int{2000, 2001}, 2)},
		{"grandchild", grand, want([]int{3000}, 2, 3)},
	} {
		c.k.mu.RLock()
		held := map[string]bool{}
		for _, f := range c.k.facts.slice() {
			if f.Predicate == "file_topology" {
				held[f.Args[0].(string)] = true
			}
		}
		indexed := map[string]bool{}
		for key, f := range c.k.factIndex.eachRow("file_topology") {
			if pos, ok := c.k.factIndex.slotOf("file_topology", key); !ok {
				t.Fatalf("%s: row %v has no slot", c.name, f.Args[0])
			} else if stored, storedKey, live := c.k.facts.at(pos); !live || storedKey != key || stored.Args[0] != f.Args[0] {
				t.Fatalf("%s: row %v's slot %d holds %v", c.name, f.Args[0], pos, stored)
			}
			indexed[f.Args[0].(string)] = true
		}
		rowCount, size, stored := c.k.factIndex.rowCount("file_topology"), c.k.factIndex.len(), c.k.facts.len()
		c.k.mu.RUnlock()
		if !maps.Equal(held, c.rows) || !maps.Equal(indexed, c.rows) {
			t.Fatalf("%s: storage holds %d rows, index %d, want %d", c.name, len(held), len(indexed), len(c.rows))
		}
		if rowCount != len(c.rows) || size != stored {
			t.Fatalf("%s: index counts %d rows of the predicate (want %d) and %d facts (storage holds %d)", c.name, rowCount, len(c.rows), size, stored)
		}
		must(c.k.EvaluatePending())
		requireSameStore(t, c.name, dumpStore(t, c.k), fullOracle(t, c.k))
	}
}

// Staging a row of a big predicate on a clone copies a path of the index's
// maps, not the predicate: the clone's maps share all but a few nodes with the
// parent's. A work count, so the proof does not depend on the clock.
func TestEDBIndexCloneStagingCopiesAPathNotThePredicate(t *testing.T) {
	const base, pred = 3000, "file_topology"
	parent := evaluatedWorldMock(t, base)
	child := parent.Clone()
	if err := child.AssertWithoutEval(worldRow(base + 1)); err != nil {
		t.Fatal(err)
	}
	if err := child.RetractExactFact(worldRow(7)); err != nil {
		t.Fatal(err)
	}
	const writes, maxPath = 2, 14
	check := func(name string, copied, total int) {
		t.Helper()
		if copied == 0 || copied > writes*maxPath {
			t.Fatalf("%s: the clone's writes copied %d of the parent's %d nodes; want a path each, at most %d", name, copied, total, writes*maxPath)
		}
	}
	unshared := func(a, b map[*pmapNode[Fact]]struct{}) (n int) {
		for node := range a {
			if _, ok := b[node]; !ok {
				n++
			}
		}
		return n
	}
	rows, parentRows := pmapNodes(child.factIndex.rows[pred]), pmapNodes(parent.factIndex.rows[pred])
	check("rows", unshared(rows, parentRows), len(parentRows))
	slots, parentSlots := pmapNodes(child.factIndex.slots[pred]), pmapNodes(parent.factIndex.slots[pred])
	copiedSlots := 0
	for node := range slots {
		if _, ok := parentSlots[node]; !ok {
			copiedSlots++
		}
	}
	check("slots", copiedSlots, len(parentSlots))
	if parent.factIndex.rowCount(pred) != base || child.factIndex.rowCount(pred) != base {
		t.Fatalf("parent holds %d rows and clone %d, want %d each", parent.factIndex.rowCount(pred), child.factIndex.rowCount(pred), base)
	}
}
