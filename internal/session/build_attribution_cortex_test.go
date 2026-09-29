package session_test

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestBuildGate_AttributionMatchesOnShardedKernel is the production shape
// of the /build attribution. Each shard evaluates the corpus over its own
// facts, so an import edge homed away from turn_written would charge the
// failure on a single kernel and not on the one that ships.
func TestBuildGate_AttributionMatchesOnShardedKernel(t *testing.T) {
	cortex := bootRetentionCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := retentionPair{t: t, cortex: cortex, single: single}

	const (
		own     = "/build_attr_own"
		foreign = "/build_attr_foreign"
		trans   = "/build_attr_trans"
		unloc   = "/build_attr_unloc"
		pass    = "/build_attr_pass"
		unknown = "/build_attr_unknown"
		sibling = "/build_attr_sibling"
	)
	ma := func(s string) types.MangleAtom { return types.MangleAtom(s) }
	ms := func(s string) types.MangleString { return types.MangleString(s) }
	gf := func(pred string, args ...any) core.Fact {
		return core.Fact{Predicate: pred, Args: args}
	}

	both.assert(
		gf("turn_build_measured", ma(own), ma("/failing")),
		gf("turn_build_diagnostic", ma(own), ms("pkg/foo.go"), ms("example.com/m/pkg")),
		gf("turn_written", ma(own), ms("pkg/foo.go"), ms(".go")),
	)
	both.assert(
		gf("turn_evidence", ma(foreign), ma("/fix"), 1, 1, 1, ma("/false"), ma("/false")),
		gf("turn_verb", ma(foreign), ma("/fix")),
		gf("turn_written", ma(foreign), ms("mine/mine.go"), ms(".go")),
		gf("turn_build_measured", ma(foreign), ma("/failing")),
		gf("turn_build_diagnostic", ma(foreign), ms("other/other.go"), ms("example.com/m/other")),
		gf("turn_written_package", ma(foreign), ms("example.com/m/mine")),
		gf("turn_pkg_imports", ma(foreign), ms("example.com/m/mine"), ms("example.com/m/leaf")),
	)
	both.assert(
		gf("turn_build_measured", ma(trans), ma("/failing")),
		gf("turn_build_diagnostic", ma(trans), ms("c/c.go"), ms("example.com/m/c")),
		gf("turn_written", ma(trans), ms("a/a.go"), ms(".go")),
		gf("turn_written_package", ma(trans), ms("example.com/m/a")),
		gf("turn_pkg_imports", ma(trans), ms("example.com/m/b"), ms("example.com/m/a")),
		gf("turn_pkg_imports", ma(trans), ms("example.com/m/c"), ms("example.com/m/b")),
	)
	both.assert(
		gf("turn_build_measured", ma(unloc), ma("/failing")),
	)
	both.assert(
		gf("turn_build_measured", ma(pass), ma("/passing")),
	)
	both.assert(
		gf("turn_build_measured", ma(unknown), ma("/failing")),
		gf("turn_build_diagnostic", ma(unknown), ms("z/z.go"), ms("example.com/m/z")),
		gf("turn_written", ma(unknown), ms("a/a.go"), ms(".go")),
		gf("turn_build_graph_unknown", ma(unknown)),
	)
	both.assert(
		gf("turn_build_measured", ma(sibling), ma("/failing")),
		gf("turn_build_diagnostic", ma(sibling), ms("pkg/b.go"), ms("example.com/m/pkg")),
		gf("turn_written", ma(sibling), ms("pkg/a.go"), ms(".go")),
		gf("turn_written_package", ma(sibling), ms("example.com/m/pkg")),
	)

	both.parity(
		"turn_gate", "turn_build_failure_attributed", "turn_build_failure_foreign",
		"turn_pkg_depends", "turn_build_depends_on_written", "turn_missing_evidence",
		"turn_executed", "turn_build_failed",
	)
	both.gate(own, "/build", "/failing")
	both.gate(trans, "/build", "/failing")
	both.gate(unloc, "/build", "/failing")
	both.gate(pass, "/build", "/passing")
	both.gate(unknown, "/build", "/failing")
	both.gate(sibling, "/build", "/failing")
	both.noGate(foreign, "/build")
	both.has("turn_build_failure_foreign", foreign, "example.com/m/other")
	both.has("turn_missing_evidence", foreign, "/build_failure_foreign")
	both.lacks("turn_missing_evidence", foreign, "/build_not_green")
	both.has("turn_build_depends_on_written", trans, "example.com/m/c")
}
