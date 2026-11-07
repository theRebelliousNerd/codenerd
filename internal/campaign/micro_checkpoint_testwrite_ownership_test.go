package campaign

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// campaign-testwrite-checkpoint-ownership: a test_write task that wrote
// nothing must not be credited with an unrelated dirty test file. Only test
// files beside its declared targets, changed since its attempt began, count.
func TestOwnedDirtyTestFiles_CreditsOnlyThisAttemptsTestsBesideItsTargets(t *testing.T) {
	ws := t.TempDir()
	for _, d := range []string{"internal/a", "internal/b"} {
		if err := os.MkdirAll(filepath.Join(ws, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel string, mod time.Time) {
		p := filepath.Join(ws, rel)
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	write("internal/a/old_test.go", start.Add(-time.Hour))  // predates the attempt
	write("internal/a/new_test.go", start.Add(time.Second)) // this attempt, beside the target
	write("internal/b/other_test.go", start.Add(time.Second))
	dirty := []string{"internal/a/old_test.go", "internal/a/new_test.go", "internal/b/other_test.go"}

	got := ownedDirtyTestFiles(ws, []string{"internal/a/a.go"}, dirty, start)
	if !slices.Equal(got, []string{"internal/a/new_test.go"}) {
		t.Fatalf("owned = %v, want only this attempt's test beside the target", got)
	}
	if got := ownedDirtyTestFiles(ws, []string{"internal/a/a.go"}, dirty, time.Time{}); got != nil {
		t.Fatalf("with no attempt start nothing may be credited, got %v", got)
	}
	if got := ownedDirtyTestFiles(ws, nil, dirty, start); got != nil {
		t.Fatalf("with no write set nothing may be credited, got %v", got)
	}
}
