package testfacts

import (
	"strings"
	"testing"
)

const passSrc = `package ok

import "testing"

func TestOK(t *testing.T) {
	t.Log("hello")
}
`

// A real passing run yields one passing package, one passing test, no
// failures, and the test's own output lines intact.
func TestParsePass(t *testing.T) {
	dir := writeModule(t, map[string]string{"ok_test.go": passSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if res.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", res.Status)
	}
	if len(res.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(res.Packages))
	}
	p := res.Packages[0]
	if p.Name != "example.com/mod" || p.Status != StatusPass {
		t.Fatalf("package = %+v, want example.com/mod/pass", p)
	}
	if len(p.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(p.Tests))
	}
	ct := p.Tests[0]
	if ct.Name != "TestOK" || ct.Status != StatusPass {
		t.Fatalf("test = %+v, want TestOK/pass", ct)
	}
	joined := strings.Join(ct.Output, "\n")
	for _, want := range []string{"=== RUN   TestOK", "ok_test.go:6: hello", "--- PASS: TestOK"} {
		if !strings.Contains(joined, want) {
			t.Errorf("test output missing %q:\n%s", want, joined)
		}
	}
	if len(res.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none", res.Failures)
	}
	if len(res.Raw) != 0 {
		t.Fatalf("Raw = %q, want none", res.Raw)
	}
}

const errSrc = `package bad

import "testing"

func TestErr(t *testing.T) {
	t.Errorf("wrong value: got %d want %d", 1, 2)
}
`

// A real t.Errorf run records the failing test and one Failure per
// `file.go:NN: message` line; the passing-test log format is identical,
// so only failing tests contribute failures.
func TestParseErrorf(t *testing.T) {
	dir := writeModule(t, map[string]string{"bad_test.go": errSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if res.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", res.Status)
	}
	p := res.Packages[0]
	if p.Status != StatusFail {
		t.Fatalf("package status = %q, want fail", p.Status)
	}
	ct := findTest(t, p, "TestErr")
	if ct.Status != StatusFail {
		t.Fatalf("test status = %q, want fail", ct.Status)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want 1", res.Failures)
	}
	f := res.Failures[0]
	if f.Package != "example.com/mod" || f.Test != "TestErr" {
		t.Errorf("Failure = %+v, want package example.com/mod test TestErr", f)
	}
	if f.File != "bad_test.go" || f.Line != 6 {
		t.Errorf("Failure = %+v, want bad_test.go:6", f)
	}
	if f.Message != "wrong value: got 1 want 2" {
		t.Errorf("Message = %q", f.Message)
	}
}

const subSrc = `package sub

import "testing"

func TestSub(t *testing.T) {
	t.Run("case one", func(t *testing.T) { t.Log("sub out") })
	t.Run("case two", func(t *testing.T) { t.Fatalf("boom happened") })
}

func TestSkipIt(t *testing.T) { t.Skip("not today") }
`

// Real subtests keep their test2json names (spaces sanitized to
// underscores); the parent fails with no failure lines of its own, the
// skipped test skips, and only the failing leaf yields a Failure.
func TestParseSubtestsAndSkip(t *testing.T) {
	dir := writeModule(t, map[string]string{"sub_test.go": subSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	p := res.Packages[0]
	names := map[string]Status{}
	for _, ct := range p.Tests {
		names[ct.Name] = ct.Status
	}
	want := map[string]Status{
		"TestSub":          StatusFail,
		"TestSub/case_one": StatusPass,
		"TestSub/case_two": StatusFail,
		"TestSkipIt":       StatusSkip,
	}
	if len(names) != len(want) {
		t.Fatalf("tests = %v, want %v", names, want)
	}
	for name, st := range want {
		if names[name] != st {
			t.Errorf("test %s status = %q, want %q", name, names[name], st)
		}
	}
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly the leaf failure", res.Failures)
	}
	f := res.Failures[0]
	if f.Test != "TestSub/case_two" || f.File != "sub_test.go" || f.Line != 7 {
		t.Errorf("Failure = %+v, want TestSub/case_two sub_test.go:7", f)
	}
	if f.Message != "boom happened" {
		t.Errorf("Message = %q", f.Message)
	}
	// The leaf's own output stays whole on the leaf.
	leaf := findTest(t, p, "TestSub/case_two")
	if got := strings.Join(leaf.Output, "\n"); !strings.Contains(got, "boom happened") {
		t.Errorf("leaf output missing message:\n%s", got)
	}
}

// A package with no test files reports skip with the marker, which the
// parser resolves to no-test-files rather than a bare skip.
func TestParseNoTestFiles(t *testing.T) {
	dir := writeModule(t, map[string]string{"doc.go": "package nt\n"})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if res.Status != StatusNoTestFiles {
		t.Fatalf("Status = %q, want no-test-files", res.Status)
	}
	p := res.Packages[0]
	if p.Status != StatusNoTestFiles {
		t.Fatalf("package status = %q, want no-test-files", p.Status)
	}
	if len(p.Tests) != 0 {
		t.Fatalf("tests = %+v, want none", p.Tests)
	}
}
