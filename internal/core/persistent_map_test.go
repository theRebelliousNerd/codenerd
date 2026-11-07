package core

import (
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"testing"
)

// collidingHash keeps three bits of a key's hash, so keys collide in every
// chunk: the full-depth collision nodes get exercised.
func collidingHash(key string) uint64 { return pmapDefaultHash(key) & 7 }

// narrowHash varies only the low chunk: deep chains of single-child nodes.
func narrowHash(key string) uint64 { return pmapDefaultHash(key) & 0xFFFF }

var pmapHashes = []struct {
	name string
	hash func(string) uint64
}{
	{"default", nil},
	{"narrow", narrowHash},
	{"colliding", collidingHash},
}

func pmapContents[V any](m pmap[V]) map[string]V {
	out := make(map[string]V, m.len())
	for k, v := range m.all() {
		out[k] = v
	}
	return out
}

// requirePmapEqual checks a pmap against the Go map that models it: size,
// every lookup, a miss, and iteration yielding each entry exactly once.
func requirePmapEqual[V comparable](t testing.TB, what string, got pmap[V], want map[string]V) {
	t.Helper()
	if got.len() != len(want) {
		t.Fatalf("%s: len %d, want %d", what, got.len(), len(want))
	}
	for k, v := range want {
		if g, ok := got.get(k); !ok || g != v {
			t.Fatalf("%s: get(%q) = %v,%v want %v", what, k, g, ok, v)
		}
	}
	if _, ok := got.get("absent\x00key"); ok {
		t.Fatalf("%s: found a key never stored", what)
	}
	seen := 0
	for k, v := range got.all() {
		seen++
		if w, ok := want[k]; !ok || w != v {
			t.Fatalf("%s: iteration yielded %q=%v, model has %v,%v", what, k, v, w, ok)
		}
	}
	if seen != len(want) {
		t.Fatalf("%s: iteration yielded %d entries, want %d", what, seen, len(want))
	}
}

func TestPmapMatchesGoMap(t *testing.T) {
	for _, h := range pmapHashes {
		t.Run(h.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(7))
			m := pmap[int]{hash: h.hash}
			model := map[string]int{}
			o := new(pmapOwner)
			for i := 0; i < 20000; i++ {
				key := fmt.Sprintf("k%d", rng.Intn(1500))
				if rng.Intn(3) == 0 {
					_, had := model[key]
					if m.del(o, key) != had {
						t.Fatalf("step %d: del(%q) disagreed with the model", i, key)
					}
					delete(model, key)
				} else {
					_, had := model[key]
					if m.put(o, key, i) == had {
						t.Fatalf("step %d: put(%q) added=%v with the key held=%v", i, key, !had, had)
					}
					model[key] = i
				}
				if i%997 == 0 {
					requirePmapEqual(t, fmt.Sprintf("step %d", i), m, model)
				}
			}
			requirePmapEqual(t, "end", m, model)
			for k := range model {
				m.del(o, k)
			}
			if m.len() != 0 || m.root != nil {
				t.Fatalf("emptied map keeps len=%d root=%v", m.len(), m.root)
			}
		})
	}
}

func TestPmapEmptyAndZeroValue(t *testing.T) {
	var m pmap[string]
	if _, ok := m.get("x"); ok || m.len() != 0 || m.has("x") {
		t.Fatal("the zero map holds something")
	}
	for range m.all() {
		t.Fatal("the zero map iterates")
	}
	if m.del(nil, "x") {
		t.Fatal("deleting from the zero map reported a removal")
	}
	m = m.with("a", "1")
	if v, _ := m.get("a"); v != "1" || m.len() != 1 {
		t.Fatal("with did not store")
	}
	if m = m.without("a"); m.len() != 0 {
		t.Fatal("without did not drop")
	}
}

// Versions made by with and without are immutable: every earlier version keeps
// its contents however many later ones are written.
func TestPmapVersionsAreImmutable(t *testing.T) {
	for _, h := range pmapHashes {
		t.Run(h.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(11))
			cur := pmap[int]{hash: h.hash}
			model := map[string]int{}
			type version struct {
				m    pmap[int]
				want map[string]int
			}
			var history []version
			for i := 0; i < 3000; i++ {
				key := fmt.Sprintf("k%d", rng.Intn(400))
				if rng.Intn(3) == 0 {
					cur = cur.without(key)
					delete(model, key)
				} else {
					cur = cur.with(key, i)
					model[key] = i
				}
				if i%50 == 0 {
					history = append(history, version{cur, maps.Clone(model)})
				}
			}
			for n, v := range history {
				requirePmapEqual(t, fmt.Sprintf("version %d", n), v.m, v.want)
			}
		})
	}
}

