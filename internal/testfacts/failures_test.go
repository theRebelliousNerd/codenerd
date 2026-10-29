package testfacts

import (
	"strings"
	"testing"
)

const buildFailSrc = `package bf

import "testing"

func TestBroken(t *testing.T) { undefinedSymbol() }
`

// A real build failure arrives as Go 1.26 build-output/build-fail events
// plus a package fail naming FailedBuild: the package resolves to
// build-failed and the compiler diagnostic becomes one BuildFailure with
// the file, line, column, and message the compiler printed.
func TestParseBuildFailure(t *testing.T) {
	dir := writeModule(t, map[string]string{"bf_test.go": buildFailSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if res.Status != StatusBuildFailed {
		t.Fatalf("Status = %q, want build-failed", res.Status)
	}
	if len(res.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(res.Packages))
	}
	p := res.Packages[0]
	if p.Status != StatusBuildFailed {
		t.Fatalf("package status = %q, want build-failed", p.Status)
	}
	if len(res.BuildFailures) != 1 {
		t.Fatalf("BuildFailures = %+v, want 1", res.BuildFailures)
	}
	bf := res.BuildFailures[0]
	if bf.Package != "example.com/mod" {
		t.Errorf("Package = %q", bf.Package)
	}
	if bf.File != "bf_test.go" || bf.Line != 5 || bf.Column != 33 {
		t.Errorf("BuildFailure = %+v, want bf_test.go:5:33", bf)
	}
	if bf.Message != "undefined: undefinedSymbol" {
		t.Errorf("Message = %q", bf.Message)
	}
	// The diagnostic lines stay in the package output for recall.
	if got := strings.Join(p.Output, "\n"); !strings.Contains(got, "undefined: undefinedSymbol") {
		t.Errorf("package output lost the diagnostic:\n%s", got)
	}
}

const panicSrc = `package pc

import "testing"

func TestPanic(t *testing.T) { panic("kaboom panic") }
`

// A real panic yields one Failure holding the panic message and the first
// stack frame in the workspace -- the test file, not the testing/runtime
// frames the repanicked trace starts with.
func TestParsePanic(t *testing.T) {
	dir := writeModule(t, map[string]string{"panic_test.go": panicSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	p := res.Packages[0]
	ct := findTest(t, p, "TestPanic")
	if ct.Status != StatusFail {
		t.Fatalf("test status = %q, want fail", ct.Status)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want 1", res.Failures)
	}
	f := res.Failures[0]
	if f.Test != "TestPanic" {
		t.Errorf("Test = %q", f.Test)
	}
	if f.Message != "kaboom panic" {
		t.Errorf("Message = %q, want the bare panic message", f.Message)
	}
	// The frame arrives absolute (`C:/Users/.../panic_test.go`, slash-spelled
	// even on Windows). CanonicalPath against the run directory is the
	// workspace-relative identity file facts use.
	if f.File != "panic_test.go" {
		t.Errorf("File = %q, want panic_test.go", f.File)
	}
	if f.Line != 5 {
		t.Errorf("Line = %d, want 5", f.Line)
	}
}

const multiSrc = `package ml

import "testing"

func TestMulti(t *testing.T) {
	t.Errorf("first problem")
	t.Errorf("second problem")
}
`

// Two Errorf calls are two Failures in output order; the first feeds
// failing_test.
func TestParseMultipleFailures(t *testing.T) {
	dir := writeModule(t, map[string]string{"ml_test.go": multiSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if len(res.Failures) != 2 {
		t.Fatalf("Failures = %+v, want 2", res.Failures)
	}
	if res.Failures[0].Message != "first problem" || res.Failures[0].Line != 6 {
		t.Errorf("first = %+v", res.Failures[0])
	}
	if res.Failures[1].Message != "second problem" || res.Failures[1].Line != 7 {
		t.Errorf("second = %+v", res.Failures[1])
	}
}

const parentSrc = `package par

import "testing"

func TestParent(t *testing.T) {
	t.Errorf("parent note")
	t.Run("sub", func(t *testing.T) { t.Errorf("sub note") })
}
`

// A parent's own failure line must not swallow its children's "--- FAIL"
// markers as continuations: markers terminate the message.
func TestParseParentMarkersNotContinuations(t *testing.T) {
	dir := writeModule(t, map[string]string{"par_test.go": parentSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if len(res.Failures) != 2 {
		t.Fatalf("Failures = %+v, want 2", res.Failures)
	}
	if res.Failures[0].Test != "TestParent" || res.Failures[0].Message != "parent note" {
		t.Errorf("parent = %+v, want bare message", res.Failures[0])
	}
	if res.Failures[1].Test != "TestParent/sub" || res.Failures[1].Message != "sub note" {
		t.Errorf("child = %+v", res.Failures[1])
	}
}
