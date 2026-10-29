package core

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

func storeRows(t *testing.T, s factstore.FactStore) []string {
	t.Helper()
	var rows []string
	if err := factstore.GetAllFacts(s, func(a ast.Atom) error {
		rows = append(rows, a.String())
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	return rows
}

func ovAtom(pred string, n int) ast.Atom {
	return ast.NewAtom(pred, ast.String(fmt.Sprintf("v%d", n)))
}

// An overlay answers exactly as a plain store given the same writes, over a
// base it never changes; a fork answers as its source did at the fork, and
// neither sees the other's later writes.
func TestOverlayStoreMatchesAPlainStore(t *testing.T) {
	rng := rand.New(rand.NewSource(20261029))
	for _, kind := range []string{"simple", "indexed"} {
		base := factstore.FactStoreWithRemove(factstore.NewSimpleInMemoryStore())
		if kind == "indexed" {
			base = factstore.NewMultiIndexedArrayInMemoryStore()
		}
		plain := factstore.NewSimpleInMemoryStore()
		for i := 0; i < 40; i++ {
			a := ovAtom([]string{"p", "q"}[i%2], i)
			base.Add(a)
			plain.Add(a)
		}
		frozen := storeRows(t, base)
		o := newOverlayStore(base)
		var fork *overlayStore
		var forkRows []string
		for step := 0; step < 2000; step++ {
			a := ovAtom([]string{"p", "q", "r"}[rng.Intn(3)], rng.Intn(60))
			if rng.Intn(2) == 0 {
				if got, want := o.Add(a), plain.Add(a); got != want {
					t.Fatalf("%s step %d: Add(%v) = %v, plain %v", kind, step, a, got, want)
				}
			} else if got, want := o.Remove(a), plain.Remove(a); got != want {
				t.Fatalf("%s step %d: Remove(%v) = %v, plain %v", kind, step, a, got, want)
			}
			if o.Contains(a) != plain.Contains(a) {
				t.Fatalf("%s step %d: Contains(%v) disagrees", kind, step, a)
			}
			if step == 1000 {
				var ok bool
				if fork, ok = o.fork(); !ok {
					t.Fatal("fork failed")
				}
				forkRows = storeRows(t, o)
			}
		}
		if got, want := storeRows(t, o), storeRows(t, plain); !slices.Equal(got, want) {
			t.Fatalf("%s: overlay holds %d rows, plain %d", kind, len(got), len(want))
		}
		if o.EstimateFactCount() != plain.EstimateFactCount() {
			t.Fatalf("%s: overlay counts %d, plain %d", kind, o.EstimateFactCount(), plain.EstimateFactCount())
		}
		if !slices.Equal(storeRows(t, fork), forkRows) {
			t.Fatalf("%s: the fork saw writes made after it", kind)
		}
		if !slices.Equal(storeRows(t, base), frozen) {
			t.Fatalf("%s: the base changed under its overlays", kind)
		}
		if got := storeRows(t, o.flatten()); !slices.Equal(got, storeRows(t, plain)) {
			t.Fatalf("%s: flatten lost rows", kind)
		}
	}
}

// A kernel whose plain store a clone shares wraps it before writing; an
// overlay whose writes outgrow its base is flattened at the next write.
func TestOwnStoreWrapsASharedStoreAndFlattensADriftedOne(t *testing.T) {
	k := newPerfKernel(t, 8)
	shared := storeRows(t, k.store)
	c := k.Clone()
	if storeKind(c.store) != "overlay" || !k.storeShared.Load() {
		t.Fatalf("clone store %s, parent shared %v; want an overlay over a shared parent store", storeKind(c.store), k.storeShared.Load())
	}
	k.mu.Lock()
	k.ownStoreLocked()
	o, ok := k.store.(*overlayStore)
	k.mu.Unlock()
	if !ok || storeKind(o.base) == "overlay" || !slices.Equal(storeRows(t, o.base), shared) || k.storeShared.Load() {
		t.Fatal("the parent did not wrap its shared store before writing")
	}
	for i := 0; i < 40; i++ {
		o.Add(ovAtom("perf_drift", i))
	}
	k.mu.Lock()
	k.ownStoreLocked()
	kind := storeKind(k.store)
	k.mu.Unlock()
	if kind == "overlay" {
		t.Fatalf("an overlay with %d writes over %d base rows was not flattened", o.writes(), o.base.EstimateFactCount())
	}
}