// A shape is a function of the keys held: any insertion order, with removals
// along the way, iterates the same entries in the same order.
func TestPmapShapeIsCanonical(t *testing.T) {
	for _, h := range pmapHashes {
		t.Run(h.name, func(t *testing.T) {
			keys := make([]string, 300)
			for i := range keys {
				keys[i] = fmt.Sprintf("canon-%d", i)
			}
			build := func(seed int64) []string {
				rng := rand.New(rand.NewSource(seed))
				order := slices.Clone(keys)
				rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
				m := pmap[int]{hash: h.hash}
				o := new(pmapOwner)
				for _, k := range order {
					m.put(o, k, 1)
					m.put(o, "extra-"+k, 1) // dropped again below
				}
				for _, k := range order {
					m.del(o, "extra-"+k)
				}
				var got []string
				for k := range m.all() {
					got = append(got, k)
				}
				return got
			}
			first := build(1)
			for seed := int64(2); seed < 6; seed++ {
				if got := build(seed); !slices.Equal(first, got) {
					t.Fatalf("seed %d iterates in a different order than seed 1", seed)
				}
			}
		})
	}
}

// pmapNodes lists every node reachable from m.
func pmapNodes[V any](m pmap[V]) map[*pmapNode[V]]struct{} {
	out := map[*pmapNode[V]]struct{}{}
	var walk func(n *pmapNode[V])
	walk = func(n *pmapNode[V]) {
		if n == nil {
			return
		}
		out[n] = struct{}{}
		for i := range n.kids {
			walk(n.kids[i].child)
		}
	}
	walk(m.root)
	return out
}

// A write copies the path to its key and shares everything else with the
// version it came from, whatever the map's size.
func TestPmapWriteSharesStructure(t *testing.T) {
	for _, size := range []int{1000, 100000} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := pmap[int]{}
			o := new(pmapOwner)
			for i := 0; i < size; i++ {
				m.put(o, fmt.Sprintf("row-%d", i), i)
			}
			base := pmapNodes(m)
			// The shared-with-nobody case: writing under a fresh owner
			// copies each node on the path once.
			copied := func(next pmap[int]) int {
				n := 0
				for node := range pmapNodes(next) {
					if _, shared := base[node]; !shared {
						n++
					}
				}
				return n
			}
			const maxPath = 14 // 13 levels plus a collision node, at worst
			for _, c := range []struct {
				what string
				next pmap[int]
			}{
				{"insert", m.with("fresh-key", 1)},
				{"replace", m.with("row-7", -1)},
				{"remove", m.without("row-9")},
			} {
				if n := copied(c.next); n == 0 || n > maxPath {
					t.Fatalf("%s copied %d nodes of %d; want a path of at most %d", c.what, n, len(base), maxPath)
				}
			}
			if v, _ := m.get("row-7"); v != 7 || m.has("fresh-key") || !m.has("row-9") || m.len() != size {
				t.Fatal("a derived version changed the original")
			}
			fresh := m.with("fresh-key", 1)
			if fresh.len() != size+1 || !fresh.has("row-9") {
				t.Fatal("the derived version lost entries")
			}
		})
	}
}

// Writers that fork a map take fresh owners; after that no write by either
// shows on the other, and neither copies more than its path. This mirrors how
// an index and its clone use the map.
func TestPmapForkedWritersAreIsolated(t *testing.T) {
	for _, h := range pmapHashes {
		t.Run(h.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(23))
			type writer struct {
				m     pmap[int]
				o     *pmapOwner
				model map[string]int
			}
			ws := []*writer{{m: pmap[int]{hash: h.hash}, o: new(pmapOwner), model: map[string]int{}}}
			for i := 0; i < 6000; i++ {
				w := ws[rng.Intn(len(ws))]
				switch r := rng.Intn(20); {
				case r == 0 && len(ws) < 12:
					fork := &writer{m: w.m, o: new(pmapOwner), model: maps.Clone(w.model)}
					w.o = new(pmapOwner)
					ws = append(ws, fork)
				case r < 7:
					key := fmt.Sprintf("k%d", rng.Intn(300))
					w.m.del(w.o, key)
					delete(w.model, key)
				default:
					key := fmt.Sprintf("k%d", rng.Intn(300))
					w.m.put(w.o, key, i)
					w.model[key] = i
				}
			}
			for n, w := range ws {
				requirePmapEqual(t, fmt.Sprintf("writer %d", n), w.m, w.model)
			}
		})
	}
}

