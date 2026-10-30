package campaign

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/gates"
)

// Drives declarations, real command execution, diagnostic extraction,
// attribution, visit projection and the shipped selection policy. The unit
// command passes while the independent workspace integration check fails.
func TestRecurseSelection_IndependentWorkspaceCheckReachesPolicy(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/selection\n\ngo 1.21\n")
	write(t, root, "store/store.go", "package store\n")
	write(t, root, "store/unit_test.go", "package store\nimport \"testing\"\nfunc TestUnit(t *testing.T) {}\n")
	write(t, root, "store/integration_test.go", "//go:build integration\n\npackage store\nimport \"testing\"\nfunc TestContract(t *testing.T) { t.Fatal(\"contract failed\") }\n")
	write(t, root, "nerd.md", `---
schema: nerd/v1
commands:
  test: go test -json -count=1 -tags=integration ./...
gates:
  - id: unit
    kind: test
    run: go test -json -count=1 -run '^TestUnit$' {pkg}
    scope: node
---
`)
	set, err := gates.Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range set.Unavailable {
		if u.Gate.ID == "nerd.md:unit" {
			t.Skip(u.Reason)
		}
	}
	node := SubsystemNode{ID: "store", Paths: []string{"store"}, Languages: []string{"go"}}
	p := newRecursePolicy(t)
	r := &recurseRun{root: root, policy: *p, state: map[string]gateRun{}, seenThisPass: map[string]bool{}}
	if err := r.measureWorkspace(context.Background(), set, []SubsystemNode{node}); err != nil {
		t.Fatal(err)
	}
	own := nodeGates(set, node)
	for _, g := range own {
		r.state[gateKey(g.ID, node.ID)] = r.measure(context.Background(), g, node.ID, []SubsystemNode{node})
	}
	unit := r.state[gateKey("nerd.md:unit", node.ID)]
	if !unit.result.Passed || len(unit.findings) != 0 {
		t.Fatalf("unit check did not pass: %+v", unit)
	}
	workspace := r.state[gateKey("nerd.md:test", "")]
	var target string
	for _, f := range workspace.findings {
		if f.Target == "store::TestContract" {
			target = f.ID
		}
	}
	if target == "" {
		t.Fatalf("workspace check has no attributed integration failure: %+v", workspace)
	}
	count := r.openCount()
	open := r.open(node, own, set)
	if err := p.visit(node.ID, open, nil, nil); err != nil {
		t.Fatal(err)
	}
	picked, err := p.next()
	if err != nil {
		t.Fatal(err)
	}
	if picked != target {
		t.Fatalf("picked %q, want independently measured finding %q; open=%+v", picked, target, open)
	}
	if r.openCount() != count {
		t.Fatal("candidate admission changed the retained measurement ledger")
	}
	facts, err := p.k.Query("recurse_finding")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(formatFacts(facts), target) {
		t.Fatal("the selected finding was not projected into policy")
	}
}
