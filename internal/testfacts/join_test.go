package testfacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// These tests pin the join contract the kernel needs. World facts identify a
// file by types.CanonicalPath: workspace-relative, slash-separated, cleaned.
// A failure whose File is ./x_test.go, a bare basename, or an absolute drive
// path does not join file_topology or test_file_for.

const subErrSrc = `package sub

import "testing"

func TestE(t *testing.T) { t.Errorf("boom") }
`

// A t.Errorf line names only the basename (testing.decorate truncates to the
// last path separator). The package directory comes from the import path, so
// the fact's File is the workspace path, not the basename.
func TestSubpackageErrorfFileIsWorkspaceRelative(t *testing.T) {
	dir := writeModule(t, map[string]string{"sub/x_test.go": subErrSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "./sub"))
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want 1", res.Failures)
	}
	if res.Failures[0].File != "sub/x_test.go" {
		t.Errorf("File = %q, want sub/x_test.go", res.Failures[0].File)
	}
}

const subBuildSrc = `package sub

import "testing"

func TestBroken(t *testing.T) { undefinedSymbol() }
`

// Running from the module root, Go 1.26 prints the diagnostic relative to
// that directory (sub\bf_test.go), which is already the workspace path once
// separators are slashes. Joining the package directory again would yield
// sub/sub/bf_test.go and miss file_topology.
func TestSubpackageBuildFileStaysWorkspaceRelative(t *testing.T) {
	dir := writeModule(t, map[string]string{"sub/bf_test.go": subBuildSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "./sub"))
	if len(res.BuildFailures) != 1 {
		t.Fatalf("BuildFailures = %+v, want 1", res.BuildFailures)
	}
	if res.BuildFailures[0].File != "sub/bf_test.go" {
		t.Errorf("File = %q, want sub/bf_test.go", res.BuildFailures[0].File)
	}
}

// At the module root the compiler prints .\bf_test.go. The leading ./ is not
// part of the canonical identity; file_topology stores bf_test.go.
func TestRootBuildFileDropsDotSlash(t *testing.T) {
	dir := writeModule(t, map[string]string{"bf_test.go": buildFailSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if len(res.BuildFailures) != 1 {
		t.Fatalf("BuildFailures = %+v, want 1", res.BuildFailures)
	}
	if res.BuildFailures[0].File != "bf_test.go" {
		t.Errorf("File = %q, want bf_test.go", res.BuildFailures[0].File)
	}
	found := false
	for _, f := range res.Facts() {
		if f.Predicate != PredFailingTest {
			continue
		}
		found = true
		assertFact(t, f, PredFailingTest, []any{
			types.MangleString(""),
			types.MangleString("undefined: undefinedSymbol"),
		})
	}
	if !found {
		t.Fatalf("build failure produced no failing_test in %v", res.Facts())
	}
}

const bothSrc = `package sub

import "testing"

func TestBoth(t *testing.T) {
	t.Errorf("before panic")
	panic("after error")
}
`

// The t.Errorf basename and the panic's absolute frame are the same file.
// Both facts have to spell it the same way or test_failure_at does not join
// itself, let alone file_topology.
func TestErrorfAndPanicShareCanonicalFile(t *testing.T) {
	dir := writeModule(t, map[string]string{"sub/both_test.go": bothSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "./sub"))
	if len(res.Failures) != 2 {
		t.Fatalf("Failures = %+v, want the Errorf and the panic", res.Failures)
	}
	for _, f := range res.Failures {
		if f.File != "sub/both_test.go" {
			t.Errorf("File = %q, want sub/both_test.go (%+v)", f.File, f)
		}
	}
}

// frameRe's \S+ stops at the first space, so a panic under a directory whose
// name contains a space records the message and an empty file.
func TestPanicFrameWithSpace(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "my module")
	if err := writeModuleAt(t, dir, map[string]string{
		"p_test.go": "package pc\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) { panic(\"space boom\") }\n",
	}); err != nil {
		t.Fatal(err)
	}
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want 1", res.Failures)
	}
	if res.Failures[0].File != "p_test.go" || res.Failures[0].Line != 5 {
		t.Errorf("Failure = %+v, want p_test.go:5", res.Failures[0])
	}
	if res.Failures[0].Message != "space boom" {
		t.Errorf("Message = %q, want space boom", res.Failures[0].Message)
	}
}

