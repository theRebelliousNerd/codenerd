package problemlog

import (
	"errors"
	"fmt"

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

// problemLevelAtom is the single mapping from a Problem level to its kernel
// /name: every error is /error, everything else (WARN or an unexpected
// level) is /warn. Facts and Hot must agree, so both go through here;
// two copies of the if would drift.
func problemLevelAtom(level string) types.MangleAtom {
	if level == LevelError {
		return levelErrorAtom
	}
	return levelWarnAtom
}

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
		level := problemLevelAtom(p.Level)
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
	if k == nil {
		return errors.New("problemlog: nil Kernel")
	}
	var errs []error
	preds := append([]string(nil), Predicates...)
	for _, pred := range preds {
		if err := k.Retract(pred); err != nil {
			errs = append(errs, fmt.Errorf("problemlog: retract %s: %w", pred, err))
		}
	}
	if facts := r.Facts(); len(facts) > 0 {
		if err := k.AssertBatch(facts); err != nil {
			errs = append(errs, fmt.Errorf("problemlog: assert %d facts: %w", len(facts), err))
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

// factString extracts the join key text of a hot fact argument. Query readback
// renders a /name as a plain string while freshly built facts carry a
// MangleAtom, and signatures carry MangleString: all three must compare
// equal, so each is unwrapped explicitly instead of relying on fmt fallback.
func factString(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case types.MangleString:
		return string(v)
	case types.MangleAtom:
		return string(v)
	default:
		return types.ExtractString(arg)
	}
}

// normalizeLevelName keeps the hot join stable when the kernel renders a
// level /name as "warn" instead of "/warn": both spellings join the same.
func normalizeLevelName(s string) string {
	if s == "" {
		return s
	}
	if s[0] == '/' {
		return s
	}
	return "/" + s
}

// Hot returns the report's problems the kernel derives as hot, in the
// report's order (most urgent first). The kernel decides; this only joins
// its verdict back to the whole problems.
func (r *Report) Hot(k Querier) ([]Problem, error) {
	if k == nil {
		return nil, errors.New("problemlog: nil Querier")
	}
	if r == nil {
		return nil, nil
	}
	rows, err := k.Query(HotPredicate)
	if err != nil {
		return nil, fmt.Errorf("problemlog: query %s: %w", HotPredicate, err)
	}
	type key struct{ sig, level, cat string }
	hot := make(map[key]bool, len(rows))
	for _, f := range rows {
		if len(f.Args) != 3 {
			continue
		}
		hot[key{factString(f.Args[0]), normalizeLevelName(factString(f.Args[1])), factString(f.Args[2])}] = true
	}
	out := make([]Problem, 0, len(r.Problems))
	for _, p := range r.Problems {
		level := normalizeLevelName(string(problemLevelAtom(p.Level)))
		if hot[key{p.Signature, level, p.Category}] {
			out = append(out, p)
		}
	}
	return out, nil
}
