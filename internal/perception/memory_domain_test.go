package perception

import (
	"testing"

	"codenerd/internal/articulation"
)

// A person's "forget X" addresses their saved long-term memory, and the host
// says so: the domain is set by the adapter, never read from a model.
func TestExtractMemoryOperations_ForgetNamesTheSavedMemoryDomain(t *testing.T) {
	tr := &UnderstandingTransducer{}
	ops := tr.extractMemoryOperations(&Understanding{ActionType: "forget", Scope: Scope{Target: "prefer-tabs"}})
	if len(ops) != 1 || ops[0].Op != "forget" || ops[0].Domain != articulation.MemoryDomainUserMemory {
		t.Fatalf("forget ops = %+v, want one forget in the user-memory domain", ops)
	}
}
