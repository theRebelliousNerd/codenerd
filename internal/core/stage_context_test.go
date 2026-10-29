package core

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// stage_context_test.go pins the per-stage context policy
// (internal/core/defaults/policy/stage_context.mg): the stage derived from
// the turn's intent shapes what the kernel admits into injectable_context
// and which tools a shard sees, as Mangle tables rather than Go switches.

// TestStageContext_GuidanceReachesInjectableContext: with an active shard and
// a /fix intent, the /debug stage's guidance string is admitted into
// injectable_context for that shard id, and no other stage's guidance is.
func TestStageContext_GuidanceReachesInjectableContext(t *testing.T) {
	k := setupMockKernel(t)
	mustAssert(t, k, "active_shard", "shard-1", types.MangleAtom("/coder"))
	assertIntent(t, k, "/mutation", "/fix", "auth middleware")

	facts, err := k.Query("injectable_context")
	if err != nil {
		t.Fatalf("Query(injectable_context) failed: %v", err)
	}
	var debug, other int
	for _, f := range facts {
		if len(f.Args) != 2 {
			continue
		}
		atom, _ := f.Args[1].(string)
		if !strings.HasPrefix(atom, "stage /") {
			continue
		}
		if strings.HasPrefix(atom, "stage /debug:") && f.Args[0] == "shard-1" {
			debug++
		} else {
			other++
		}
	}
	if debug != 1 {
		t.Errorf("want exactly one /debug guidance for shard-1, got %d", debug)
	}
	if other != 0 {
		t.Errorf("guidance for a stage the turn is not in leaked: %d", other)
	}
}

// TestStageContext_ToolListFollowsStageAndBudget: the stage view over
// tool_capability admits permitted capabilities minus suppressed ones under a
// sufficient budget and only preferred ones under a constrained budget.
func TestStageContext_ToolListFollowsStageAndBudget(t *testing.T) {
	k := setupMockKernel(t)
	mustAssert(t, k, "active_shard", "shard-1", types.MangleAtom("/coder"))
	assertIntent(t, k, "/query", "/review", "internal/core")
	// /review prefers /inspection and /analysis; suppresses /generation and /execution.
	for name, cap := range map[string]string{
		"read_file":  "/inspection",
		"write_file": "/generation",
		"run_tests":  "/execution",
	} {
		mustAssert(t, k, "tool_registered", name, int64(1))
		mustAssert(t, k, "tool_capability", name, types.MangleAtom(cap))
	}
	mustAssert(t, k, "context_budget", "shard-1", int64(20000))

	if !queryDerived(t, k, `stage_shard_tool_allowed("shard-1", "read_file")`) {
		t.Error("read_file (/inspection) must be allowed in /review")
	}
	if queryDerived(t, k, `stage_shard_tool_allowed("shard-1", "write_file")`) {
		t.Error("write_file (/generation) must be suppressed in /review")
	}
	if queryDerived(t, k, `stage_shard_tool_allowed("shard-1", "run_tests")`) {
		t.Error("run_tests (/execution) must be suppressed in /review")
	}
	if !queryDerived(t, k, `stage_suppressed_tool(/coder, "write_file")`) {
		t.Error("stage_suppressed_tool must name write_file for the /coder shard type")
	}
}

// TestStageContext_NoStageNoShaping: an unmapped verb derives no stage, so
// nothing stage-shaped is admitted and the tool list is untouched.
func TestStageContext_NoStageNoShaping(t *testing.T) {
	k := setupMockKernel(t)
	mustAssert(t, k, "active_shard", "shard-1", types.MangleAtom("/coder"))
	assertIntent(t, k, "/query", "/greet", "none")
	mustAssert(t, k, "tool_registered", "read_file", int64(1))
	mustAssert(t, k, "tool_capability", "read_file", types.MangleAtom("/inspection"))
	mustAssert(t, k, "context_budget", "shard-1", int64(20000))

	if queryDerived(t, k, "stage_required_active") || queryDerived(t, k, "stage_shard_tool_allowed") {
		t.Error("no stage must mean no stage-shaped context or tool list")
	}
}
