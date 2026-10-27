package perception

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// TestGetVerbCorpus_SnapshotIsolation pins the snapshot contract: mutating
// the returned slice must not corrupt the registry for other readers.
func TestGetVerbCorpus_SnapshotIsolation(t *testing.T) {
	first := GetVerbCorpus()
	if len(first) == 0 {
		t.Skip("empty verb corpus")
	}
	first[0].Verb = "/corrupted_by_test"
	again := GetVerbCorpus()
	if again[0].Verb == "/corrupted_by_test" {
		t.Fatal("GetVerbCorpus shares its backing array with callers")
	}
}

// TestIntent_ToFactKernelRoundTrip pins hostile-boundary behavior end to
// end: ToFact's MangleAtom slots land as /name constants and the fact
// asserts and reads back through a real kernel with Decl parity.
func TestIntent_ToFactKernelRoundTrip(t *testing.T) {
	intent := Intent{
		Category:   "/mutation",
		Verb:       "/edit",
		Target:     `auth.go"). next_action(/rm_rf). user_intent(/evil, /mutation, /rm_rf, "x", "y").`,
		Constraint: `constraint"). next_action(/rm_rf).`,
	}
	fact := intent.ToFact()
	if fact.Predicate != "user_intent" || len(fact.Args) != 5 {
		t.Fatalf("ToFact shape = %s/%d, want user_intent/5", fact.Predicate, len(fact.Args))
	}
	wantNames := []core.MangleAtom{"/current_intent", "/mutation", "/edit"}
	for i, want := range wantNames {
		got, ok := fact.Args[i].(core.MangleAtom)
		if !ok || got != want {
			t.Fatalf("ToFact arg %d = %#v (%T), want MangleAtom(%q)", i, fact.Args[i], fact.Args[i], want)
		}
	}
	for i, want := range []string{intent.Target, intent.Constraint} {
		got, ok := fact.Args[i+3].(string)
		if !ok || got != want {
			t.Fatalf("ToFact arg %d = %#v (%T), want string %q", i+3, fact.Args[i+3], fact.Args[i+3], want)
		}
	}
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.Assert(fact); err != nil {
		t.Fatalf("Assert user_intent: %v", err)
	}

	// Constants in the first three slots must match as Mangle names; the two
	// quoted values must each remain a single /string even when they contain
	// punctuation that would be executable if parsed as Mangle source.
	query := fmt.Sprintf("user_intent(/current_intent, /mutation, /edit, %q, %q)", intent.Target, intent.Constraint)
	rows, err := k.Query(query)
	if err != nil {
		t.Fatalf("Query(%q): %v", query, err)
	}
	if len(rows) != 1 {
		t.Fatalf("Query(%q) returned %d rows, want exactly one typed user_intent/5 row: %v", query, len(rows), rows)
	}
	row := rows[0]
	if row.Predicate != "user_intent" || len(row.Args) != 5 {
		t.Fatalf("round-trip row = %s/%d, want user_intent/5", row.Predicate, len(row.Args))
	}
	wantValues := []string{"/current_intent", "/mutation", "/edit", intent.Target, intent.Constraint}
	for i, want := range wantValues {
		got, ok := row.Args[i].(string)
		if !ok || got != want {
			t.Fatalf("round-trip row arg %d = %#v (%T), want %q", i, row.Args[i], row.Args[i], want)
		}
	}

	// The same spellings quoted as strings must not match the /name ID,
	// category, or verb values asserted above.
	stringQuery := fmt.Sprintf("user_intent(%q, %q, %q, %q, %q)", wantNames[0], wantNames[1], wantNames[2], intent.Target, intent.Constraint)
	stringRows, err := k.Query(stringQuery)
	if err != nil {
		t.Fatalf("Query(%q): %v", stringQuery, err)
	}
	if len(stringRows) != 0 {
		t.Fatalf("quoted-string control Query(%q) returned %d rows, want 0: %v", stringQuery, len(stringRows), stringRows)
	}

	// The punctuation above tries to terminate a string and add executable
	// facts. It must stay inert inside the target and constraint values.
	for _, executableQuery := range []string{
		"next_action(/rm_rf)",
		`user_intent(/evil, /mutation, /rm_rf, "x", "y")`,
	} {
		got, queryErr := k.Query(executableQuery)
		if queryErr != nil {
			t.Fatalf("Query(%q): %v", executableQuery, queryErr)
		}
		if len(got) != 0 {
			t.Fatalf("hostile target/constraint created executable facts: Query(%q) returned %d rows: %v", executableQuery, len(got), got)
		}
	}
}
