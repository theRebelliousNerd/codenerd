package prompt

import (
	"sort"
	"sync"
)

// This file answers Q2: do prompt atoms actually cluster the way the lane
// taxonomy assumes?
//
// The Phase 5 lane taxonomy -- architecture / implementation / verification --
// is how human software teams are organized. That is a fact about org charts,
// not evidence about how information clusters. The natural cut in the data
// might be "code under active edit versus everything else", or per-package, or
// something nobody would guess. Hard-coding an org chart as an information
// architecture is cheap to do and expensive to undo, so measure first.
//
// The measurement is co-use: which atoms get selected together in turns that
// succeeded. Nothing here decides anything; it makes the decision answerable.

// Outcome is what happened to a turn whose atom selection was recorded.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

// coUseLimits bound the recorder so a long session cannot turn it into a leak.
const (
	// maxPendingTurns bounds turns awaiting an outcome. A turn that never
	// settles is evicted oldest-first and its selections counted as dropped, so
	// lost evidence shows up as a number rather than as a quietly smaller
	// sample.
	maxPendingTurns = 512

	// maxSelectionsPerTurn bounds how many compilations one turn may contribute.
	// A turn that compiles in a loop would otherwise let a single outcome
	// dominate the whole sample.
	maxSelectionsPerTurn = 64

	// maxTrackedPairs bounds the pair matrix. On reaching it the recorder stops
	// admitting *new* pairs but keeps counting the ones it has, and sets
	// Truncated on the report. A partial report that says it is partial is
	// usable; one that silently drops the tail is not.
	maxTrackedPairs = 250_000
)

// pairKey identifies an unordered atom pair. Ordering the two ids on insert is
// what makes (A,B) and (B,A) the same cell instead of two half-counted ones.
type pairKey struct{ a, b string }

func makePairKey(x, y string) pairKey {
	if x > y {
		x, y = y, x
	}
	return pairKey{x, y}
}

// counts holds the per-outcome tallies shared by atoms and pairs.
type counts struct {
	success int
	failure int
}

// CoUseRecorder accumulates which atoms are selected together, split by the
// outcome of the turn the selection was made for.
//
// The sample unit is one *selection* -- one prompt compilation -- not one turn.
// A single turn can compile several prompts: the turn's own, and one per shard
// it spawns. Merging those into a turn-level set would report atoms as co-used
// when they were never in the same prompt, which is precisely the claim this
// analysis exists to test. Outcomes are still attributed per turn, because that
// is the granularity at which success is known.
//
// It stores tallies rather than histories: the analysis needs marginal and
// joint frequencies, and keeping every selection would grow without bound for
// no extra answer.
type CoUseRecorder struct {
	mu sync.Mutex

	selections counts
	atoms      map[string]*counts
	pairs      map[pairKey]*counts
	pending    map[string][][]string
	order      []string // pending turn ids, oldest first

	dropped      int // selections evicted before an outcome arrived
	unattributed int // selections observed with no turn identity
	truncated    bool

	catState
	logState
}

// NewCoUseRecorder returns an empty recorder.
func NewCoUseRecorder() *CoUseRecorder {
	return &CoUseRecorder{
		atoms:   make(map[string]*counts),
		pairs:   make(map[pairKey]*counts),
		pending: make(map[string][][]string),
	}
}

// Observe records one prompt compilation's atom selection against a turn. The
// selection is held until Settle supplies the turn's outcome, because a co-use
// statistic over turns whose result is unknown answers a different and much
// less interesting question.
//
// Several selections may be observed for one turn; each stays a separate
// sample. Duplicate atom ids *within* one selection are collapsed: an atom
// included twice is still one piece of evidence about co-use, and counting it
// twice would let a duplication bug masquerade as a correlation.
func (r *CoUseRecorder) Observe(turnID string, atomIDs []string) {
	if r == nil || len(atomIDs) == 0 {
		return
	}
	if turnID == "" {
		// No turn identity means no outcome will ever arrive, so this selection
		// cannot join the sample. Count it rather than dropping it silently:
		// a large number here means a whole class of compilations is invisible
		// to the analysis, which is a wiring gap, not an absence of evidence.
		r.mu.Lock()
		r.unattributed++
		r.mu.Unlock()
		return
	}

	seen := make(map[string]struct{}, len(atomIDs))
	uniq := make([]string, 0, len(atomIDs))
	for _, id := range atomIDs {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return
	}
	sort.Strings(uniq)

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.pending[turnID]
	if !exists {
		r.order = append(r.order, turnID)
	}
	if len(existing) >= maxSelectionsPerTurn {
		r.dropped++
		return
	}
	r.pending[turnID] = append(existing, uniq)

	for len(r.order) > maxPendingTurns {
		oldest := r.order[0]
		r.order = r.order[1:]
		if lost, still := r.pending[oldest]; still {
			delete(r.pending, oldest)
			r.dropped += len(lost)
		}
	}
}

// Settle applies an outcome to every selection observed for a turn and folds
// them into the tallies. A turn that was never observed, or that settles twice,
// is ignored: double counting one turn's atoms would inflate every pair it
// contains against every pair it does not.
func (r *CoUseRecorder) Settle(turnID string, outcome Outcome) {
	if r == nil || turnID == "" {
		return
	}

	r.mu.Lock()
	selections, ok := r.pending[turnID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.pending, turnID)
	for i, id := range r.order {
		if id == turnID {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	for _, atoms := range selections {
		r.foldLocked(atoms, outcome)
	}
	r.mu.Unlock()

	// Persist outside the tally mutex. Writing to disk under the lock that
	// every Observe contends on would put file I/O on the compilation path,
	// which is the one place this measurement must not be felt.
	for _, atoms := range selections {
		r.appendToLog(atoms, outcome)
	}
}

// foldLocked accumulates one selection. Caller holds r.mu.
func (r *CoUseRecorder) foldLocked(atoms []string, outcome Outcome) {
	bump(&r.selections, outcome)

	for _, id := range atoms {
		c, ok := r.atoms[id]
		if !ok {
			c = &counts{}
			r.atoms[id] = c
		}
		bump(c, outcome)
	}

	for i := 0; i < len(atoms); i++ {
		for j := i + 1; j < len(atoms); j++ {
			key := makePairKey(atoms[i], atoms[j])
			c, ok := r.pairs[key]
			if !ok {
				if len(r.pairs) >= maxTrackedPairs {
					r.truncated = true
					continue
				}
				c = &counts{}
				r.pairs[key] = c
			}
			bump(c, outcome)
		}
	}
}

func bump(c *counts, outcome Outcome) {
	if outcome == OutcomeFailure {
		c.failure++
		return
	}
	c.success++
}
