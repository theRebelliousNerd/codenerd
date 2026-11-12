package core

import (
	"runtime"
	"sync"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// preparedFact is a fact ready for the EDB: interned, keyed for dedupe and,
// when converted, admitted as its store atom (or the reason it was refused).
type preparedFact struct {
	fact      Fact
	key       string
	atom      ast.Atom
	err       error
	converted bool
}

// parallelPrepareMinFacts is the batch size from which LoadFacts prepares
// facts on every core. Interning, keying and admitting a fact read only the
// fact, the analyzed program and the admission cache, so the work splits
// cleanly; below this size the goroutines cost more than they save.
//
// Measured 2026-11-12 on codeNERD's own world model (575k facts): preparing
// was 1.4 s of LoadFacts' 3.6 s, all of it on one core while the kernel lock
// was held at boot.
const parallelPrepareMinFacts = 4096

// prepareFactsLocked interns, keys and converts every fact of a load. Each
// prepared fact is exactly what addFactIfNewLockedErr computes for it, in the
// same order, so inserting them one by one keeps LoadFacts' semantics: the
// first of two equal facts wins, a refused fact is reported as before. Caller
// holds k.mu exclusively; the workers only read the kernel.
func (k *RealKernel) prepareFactsLocked(facts []Fact) []preparedFact {
	k.ensureFactIndexLocked()
	out := make([]preparedFact, len(facts))
	prepare := func(i int) {
		f := internFact(sanitizeFactForNumericPredicates(facts[i]))
		p := preparedFact{fact: f, key: k.canonFact(f)}
		p.atom, p.err = k.factToAtomLocked(f)
		p.converted = true
		out[i] = p
	}
	workers := runtime.GOMAXPROCS(0)
	// Before the first evaluation factToAtomLocked marks the atom cache stale,
	// a kernel write; keep that case on one goroutine.
	if len(facts) < parallelPrepareMinFacts || workers < 2 || k.programInfo == nil {
		for i := range facts {
			prepare(i)
		}
		return out
	}
	// The admission cache is replaced when the program changed; do that here,
	// once, so the workers only look entries up in it.
	k.ensureAdmissionSchemaLocked()
	chunk := (len(facts) + workers - 1) / workers
	var wg sync.WaitGroup
	for start := 0; start < len(facts); start += chunk {
		end := min(start+chunk, len(facts))
		wg.Go(func() {
			for i := start; i < end; i++ {
				prepare(i)
			}
		})
	}
	wg.Wait()
	return out
}

// insertPreparedLoadLocked is addFactIfNewLocked for a prepared fact: the EDB
// limit, then the insert.
func (k *RealKernel) insertPreparedLoadLocked(p *preparedFact) bool {
	if err := k.checkFactLimitLocked(p.fact.Predicate); err != nil {
		return false
	}
	added, _ := k.insertPreparedFactLocked(p)
	return added
}
