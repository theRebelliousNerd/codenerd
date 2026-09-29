// limits_cleanup_test.go pins the LIMITS CLEANUP behavior changes in package
// system: model-facing prompt content passes whole instead of being cut at
// hardcoded byte/rune bounds.
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// The repair model fixes the previous invalid output, so it must see all of
// it: an 800-byte cut hid the exact syntax it must not repeat.
func TestRepairPrompt_PreviousInvalidOutputReturnedWhole(t *testing.T) {
	shard := NewMangleRepairShard()
	lastResponse := strings.Repeat("broken syntax fragment ", 100) + "TAIL-MARKER-800-PLUS"
	if len(lastResponse) <= 800 {
		t.Fatalf("fixture too short to exercise the old 800-byte cut: %d", len(lastResponse))
	}
	got := shard.buildRepairPrompt("next_action(/start).", []string{"some error"}, nil, lastResponse, "")
	if !strings.Contains(got, lastResponse) {
		t.Error("repair prompt dropped part of the previous invalid output")
	}
	if !strings.Contains(got, "TAIL-MARKER-800-PLUS") {
		t.Error("repair prompt lost the tail past byte 800")
	}
}

// Predicate descriptions carry the argument gloss the repair model needs to
// use the predicate correctly; a 50-rune cut hid it.
func TestFormatSelectedPredicate_LongDescriptionReturnedWhole(t *testing.T) {
	desc := strings.Repeat("argument gloss ", 20) + "TAIL"
	p := prompt.SelectedPredicate{Name: "permitted", Arity: 1, Domain: "safety", Description: desc}
	got := formatSelectedPredicate(p)
	if !strings.Contains(got, desc) {
		t.Errorf("predicate line cut the description: %q", got)
	}
	if strings.HasSuffix(strings.TrimSpace(got), "...") {
		t.Errorf("predicate line still carries a truncation ellipsis: %q", got)
	}
}

// An incremental scan that hits MaxFilesPerScan defers the remainder to the
// next tick instead of dropping it: deferred files keep stale entries, so
// successive ticks converge on the full set.
func TestWorldModelIncrementalScan_DefersPastCapToNextTick(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(p, []byte("package pkg\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultWorldModelConfig()
	cfg.RootPath = dir
	cfg.MaxFilesPerScan = 2
	w := NewWorldModelIngestorShardWithConfig(cfg)
	w.Kernel = kernel
	ctx := context.Background()

	if err := w.performIncrementalScan(ctx); err != nil {
		t.Fatalf("tick 1: %v", err)
	}
	if got := len(w.files); got != 2 {
		t.Fatalf("tick 1 tracked %d files, want 2 (cap)", got)
	}
	if err := w.performIncrementalScan(ctx); err != nil {
		t.Fatalf("tick 2: %v", err)
	}
	if got := len(w.files); got != 4 {
		t.Fatalf("tick 2 tracked %d files, want 4", got)
	}
	if err := w.performIncrementalScan(ctx); err != nil {
		t.Fatalf("tick 3: %v", err)
	}
	if got := len(w.files); got != 5 {
		t.Fatalf("tick 3 tracked %d files, want all 5; the cap dropped files", got)
	}
}

// A non-positive MaxFilesPerScan means unbounded: one tick takes everything.
func TestWorldModelIncrementalScan_NonPositiveCapIsUnbounded(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(p, []byte("package pkg\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultWorldModelConfig()
	cfg.RootPath = dir
	cfg.MaxFilesPerScan = 0
	w := NewWorldModelIngestorShardWithConfig(cfg)
	w.Kernel = kernel

	if err := w.performIncrementalScan(context.Background()); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got := len(w.files); got != 5 {
		t.Fatalf("tracked %d files, want all 5 with an unbounded cap", got)
	}
}
