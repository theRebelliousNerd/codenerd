package codedom

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"codenerd/internal/observation/precondition"
)

// batchPreconditionFixture writes two files and mints a read handle over a's
// lines 1-3, exactly as read_file would have.
func batchPreconditionFixture(t *testing.T) (dir, aAbs, bAbs, handle, aOrig string) {
	t.Helper()
	dir = resolvedTempDir(t)
	seq := preconditionFixtureSeq.Add(1)
	aOrig = fmt.Sprintf("package a\n\n// fixture %d\nconst Limit = 10\n\nconst Other = 1\n", seq)
	aAbs = writeFixture(t, dir, "a.go", aOrig)
	bAbs = writeFixture(t, dir, "b.go", "package b\n\nconst B = 1\n")
	handle = precondition.Shared().Mint(precondition.Read{
		Path: aAbs, Display: "a.go", Content: aOrig, Start: 3, End: 4,
	})
	return dir, aAbs, bAbs, handle, aOrig
}

func batchArgs(handle any) map[string]any {
	a := map[string]any{"operation": "edit_lines", "path": "a.go", "start_line": 4, "end_line": 4, "new_content": "const Limit = 30"}
	if handle != nil {
		a[precondition.Arg] = handle
	}
	return map[string]any{"edits": []any{
		a,
		map[string]any{"operation": "edit_lines", "path": "b.go", "start_line": 3, "end_line": 3, "new_content": "const B = 2"},
	}}
}

