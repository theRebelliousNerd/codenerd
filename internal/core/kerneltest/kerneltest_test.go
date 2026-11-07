package kerneltest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"codenerd/internal/core"
)

func factKeys(k *core.RealKernel) []string {
	var keys []string
	for f := range k.GetAllFactsSeq() {
		keys = append(keys, fmt.Sprint(f.Predicate, f.Args))
	}
	slices.Sort(keys)
	return keys
}

// The template serves the same kernel a boot builds: the same facts, and a
// kernel that answers queries the way the booted one does.
func TestNew_IsEquivalentToABootedKernel(t *testing.T) {
	t.Parallel()
	booted, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	served := New(t)
	if got, want := factKeys(served), factKeys(booted); !slices.Equal(got, want) {
		t.Fatalf("served kernel has %d facts, a booted one %d", len(got), len(want))
	}
	if served.GetWorkspace() != "" {
		t.Fatalf("workspace = %q, want none", served.GetWorkspace())
	}
	for _, pred := range []string{"intent_verb_persona", "verb_persona", "permitted"} {
		a, aerr := served.Query(pred)
		b, berr := booted.Query(pred)
		if (aerr == nil) != (berr == nil) || len(a) != len(b) {
			t.Fatalf("%s: served %d rows (%v), booted %d rows (%v)", pred, len(a), aerr, len(b), berr)
		}
	}
}

// Every caller owns its kernel: a write through one never shows in another or
// in the template, including when callers write concurrently.
func TestNew_ClonesAreIndependent(t *testing.T) {
	t.Parallel()
	const writers = 8
	kernels := make([]*core.RealKernel, writers)
	for i := range kernels {
		kernels[i] = New(t)
	}
	var wg sync.WaitGroup
	for i, k := range kernels {
		wg.Go(func() {
			if err := k.Assert(core.Fact{Predicate: "user_intent", Args: []any{fmt.Sprintf("/kt_%d", i), "/query", "/explain", "target", ""}}); err != nil {
				t.Errorf("assert %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	for i, k := range kernels {
		rows, err := k.Query("user_intent")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || fmt.Sprint(rows[0].Args[0]) != fmt.Sprintf("/kt_%d", i) {
			t.Fatalf("kernel %d sees %d user_intent row(s) %v, want only its own", i, len(rows), rows)
		}
	}
	fresh := New(t)
	if rows, _ := fresh.Query("user_intent"); len(rows) != 0 {
		t.Fatalf("a later kernel inherited %d user_intent row(s) from earlier writers", len(rows))
	}
}

// A workspace without user extensions is served from the template with its
// root bound; one that carries .nerd/mangle files gets the real boot, so its
// extension loads exactly as it does in production.
func TestWithWorkspace_BindsTheRootAndBootsWhenTheWorkspaceExtendsTheKernel(t *testing.T) {
	t.Parallel()
	plain := t.TempDir()
	k := WithWorkspace(t, plain)
	if abs, _ := filepath.Abs(plain); k.GetWorkspace() != abs {
		t.Fatalf("workspace = %q, want %q", k.GetWorkspace(), abs)
	}
	if rows, _ := k.Query("kerneltest_probe"); len(rows) != 0 {
		t.Fatal("a plain workspace's kernel answers an undeclared predicate")
	}

	extended := t.TempDir()
	dir := filepath.Join(extended, ".nerd", "mangle")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extensions.mg"), []byte("Decl kerneltest_probe(X).\nkerneltest_probe(7).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasUserExtensions(extended) || hasUserExtensions(plain) {
		t.Fatal("hasUserExtensions disagrees with the files on disk")
	}
	ek := WithWorkspace(t, extended)
	rows, err := ek.Query("kerneltest_probe")
	if err != nil || len(rows) != 1 {
		t.Fatalf("extension was not loaded: %v rows, err %v", rows, err)
	}
	if rows, _ := New(t).Query("kerneltest_probe"); len(rows) != 0 {
		t.Fatal("an extension leaked into the template")
	}
}

// BenchmarkBoot and BenchmarkServed measure what the package saves: a real
// boot per kernel against a clone of the evaluated template.
func BenchmarkBoot(b *testing.B) {
	for b.Loop() {
		if _, err := core.NewRealKernelWithWorkspace(b.TempDir()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkServed(b *testing.B) {
	_ = New(b) // build the template outside the timed loop
	b.ResetTimer()
	for b.Loop() {
		_ = WithWorkspace(b, b.TempDir())
	}
}