// A compiler-shaped line that reaches the test-output scanner
// (file.go:5:33: msg) is a line and a column, not a line whose message
// starts with the column.
func TestCompilerShapeInTestOutputKeepsMessage(t *testing.T) {
	stream := `{"Action":"run","Package":"example.com/mod","Test":"TestC"}` + "\n" +
		`{"Action":"output","Package":"example.com/mod","Test":"TestC","Output":"    file.go:5:33: undefined: x\n"}` + "\n" +
		`{"Action":"fail","Package":"example.com/mod","Test":"TestC"}` + "\n" +
		`{"Action":"fail","Package":"example.com/mod"}` + "\n"
	res := parseString(t, "", stream)
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want 1", res.Failures)
	}
	f := res.Failures[0]
	if f.Line != 5 || f.Message != "undefined: x" {
		t.Errorf("Failure = %+v, want line 5 message %q", f, "undefined: x")
	}
}

const repeatErrSrc = `package rp

import "testing"

func TestDup(t *testing.T) {
	for i := 0; i < 5; i++ {
		t.Errorf("same")
	}
}
`

// A loop of the same t.Errorf is one test_failure_at, with the count on it.
// Emitting one fact per printed line re-expands the flood this package exists
// to collapse.
func TestRepeatedErrorfIsOneFact(t *testing.T) {
	dir := writeModule(t, map[string]string{"rp_test.go": repeatErrSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	var n, count int
	for _, f := range res.Facts() {
		if f.Predicate != PredTestFailureAt {
			continue
		}
		n++
		if len(f.Args) >= 6 {
			if c, ok := f.Args[5].(int64); ok {
				count = int(c)
			}
		}
	}
	if n != 1 || count != 5 {
		t.Fatalf("test_failure_at facts = %d count = %d, want 1 fact with count 5\n%v", n, count, res.Facts())
	}
}

const fakePanicSrc = `package fp

import "testing"

func TestFake(t *testing.T) { t.Errorf("saw panic: in the text") }
`

// The runtime prints a recovered panic as a line that starts with "panic:".
// A t.Errorf whose text contains that substring is not a second failure.
func TestPanicSubstringIsNotAPanic(t *testing.T) {
	dir := writeModule(t, map[string]string{"fp_test.go": fakePanicSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "."))
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want only the Errorf", res.Failures)
	}
	if res.Failures[0].Message != "saw panic: in the text" {
		t.Errorf("Message = %q", res.Failures[0].Message)
	}
}

const initSrc = `package sub

import "testing"

func init() { panic("init boom") }

func TestNever(t *testing.T) {}
`

// An init panic is package output: empty Test, package fail. Nothing scans
// that output, so the repair rule (failing_test) stays dark and the frame
// never becomes a file fact.
func TestInitPanicIsPackageFailure(t *testing.T) {
	dir := writeModule(t, map[string]string{"sub/init_test.go": initSrc})
	res := parseString(t, dir, runGoTestJSON(t, dir, "./sub"))
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want the init panic", res.Failures)
	}
	f := res.Failures[0]
	if f.Test != "" || f.File != "sub/init_test.go" || f.Line != 5 || f.Message != "init boom" {
		t.Errorf("Failure = %+v, want package-level sub/init_test.go:5 init boom", f)
	}
	found := false
	for _, fact := range res.Facts() {
		if fact.Predicate != PredFailingTest {
			continue
		}
		found = true
		assertFact(t, fact, PredFailingTest, []any{
			types.MangleString(""),
			types.MangleString("init boom"),
		})
	}
	if !found {
		t.Fatalf("init panic produced no failing_test in %v", res.Facts())
	}
	if !strings.Contains(res.Summary(), "FAIL example.com/mod/sub sub/init_test.go:5: init boom") {
		t.Errorf("summary = %q, want the package-level FAIL line", res.Summary())
	}
}

// writeModuleAt is writeModule aimed at a directory the caller chose, so a
// test can put the module under a path that contains a space.
func writeModuleAt(t *testing.T, dir string, files map[string]string) error {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/mod\n\ngo 1.26.0\n"), 0o644)
}
