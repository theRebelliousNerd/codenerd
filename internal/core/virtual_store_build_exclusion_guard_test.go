package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// exclusionGuardKernel answers the forbidden-path query and accepts the
// dreamer's injected fact. It is not a RealKernel, so the dreamer stays
// unavailable and an allowed write surfaces as InteractiveGateError.
type exclusionGuardKernel struct{ types.Kernel }

func (exclusionGuardKernel) Query(string) ([]types.Fact, error) { return nil, nil }
func (exclusionGuardKernel) Assert(types.Fact) error            { return nil }

func TestToolWriteGuard_BuildExclusion(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	dir := t.TempDir()
	v := &VirtualStore{kernel: exclusionGuardKernel{}, workspaceRoot: dir}
	guard := v.toolWriteGuard()
	ctx := context.Background()
	compiled := "package p\n\nfunc A() int { return 1 }\n"
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte(compiled), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("added ignore", func(t *testing.T) {
		err := guard(ctx, "edit_file", map[string]any{
			"path": "a.go", "old_text": "package p\n", "new_text": "//go:build ignore\n\npackage p\n",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the added ignore refused", err)
		}
		var gate *InteractiveGateError
		if errors.As(err, &gate) {
			t.Fatalf("exclusion surfaced as the dreamer gate: %v", err)
		}
		if data, _ := os.ReadFile(path); string(data) != compiled {
			t.Fatalf("the guard wrote: %q", data)
		}
	})

	t.Run("already ignored", func(t *testing.T) {
		ignored := "//go:build ignore\n\n" + compiled
		if err := os.WriteFile(path, []byte(ignored), 0o644); err != nil {
			t.Fatal(err)
		}
		err := guard(ctx, "edit_file", map[string]any{
			"path": "a.go", "old_text": "return 1", "new_text": "return 2",
		})
		var gate *InteractiveGateError
		if !errors.As(err, &gate) {
			t.Fatalf("err = %v, want the dreamer gate after the exclusion check allows the edit", err)
		}
		if strings.Contains(err.Error(), "excludes") {
			t.Fatalf("an already-excluded file was refused: %v", err)
		}
	})

	t.Run("mention", func(t *testing.T) {
		if err := os.WriteFile(path, []byte(compiled), 0o644); err != nil {
			t.Fatal(err)
		}
		err := guard(ctx, "edit_file", map[string]any{
			"path":     "a.go",
			"old_text": "func A() int { return 1 }",
			"new_text": "func A() int {\n\t// Never add //go:build ignore here.\n\treturn 1\n}",
		})
		var gate *InteractiveGateError
		if !errors.As(err, &gate) {
			t.Fatalf("err = %v, want the dreamer gate for a mention", err)
		}
	})

	t.Run("header element", func(t *testing.T) {
		body := "package p\n\nfunc A() int { return 1 }\n\nfunc B() {\n\t// package p\n}\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		err := guard(ctx, "edit_element", map[string]any{
			"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p",
		})
		if err == nil || !strings.Contains(err.Error(), "excludes") {
			t.Fatalf("err = %v, want the header edit refused at the registry guard", err)
		}
		if data, _ := os.ReadFile(path); string(data) != body {
			t.Fatalf("the guard wrote: %q", data)
		}
	})
}
