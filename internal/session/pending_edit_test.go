package session

import (
	"errors"
	"testing"

	"codenerd/internal/types"
)

// recordingKernel captures Assert/RetractFact so a test can prove the
// pending_edit lifecycle, not just that a function was called.
type recordingKernel struct {
	types.Kernel // embedded: only the methods used here are implemented

	asserted  []types.Fact
	retracted []types.Fact
	assertErr error
}

func (k *recordingKernel) Assert(f types.Fact) error {
	if k.assertErr != nil {
		return k.assertErr
	}
	k.asserted = append(k.asserted, f)
	return nil
}

func (k *recordingKernel) RetractFact(f types.Fact) error {
	k.retracted = append(k.retracted, f)
	return nil
}

func factsWithPredicate(facts []types.Fact, pred string) []types.Fact {
	var out []types.Fact
	for _, f := range facts {
		if f.Predicate == pred {
			out = append(out, f)
		}
	}
	return out
}

// pending_edit is the root fact for 26 rules across 7 policy files
// (coder_safety, coder_quality, coder_impact, coder_workflow, coder_tdd,
// coder_observability, projectdoc). Every one of them derived nothing because
// no Go code asserted it.
func TestAssertPendingEdit_AssertsForWriteMutationTools(t *testing.T) {
	k := &recordingKernel{}
	e := &Executor{kernel: k}

	fact, ok := e.assertPendingEdit(ToolCall{
		Name: "write_file",
		Args: map[string]any{"path": "internal/foo.go", "content": "package foo"},
	})

	if !ok {
		t.Fatal("write_file did not assert pending_edit")
	}
	if fact.Predicate != "pending_edit" {
		t.Errorf("predicate = %q, want pending_edit", fact.Predicate)
	}
	got := factsWithPredicate(k.asserted, "pending_edit")
	if len(got) != 1 {
		t.Fatalf("asserted %d pending_edit facts, want 1", len(got))
	}
	if got[0].Args[0] != "internal/foo.go" {
		t.Errorf("FilePath = %v, want internal/foo.go", got[0].Args[0])
	}
}

// A read must not claim an edit is in flight.
func TestAssertPendingEdit_IgnoresNonMutatingTools(t *testing.T) {
	k := &recordingKernel{}
	e := &Executor{kernel: k}

	for _, name := range []string{"read_file", "grep", "glob", "list_files", "get_elements"} {
		if _, ok := e.assertPendingEdit(ToolCall{Name: name, Args: map[string]any{"path": "x.go"}}); ok {
			t.Errorf("%s asserted pending_edit; only write-mutation tools may", name)
		}
	}
	if len(k.asserted) != 0 {
		t.Errorf("asserted %d facts for read-only tools, want 0", len(k.asserted))
	}
}

// The load-bearing half. pending_edit means "an edit is in flight RIGHT NOW",
// so a fact left behind makes all 26 dependent rules reason about work that
// already finished -- and the facts accumulate without bound against the
// kernel's fact ceiling.
func TestRetractPendingEdit_RemovesExactlyWhatWasAsserted(t *testing.T) {
	k := &recordingKernel{}
	e := &Executor{kernel: k}

	fact, ok := e.assertPendingEdit(ToolCall{
		Name: "edit_lines",
		Args: map[string]any{"path": "internal/bar.go", "new_content": "x"},
	})
	if !ok {
		t.Fatal("expected an assertion")
	}

	e.retractPendingEdit(fact)

	if len(k.retracted) != 1 {
		t.Fatalf("retracted %d facts, want 1", len(k.retracted))
	}
	if k.retracted[0].Predicate != "pending_edit" {
		t.Errorf("retracted predicate = %q, want pending_edit", k.retracted[0].Predicate)
	}
	if k.retracted[0].Args[0] != fact.Args[0] {
		t.Errorf("retracted FilePath %v does not match asserted %v", k.retracted[0].Args[0], fact.Args[0])
	}
}

// A failed assertion must report false, or the caller defers a retraction for a
// fact that was never asserted.
func TestAssertPendingEdit_FailedAssertReportsNotAsserted(t *testing.T) {
	k := &recordingKernel{assertErr: errors.New("kernel down")}
	e := &Executor{kernel: k}

	if _, ok := e.assertPendingEdit(ToolCall{
		Name: "write_file",
		Args: map[string]any{"path": "x.go", "content": "y"},
	}); ok {
		t.Error("a failed Assert reported success; the caller would retract a fact that does not exist")
	}
}

// No kernel attached must not panic -- the executor runs without one in some
// delegated paths.
func TestPendingEdit_NoKernelIsSafe(t *testing.T) {
	e := &Executor{}

	if _, ok := e.assertPendingEdit(ToolCall{Name: "write_file", Args: map[string]any{"path": "x.go"}}); ok {
		t.Error("asserted with no kernel attached")
	}
	e.retractPendingEdit(types.Fact{Predicate: "pending_edit", Args: []any{"x.go", ""}})
}

// Every write-mutation tool in the registry must participate, or the policy
// layer sees an incomplete picture of what is in flight.
func TestAssertPendingEdit_CoversEveryWriteMutationTool(t *testing.T) {
	writeTools := []string{
		"write_file", "edit_file", "delete_file",
		"edit_lines", "insert_lines", "delete_lines",
		"edit_element", "fs_write",
		"apply_patch", "str_replace", "create_file", "replace_in_file", "multi_edit",
	}

	for _, name := range writeTools {
		k := &recordingKernel{}
		e := &Executor{kernel: k}
		if _, ok := e.assertPendingEdit(ToolCall{
			Name: name,
			Args: map[string]any{"path": "f.go", "content": "c"},
		}); !ok {
			t.Errorf("%s is a write-mutation tool but did not assert pending_edit", name)
		}
	}
}
