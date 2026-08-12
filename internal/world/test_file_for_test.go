package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Verifies the test_file_for(TestFile, SourceFile) emission added at
// fs.go:362 and incremental_scan.go:314. Both args are types.MangleString
// in canonical (workspace-relative, slash-normalized) form matching
// file_topology's Path slot.
//
// Full-scan path (ScanDirectory) is exercised in every test via t.TempDir
// with real files on disk so the os.Stat guard is hit. The incremental delta
// path is exercised in TestTestFileFor_IncrementalDeltaEmitsPair — see its
// comment for the one limitation that cannot be re-verified as a same-scan
// join on delta.

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %q: %v", path, err)
	}
}

func collectByPredicate(facts []core.Fact, pred string) []core.Fact {
	var out []core.Fact
	for _, f := range facts {
		if f.Predicate == pred {
			out = append(out, f)
		}
	}
	return out
}

func mustMangleString(t *testing.T, f core.Fact, idx int) types.MangleString {
	t.Helper()
	ms, ok := f.Args[idx].(types.MangleString)
	if !ok {
		t.Fatalf("test_file_for arg[%d] type %T, want types.MangleString", idx, f.Args[idx])
	}
	if _, isAtom := f.Args[idx].(types.MangleAtom); isAtom {
		t.Fatalf("test_file_for arg[%d] is MangleAtom, want MangleString", idx)
	}
	return ms
}

func assertCanonical(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, "\\") {
		t.Errorf("path contains backslash (not canonical): %q", s)
	}
	if filepath.IsAbs(s) {
		t.Errorf("path is absolute, want workspace-relative: %q", s)
	}
	if len(s) >= 2 && s[1] == ':' {
		t.Errorf("path begins with drive letter: %q", s)
	}
}

func keysOf(m map[string]core.Fact) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestTestFileFor_PairedProducesExactlyOne — case 1: directory containing
// foo.go and foo_test.go produces exactly one pairing, test file first.
func TestTestFileFor_PairedProducesExactlyOne(t *testing.T) {
	tmpDir := t.TempDir()
	fooPath := filepath.Join(tmpDir, "foo.go")
	fooTestPath := filepath.Join(tmpDir, "foo_test.go")
	writeFile(t, fooPath, "package foo\nfunc Foo() {}\n")
	writeFile(t, fooTestPath, "package foo\nfunc TestFoo(t *testing.T) {}\n")

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) != 1 {
		t.Fatalf("expected exactly 1 test_file_for, got %d", len(tff))
	}
	f := tff[0]
	if len(f.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(f.Args))
	}
	testArg := mustMangleString(t, f, 0)
	srcArg := mustMangleString(t, f, 1)

	expectedTest := canonicalScanPath(tmpDir, fooTestPath)
	expectedSource := canonicalScanPath(tmpDir, fooPath)
	if string(testArg) != expectedTest {
		t.Errorf("TestFile = %q, want %q", string(testArg), expectedTest)
	}
	if string(srcArg) != expectedSource {
		t.Errorf("SourceFile = %q, want %q", string(srcArg), expectedSource)
	}
	if !strings.HasSuffix(string(testArg), "_test.go") {
		t.Errorf("first arg should have _test.go suffix, got %q", string(testArg))
	}
	if strings.HasSuffix(string(srcArg), "_test.go") {
		t.Errorf("second arg should not have _test.go suffix, got %q", string(srcArg))
	}
	assertCanonical(t, string(testArg))
	assertCanonical(t, string(srcArg))
}

// TestTestFileFor_OrphanTestProducesNoFact — case 2: orphan test file.
func TestTestFileFor_OrphanTestProducesNoFact(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "foo_test.go"), "package foo\nfunc TestFoo(t *testing.T) {}\n")
	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) != 0 {
		t.Fatalf("expected 0 test_file_for for orphan, got %d: %v", len(tff), tff)
	}
}

// TestTestFileFor_PlainSourceProducesNoFact — case 3: source without test.
func TestTestFileFor_PlainSourceProducesNoFact(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "foo.go"), "package foo\nfunc Foo() {}\n")
	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) != 0 {
		t.Fatalf("expected 0 test_file_for, got %d", len(tff))
	}
}

// TestTestFileFor_NonGoSuffixProducesNoFact — case 4: foo_test.txt.
func TestTestFileFor_NonGoSuffixProducesNoFact(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "foo_test.txt"), "just text\n")
	writeFile(t, filepath.Join(tmpDir, "foo.go"), "package foo\nfunc Foo() {}\n")
	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) != 0 {
		t.Fatalf("expected 0 for foo_test.txt, got %d: %v", len(tff), tff)
	}
}

