package session

import (
	"context"
	"strings"
	"testing"
)

// stubFileContext stands in for world.HolographicProvider through the narrow
// interface the session declares, so this package needs no import of
// internal/world.
type stubFileContext struct {
	section string
	calls   []string
}

func (s *stubFileContext) PromptSection(_ context.Context, filePath string) string {
	s.calls = append(s.calls, filePath)
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
