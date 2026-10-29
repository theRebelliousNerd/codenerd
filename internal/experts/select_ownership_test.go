package experts

import (
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// pausingKernel pauses one Select between its assertion and its first read:
// after the wrapped AssertBatch it signals asserted and waits for release.
type pausingKernel struct {
	*core.RealKernel
	asserted chan struct{}
	release  chan struct{}
}

func (k *pausingKernel) AssertBatch(facts []types.Fact) error {
	err := k.RealKernel.AssertBatch(facts)
	close(k.asserted)
	<-k.release
	return err
}

// Each selection owns every fact it asserts. Two selections for the same
// file overlap on one kernel: A asserts and pauses, B asserts, reads and
// retracts, then A reads. A must still see its expert. Before, both asserted
// the same untagged file_extension(path, ext) tuple; the kernel kept one copy,
// B's cleanup removed it, and A returned no expert without an error (an
// external source audit predicted this; this test reproduced it).
func TestOverlappingSelectionsOwnTheirFacts(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{AgentAtom("MangleExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{AgentAtom("MangleExpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
	}); err != nil {
		t.Fatal(err)
	}
	// .gl is /mangle by policy and no scan supports it: the extension is the
	// only evidence, so losing it loses the expert.
	task := Task{Verb: "/review", Targets: []string{"internal/policy/rules.gl"}}

	a := &pausingKernel{RealKernel: k, asserted: make(chan struct{}), release: make(chan struct{})}
	type outcome struct {
		sel Selection
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		sel, err := Select(a, task)
		done <- outcome{sel, err}
	}()
	<-a.asserted

	b, err := Select(k, task)
	if err != nil || !slices.Equal(b.Agents(), []string{"mangleexpert"}) {
		t.Fatalf("selection B = %v, %v; want [mangleexpert]", b.Agents(), err)
	}
	close(a.release)
	got := <-done
	if got.err != nil {
		t.Fatalf("selection A: %v", got.err)
	}
	if !slices.Equal(got.sel.Agents(), []string{"mangleexpert"}) {
		t.Errorf("selection A = %v after B's cleanup; want [mangleexpert]: B removed a fact A asserted", got.sel.Agents())
	}
	for _, predicate := range []string{"expert_task", "expert_task_target", "expert_task_extension", "file_extension"} {
		if rows, _ := k.Query(predicate); len(rows) != 0 {
			t.Errorf("%s left behind after both selections: %v", predicate, rows)
		}
	}
}
