package testfacts

import (
	"math"

	"codenerd/internal/types"
)

// Kernel predicate names asserted from test results. The declarations
// live in schemas_reviewer.mg next to failing_test. failing_test is what
// policy/context_compilation.mg reads (any row means tests are failing).
const (
	// PredTestCase names test_case(Package, Test, Status, ElapsedMs):
	// one per test with a verdict. Status is a name: /pass, /fail, /skip.
	PredTestCase = "test_case"
	// PredTestFailureAt names test_failure_at(Package, Test, File, Line,
	// Message, Count): one per distinct failure location. Count is how
	// many times that line was printed. Test is empty for a package-level
	// panic.
	PredTestFailureAt = "test_failure_at"
	// PredTestBuildFailure names test_build_failure(Package, File, Line,
	// Message): one per compiler diagnostic in build output.
	PredTestBuildFailure = "test_build_failure"
	// PredTestOutputRepeat names test_output_repeat(Line, Count): one per
	// output line seen more than once across the run.
	PredTestOutputRepeat = "test_output_repeat"
	// PredFailingTest names failing_test(Test, Message): one per failing
	// test, with its first failure's message, and one with an empty Test
	// for a package that failed to build or panicked before any test ran.
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
				int64(f.Line),
				types.MangleString(f.Message),
				int64(failureCount(f.Count)),
			},
		})
	}
	for _, b := range r.BuildFailures {
		out = append(out, types.Fact{
			Predicate: PredTestBuildFailure,
			Args: []any{
				types.MangleString(b.Package),
				types.MangleString(b.File),
				int64(b.Line),
				types.MangleString(b.Message),
			},
		})
	}
	for _, rp := range r.Repeats {
		out = append(out, types.Fact{
			Predicate: PredTestOutputRepeat,
			Args: []any{
				types.MangleString(rp.Line),
				int64(rp.Count),
			},
		})
	}
	for _, ft := range r.failingTests() {
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

// failureCount is the repeat count on a test_failure_at row. dedupeFailures
// stores at least 1; a hand-built Failure with a zero Count is still one
// occurrence, not zero.
func failureCount(n int) int {
	if n < 1 {
		return 1
	}
	return n
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

// failingTests is every firstFailures row plus one package-level row for
// a build that produced no test failure. Test is empty on that row: no
// test ran, and the Decl is (TestName, ErrorMessage), so an empty name is
// the only value that cannot be mistaken for a test the runner executed.
// The message is the first compiler diagnostic, or "build failed" when
// the build-fail event carried no file:line:col line.
//
// The Decl has no package argument, so two packages whose messages match
// collapse to one fact. The file stays on test_build_failure, which does
// carry the package. context_compilation.mg only tests failing_test(_, _)
// for existence, so the collapse does not leave a build failure dark.
func (r *Result) failingTests() []Failure {
	out := firstFailures(r.Failures)
	have := make(map[string]bool, len(out))
	for _, f := range out {
		have[f.Package] = true
	}
	seen := make(map[string]bool)
	for _, b := range r.BuildFailures {
		if have[b.Package] || seen[b.Package] {
			continue
		}
		seen[b.Package] = true
		msg := b.Message
		if msg == "" {
			msg = "build failed"
		}
		out = append(out, Failure{Package: b.Package, Message: msg})
	}
	for _, p := range r.Packages {
		if p.Status != StatusBuildFailed || have[p.Name] || seen[p.Name] {
			continue
		}
		out = append(out, Failure{Package: p.Name, Message: "build failed"})
	}
	return out
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
