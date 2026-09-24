package core

import (
	"errors"
	"testing"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// twoShardCortex is a Cortex whose shards each own one base predicate and
// derive the unowned gamma from it: gamma is found in both shards, and a read
// of one shard sees half of it.
func twoShardCortex(t *testing.T) (*CortexKernel, *KernelShard, *KernelShard) {
	t.Helper()
	const program = `
Decl alpha(Value).
Decl beta(Value).
Decl gamma(Value).
gamma(X) :- alpha(X).
gamma(X) :- beta(X).
`
	c := NewCortexKernel("cortex")
	var shards []*KernelShard
	for _, cfg := range []KernelShardConfig{
		{Domain: "cortex", OwnedPredicates: []string{"alpha"}},
		{Domain: "other", OwnedPredicates: []string{"beta"}},
	} {
		s, err := NewKernelShard(cfg)
		if err != nil {
			t.Fatalf("NewKernelShard(%s): %v", cfg.Domain, err)
		}
		s.kernel.AppendPolicy(program)
		if err := c.RegisterShard(s); err != nil {
			t.Fatalf("RegisterShard(%s): %v", cfg.Domain, err)
		}
		shards = append(shards, s)
	}
	if err := c.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if err := c.AssertString("alpha(/a)"); err != nil {
		t.Fatalf("AssertString(alpha): %v", err)
	}
	if err := c.AssertString("beta(/b)"); err != nil {
		t.Fatalf("AssertString(beta): %v", err)
	}
	return c, shards[0], shards[1]
}

// A fact asserted as a string lands in the shard that owns its predicate, as
// one asserted as a Fact does.
func TestCortexKernel_AssertStringLandsInTheOwnersShard(t *testing.T) {
	_, catchAll, other := twoShardCortex(t)
	if rows, _ := other.kernel.Query("beta"); len(rows) != 1 {
		t.Errorf("the owner's shard holds %d beta rows, want 1", len(rows))
	}
	if rows, _ := catchAll.kernel.Query("beta"); len(rows) != 0 {
		t.Errorf("the catch-all holds %d beta rows, want 0: the string went past the owner", len(rows))
	}
}

// A trace reads where the query reads: gamma derives in both shards, and the
// trace has one root per derived fact, as the query has one row per fact.
// The catch-all alone -- what the chat session traced until 2026-09-23 --
// has one of the two.
func TestCortexKernel_TraceQueryReadsWhereTheQueryReads(t *testing.T) {
	c, catchAll, _ := twoShardCortex(t)
	rows, err := c.Query("gamma")
	if err != nil || len(rows) != 2 {
		t.Fatalf("Query(gamma) = %v, %v; the fixture needs two rows", rows, err)
	}
	trace, err := c.TraceQuery(t.Context(), "gamma")
	if err != nil {
		t.Fatalf("TraceQuery: %v", err)
	}
	if len(trace.RootNodes) != len(rows) {
		t.Errorf("trace roots = %d, want one per query row (%d)", len(trace.RootNodes), len(rows))
	}
	if len(trace.AllNodes) < len(trace.RootNodes) {
		t.Errorf("AllNodes (%d) misses roots (%d)", len(trace.AllNodes), len(trace.RootNodes))
	}
	single, err := catchAll.kernel.TraceQuery(t.Context(), "gamma")
	if err != nil || len(single.RootNodes) != 1 {
		t.Fatalf("the catch-all's own trace = %v, %v; the fixture needs one root there", single, err)
	}
}

// A goal is proved in the shard that derived it; recording is on in every
// shard or it is not on.
func TestCortexKernel_ExplainFindsTheProofInItsShard(t *testing.T) {
	c, _, _ := twoShardCortex(t)
	if c.IsProvenanceEnabled() {
		t.Fatal("provenance on before it was enabled")
	}
	c.EnableProvenance()
	if !c.IsProvenanceEnabled() {
		t.Fatal("EnableProvenance left a shard without recording")
	}
	// A fresh assertion re-evaluates both shards with the recorders on.
	if err := c.AssertString("beta(/c)"); err != nil {
		t.Fatal(err)
	}
	proofs, err := c.Explain("gamma(/c)", ExplainOptions{})
	if err != nil || len(proofs) == 0 {
		t.Fatalf("Explain(gamma(/c)) = %v, %v; want a proof from the shard that owns beta", proofs, err)
	}
	if _, err := c.Explain("gamma(/never)", ExplainOptions{}); !errors.Is(err, provenance.ErrNoProof) {
		t.Errorf("Explain of an underived goal = %v, want ErrNoProof", err)
	}
	c.DisableProvenance()
	if c.IsProvenanceEnabled() {
		t.Error("DisableProvenance left recording on")
	}
}
