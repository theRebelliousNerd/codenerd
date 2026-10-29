package campaign

import (
	"testing"

	"codenerd/internal/gates"
)

// The error-free pick is policy: a single-file group before a class spread
// over several files, the workspace's own lint findings beside the single-file
// groups, and an unused-code group never (its functions are audited for their
// wiring first, not deleted by a cycle).
func TestRecursePolicy_ErrorFreeGroupsSingleFileFirstUnusedNever(t *testing.T) {
	t.Parallel()
	p := newRecursePolicy(t)
	open := []gates.Finding{
		finding("ef_a_unused", "store", gates.Lint),
		finding("ef_b_class", "store", gates.Lint),
		finding("ef_c_single", "store", gates.Lint),
	}
	groups := map[string]gates.FindingGroup{
		"ef_a_unused": {ID: "ef_a_unused", Gate: "staticcheck", Check: "U1000", Files: 1},
		"ef_b_class":  {ID: "ef_b_class", Gate: "staticcheck", Check: "ST1005", Files: 3},
		"ef_c_single": {ID: "ef_c_single", Gate: "golangci_lint", Check: "errcheck", Files: 1},
	}
	want := []string{"ef_c_single", "ef_b_class", ""}
	for i, w := range want {
		if err := p.visit("store", open, nil, groups); err != nil {
			t.Fatal(err)
		}
		got, err := p.next()
		if err != nil {
			t.Fatal(err)
		}
		if got != w {
			t.Fatalf("pick %d = %q, want %q", i, got, w)
		}
		if got != "" {
			if err := p.attempted(got); err != nil {
				t.Fatal(err)
			}
		}
	}
	// golangci-lint's unused linter is the same class as U1000.
	unused := map[string]gates.FindingGroup{"ef_a_unused": {ID: "ef_a_unused", Gate: "golangci_lint", Check: "unused", Files: 1}}
	if err := p.beginVisit(); err != nil {
		t.Fatal(err)
	}
	if err := p.visit("store", open[:1], nil, unused); err != nil {
		t.Fatal(err)
	}
	if got, err := p.next(); err != nil || got != "" {
		t.Fatalf("an unused-linter group was picked: %q, %v", got, err)
	}
}
