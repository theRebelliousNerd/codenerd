package testfacts

// Status is a package, test, or overall run verdict. The vocabulary covers
// every verdict test2json can report; bench maps to StatusPass at parse
// time because the test2json manual defines bench as "printed log output
// but did not fail".
type Status string

// Verdict vocabulary for Result, Package, and Test.
const (
	// StatusPass is a clean run: tests passed, package ok.
	StatusPass Status = "pass"
	// StatusFail covers failed tests and failed packages.
	StatusFail Status = "fail"
	// StatusSkip is a package-level skip without the no-test-files marker
	// (test2json emits skip for the package only when it holds no tests,
	// but the marker check keeps an unexpected skip distinguishable).
	StatusSkip Status = "skip"
	// StatusBuildFailed marks a package whose fail event carries
	// FailedBuild, or that a build-fail event names.
	StatusBuildFailed Status = "build-failed"
	// StatusNoTestFiles is a package skip whose output holds the
	// "[no test files]" marker.
	StatusNoTestFiles Status = "no-test-files"
	// StatusUnknown means no verdict event arrived: a test that started
	// but never reported (binary died mid-run), or a Result parsed from a
	// stream with no package events at all.
	StatusUnknown Status = "unknown"
)

// Result is one parsed `go test -json` stream. Slices are sorted for
// determinism (parallel `go test` interlaces package events
// nondeterministically); only Raw keeps stream order, since it is the
// fallback record of a stream that had no structure to sort by.
type Result struct {
	// Status is the worst package verdict by severity
	// (build-failed > fail > unknown > pass > skip > no-test-files),
	// or unknown when the stream held no package events.
	Status Status
	// Packages holds one entry per tested package, sorted by Name.
	Packages []*Package
	// Failures holds one entry per `file.go:NN: message` line printed by
	// a failing test (plus one per panic), in package, test, then output
	// order.
	Failures []Failure
	// BuildFailures holds compiler diagnostics from build-output events,
	// in package, file, line, then column order.
	BuildFailures []BuildFailure
	// Repeats counts every output line seen more than once across the
	// run, sorted by count descending then line ascending so the flood
	// line -- the failure mode this package exists for -- sorts first.
	Repeats []Repeat
	// Raw holds stream lines that are not JSON objects, in stream order.
	Raw []string
}

// Package is one tested package's verdict and output.
type Package struct {
	// Name is the test2json Package field, e.g. "example.com/mod/pkg".
	Name string
	// Status is the package verdict; see Status for the vocabulary.
	Status Status
	// Elapsed is the verdict event's Elapsed field, in seconds.
	Elapsed float64
	// Tests holds one entry per test and subtest, sorted by Name.
	// Subtests keep their test2json form ("TestSub/case_one": spaces in
	// t.Run names arrive sanitized to underscores).
	Tests []*Test
	// Output holds the package's own output lines: build-output lines
	// for this package first (the build precedes the run), then
	// package-level output event lines.
	Output []string
}

// Test is one test's (or subtest's, benchmark's) verdict and full output.
// Nothing is dropped here: repeats are counted in Result.Repeats but
// every line stays in its test's Output.
type Test struct {
	// Name is the test2json Test field including subtest suffixes.
	Name string
	// Status is pass, fail, skip, or unknown (no verdict event).
	Status Status
	// Elapsed is the verdict event's Elapsed field, in seconds.
	Elapsed float64
	// Output holds every output line of this test, in stream order.
	Output []string
}

// Failure is one distinct `file.go:NN: message` from a failing test's
// output, a continuation of such a line, a panic record, or a
// package-level panic (Test is empty: an init panic never starts a test).
// t.Log, t.Errorf, and t.Fatalf share one output format, so every matching
// line in a failing test counts; passing and skipped tests contribute
// none. File is workspace-relative slash form, or absolute slash form
// when the file is outside the workspace.
type Failure struct {
	Package string
	Test    string
	File    string
	Line    int
	Message string
	// Count is how many times this package, test, file, line, and message
	// was printed. One t.Errorf in a loop is one Failure.
	Count int
}

// BuildFailure is one compiler diagnostic (`file.go:line:col: message`)
// from a build-output event. File uses the same identity as Failure.File.
type BuildFailure struct {
	Package string
	File    string
	Line    int
	Column  int
	Message string
}

// Repeat counts one output line seen more than once anywhere in the run:
// test output, package output, build output, or raw lines.
type Repeat struct {
	Line         string
	Count        int
	FirstPackage string
	FirstTest    string
}