// The audit's witness (EDIT-COND-01): the model read Limit = 10, somebody
// changed it to 20, and the batch carrying the original handle must refuse
// before any file changes, as the single-file verb would. Before the fix the
// handle was dropped while parsing and both files were written.
func TestApplyEdits_StalePreconditionRefusesWholeBatch(t *testing.T) {
	dir, aAbs, bAbs, handle, aOrig := batchPreconditionFixture(t)
	changed := strings.Replace(aOrig, "Limit = 10", "Limit = 20", 1)
	if err := os.WriteFile(aAbs, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := executeApplyEdits(ctxForDir(dir), batchArgs(handle))
	if err == nil {
		t.Fatal("stale precondition was not checked: the batch committed")
	}
	if !strings.Contains(err.Error(), "edits[0]") || !strings.Contains(err.Error(), "no file was changed") {
		t.Fatalf("refusal does not name the entry: %v", err)
	}
	if got := readFile(t, aAbs); got != changed {
		t.Fatalf("a.go changed: %q", got)
	}
	if got := readFile(t, bAbs); got != "package b\n\nconst B = 1\n" {
		t.Fatalf("b.go changed: %q", got)
	}

	// Parity: the single-file verb refuses the same handle on the same bytes.
	if _, err := executeEditLines(ctxForDir(dir), map[string]any{
		"path": "a.go", "start_line": 4, "end_line": 4, "new_content": "const Limit = 30", precondition.Arg: handle,
	}); err == nil {
		t.Fatal("direct edit_lines accepted the stale handle; parity fixture is wrong")
	}
}

func TestApplyEdits_PreconditionHandlesThatCannotBeCheckedRefuse(t *testing.T) {
	for name, h := range map[string]any{
		"unknown":    "obs:fr:0000000000000000",
		"non-string": 42,
	} {
		t.Run(name, func(t *testing.T) {
			dir, aAbs, _, _, aOrig := batchPreconditionFixture(t)
			if _, err := executeApplyEdits(ctxForDir(dir), batchArgs(h)); err == nil {
				t.Fatalf("handle %v was silently treated as no precondition", h)
			}
			if readFile(t, aAbs) != aOrig {
				t.Fatal("a.go changed after a refused precondition")
			}
		})
	}
	t.Run("wrong file", func(t *testing.T) {
		dir, _, bAbs, handle, _ := batchPreconditionFixture(t)
		args := batchArgs(nil)
		args["edits"].([]any)[1].(map[string]any)[precondition.Arg] = handle // a.go's handle on b.go
		if _, err := executeApplyEdits(ctxForDir(dir), args); err == nil {
			t.Fatal("a handle for a.go was accepted for b.go")
		}
		if readFile(t, bAbs) != "package b\n\nconst B = 1\n" {
			t.Fatal("b.go changed")
		}
	})
}

// A fresh handle commits and the result tells a checked entry from an
// unchecked one; a handle whose region is intact in a changed file keeps the
// direct verb's caveat instead of losing it with the staged success string.
func TestApplyEdits_PreconditionStatesAreReportedPerEntry(t *testing.T) {
	type result struct {
		Changed       []string            `json:"changed"`
		Preconditions []entryPrecondition `json:"preconditions"`
	}
	run := func(t *testing.T, mutate func(aAbs, aOrig string)) result {
		dir, aAbs, _, handle, aOrig := batchPreconditionFixture(t)
		if mutate != nil {
			mutate(aAbs, aOrig)
		}
		out, err := executeApplyEdits(ctxForDir(dir), batchArgs(handle))
		if err != nil {
			t.Fatalf("apply_edits: %v", err)
		}
		var r result
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("decode %q: %v", out, err)
		}
		if len(r.Preconditions) != 2 || r.Preconditions[0].Path != "a.go" || r.Preconditions[1].Path != "b.go" {
			t.Fatalf("preconditions = %+v", r.Preconditions)
		}
		if r.Preconditions[1].State != preconditionNotSupplied {
			t.Fatalf("b.go supplied no handle but is %q", r.Preconditions[1].State)
		}
		return r
	}
	t.Run("fresh", func(t *testing.T) {
		r := run(t, nil)
		if r.Preconditions[0].State != preconditionSatisfied || r.Preconditions[0].Caveat != "" {
			t.Fatalf("a.go = %+v", r.Preconditions[0])
		}
	})
	t.Run("intact region, changed file", func(t *testing.T) {
		r := run(t, func(aAbs, aOrig string) {
			changed := strings.Replace(aOrig, "Other = 1", "Other = 2", 1)
			if err := os.WriteFile(aAbs, []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
		})
		if r.Preconditions[0].State != preconditionSatisfiedWithCaveat || r.Preconditions[0].Caveat == "" {
			t.Fatalf("a.go caveat lost: %+v", r.Preconditions[0])
		}
	})
}

// EDIT-COND-03: a line verb computes a whole-file candidate from its read. A
// codedom commit landing between that read and the write used to be
// overwritten by the stale candidate (the other writer's change silently
// lost). The verb now publishes through commitFiles, so it refuses instead.
func TestLineVerbs_InterleavedCommitIsRefusedNotOverwritten(t *testing.T) {
	verbs := map[string]func(dir string) error{
		"edit_lines": func(dir string) error {
			_, err := executeEditLines(ctxForDir(dir), map[string]any{"path": "a.go", "start_line": 4, "end_line": 4, "new_content": "const Limit = 30"})
			return err
		},
		"insert_lines": func(dir string) error {
			_, err := executeInsertLines(ctxForDir(dir), map[string]any{"path": "a.go", "after_line": 4, "content": "const Added = 3"})
			return err
		},
		"delete_lines": func(dir string) error {
			_, err := executeDeleteLines(ctxForDir(dir), map[string]any{"path": "a.go", "start_line": 4, "end_line": 4})
			return err
		},
	}
	for name, run := range verbs {
		t.Run(name, func(t *testing.T) {
			dir, aAbs, _, _, aOrig := batchPreconditionFixture(t)
			other := strings.Replace(aOrig, "Other = 1", "Other = 2", 1)
			commitBeforeHook = func() {
				if err := os.WriteFile(aAbs, []byte(other), 0o644); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(func() { commitBeforeHook = nil })
			err := run(dir)
			if err == nil || !strings.Contains(err.Error(), "optimistic conflict") {
				t.Fatalf("interleaved commit not refused: %v", err)
			}
			if got := readFile(t, aAbs); got != other {
				t.Fatalf("the other writer's change was overwritten: %q", got)
			}
		})
	}
}
