package core

import (
	"hash/maphash"
	"iter"
	"math/bits"
	"slices"
	"strings"
)

// =============================================================================
// PERSISTENT STRING-KEYED MAP
// =============================================================================
// pmap is a hash array mapped trie keyed by string, with structural sharing:
// copying a pmap header is O(1) and the copy shares every node, and a write
// copies only the nodes on the path to the key (one per level, log32 n of
// them), never the map. The EDB index keeps one pmap per predicate and
// field, so a kernel clone that stages a row of a 48K-row predicate copies a
// handful of nodes where a Go map copied the predicate (4.7 ms measured
// 2026-11-06, kernel_edb_index.go).
//
// Writes take an owner. A node created under an owner may be written in place
// by that owner; every other node is copied first. A writer that hands out a
// copy (a kernel clone) then takes a fresh owner, which makes every node it
// had created foreign to itself and to the copy at once: the next write on
// either side copies its own path and the other side never sees it. The nil
// owner owns nothing, which gives the purely persistent with/without.
//
// Shape is canonical: it depends only on the keys held, not on the order they
// were written (a removal that leaves a node holding a single entry folds the
// entry into the parent). Iteration order is therefore deterministic for a key
// set within one process, and callers must not rely on it beyond that.
//
// A pmap must not be written while it is being iterated, and an owned pmap
// must not be iterated by one goroutine while its owner writes it from
// another; frozen copies (made by handing out a header and taking a fresh
// owner) are safe to read concurrently.

const (
	pmapBits  = 5
	pmapMask  = 1<<pmapBits - 1
	pmapDepth = 64 // shifts at or beyond this have no hash bits left: collision node
)

// pmapOwner identifies a writer. The byte keeps distinct owners at distinct
// addresses.
type pmapOwner struct{ _ byte }

var pmapSeed = maphash.MakeSeed()

func pmapDefaultHash(key string) uint64 { return maphash.String(pmapSeed, key) }

// pmapSlot is a stored key and value; child is set instead when the slot
// descends into a deeper node.
type pmapSlot[V any] struct {
	key   string
	val   V
	child *pmapNode[V]
}

// pmapNode holds its slots in bit order. Below pmapDepth the bitmap says which
// of the 32 hash chunks are present and kids is dense; a collision node (every
// hash bit used) keeps its leaf entries sorted by key with a zero bitmap.
type pmapNode[V any] struct {
	owner  *pmapOwner
	bitmap uint32
	kids   []pmapSlot[V]
}

// pmap is the map header. The zero value is an empty map. hash overrides the
// key hash (nil means the process-seeded default); it exists so tests can force
// collisions.
type pmap[V any] struct {
	root *pmapNode[V]
	n    int
	hash func(string) uint64
}

func (m pmap[V]) hashOf(key string) uint64 {
	if m.hash != nil {
		return m.hash(key)
	}
	return pmapDefaultHash(key)
}

// len is the number of entries.
func (m pmap[V]) len() int { return m.n }

// get returns the value held under key.
func (m pmap[V]) get(key string) (V, bool) {
	var zero V
	n := m.root
	if n == nil {
		return zero, false
	}
	h := m.hashOf(key)
	for shift := uint(0); ; shift += pmapBits {
		if shift >= pmapDepth {
			if i, found := n.collided(key); found {
				return n.kids[i].val, true
			}
			return zero, false
		}
		bit := uint32(1) << ((h >> shift) & pmapMask)
		if n.bitmap&bit == 0 {
			return zero, false
		}
		s := &n.kids[bits.OnesCount32(n.bitmap&(bit-1))]
		if s.child == nil {
			if s.key == key {
				return s.val, true
			}
			return zero, false
		}
		n = s.child
	}
}

// has reports whether key is held.
func (m pmap[V]) has(key string) bool {
	_, ok := m.get(key)
	return ok
}

// all yields every entry, in an order that depends only on the keys held.
func (m pmap[V]) all() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		if m.root != nil {
			m.root.each(yield)
		}
	}
}

func (n *pmapNode[V]) each(yield func(string, V) bool) bool {
	for i := range n.kids {
		s := &n.kids[i]
		if s.child != nil {
			if !s.child.each(yield) {
				return false
			}
		} else if !yield(s.key, s.val) {
			return false
		}
	}
	return true
}

// editable returns n when o may write it, else a copy that o owns with room
// for one more slot.
func (n *pmapNode[V]) editable(o *pmapOwner) *pmapNode[V] {
	if o != nil && n.owner == o {
		return n
	}
	kids := make([]pmapSlot[V], len(n.kids), len(n.kids)+1)
	copy(kids, n.kids)
	return &pmapNode[V]{owner: o, bitmap: n.bitmap, kids: kids}
}

// put stores val under key, writing nodes o owns in place and copying the
// rest. It reports whether the key is new.
func (m *pmap[V]) put(o *pmapOwner, key string, val V) (added bool) {
	h := m.hashOf(key)
	if m.root == nil {
		m.root = &pmapNode[V]{owner: o}
	}
	m.root, added = m.root.put(o, m, h, 0, key, val)
	if added {
		m.n++
	}
	return added
}

