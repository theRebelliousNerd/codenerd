package testfacts

import (
	"math"

	"codenerd/internal/types"
)

// Kernel predicate names asserted from test results. A later lane adds
// the .mg declarations and wires Facts() into the gates; failing_test is
// already declared (schemas_reviewer.mg) and read by
// policy/context_compilation.mg, and nothing in Go asserts it yet.
const (
	// PredTestCase names test_case(Package, Test, Status, ElapsedMs):
	// one per test with a verdict. Status is a name: /pass, /fail, /skip.
	PredTestCase = "test_case"
	// PredTestFailureAt names test_failure_at(Package, Test, File, Line,
	// Message): one per failure line a failing test printed.
	PredTestFailureAt = "test_failure_at"
	// PredTestBuildFailure names test_build_failure(Package, File, Line,
	// Message): one per compiler diagnostic in build output.
	PredTestBuildFailure = "test_build_failure"
	// PredTestOutputRepeat names test_output_repeat(Line, Count): one per
	// output line seen more than once across the run.
	PredTestOutputRepeat = "test_output_repeat"
	// PredFailingTest names the already-declared failing_test(Test,
	// Message): one per failing test, with its first failure's message.
	PredFailingTest = "failing_test"
)

// Test status atoms for test_case's Status argument.
const (
	atomPass types.MangleAtom = "/pass"
	atomFail types.MangleAtom = "/fail"
	atomSkip types.MangleAtom = "/skip"
)

// Facts renders the Result as kernel facts in predicate-group order, each
// group in the Result's deterministic order. Package, test, file, line,
// and message arguments use MangleString so values containing slashes
// (subtests, paths) can never parse as names; only Status is a MangleAtom.
// Tests without a verdict contribute no test_case: the /pass /fail /skip
// vocabulary cannot say "no verdict", and the Result still carries them.
func (r *Result) Facts() []types.Fact {
	if r == nil {
		return nil
	}
	var out []types.Fact
	for _, p := range r.Packages {
		for _, t := range p.Tests {
			atom, ok := statusAtom(t.Status)
			if !ok {
				continue
			}
			out = append(out, types.Fact{
				Predicate: PredTestCase,
				Args: []any{
					types.MangleString(p.Name),
					types.MangleString(t.Name),
					atom,
					elapsedMs(t.Elapsed),
				},
			})
		}
	}
	for _, f := range r.Failures {
		out = append(out, types.Fact{
			Predicate: PredTestFailureAt,
			Args: []any{
				types.MangleString(f.Package),
				types.MangleString(f.Test),
				types.MangleString(f.File),
				f.Line,
				types.MangleString(f.Message),
			},
		})
	}
	for _, b := range r.BuildFailures {
		out = append(out, types.Fact{
			Predicate: PredTestBuildFailure,
			Args: []any{
				types.MangleString(b.Package),
				types.MangleString(b.File),
				b.Line,
				types.MangleString(b.Message),
			},
		})
	}
	for _, rp := range r.Repeats {
		out = append(out, types.Fact{
			Predicate: PredTestOutputRepeat,
			Args: []any{
				types.MangleString(rp.Line),
				rp.Count,
			},
		})
	}
	for _, ft := range firstFailures(r.Failures) {
		out = append(out, types.Fact{
			Predicate: PredFailingTest,
			Args: []any{
				types.MangleString(ft.Test),
				types.MangleString(ft.Message),
			},
		})
	}
	return out
}

// statusAtom maps a test verdict to its kernel name; unknown has none.
func statusAtom(s Status) (types.MangleAtom, bool) {
	switch s {
	case StatusPass:
		return atomPass, true
	case StatusFail:
		return atomFail, true
	case StatusSkip:
		return atomSkip, true
	}
	return "", false
}

// elapsedMs converts test2json seconds to whole milliseconds.
func elapsedMs(seconds float64) int64 {
	return int64(math.Round(seconds * 1000))
}

// firstFailures keeps the first Failure per package+test pair. The
// predicate carries no package, so two packages failing the same test
// name yield two facts; that collision is the schema's shape, not this
// transducer's to resolve.
func firstFailures(failures []Failure) []Failure {
	var out []Failure
	seen := make(map[[2]string]bool)
	for _, f := range failures {
		key := [2]string{f.Package, f.Test}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
