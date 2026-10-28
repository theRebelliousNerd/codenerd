package problemlog

import (
	"errors"

	"codenerd/internal/types"
)

// The facts a Report becomes (policy/self_problems.mg declares them). The
// chat asserts DebugSelfActivePredicate itself while /debug-self is on; it is
// the gate on injecting the hot problems into the model's context.
const (
	ProblemPredicate         = "self_problem"
	SeenPredicate            = "self_problem_seen"
	OmittedPredicate         = "self_problem_omitted"
	DebugSelfActivePredicate = "debug_self_active"
)

const (
	levelErrorAtom = types.MangleAtom("/error")
	levelWarnAtom  = types.MangleAtom("/warn")
)

// Predicates are the base predicates Facts writes, for retract-and-replace.
var Predicates = []string{ProblemPredicate, SeenPredicate, OmittedPredicate}

// Facts projects the report as self_problems.mg's base facts: one
// self_problem and one self_problem_seen per distinct problem, and a
// self_problem_omitted when the cut left problems out. Strings are explicit
// MangleStrings so a message that looks like a name stays a string.
func (r *Report) Facts() []types.Fact {
	if r == nil {
		return nil
	}
	out := make([]types.Fact, 0, 2*len(r.Problems)+1)
	for _, p := range r.Problems {
		level := levelWarnAtom
		if p.Level == LevelError {
			level = levelErrorAtom
		}
		sig := types.MangleString(p.Signature)
		cat := types.MangleString(p.Category)
		out = append(out,
			types.Fact{Predicate: ProblemPredicate, Args: []any{sig, level, cat, int64(p.Count), types.MangleString(p.Sample)}},
			types.Fact{Predicate: SeenPredicate, Args: []any{sig, level, cat, p.First.Unix(), p.Last.Unix()}},
		)
	}
	if r.Omitted() {
		out = append(out, types.Fact{Predicate: OmittedPredicate, Args: []any{int64(r.OmittedProblems), int64(r.OmittedEntries)}})
	}
	return out
}

// Kernel is what Replace needs of a kernel.
type Kernel interface {
	Retract(predicate string) error
	AssertBatch(facts []types.Fact) error
}

// Replace swaps the kernel's problem facts for the report's: a fresh read
// supersedes the last one wholesale, since counts and last-seen only grow and
// a problem that fell out of the cut must not linger. Every retraction error
// and the assert error come back joined.
func Replace(k Kernel, r *Report) error {
	var errs []error
	for _, pred := range Predicates {
		if err := k.Retract(pred); err != nil {
			errs = append(errs, err)
		}
	}
	if facts := r.Facts(); len(facts) > 0 {
		if err := k.AssertBatch(facts); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// HotPredicate is self_problem_hot(Signature, Level, Category): what the
// kernel judges worth diagnosing (self_problems.mg).
const HotPredicate = "self_problem_hot"

// Querier is what Hot needs of a kernel.
type Querier interface {
	Query(predicate string) ([]types.Fact, error)
}

// Hot returns the report's problems the kernel derives as hot, in the
// report's order (most urgent first). The kernel decides; this only joins
// its verdict back to the whole problems.
func (r *Report) Hot(k Querier) ([]Problem, error) {
	if r == nil {
		return nil, nil
	}
	rows, err := k.Query(HotPredicate)
	if err != nil {
		return nil, err
	}
	type key struct{ sig, cat string }
	hot := make(map[key]bool, len(rows))
	for _, f := range rows {
		if len(f.Args) != 3 {
			continue
		}
		hot[key{types.ExtractString(f.Args[0]), types.ExtractString(f.Args[2])}] = true
	}
	var out []Problem
	for _, p := range r.Problems {
		if hot[key{p.Signature, p.Category}] {
			out = append(out, p)
		}
	}
	return out, nil
}
