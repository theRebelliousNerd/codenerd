package core

import "testing"

// element_edit_blocked/2 and edit_unsafe/2 each have two producers: the rules in
// defaults/policy/codedom_edit.mg, and Go (VirtualStore.handleEditElement and
// world.FileScope respectively). The Go side asserted its Reason as a bare
// string while the rules emitted a name constant, so the two halves of each
// relation never unified — element_edit_blocked(_, /concurrent_modification) was
// written by both and matched by neither's consumer. Nothing errored; the rows
// were simply invisible to any bound query.
//
// Both Decls now bind Reason to /name and both Go sites emit MangleAtom. This
// pins that: the derived row and the asserted row must come back from one bound
// query, and the pre-fix string form must NOT — which is what makes the control
// assertions below the actual evidence rather than decoration.
func TestCodedomReasonAtomsUnify(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	facts := []Fact{
		// EDB that drives the .mg rules in policy/codedom_edit.mg.
		{Predicate: "code_element", Args: []any{"fn:demo.Rule", MangleAtom("/function"), "demo.go", int64(1), int64(5)}},
		{Predicate: "file_modified_externally", Args: []any{"demo.go"}},
		{Predicate: "code_element", Args: []any{"fn:gen.Rule", MangleAtom("/function"), "gen.go", int64(1), int64(3)}},
		{Predicate: "generated_code", Args: []any{"gen.go", MangleAtom("/protobuf"), "// Code generated"}},

		// Exactly what the fixed Go producers now emit.
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Go", MangleAtom("/concurrent_modification")}},
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Go", MangleAtom("/hash_verification_failed")}},
		{Predicate: "edit_unsafe", Args: []any{"gen.go", MangleAtom("/generated_code")}},

		// What the Go producers emitted BEFORE the fix, kept as a control.
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Old", "concurrent_modification"}},
		{Predicate: "edit_unsafe", Args: []any{"old.go", "generated_code_will_be_overwritten"}},
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts error = %v", err)
	}

	check := func(query string, want int) {
		t.Helper()
		rows, qErr := k.Query(query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v", query, qErr)
		}
		if len(rows) != want {
			t.Errorf("Query(%s) got %d rows, want %d: %v", query, len(rows), want, rows)
		}
	}

	// The rule-derived row and the Go-asserted row now share one relation.
	check("element_edit_blocked(R, /concurrent_modification)", 2)
	check("element_edit_blocked(R, /hash_verification_failed)", 1)
	check("edit_unsafe(R, /generated_code)", 2)

	// Control: the pre-fix string form is a different value and is invisible to
	// the same bound query. This is the bug that was being closed.
	check(`element_edit_blocked(R, "concurrent_modification")`, 1)
	check(`edit_unsafe(R, "generated_code_will_be_overwritten")`, 1)

	// Unbound consumers (virtual_store.go clearCodeDOMFacts, dom_cmd.go counts)
	// still see every row regardless of constant type.
	// 2 Go-asserted + 2 rule-derived (/concurrent_modification on fn:demo.Rule,
	// /generated_code on fn:gen.Rule) + 1 pre-fix string control.
	check("element_edit_blocked", 5)

	// Kernel still reaches a fixpoint: safe_action must be non-empty.
	sa, err := k.Query("safe_action")
	if err != nil {
		t.Fatalf("Query(safe_action) error = %v", err)
	}
	t.Logf("safe_action rows = %d", len(sa))
	if len(sa) == 0 {
		t.Fatal("safe_action derived 0 rows — analysis is broken")
	}
}
