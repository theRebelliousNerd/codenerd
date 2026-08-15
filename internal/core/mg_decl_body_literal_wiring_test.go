package core

import (
	"testing"

	"codenerd/internal/types"
)

// These tests are the runtime half of mg_decl_literal_conformance_test.go.
//
// That guard is static: it proves a .mg literal agrees with its own Decl. It
// cannot prove the Go producer agrees with either, and that is exactly where
// this bug class lives — a rule body matching /session_planner against a fact
// Go asserted as the bare "session_planner" analyses cleanly, derives nothing,
// and reports no error. A negated one is worse than silent: it inverts, and
// the guard it was written to be stays permanently open.
//
// So each case below asserts the EDB in the shape the live Go producer emits
// and checks the DERIVATION, with the paired control that flips it. A case
// that only checked "derives" could pass against a rule that derives for
// everything; a case that only checked "does not derive" could pass against a
// rule that is simply dead.

func mustAssertFact(t *testing.T, k *RealKernel, pred string, args ...any) {
	t.Helper()
	if err := k.Assert(Fact{Predicate: pred, Args: args}); err != nil {
		t.Fatalf("assert %s%v: %v", pred, args, err)
	}
}

// -----------------------------------------------------------------------------
// system_shard_healthy/1 — the heartbeat guard on on-demand shard activation
// -----------------------------------------------------------------------------

// TestSystemShardHealthy_HeartbeatClosesTheActivationGuard covers
// policy/system_shards.mg:
//
//	activate_shard(/session_planner) :-
//	    current_campaign(_), !system_shard_healthy(/session_planner).
//
// system_shard_healthy is fed only by system_heartbeat/2, which
// BaseSystemShard.EmitHeartbeat asserts. While that arg was a bare Go string
// the negation could never be satisfied, so the guard was decorative and
// activate_shard fired on every campaign whether the planner was alive or not.
func TestSystemShardHealthy_HeartbeatClosesTheActivationGuard(t *testing.T) {
	t.Run("no heartbeat activates", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "current_campaign", "camp-1")

		if !queryDerived(t, k, "activate_shard(/session_planner)") {
			t.Fatal("activate_shard(/session_planner) must derive when no heartbeat exists — " +
				"the control for this test is broken, not the rule")
		}
	})

	t.Run("heartbeat suppresses activation", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "current_campaign", "camp-1")
		// Exactly what EmitHeartbeat sends.
		mustAssertFact(t, k, "system_heartbeat", types.MangleAtom("/session_planner"), int64(1700000000))

		if !queryDerived(t, k, "system_shard_healthy(/session_planner)") {
			t.Error("system_shard_healthy(/session_planner) not derived from a /name heartbeat")
		}
		if queryDerived(t, k, "activate_shard(/session_planner)") {
			t.Error("activate_shard(/session_planner) still derived for a shard with a live heartbeat — " +
				"the !system_shard_healthy guard is not binding")
		}
	})

	t.Run("bare string heartbeat is the regression", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "current_campaign", "camp-1")
		// The pre-fix producer shape. Kept as an explicit regression witness:
		// if someone reverts EmitHeartbeat to b.ID, this documents what breaks.
		mustAssertFact(t, k, "system_heartbeat", "session_planner", int64(1700000000))

		if queryDerived(t, k, "system_shard_healthy(/session_planner)") {
			t.Error("a bare-string heartbeat should NOT satisfy the /name literal; " +
				"if this now passes, the two constant kinds have started unifying")
		}
	})
}

// -----------------------------------------------------------------------------
// shard_executed/4 arg 3 is the Task, not an outcome
// -----------------------------------------------------------------------------

