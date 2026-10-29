package core

import (
	"reflect"
	"testing"
)

func TestClone_FirstReadMaterializesCleanParentSnapshot(test *testing.T) {
	parent, err := NewRealKernelWithWorkspace(test.TempDir())
	if err != nil {
		test.Fatal(err)
	}
	parent.AppendPolicy(`
Decl clone_snapshot_input(Value) bound [/string].
Decl clone_snapshot_output(Value) bound [/string].
clone_snapshot_output(Value) :- clone_snapshot_input(Value).
`)
	if err := parent.Assert(Fact{Predicate: "clone_snapshot_input", Args: []any{"retained evidence"}}); err != nil {
		test.Fatal(err)
	}
	expected, err := parent.QueryAll()
	if err != nil {
		test.Fatal(err)
	}
	if parent.factsDirty.Load() || len(expected["clone_snapshot_output"]) != 1 {
		test.Fatal("parent was not evaluated to a clean positive snapshot")
	}
	clone := parent.Clone()
	actual, err := clone.QueryAll()
	if err != nil {
		test.Fatal(err)
	}
	for _, predicate := range []string{"clone_snapshot_input", "clone_snapshot_output"} {
		if !reflect.DeepEqual(expected[predicate], actual[predicate]) {
			test.Fatalf("first clone read lost %s: got %+v, want %+v", predicate, actual[predicate], expected[predicate])
		}
	}
	if err := clone.Assert(Fact{Predicate: "clone_snapshot_input", Args: []any{"private evidence"}}); err != nil {
		test.Fatal(err)
	}
	private, err := clone.Query("clone_snapshot_output")
	if err != nil || len(private) != 2 {
		test.Fatalf("clone did not derive private evidence: %+v, %v", private, err)
	}
	retained, err := parent.Query("clone_snapshot_output")
	if err != nil || !reflect.DeepEqual(retained, expected["clone_snapshot_output"]) {
		test.Fatalf("private clone changed parent: %+v, %v", retained, err)
	}
}
