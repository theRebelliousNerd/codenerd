package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// edit_failed(Path, Reason) and delete_blocked(Path, Reason) declare the
// reason /name. Both were asserted as quoted strings, which the kernel stores
// as StringType: a rule matching /pattern_not_found or /no_confirmation would
// never fire, with no error to say so. The reasons are name constants now, and
// they read back from the kernel as the names the Decl promises.
func TestFileActionReasons_ShouldBeNameConstantsTheKernelKeeps(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	path := filepath.Join(dir, "reason.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	edit, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "reason-edit",
		Target:   filepath.Base(path),
		Payload:  map[string]any{"old": "no such text", "new": "x"},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	del, err := vs.handleDeleteFile(context.Background(), ActionRequest{
		ActionID: "reason-delete",
		Target:   filepath.Base(path),
		Payload:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("handleDeleteFile: %v", err)
	}

	want := map[string]types.MangleAtom{
		"edit_failed":    "/pattern_not_found",
		"delete_blocked": "/no_confirmation",
	}
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	for _, res := range []ActionResult{edit, del} {
		if len(res.FactsToAdd) != 1 {
			t.Fatalf("want one reason fact, got %+v", res.FactsToAdd)
		}
		f := res.FactsToAdd[0]
		if got, ok := f.Args[1].(types.MangleAtom); !ok || got != want[f.Predicate] {
			t.Fatalf("%s reason = %#v, want the name %s", f.Predicate, f.Args[1], want[f.Predicate])
		}
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("Assert %s: %v", f.Predicate, err)
		}
		rows, err := kernel.Query(f.Predicate)
		if err != nil || len(rows) != 1 {
			t.Fatalf("Query %s = %v, %v", f.Predicate, rows, err)
		}
		if got := types.ExtractName(rows[0].Args[1]); got != string(want[f.Predicate]) {
			t.Fatalf("%s reason read back as %q, want %s", f.Predicate, got, want[f.Predicate])
		}
	}
}