// TestHasSuccessfulShard_ReadsShardSuccessNotTheTaskSlot pins the rewrite of
// has_successful_shard. The old body was
// shard_executed(ShardID, _, /success, _), but ShardManager.ResultToFacts puts
// the task DESCRIPTION in arg 3 and reports the outcome as shard_success/1.
func TestHasSuccessfulShard_ReadsShardSuccessNotTheTaskSlot(t *testing.T) {
	// ResultToFacts's exact output for one successful "coder" run.
	seedExecuted := func(t *testing.T, k *RealKernel) {
		t.Helper()
		mustAssertFact(t, k, "compile_shard", "coder-1", types.MangleAtom("/coder"))
		mustAssertFact(t, k, "shard_executed", "coder-1", types.MangleAtom("/coder"),
			"fix the auth bug", int64(1700000000))
	}

	t.Run("executed alone does not signal success", func(t *testing.T) {
		k := setupMockKernel(t)
		seedExecuted(t, k)
		if queryDerived(t, k, "has_successful_shard") {
			t.Error("has_successful_shard derived from shard_executed alone — " +
				"execution is not an outcome")
		}
	})

	t.Run("shard_success signals", func(t *testing.T) {
		k := setupMockKernel(t)
		seedExecuted(t, k)
		mustAssertFact(t, k, "shard_success", "coder-1")

		if !queryDerived(t, k, "has_successful_shard") {
			t.Error("has_successful_shard not derived despite compile_shard + shard_success")
		}
		// And it reaches the learning signal it exists to feed.
		mustAssertFact(t, k, "is_mandatory", "atom-1")
		mustAssertFact(t, k, "prompt_atom", "atom-1", types.MangleAtom("/identity"),
			int64(90), int64(40), types.MangleAtom("/true"))
		if !queryDerived(t, k, "effective_prompt_atom") {
			t.Error("effective_prompt_atom not derived for a selected atom on a successful shard")
		}
	})

	t.Run("context injection effectiveness", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "final_injectable", "coder-1", "atom-1")

		if queryDerived(t, k, "context_injection_effective") {
			t.Fatal("context_injection_effective derived with no outcome fact at all")
		}
		mustAssertFact(t, k, "shard_success", "coder-1")
		if !queryDerived(t, k, "context_injection_effective") {
			t.Error("context_injection_effective not derived for an injected atom on a successful shard")
		}
	})
}

// -----------------------------------------------------------------------------
// intelligence_shard_advice/5 — advisory ballot
// -----------------------------------------------------------------------------

// TestIntelligenceAdvisoryBoard_VotesAreNameConstants covers the consensus
// rules in policy/intelligence.mg, which match /coder, /tester and /approve as
// name constants. Decomposer.seedIntelligenceFacts previously passed both the
// specialist and the ballot through as bare strings.
func TestIntelligenceAdvisoryBoard_VotesAreNameConstants(t *testing.T) {
	advise := func(t *testing.T, k *RealKernel, shard, vote string, conf int64) {
		t.Helper()
		mustAssertFact(t, k, "intelligence_shard_advice", "camp-1",
			types.MangleAtom(shard), types.MangleAtom(vote), conf, "looks fine to me")
	}

	t.Run("approves", func(t *testing.T) {
		k := setupMockKernel(t)
		advise(t, k, "/coder", "/approve", 80)
		advise(t, k, "/tester", "/approve", 75)
		if !queryDerived(t, k, "intelligence_advisory_approved") {
			t.Error("intelligence_advisory_approved not derived from /coder+/tester approvals")
		}
	})

	t.Run("a tester reject blocks", func(t *testing.T) {
		k := setupMockKernel(t)
		advise(t, k, "/coder", "/approve", 80)
		advise(t, k, "/tester", "/reject", 75)
		if queryDerived(t, k, "intelligence_advisory_approved") {
			t.Error("approval survived a /reject from the tester")
		}
		if !queryDerived(t, k, "intelligence_advisory_concerns") {
			t.Error("intelligence_advisory_concerns not derived for a /reject ballot")
		}
	})

	t.Run("bare string ballots are the regression", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "intelligence_shard_advice", "camp-1", "coder", "approve", int64(80), "ok")
		mustAssertFact(t, k, "intelligence_shard_advice", "camp-1", "tester", "approve", int64(75), "ok")
		if queryDerived(t, k, "intelligence_advisory_approved") {
			t.Error("bare-string ballots should not satisfy the /coder and /approve literals")
		}
	})
}

// -----------------------------------------------------------------------------
// knowledge_link/3 — graph edge labels
// -----------------------------------------------------------------------------

