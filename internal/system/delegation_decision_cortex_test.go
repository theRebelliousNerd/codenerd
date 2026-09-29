package system

import (
	"testing"

	"codenerd/internal/core"
)

// TestDelegationDecisions_OnProductionCortex is the sharded form of the two
// chat-delegation decisions. NewDomainCortex boots the domain shards from
// shards.DefaultShardPredicateManifests, the same table factory.go installs.
// review_finding_citation and asked_delegation_verb are unowned, so they land
// in the catch-all; severity_rank and configured_execution_mode are program
// facts, present in every shard. The rules join those two and the catch-all
// answers the query. A single RealKernel fed the same facts is the oracle.
func TestDelegationDecisions_OnProductionCortex(t *testing.T) {
	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	both := verdictPair{t: t, cortex: ck, single: single}

	// Count wins: three lows beat one critical.
	// Severity wins a count tie: critical beats two lows.
	// Index wins a full tie: the earlier medium.
	// /unknown is rank 0: it loses a count tie to /low, and three of them
	// still beat one critical.
	both.assert(
		citation("/r_count", "rare/once.go", "/critical", 0),
		citation("/r_count", "common/many.go", "/low", 1),
		citation("/r_count", "common/many.go", "/low", 2),
		citation("/r_count", "common/many.go", "/low", 3),
		citation("/r_sev", "a/first.go", "/low", 0),
		citation("/r_sev", "a/first.go", "/low", 1),
		citation("/r_sev", "b/second.go", "/medium", 2),
		citation("/r_sev", "b/second.go", "/critical", 3),
		citation("/r_idx", "a/earlier.go", "/medium", 0),
		citation("/r_idx", "z/later.go", "/medium", 1),
		citation("/r_idx", "a/earlier.go", "/low", 2),
		citation("/r_idx", "z/later.go", "/low", 3),
		citation("/r_unk", "a.go", "/low", 0),
		citation("/r_unk", "b.go", "/unknown", 1),
		citation("/r_unkcount", "rare/once.go", "/critical", 0),
		citation("/r_unkcount", "common/many.go", "/unknown", 1),
		citation("/r_unkcount", "common/many.go", "/unknown", 2),
		citation("/r_unkcount", "common/many.go", "/unknown", 3),
	)
	both.parity("delegation_target_file", "file_citation_summary")
	both.exact("delegation_target_file", "/r_count", "common/many.go")
	both.exact("delegation_target_file", "/r_sev", "b/second.go")
	both.exact("delegation_target_file", "/r_idx", "a/earlier.go")
	both.exact("delegation_target_file", "/r_unk", "a.go")
	both.exact("delegation_target_file", "/r_unkcount", "common/many.go")

	// The seven table rows are program facts. An unasked verb outside the
	// table has no row: /parallel is derived only when the verb is asked.
	both.parity("execution_mode", "has_configured_execution_mode")
	both.exact("execution_mode", "/review", "/parallel")
	both.exact("execution_mode", "/security", "/parallel")
	both.exact("execution_mode", "/test", "/parallel")
	both.exact("execution_mode", "/create", "/advisory")
	both.exact("execution_mode", "/debug", "/advisory")
	both.exact("execution_mode", "/fix", "/advisory_with_critique")
	both.exact("execution_mode", "/refactor", "/advisory_with_critique")
	both.exact("execution_mode", "/no_such_verb")

	both.assert(core.Fact{
		Predicate: "asked_delegation_verb",
		Args:      []any{atom("/no_such_verb")},
	})
	both.parity("execution_mode")
	both.exact("execution_mode", "/no_such_verb", "/parallel")
	// The default rule must not also attach /parallel to a verb the table names.
	both.exact("execution_mode", "/fix", "/advisory_with_critique")
}

func citation(review, file, severity string, index int64) core.Fact {
	return core.Fact{
		Predicate: "review_finding_citation",
		Args:      []any{atom(review), mstr(file), atom(severity), index},
	}
}