func (n *pmapNode[V]) put(o *pmapOwner, m *pmap[V], h uint64, shift uint, key string, val V) (*pmapNode[V], bool) {
	w := n.editable(o)
	if shift >= pmapDepth {
		i, found := w.collided(key)
		if found {
			w.kids[i].val = val
			return w, false
		}
		w.kids = slices.Insert(w.kids, i, pmapSlot[V]{key: key, val: val})
		return w, true
	}
	bit := uint32(1) << ((h >> shift) & pmapMask)
	idx := bits.OnesCount32(w.bitmap & (bit - 1))
	if w.bitmap&bit == 0 {
		w.kids = slices.Insert(w.kids, idx, pmapSlot[V]{key: key, val: val})
		w.bitmap |= bit
		return w, true
	}
	s := &w.kids[idx]
	switch {
	case s.child != nil:
		child, added := s.child.put(o, m, h, shift+pmapBits, key, val)
		s.child = child
		return w, added
	case s.key == key:
		s.val = val
		return w, false
	}
	held := *s
	s.key, s.val = "", *new(V)
	s.child = m.pair(o, shift+pmapBits, held.key, held.val, m.hashOf(held.key), key, val, h)
	return w, true
}

// pair builds the subtree holding two entries whose hashes agree on every
// chunk above shift.
func (m *pmap[V]) pair(o *pmapOwner, shift uint, k1 string, v1 V, h1 uint64, k2 string, v2 V, h2 uint64) *pmapNode[V] {
	n := &pmapNode[V]{owner: o}
	if shift >= pmapDepth {
		n.kids = []pmapSlot[V]{{key: k1, val: v1}, {key: k2, val: v2}}
		if k2 < k1 {
			n.kids[0], n.kids[1] = n.kids[1], n.kids[0]
		}
		return n
	}
	c1, c2 := (h1>>shift)&pmapMask, (h2>>shift)&pmapMask
	switch {
	case c1 == c2:
		n.kids = []pmapSlot[V]{{child: m.pair(o, shift+pmapBits, k1, v1, h1, k2, v2, h2)}}
		n.bitmap = 1 << c1
	case c1 < c2:
		n.kids = []pmapSlot[V]{{key: k1, val: v1}, {key: k2, val: v2}}
		n.bitmap = 1<<c1 | 1<<c2
	default:
		n.kids = []pmapSlot[V]{{key: k2, val: v2}, {key: k1, val: v1}}
		n.bitmap = 1<<c1 | 1<<c2
	}
	return n
}

// del drops key, writing nodes o owns in place and copying the rest. It
// reports whether the key was held; a map that did not hold it is untouched.
func (m *pmap[V]) del(o *pmapOwner, key string) (removed bool) {
	if m.root == nil {
		return false
	}
	root, removed := m.root.del(o, m.hashOf(key), 0, key)
	if !removed {
		return false
	}
	m.root = root
	m.n--
	return true
}

// del returns the node that replaces n (nil when n empties) and whether key
// was removed.
func (n *pmapNode[V]) del(o *pmapOwner, h uint64, shift uint, key string) (*pmapNode[V], bool) {
	if shift >= pmapDepth {
		if i, found := n.collided(key); found {
			return n.without(o, 0, i), true
		}
		return n, false
	}
	bit := uint32(1) << ((h >> shift) & pmapMask)
	if n.bitmap&bit == 0 {
		return n, false
	}
	idx := bits.OnesCount32(n.bitmap & (bit - 1))
	s := &n.kids[idx]
	if s.child == nil {
		if s.key != key {
			return n, false
		}
		return n.without(o, bit, idx), true
	}
	child, removed := s.child.del(o, h, shift+pmapBits, key)
	if !removed {
		return n, false
	}
	if child == nil {
		return n.without(o, bit, idx), true
	}
	w := n.editable(o)
	if len(child.kids) == 1 && child.kids[0].child == nil {
		// One entry left below: it belongs in this node, where it was
		// before the colliding sibling arrived.
		w.kids[idx] = child.kids[0]
	} else {
		w.kids[idx].child = child
	}
	return w, true
}

// without returns n minus slot idx (whose bitmap bit is bit; zero in a
// collision node), nil when nothing is left.
func (n *pmapNode[V]) without(o *pmapOwner, bit uint32, idx int) *pmapNode[V] {
	if len(n.kids) == 1 {
		return nil
	}
	w := n.editable(o)
	w.kids = slices.Delete(w.kids, idx, idx+1)
	w.bitmap &^= bit
	return w
}

// with returns m plus key=val as a new map; m is unchanged.
func (m pmap[V]) with(key string, val V) pmap[V] {
	m.put(nil, key, val)
	return m
}

// without returns m minus key as a new map; m is unchanged.
func (m pmap[V]) without(key string) pmap[V] {
	m.del(nil, key)
	return m
}

// collided finds key in a collision node, whose entries are sorted by key.
func (n *pmapNode[V]) collided(key string) (int, bool) {
	return slices.BinarySearchFunc(n.kids, key, func(s pmapSlot[V], k string) int { return strings.Compare(s.key, k) })
}
