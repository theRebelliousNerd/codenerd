package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The production prompt path never read a file for a caller's span, so a
// caller was addressed by path alone. The span now comes from the same
// code_defines rows the impact rules join (their symbol is the caller's own
// name, receiver included): exact, no file read, and a (file, symbol) that two
// declarations share has no single span to print.
func TestPromptSection_CallersCarryTheirDeclaredSpan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target.go"), []byte("package p\n\nfunc Target() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fn := core.MangleAtom("/function")
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"context_priority_file": {
			{Predicate: "context_priority_file", Args: []any{"a/close.go", "p.A.Close", int64(3)}},
			{Predicate: "context_priority_file", Args: []any{"a/close.go", "p.init", int64(2)}},
		},
		"code_defines": {
			{Predicate: "code_defines", Args: []any{"a/close.go", "p.A.Close", fn, int64(6), int64(9)}},
			{Predicate: "code_defines", Args: []any{"a/close.go", "p.B.Close", fn, int64(11), int64(14)}},
			// One init per build tag: two spans under one (file, symbol).
			{Predicate: "code_defines", Args: []any{"a/close.go", "p.init", fn, int64(16), int64(18)}},
			{Predicate: "code_defines", Args: []any{"a/close.go", "p.init", fn, int64(20), int64(22)}},
		},
	}}, dir)

	section := h.PromptSection(context.Background(), filepath.Join(dir, "target.go"))
	if !strings.Contains(section, "`p.A.Close` — `a/close.go:6-9`") {
		t.Errorf("the caller must carry its own declaration's span, not the other receiver's:\n%s", section)
	}
	if strings.Contains(section, "a/close.go:11") {
		t.Errorf("p.B.Close is not a caller and must not lend its span:\n%s", section)
	}
	if !strings.Contains(section, "`p.init` — `a/close.go` (2 declarations share this name in the file; no single span)") {
		t.Errorf("a name two declarations share has no single span:\n%s", section)
	}
}