// Readers of a frozen map (a fork the writer has moved away from) may run
// concurrently with the writer. Run with -race.
func TestPmapFrozenCopyReadsWhileWriterWrites(t *testing.T) {
	m := pmap[int]{}
	o := new(pmapOwner)
	for i := 0; i < 2000; i++ {
		m.put(o, fmt.Sprintf("k%d", i), i)
	}
	frozen := m
	o = new(pmapOwner)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2000; i++ {
			if v, ok := frozen.get(fmt.Sprintf("k%d", i)); !ok || v != i {
				t.Errorf("frozen copy lost k%d", i)
				return
			}
		}
		n := 0
		for range frozen.all() {
			n++
		}
		if n != 2000 {
			t.Errorf("frozen copy iterates %d entries", n)
		}
	}()
	for i := 0; i < 2000; i++ {
		m.put(o, fmt.Sprintf("k%d", i), -i)
		m.del(o, fmt.Sprintf("k%d", i+1000))
	}
	<-done
}

func TestPmapIterationStopsEarly(t *testing.T) {
	m := pmap[int]{}
	for i := 0; i < 500; i++ {
		m = m.with(fmt.Sprintf("k%d", i), i)
	}
	n := 0
	for range m.all() {
		if n++; n == 3 {
			break
		}
	}
	if n != 3 {
		t.Fatalf("break after 3 saw %d", n)
	}
}

// FuzzPmapAgainstGoMap drives a pmap and a Go map with the same operations
// and compares them, over every hash flavour; the writers fork and keep
// writing, and a snapshot taken at each fork must not change afterwards.
func FuzzPmapAgainstGoMap(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13})
	f.Add([]byte("put put put delete fork put delete"))
	f.Fuzz(func(t *testing.T, ops []byte) {
		for _, h := range pmapHashes {
			type writer struct {
				m     pmap[int]
				o     *pmapOwner
				model map[string]int
			}
			type snapshot struct {
				m    pmap[int]
				want map[string]int
			}
			cur := &writer{m: pmap[int]{hash: h.hash}, o: new(pmapOwner), model: map[string]int{}}
			var other *writer
			var snaps []snapshot
			for i := 0; i+1 < len(ops); i += 2 {
				op, arg := ops[i], ops[i+1]
				w := cur
				if other != nil && op&0x80 != 0 {
					w = other
				}
				key := fmt.Sprintf("k%d", arg%64)
				switch op & 3 {
				case 0, 1:
					w.m.put(w.o, key, i)
					w.model[key] = i
				case 2:
					w.m.del(w.o, key)
					delete(w.model, key)
				case 3:
					snaps = append(snaps, snapshot{w.m, maps.Clone(w.model)})
					fork := &writer{m: w.m, o: new(pmapOwner), model: maps.Clone(w.model)}
					w.o = new(pmapOwner)
					other = fork
				}
			}
			requirePmapEqual(t, h.name+" current", cur.m, cur.model)
			if other != nil {
				requirePmapEqual(t, h.name+" fork", other.m, other.model)
			}
			for n, s := range snaps {
				requirePmapEqual(t, fmt.Sprintf("%s snapshot %d", h.name, n), s.m, s.want)
			}
		}
	})
}

// BenchmarkPmapWrite compares a write on a forked 48K-entry map (a path copy)
// with the Go map copy it replaces.
func BenchmarkPmapWrite(b *testing.B) {
	const size = 48000
	m := pmap[int]{}
	o := new(pmapOwner)
	model := make(map[string]int, size)
	for i := 0; i < size; i++ {
		k := fmt.Sprintf("file_topology(row-%d)", i)
		m.put(o, k, i)
		model[k] = i
	}
	b.Run("pmap-fork-and-write", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			fork := m
			fork.put(new(pmapOwner), fmt.Sprintf("probe-%d", i), i)
		}
	})
	b.Run("gomap-clone-and-write", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := maps.Clone(model)
			c[fmt.Sprintf("probe-%d", i)] = i
		}
	})
}
