package core

import (
	"iter"
	"slices"
	"sync/atomic"
)

// =============================================================================
// EDB FACT STORE
// =============================================================================
// edbFacts is the authoritative, ordered collection of the kernel's base facts.
// It replaced a dense []Fact whose every operation cost the size of the world:
// one retraction compared every fact (11 ms of 290K rows), and a clone copied
// the slice only for its first write to copy it again.
//
// Facts live in fixed-size chunks. A chunk is written only by the store that
// owns it; a clone copies the list of chunk pointers (one pointer per
// edbChunkSlots facts) and shares the chunks, and the first write to a shared
// chunk, on either side, copies that chunk alone (copy-on-write). Appends
// extend the last chunk. A removal leaves a tombstone in the fact's slot, so
// the order of the rest is untouched and nothing shifts; the edbIndex knows
// every fact's slot position (slotOf), so a removal by canonical key finds its
// slot without a scan. Tombstones are reclaimed when they outnumber a quarter
// of the slots (compact), which moves every position once: O(held rows) per
// held/4 removals, constant amortized.
//
// Positions are append order. They are stable until compact, which reports
// every move to the caller so the index can follow.

const (
	edbChunkShift = 8
	edbChunkSlots = 1 << edbChunkShift // facts per chunk
	edbChunkMask  = edbChunkSlots - 1

	// compactMinDead keeps small EDBs from compacting on every few removals.
	compactMinDead = 64
)

// edbSlot is one fact and the canonical key it is indexed under. An empty key
// marks a tombstone; no canonical key is empty (it starts with the predicate).
type edbSlot struct {
	fact Fact
	key  string
}

// edbOwner is a writer's identity. A chunk whose owner is not the store's
// current one is shared with a clone and must be copied before it is written.
type edbOwner struct{ _ byte }

type edbChunk struct {
	slots []edbSlot
	owner *edbOwner
}

type edbFacts struct {
	chunks []*edbChunk
	owner  *edbOwner
	n      int // slots in use, tombstones included: the next append's position
	dead   int // tombstones among them

	// shared is raised by snapshot, which runs under the kernel's read lock and
	// so may not touch owner; the next write, under the write lock, takes a new
	// owner (writable), which un-owns every chunk the clone can see.
	shared atomic.Bool

	// Work counters, written under the kernel's write lock except scans, which
	// readers bump. They let a test prove a write touched its slot and chunk
	// alone, whatever the clock says.
	chunkCopies int64
	tombstoned  int64
	compactions int64
	scans       atomic.Int64
}

// len is the number of live facts.
func (f *edbFacts) len() int { return f.n - f.dead }

// writable takes a fresh owner when a clone shares the chunks, and the first
// owner of a store that has none. Callers hold the kernel's write lock.
func (f *edbFacts) writable() {
	if f.owner == nil || f.shared.Load() {
		f.shared.Store(false)
		f.owner = new(edbOwner)
	}
}

// chunkForWrite returns chunk ci, copying it first when a clone shares it.
func (f *edbFacts) chunkForWrite(ci int) *edbChunk {
	c := f.chunks[ci]
	if c.owner == f.owner {
		return c
	}
	cp := &edbChunk{slots: make([]edbSlot, len(c.slots), edbChunkSlots), owner: f.owner}
	copy(cp.slots, c.slots)
	f.chunks[ci] = cp
	f.chunkCopies++
	return cp
}

// append stores fact under key at the end and returns its position.
func (f *edbFacts) append(fact Fact, key string) int {
	f.writable()
	pos := f.n
	ci := pos >> edbChunkShift
	if ci == len(f.chunks) {
		f.chunks = append(f.chunks, &edbChunk{slots: make([]edbSlot, 0, edbChunkSlots), owner: f.owner})
	}
	c := f.chunkForWrite(ci)
	c.slots = append(c.slots, edbSlot{fact: fact, key: key})
	f.n++
	return pos
}

// at returns the live fact at pos and its key.
func (f *edbFacts) at(pos int) (Fact, string, bool) {
	if pos < 0 || pos >= f.n {
		return Fact{}, "", false
	}
	s := f.chunks[pos>>edbChunkShift].slots[pos&edbChunkMask]
	return s.fact, s.key, s.key != ""
}