// TestTestFileFor_SourcePathIdenticalToFileTopology — case 5: SourceFile
// byte-identical to file_topology Path in same scan.
func TestTestFileFor_SourcePathIdenticalToFileTopology(t *testing.T) {
	tmpDir := t.TempDir()
	fooPath := filepath.Join(tmpDir, "foo.go")
	fooTestPath := filepath.Join(tmpDir, "foo_test.go")
	writeFile(t, fooPath, "package foo\nfunc Foo() {}\n")
	writeFile(t, fooTestPath, "package foo\nfunc TestFoo(t *testing.T) {}\n")

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) != 1 {
		t.Fatalf("expected 1 test_file_for, got %d", len(tff))
	}
	srcArg := string(mustMangleString(t, tff[0], 1))
	testArg := string(mustMangleString(t, tff[0], 0))

	topoByPath := map[string]core.Fact{}
	for _, f := range collectByPredicate(result.Facts, "file_topology") {
		if len(f.Args) < 1 {
			continue
		}
		p, ok := f.Args[0].(string)
		if ok {
			topoByPath[p] = f
		}
	}
	if _, ok := topoByPath[srcArg]; !ok {
		t.Fatalf("SourceFile %q has no byte-identical file_topology Path; keys: %v", srcArg, keysOf(topoByPath))
	}
	if _, ok := topoByPath[testArg]; !ok {
		t.Fatalf("TestFile %q has no file_topology Path; keys: %v", testArg, keysOf(topoByPath))
	}
	expectedSrc := canonicalScanPath(tmpDir, fooPath)
	expectedTest := canonicalScanPath(tmpDir, fooTestPath)
	if srcArg != expectedSrc {
		t.Errorf("SourceFile not canonical: got %q want %q", srcArg, expectedSrc)
	}
	if testArg != expectedTest {
		t.Errorf("TestFile not canonical: got %q want %q", testArg, expectedTest)
	}
	// Byte-identical check via direct map lookup already proved equality.
}

// TestTestFileFor_ArgsAreMangleStringNotAtom — case 6: both args stored as
// strings (MangleString), not MangleAtom.
func TestTestFileFor_ArgsAreMangleStringNotAtom(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "foo.go"), "package foo\nfunc Foo() {}\n")
	writeFile(t, filepath.Join(tmpDir, "foo_test.go"), "package foo\nfunc TestFoo(t *testing.T) {}\n")

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	tff := collectByPredicate(result.Facts, "test_file_for")
	if len(tff) == 0 {
		t.Fatal("expected at least 1 test_file_for")
	}
	for _, f := range tff {
		if len(f.Args) != 2 {
			t.Fatalf("expected 2 args, got %d", len(f.Args))
		}
		for i := range 2 {
			ms := mustMangleString(t, f, i)
			s := f.String()
			quoted := "\"" + string(ms) + "\""
			if !strings.Contains(s, quoted) {
				t.Errorf("fact.String() %q should contain quoted %q", s, quoted)
			}
		}
		if _, err := f.ToAtom(); err != nil {
			t.Fatalf("ToAtom failed: %v", err)
		}
	}
}

// TestTestFileFor_IncrementalDeltaEmitsPair exercises the incremental scanner
// delta path (incremental_scan.go:314). Full-scan fallback is covered above;
// this forces the delta by seeding the cache then adding the test file.
//
// Limitation: delta only re-emits file_topology for new/changed files, so the
// source file_topology is absent from the second delta's NewFacts. The
// byte-identical join (case 5) is therefore proven in the full-scan test;
// here we verify pairing, canonical form, and type contract on the delta.
func TestTestFileFor_IncrementalDeltaEmitsPair(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	scanner := NewScanner()
	fooPath := filepath.Join(tmpDir, "foo.go")
	fooTestPath := filepath.Join(tmpDir, "foo_test.go")

	writeFile(t, fooPath, "package foo\nfunc Foo() {}\n")
	res1, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, IncrementalOptions{})
	if err != nil {
		t.Fatalf("first incremental: %v", err)
	}
	if !res1.Full {
		t.Fatalf("first scan should be Full")
	}
	if len(collectByPredicate(res1.NewFacts, "test_file_for")) != 0 {
		t.Fatal("first scan should have 0 test_file_for")
	}

	writeFile(t, fooTestPath, "package foo\nfunc TestFoo(t *testing.T) {}\n")
	res2, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, IncrementalOptions{})
	if err != nil {
		t.Fatalf("second incremental: %v", err)
	}
	if res2.Full {
		t.Fatalf("second scan should be delta")
	}
	tff := collectByPredicate(res2.NewFacts, "test_file_for")
	if len(tff) != 1 {
		t.Fatalf("delta expected 1 test_file_for, got %d", len(tff))
	}
	f := tff[0]
	testArg := mustMangleString(t, f, 0)
	srcArg := mustMangleString(t, f, 1)
	if string(testArg) != canonicalScanPath(tmpDir, fooTestPath) {
		t.Errorf("delta TestFile = %q", string(testArg))
	}
	if string(srcArg) != canonicalScanPath(tmpDir, fooPath) {
		t.Errorf("delta SourceFile = %q", string(srcArg))
	}
	assertCanonical(t, string(testArg))
	assertCanonical(t, string(srcArg))
}
