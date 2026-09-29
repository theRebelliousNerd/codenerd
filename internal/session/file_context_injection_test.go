package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
	working "codenerd/internal/context"
)

// stubFileContext stands in for world.HolographicProvider through the narrow
// interface the session declares, so this package needs no import of
// internal/world.
type stubFileContext struct {
	section string
	// calls records every render by file, whichever method rendered it:
	// the ledger tests read this to learn which file's view a request
	// carried. budgeted records only the policy-sized renders, with the
	// budget the last one was given.
	calls    []string
	budgeted []string
	budget   int
}

func (s *stubFileContext) PromptSection(_ context.Context, filePath string) string {
	s.calls = append(s.calls, filePath)
	return s.section
}

func (s *stubFileContext) PromptSectionWithCallerBudget(_ context.Context, filePath string, budgetBytes int, _ func(ctx context.Context, target string, totalCallers, avgBytesPerCaller, budgetBytes int) (int, error)) string {
	s.calls = append(s.calls, filePath)
	s.budgeted = append(s.budgeted, filePath)
	s.budget = budgetBytes
	return s.section
}

// TestWithFileContext_AppendsAndDegrades pins the last link of the holographic
// pipeline: the provider produces a section, and this is the only thing that
// puts it in front of the model.
//
// It had no test of any kind. Every part of the pipeline underneath it was
// covered, which is exactly how a chain stays broken while its pieces stay
// green.
func TestWithFileContext_AppendsAndDegrades(t *testing.T) {
	const base = "SYSTEM PROMPT"

	t.Run("appends the section for a file target", func(t *testing.T) {
		p := &stubFileContext{section: "## Holographic Context (a.go)\n\nbody"}
		e := &Executor{}
		e.SetFileContextProvider(p)

		got := e.withFileContext(context.Background(), base, "a.go")
		if !strings.HasPrefix(got, base) {
			t.Fatalf("the compiled prompt must survive: %q", got)
		}
		if !strings.Contains(got, "Holographic Context") {
			t.Fatalf("section not appended: %q", got)
		}
		if len(p.calls) != 1 || p.calls[0] != "a.go" {
			t.Fatalf("provider called with %v, want [a.go]", p.calls)
		}
	})

	t.Run("no provider is a no-op", func(t *testing.T) {
		e := &Executor{}
		if got := e.withFileContext(context.Background(), base, "a.go"); got != base {
			t.Fatalf("got %q, want the prompt unchanged", got)
		}
	})

	t.Run("empty target does not call the provider", func(t *testing.T) {
		p := &stubFileContext{section: "should not appear"}
		e := &Executor{}
		e.SetFileContextProvider(p)

		for _, target := range []string{"", "   ", "\t\n"} {
			if got := e.withFileContext(context.Background(), base, target); got != base {
				t.Errorf("target %q produced %q", target, got)
			}
		}
		if len(p.calls) != 0 {
			t.Errorf("provider was called for an empty target: %v", p.calls)
		}
	})

	t.Run("empty section is not appended", func(t *testing.T) {
		// The provider returns "" when it has nothing substantive to say — a
		// file that does not exist, a non-Go target. Appending the separator
		// anyway would spend tokens on a blank section.
		p := &stubFileContext{section: "   \n  "}
		e := &Executor{}
		e.SetFileContextProvider(p)

		if got := e.withFileContext(context.Background(), base, "missing.go"); got != base {
			t.Fatalf("blank section was appended: %q", got)
		}
	})

	t.Run("nil provider clears a previously set one", func(t *testing.T) {
		p := &stubFileContext{section: "section"}
		e := &Executor{}
		e.SetFileContextProvider(p)
		e.SetFileContextProvider(nil)

		if got := e.withFileContext(context.Background(), base, "a.go"); got != base {
			t.Fatalf("provider was not cleared: %q", got)
		}
	})

	t.Run("a working loop renders with the live budget", func(t *testing.T) {
		p := &stubFileContext{section: "## Holographic Context (a.go)\n\nbody"}
		e := &Executor{}
		e.SetConfig(ExecutorConfig{TokenBudget: 8000})
		e.SetFileContextProvider(p)
		set, err := working.NewWorkingSet(t.TempDir(), "budget", config.DefaultWorkingConfig())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = set.Close() })
		ctx := context.WithValue(context.Background(), workingLoopKey{}, &workingLoop{set: set, focus: "a.go"})

		got := e.withFileContext(ctx, base, "a.go")
		if !strings.Contains(got, "Holographic Context") {
			t.Fatalf("section not appended: %q", got)
		}
		if len(p.calls) != 1 || p.calls[0] != "a.go" {
			t.Fatalf("provider called with %v, want [a.go]", p.calls)
		}
		if len(p.budgeted) != 1 || p.budgeted[0] != "a.go" {
			t.Fatalf("budgeted renders = %v, want [a.go]", p.budgeted)
		}
		if want := 8000 * config.BytesPerToken; p.budget != want {
			t.Fatalf("budget = %d bytes, want the live window %d", p.budget, want)
		}
	})

	t.Run("a loop without a set renders whole", func(t *testing.T) {
		p := &stubFileContext{section: "## Holographic Context (a.go)\n\nbody"}
		e := &Executor{}
		e.SetFileContextProvider(p)
		ctx := context.WithValue(context.Background(), workingLoopKey{}, &workingLoop{})

		if got := e.withFileContext(ctx, base, "a.go"); !strings.Contains(got, "Holographic Context") {
			t.Fatalf("section not appended: %q", got)
		}
		if len(p.calls) != 1 || p.calls[0] != "a.go" {
			t.Fatalf("provider called with %v, want [a.go]", p.calls)
		}
		if len(p.budgeted) != 0 {
			t.Fatalf("budgeted renders = %v, want none without an engine", p.budgeted)
		}
	})
}

// TestWithProjectInstructions_NilDocIsANoOp covers the sibling injection on the
// same seam. Write protection does not depend on this call — it is enforced
// from kernel facts — so a nil doc must lose the prose and nothing else.
func TestWithProjectInstructions_NilDocIsANoOp(t *testing.T) {
	e := &Executor{}
	const base = "SYSTEM PROMPT"
	if got := e.withProjectInstructions(base); got != base {
		t.Fatalf("got %q, want the prompt unchanged", got)
	}
}