// TestKnowledgeLink_RelationIsANameConstant covers the spreading-activation
// rules in policy/knowledge.mg. LocalStore.HydrateKnowledgeGraph now prefixes
// stored labels so a relation that came from a free text memory operation
// still reaches the kernel as /related_to.
func TestKnowledgeLink_RelationIsANameConstant(t *testing.T) {
	t.Run("related_to spreads activation", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "activation", "internal/core/kernel.go", int64(90))
		mustAssertFact(t, k, "knowledge_link", "internal/core/kernel.go",
			types.MangleAtom("/related_to"), "internal/core/kernel_eval.go")

		if !queryDerived(t, k, "activation(\"internal/core/kernel_eval.go\", 60)") {
			t.Error("activation did not spread across a /related_to link")
		}
	})

	t.Run("bare relation is the regression", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssertFact(t, k, "activation", "internal/core/kernel.go", int64(90))
		mustAssertFact(t, k, "knowledge_link", "internal/core/kernel.go",
			types.MangleString("related_to"), "internal/core/kernel_eval.go")

		if queryDerived(t, k, "activation(\"internal/core/kernel_eval.go\", 60)") {
			t.Error("a bare \"related_to\" string should not match the /related_to literal")
		}
	})
}

// -----------------------------------------------------------------------------
// doc_tag/2 — document selection
// -----------------------------------------------------------------------------

// TestDocTag_TagIsANameConstant covers selection_policy.mg. Decomposer
// .seedDocFacts asserts fmt.Sprintf("/%s", tag), which types.Fact.ToAtom
// promotes to a name constant, so the Decl had to follow.
func TestDocTag_TagIsANameConstant(t *testing.T) {
	k := setupMockKernel(t)
	mustAssertFact(t, k, "doc_tag", "Docs/architecture/spike.md", types.MangleAtom("/experimental"))

	if !queryDerived(t, k, "is_irrelevant(\"Docs/architecture/spike.md\")") {
		t.Error("is_irrelevant not derived for a doc tagged /experimental")
	}
	if queryDerived(t, k, "is_irrelevant(\"Docs/architecture/roadmap.md\")") {
		t.Error("is_irrelevant derived for an untagged doc")
	}
}

// -----------------------------------------------------------------------------
// learned_preference/2 — cold storage keys
// -----------------------------------------------------------------------------

// TestLearnedPreference_KeyIsAString is the one predicate in this sweep where
// the Decl was right and the .mg literal was wrong. VirtualStore
// .HydrateLearnings asserts toAtomOrString(storedFact.Predicate) over
// cold_storage rows, and those keys never carry a leading "/", so they arrive
// as string constants.
func TestLearnedPreference_KeyIsAString(t *testing.T) {
	seedTool := func(t *testing.T, k *RealKernel) {
		t.Helper()
		mustAssertFact(t, k, "tool_capabilities", types.MangleAtom("/write_file"), types.MangleAtom("/code_generation"))
		mustAssertFact(t, k, "tool_language", types.MangleAtom("/write_file"), types.MangleAtom("/go"))
	}
	const boosted = "activation(/write_file, 85)"

	t.Run("string key boosts the tool", func(t *testing.T) {
		k := setupMockKernel(t)
		seedTool(t, k)
		if queryDerived(t, k, boosted) {
			t.Fatal("activation boost derived before any preference was stored — " +
				"this test cannot distinguish anything")
		}
		// HydrateLearnings' shape: toAtomOrString leaves a slash-less
		// cold_storage key alone, so it lands as a string constant.
		mustAssertFact(t, k, "learned_preference", "prefer_language", "go")
		if !queryDerived(t, k, boosted) {
			t.Error("no activation boost after storing the prefer_language preference")
		}
	})

	t.Run("name key is the regression", func(t *testing.T) {
		k := setupMockKernel(t)
		seedTool(t, k)
		mustAssertFact(t, k, "learned_preference", types.MangleAtom("/prefer_language"), "go")
		if queryDerived(t, k, boosted) {
			t.Error("a /prefer_language name constant matched the \"prefer_language\" string literal")
		}
	})
}