// replace swaps the fact at pos for fact under key, keeping its place in the
// order. pos must be live.
func (f *edbFacts) replace(pos int, fact Fact, key string) {
	f.writable()
	f.chunkForWrite(pos >> edbChunkShift).slots[pos&edbChunkMask] = edbSlot{fact: fact, key: key}
}

// remove tombstones the fact at pos. It reports false when pos holds none.
func (f *edbFacts) remove(pos int) bool {
	if _, _, live := f.at(pos); !live {
		return false
	}
	f.writable()
	f.chunkForWrite(pos >> edbChunkShift).slots[pos&edbChunkMask] = edbSlot{}
	f.dead++
	f.tombstoned++
	f.trim()
	return true
}

// trim drops the tombstones at the end: nothing indexes them, and a retract
// followed by the same assert (a heartbeat, a refreshed row) would otherwise
// leave a tombstone behind every time.
func (f *edbFacts) trim() {
	for f.n > 0 {
		ci := (f.n - 1) >> edbChunkShift
		c := f.chunks[ci]
		if c.slots[len(c.slots)-1].key != "" {
			return
		}
		c = f.chunkForWrite(ci)
		c.slots = c.slots[:len(c.slots)-1]
		f.n--
		f.dead--
		if len(c.slots) == 0 {
			f.chunks = f.chunks[:ci]
		}
	}
}

// each yields every live fact and its key in order. The store must not be
// written while it runs.
func (f *edbFacts) each() iter.Seq2[Fact, string] {
	return func(yield func(Fact, string) bool) {
		f.scans.Add(1)
		for _, c := range f.chunks {
			for _, s := range c.slots {
				if s.key != "" && !yield(s.fact, s.key) {
					return
				}
			}
		}
	}
}

// all yields every live fact in order.
func (f *edbFacts) all() iter.Seq[Fact] {
	return func(yield func(Fact) bool) {
		for fact := range f.each() {
			if !yield(fact) {
				return
			}
		}
	}
}

// slice returns the live facts in order as a new dense slice.
func (f *edbFacts) slice() []Fact {
	out := make([]Fact, 0, f.len())
	for fact := range f.each() {
		out = append(out, fact)
	}
	return out
}

// snapshotTo makes dst an independent copy of f: the chunk pointers are copied
// and the chunks shared copy-on-write, so a write to either side copies the
// chunk it touches and never shows on the other. It runs under the kernel's
// read lock (concurrent snapshots are safe), so it only flags f as shared.
func (f *edbFacts) snapshotTo(dst *edbFacts) {
	dst.chunks = slices.Clone(f.chunks)
	dst.owner = nil
	dst.n, dst.dead = f.n, f.dead
	dst.shared.Store(false)
	f.shared.Store(true)
}

// reset empties the store.
func (f *edbFacts) reset() {
	f.chunks, f.owner, f.n, f.dead = nil, nil, 0, 0
	f.shared.Store(false)
}

// adopt takes over src's facts; src is discarded by the caller.
func (f *edbFacts) adopt(src *edbFacts) {
	f.chunks, f.owner, f.n, f.dead = src.chunks, src.owner, src.n, src.dead
	f.shared.Store(src.shared.Load())
}

// needsCompaction reports whether tombstones outnumber a quarter of the slots.
func (f *edbFacts) needsCompaction() bool {
	return f.dead > compactMinDead && f.dead*4 > f.n
}

// compact rewrites the live facts densely, in order, into chunks of their own,
// and reports each fact's new position to moved. It is the one operation whose
// cost is the held rows.
func (f *edbFacts) compact(moved func(fact Fact, key string, pos int)) {
	f.writable()
	old := f.chunks
	f.chunks, f.n, f.dead = make([]*edbChunk, 0, len(old)), 0, 0
	for _, c := range old {
		for _, s := range c.slots {
			if s.key == "" {
				continue
			}
			pos := f.append(s.fact, s.key)
			if moved != nil {
				moved(s.fact, s.key, pos)
			}
		}
	}
	f.compactions++
}

// removeWhere tombstones every fact for which match is true, scanning all of
// them. It is for rebuilds; a retraction names its facts through the index.
func (f *edbFacts) removeWhere(match func(Fact) bool) (removed int) {
	pos := 0
	for _, c := range slices.Clone(f.chunks) {
		for _, s := range c.slots {
			if s.key != "" && match(s.fact) && f.remove(pos) {
				removed++
			}
			pos++
		}
	}
	return removed
}
