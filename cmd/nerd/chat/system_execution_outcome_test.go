package chat

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/perception"
)

// A system execution the router measured as failed was turned into a
// successful shard result: the adapter handed ResultToFacts a nil error, and
// its nil branch emits shard_success. The measured outcome now travels with
// the result.
func TestSystemExecution_AFailedRowStaysAFailure(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	m.shardMgr = coreshards.NewShardManager()
	intent := perception.Intent{Verb: "/review", Target: "billing.go"}
	failed := []systemExecutionResult{{ActionID: "a1", ActionType: "delegate_reviewer", Target: "billing.go", Success: false, Timestamp: 1}}

	msg := m.buildResponseFromExecutions(context.Background(), "review billing.go", intent, nil, failed, systemResultBaseline{})
	reply, ok := msg.(assistantMsg)
	if !ok || reply.ShardResult == nil {
		t.Fatalf("reply = %#v, want an assistant message carrying the shard result", msg)
	}
	joined := factPredicates(reply.ShardResult)
	if strings.Contains(joined, "shard_success") || !strings.Contains(joined, "shard_error") {
		t.Fatalf("a failed execution row became facts:\n%s", joined)
	}

	ok1 := []systemExecutionResult{{ActionID: "a2", ActionType: "delegate_reviewer", Target: "billing.go", Success: true, Output: "no findings", Timestamp: 2}}
	reply = m.buildResponseFromExecutions(context.Background(), "review billing.go", intent, nil, ok1, systemResultBaseline{}).(assistantMsg)
	if !strings.Contains(factPredicates(reply.ShardResult), "shard_success") {
		t.Fatal("control: a successful execution row lost its success")
	}
}

func factPredicates(p *ShardResultPayload) string {
	var names []string
	for _, f := range p.Facts {
		names = append(names, f.Predicate)
	}
	return strings.Join(names, "\n")
}
