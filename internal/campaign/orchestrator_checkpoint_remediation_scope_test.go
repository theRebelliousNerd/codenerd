package campaign

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// campaign-checkpoint-scope-idempotence #1: a remediation task's write set is
// the previous scope (directories). The next round must not take their
// parents, or the scope widens one level per failed checkpoint.
func TestPhaseWriteScope_IsIdempotentOverItsOwnOutput(t *testing.T) {
	ws := t.TempDir()
	pkg := filepath.Join(ws, "internal", "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "a.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	phase := &Phase{Tasks: []Task{{ID: "/t1", WriteSet: []string{"internal/pkg/a.go"}}}}
	first := phaseWriteScope(ws, phase)
	phase.Tasks = append(phase.Tasks, Task{ID: "/t1_ckpt_1", WriteSet: first})
	second := phaseWriteScope(ws, phase)
	phase.Tasks = append(phase.Tasks, Task{ID: "/t1_ckpt_2", WriteSet: second})
	third := phaseWriteScope(ws, phase)
	if !slices.Equal(first, second) || !slices.Equal(second, third) {
		t.Fatalf("scope widened across rounds: %v -> %v -> %v", first, second, third)
	}
	if len(first) != 1 || filepath.Base(first[0]) != "pkg" {
		t.Fatalf("scope = %v, want the package directory", first)
	}
}
