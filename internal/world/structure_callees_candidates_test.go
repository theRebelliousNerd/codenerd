package world

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Limits cleanup 2026-09-29: resolveCallSites hard-sliced the candidates of
// one call to 6 with an ellipsis row. Every candidate is a valid resolution
// target on an overloaded name, and the ellipsis named nothing the model
// could redeem. callees_of pages the rendered rows with an announced offset,
// so the index returns every candidate whole.
func TestStructureIndexCalleesReturnEveryCandidate(t *testing.T) {
	root := t.TempDir()
	writeStructFile(t, root, "go.mod", "module fixture\n\ngo 1.24\n")
	writeStructFile(t, root, "pkg/caller.go", "package pkg\n\nfunc Caller() {\n\tvar v *T0\n\tv.Run()\n}\n")
	const methods = 8 // past the deleted 6-candidate cut
	for i := 0; i < methods; i++ {
		writeStructFile(t, root, fmt.Sprintf("pkg/t%d.go", i),
			fmt.Sprintf("package pkg\n\ntype T%d struct{}\n\nfunc (t *T%d) Run() {}\n", i, i))
	}

	_, callees, _, err := NewStructureIndex(root).Callees(context.Background(), "pkg.Caller")
	if err != nil {
		t.Fatalf("Callees: %v", err)
	}
	if len(callees) != 1 || callees[0].Call != "v.Run" {
		t.Fatalf("Callees(pkg.Caller) = %+v, want the one v.Run call", callees)
	}
	got := callees[0].Candidates
	if len(got) != methods {
		t.Fatalf("candidates = %d, want all %d Run methods:\n%s", len(got), methods, strings.Join(got, "\n"))
	}
	for _, c := range got {
		if strings.Contains(c, "...") {
			t.Fatalf("candidate %q carries an ellipsis; the cut is back", c)
		}
	}
}
