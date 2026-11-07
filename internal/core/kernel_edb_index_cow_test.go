package core

import (
	"fmt"
	"sync"
	"testing"
)

// A clone shares the index copy-on-write: a write on either side, to a shared
// predicate or a new one, never shows on the other, and concurrent clones
// (each under the read lock) are safe. Run with -race.
func TestEDBIndexCloneIsCopyOnWrite(t *testing.T) {
	parent := setupMockKernel(t)
	row := func(i int) Fact {
		return Fact{Predicate: "file_topology", Args: []any{fmt.Sprintf("f%d.go", i), "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}
	}
	for i := 0; i < 50; i++ {
		if err := parent.AssertWithoutEval(row(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := parent.Evaluate(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	clones := make([]*RealKernel, 8)
	for i := range clones {
		wg.Add(1)
		go func() { defer wg.Done(); clones[i] = parent.Clone() }()
	}
	wg.Wait()
	child := clones[0]

	if err := child.RetractExactFact(row(3)); err != nil {
		t.Fatal(err)
	}
	if err := child.AssertWithoutEval(row(100)); err != nil {
		t.Fatal(err)
	}
	if err := parent.AssertWithoutEval(row(200)); err != nil {
		t.Fatal(err)
	}
	if err := parent.RetractExactFact(row(4)); err != nil {
		t.Fatal(err)
	}
	held := func(k *RealKernel, f Fact) bool {
		k.mu.RLock()
		defer k.mu.RUnlock()
		return k.factIndex.has(f.Predicate, k.canonFact(f))
	}
	for _, c := range []struct {
		k    *RealKernel
		f    Fact
		want bool
		what string
	}{
		{parent, row(3), true, "the child's retract reached the parent"},
		{parent, row(100), false, "the child's insert reached the parent"},
		{child, row(200), false, "the parent's insert reached the child"},
		{child, row(4), true, "the parent's retract reached the child"},
		{clones[1], row(3), true, "a sibling clone lost a row"},
		{clones[1], row(200), false, "a sibling clone gained the parent's row"},
	} {
		if got := held(c.k, c.f); got != c.want {
			t.Fatal(c.what)
		}
	}
	for _, k := range []*RealKernel{parent, child, clones[1]} {
		if err := k.EvaluatePending(); err != nil {
			t.Fatal(err)
		}
		requireSameStore(t, "after copy-on-write writes", dumpStore(t, k), fullOracle(t, k))
	}
}
