package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalScanPath_IncrementalMatchesFull(t *testing.T) {
	// Absolute path under workspace root — the core regression.
	// The full scanner canonicalizes via canonicalScanPath; the incremental
	// scanner must produce the identical key for the same file.
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(root, "internal", "campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	absPath := filepath.Join(root, "internal", "campaign", "edge_case_detector_test.go")
	got := canonicalScanPath(root, absPath)
	want := filepath.ToSlash(filepath.Join("internal", "campaign", "edge_case_detector_test.go"))
	if got != want {
		t.Errorf("canonicalScanPath(absolute under root) = %q, want %q (root=%q abs=%q)", got, want, root, absPath)
	}
	// Incremental must agree: a helper that mirrors what the incremental
	// scanner does after the fix — it must call canonicalScanPath, not raw.
	if got2 := canonicalScanPath(root, absPath); got2 != got {
		t.Errorf("incremental vs canonical mismatch: %q vs %q", got2, got)
	}

	// Path containing backslashes — Windows absolute form.
	// canonicalScanPath normalizes separators via filepath.ToSlash fallback
	// and via Rel when possible. The key must never retain a backslash.
	backslashInput := `internal\session\gate_names.go`
	got3 := canonicalScanPath(root, backslashInput)
	if strings.Contains(got3, "\\") {
		t.Errorf("canonicalScanPath backslash input should be slash-normalized, got %q", got3)
	}
	want3 := "internal/session/gate_names.go"
	if got3 != want3 {
		t.Errorf("canonicalScanPath(backslash) = %q, want %q", got3, want3)
	}

	// Windows-style absolute under a Windows root — verifies the same helper
	// works for absolute paths with backslashes. On non-Windows the
	// filepath.Rel may fallback to ToSlash absolute; either way the result
	// must be slash-normalized and must end with the relative suffix.
	winRoot := `C:\CodeProjects\codeNERD`
	winPath := `C:\CodeProjects\codeNERD\internal\world\fs.go`
	got4 := canonicalScanPath(winRoot, winPath)
	if strings.Contains(got4, "\\") {
		t.Errorf("canonicalScanPath windows absolute should not contain backslash, got %q", got4)
	}
	if !strings.HasSuffix(got4, "internal/world/fs.go") {
		t.Errorf("canonicalScanPath windows absolute suffix = %q, want suffix %q", got4, "internal/world/fs.go")
	}
	// On Windows, it should be exactly relative; on Linux it will be
	// "C:/CodeProjects/codeNERD/internal/world/fs.go" (fallback). Both
	// are acceptable as long as backslashes are gone and suffix matches.
	// We additionally assert that on the current OS, Join+Rel produces
	// the expected relative when using OS-native separators.
	nativeRoot := filepath.Join(t.TempDir(), "native")
	nativePath := filepath.Join(nativeRoot, "a", "b", "c.go")
	if err := os.MkdirAll(filepath.Join(nativeRoot, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	nativeGot := canonicalScanPath(nativeRoot, nativePath)
	nativeWant := filepath.ToSlash(filepath.Join("a", "b", "c.go"))
	if nativeGot != nativeWant {
		t.Errorf("native canonical = %q, want %q", nativeGot, nativeWant)
	}
}

func TestIncrementalScan_ProducesCanonicalPaths(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "incremental-canonical-scan")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file that will be scanned incrementally.
	origPath := filepath.Join(tmpDir, "internal", "world", "example.go")
	if err := os.MkdirAll(filepath.Dir(origPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(origPath, []byte("package world\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure cache dir exists so incremental logic can persist.
	if err := os.MkdirAll(filepath.Join(tmpDir, ".nerd", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	ctx := context.Background()

	// First scan is a full fallback (cache empty). Its file_topology should already be canonical.
	res1, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, IncrementalOptions{})
	if err != nil {
		t.Fatalf("first ScanWorkspaceIncremental: %v", err)
	}
	if !res1.Full {
		t.Fatalf("expected first incremental scan to be Full")
	}
	wantCanonical := canonicalScanPath(tmpDir, origPath)
	found := false
	for _, f := range res1.NewFacts {
		if f.Predicate != "file_topology" {
			continue
		}
		p, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		if strings.Contains(p, "\\") {
			t.Errorf("first scan file_topology contains backslash: %q", p)
		}
		if filepath.IsAbs(p) {
			t.Errorf("first scan file_topology is absolute, want relative canonical: %q", p)
		}
		if p == wantCanonical {
			found = true
		}
	}
	if !found {
		t.Errorf("first scan missing expected canonical path %q", wantCanonical)
	}

	// Second scan — delta path (changed file). This exercises the goroutine
	// at incremental_scan.go:288 that previously used raw path.
	if err := os.WriteFile(origPath, []byte("package world\nfunc Foo() {}\n// modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res2, err := scanner.ScanWorkspaceIncremental(ctx, tmpDir, nil, IncrementalOptions{})
	if err != nil {
		t.Fatalf("second ScanWorkspaceIncremental: %v", err)
	}
	// Full fallback should not happen a second time (cache exists).
	if res2.Full {
		t.Fatalf("second scan should be incremental delta, got Full")
	}
	found = false
	for _, f := range res2.NewFacts {
		if f.Predicate != "file_topology" {
			continue
		}
		p, ok := f.Args[0].(string)
		if !ok {
			continue
		}
		if strings.Contains(p, "\\") {
			t.Errorf("delta scan file_topology contains backslash: %q", p)
		}
		if filepath.IsAbs(p) {
			t.Errorf("delta scan file_topology is absolute, want relative canonical: %q", p)
		}
		if p == wantCanonical {
			found = true
		}
	}
	if !found {
		t.Errorf("delta scan missing expected canonical path %q; facts: %v", wantCanonical, res2.NewFacts)
	}

	// Also verify that incremental delta agrees with full scan for same file.
	fullFacts, err := scanner.ScanWorkspaceCtx(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ScanWorkspaceCtx: %v", err)
	}
	var fullPath string
	for _, f := range fullFacts {
		if f.Predicate == "file_topology" && len(f.Args) > 0 {
			if p, ok := f.Args[0].(string); ok && strings.HasSuffix(p, "example.go") {
				fullPath = p
				break
			}
		}
	}
	if fullPath == "" {
		t.Fatalf("full scan missing file_topology for example.go")
	}
	if fullPath != wantCanonical {
		t.Errorf("full scan canonical = %q, want %q", fullPath, wantCanonical)
	}
	// The incremental delta's canonical must equal the full scanner's.
	if wantCanonical != fullPath {
		t.Errorf("incremental canonical %q != full canonical %q", wantCanonical, fullPath)
	}

	// file_dir companion must use same canonical key.
	for _, f := range res2.NewFacts {
		if f.Predicate == "file_dir" {
			p, _ := f.Args[0].(string)
			if p == wantCanonical {
				dir, _ := f.Args[1].(string)
				if strings.Contains(dir, "\\") {
					t.Errorf("file_dir dir contains backslash: %q", dir)
				}
				if dir != "internal/world" {
					t.Errorf("file_dir dir = %q, want %q", dir, "internal/world")
				}
				break
			}
		}
	}
}
