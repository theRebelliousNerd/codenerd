package session

import (
	"reflect"
	"testing"

	working "codenerd/internal/context"
)

// A call id that answers several results is one ledger row -- its bytes
// summed, at its latest arrival and round -- because the request evicts by
// id. Before the shed scan required unique, round-ordered rows a reused id
// went to the policy twice; now it would be refused, and a scripted client
// that reuses "run" every round would lose its request.
func TestMergeRepeatedCalls_OneRowPerCallInRoundOrder(t *testing.T) {
	in := []working.LedgerEntry{
		{Call: "run", ID: "o1", Bytes: 100, Round: 1},
		{Call: "read", ID: "o2", Bytes: 50, Round: 1},
		{Call: "run", ID: "o3", Bytes: 30, Round: 2},
		{Call: "edit", ID: "o4", Bytes: 7, Round: 3},
	}
	want := []working.LedgerEntry{
		{Call: "read", ID: "o2", Bytes: 50, Round: 1},
		{Call: "run", ID: "o3", Bytes: 130, Round: 2},
		{Call: "edit", ID: "o4", Bytes: 7, Round: 3},
	}
	if got := mergeRepeatedCalls(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %+v, want %+v", got, want)
	}
	unique := want
	if got := mergeRepeatedCalls(unique); !reflect.DeepEqual(got, unique) {
		t.Fatalf("unique calls changed: %+v", got)
	}
}
