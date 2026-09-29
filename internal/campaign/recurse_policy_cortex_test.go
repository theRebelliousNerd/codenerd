package campaign_test

import (
	"slices"
	"sort"
	"testing"

	"codenerd/internal/core"
	nerdsystem "codenerd/internal/system"
	"codenerd/internal/types"
)

// The recurse stall/next verdicts on the production kernel: the domain shards
// the factory boots, where recurse.mg's rules fire only if their facts share
// a shard. The same facts go to a single-store RealKernel and every verdict
// is compared, so a split join in the last-two aggregation shows as a
// mismatch. Expected values pin the behavior on both.
func TestRecursePolicy_OnProductionCortex(t *testing.T) {
	ck, err := nerdsystem.NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	single, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	both := recursePair{t: t, cortex: ck, single: single}

	both.assert(
		core.Fact{Predicate: "recurse_visit", Args: []interface{}{"store"}},
		core.Fact{Predicate: "recurse_finding", Args: []interface{}{"s1", "store", "g:test", "/test", "store/x", "sig s1"}},
		core.Fact{Predicate: "recurse_finding", Args: []interface{}{"s2", "store", "g:test", "/test", "store/x", "sig s2"}},
		// s1's last two of three failed the same way: stalled. s2's
		// first and third agree but the middle differs: not stalled.
		attempt("s1", "store", 1, "/reverted", "X"),
		attempt("s1", "store", 2, "/reverted", "Y"),
		attempt("s1", "store", 3, "/reverted", "Y"),
		attempt("s2", "store", 4, "/reverted", "A"),
		attempt("s2", "store", 5, "/reverted", "B"),
		attempt("s2", "store", 6, "/reverted", "A"),
	)
	both.want("finding_stalled", "s1")
	both.want("finding_refused")
	both.want("recurse_next", "s2")

	// A kept change newer than s1's stall lifts it on both kernels.
	both.assert(core.Fact{Predicate: "recurse_node_kept", Args: []interface{}{"store", 7}})
	both.want("finding_stalled")
	both.want("recurse_next", "s1", "s2")

	// A refusal takes s2 out on both kernels.
	both.assert(attempt("s2", "store", 8, "/refused", "forbidden"))
	both.want("finding_refused", "s2")
	both.want("recurse_next", "s1")
}

func attempt(id, node string, cycle int, outcome, sig string) core.Fact {
	return core.Fact{Predicate: "recurse_attempt", Args: []interface{}{id, node, cycle, outcome, sig}}
}

// recursePair feeds two kernels the same facts and compares one column of a
// derived predicate on both, against the expected set.
type recursePair struct {
	t      *testing.T
	cortex core.Kernel
	single core.Kernel
}

func (p recursePair) assert(facts ...core.Fact) {
	p.t.Helper()
	if err := p.cortex.AssertBatch(facts); err != nil {
		p.t.Fatalf("cortex assert: %v", err)
	}
	if err := p.single.AssertBatch(facts); err != nil {
		p.t.Fatalf("single assert: %v", err)
	}
}

func (p recursePair) want(pred string, ids ...string) {
	p.t.Helper()
	gotCortex := recurseColumn(p.t, p.cortex, pred)
	gotSingle := recurseColumn(p.t, p.single, pred)
	if !slices.Equal(gotCortex, gotSingle) {
		p.t.Fatalf("%s: cortex %v, single %v", pred, gotCortex, gotSingle)
	}
	want := append([]string{}, ids...)
	sort.Strings(want)
	if !slices.Equal(gotCortex, want) {
		p.t.Fatalf("%s = %v, want %v", pred, gotCortex, want)
	}
}

func recurseColumn(t *testing.T, k core.Kernel, pred string) []string {
	t.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	seen := map[string]bool{}
	var out []string
	for _, f := range rows {
		if len(f.Args) == 0 {
			continue
		}
		v := types.ExtractString(f.Args[0])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
