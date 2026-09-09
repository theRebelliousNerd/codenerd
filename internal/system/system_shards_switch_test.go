package system

import (
	"testing"

	"codenerd/internal/features"
)

// TestSystemShardsMasterSwitch_IsHonoured pins the wiring of a flag that did
// nothing.
//
// features.IsSystemShardsEnabled is documented as "the master switch for
// booting the autopoiesis/observer background shards" and is listed by
// `nerd features`, but it had no non-test caller anywhere in the repo:
// CODENERD_SYSTEM_SHARDS=0 had no effect. A flag an operator can see, read a
// description of, and set with no result is worse than an absent one.
//
// The switch is read in startSystemShards immediately before
// ShardManager.StartSystemShards. This test asserts the resolver itself
// responds to the env var; the call-site guard is a single `if` visible in the
// same function.
func TestSystemShardsMasterSwitch_IsHonoured(t *testing.T) {
	t.Setenv("CODENERD_SYSTEM_SHARDS", "0")
	if features.IsSystemShardsEnabled() {
		t.Fatal("CODENERD_SYSTEM_SHARDS=0 must disable system shards")
	}

	t.Setenv("CODENERD_SYSTEM_SHARDS", "1")
	if !features.IsSystemShardsEnabled() {
		t.Fatal("CODENERD_SYSTEM_SHARDS=1 must enable system shards")
	}

	t.Setenv("CODENERD_SYSTEM_SHARDS", "")
	if !features.IsSystemShardsEnabled() {
		t.Fatal("system shards must default to on")
	}
}

// TestSystemShardsSwitch_HasAProductionCaller is the regression that matters:
// the defect was not a wrong value, it was that nothing read the value. If the
// guard is removed from startSystemShards, this fails.
func TestSystemShardsSwitch_HasAProductionCaller(t *testing.T) {
	callers, err := nonTestCallersOf("IsSystemShardsEnabled")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(callers) == 0 {
		t.Fatal("features.IsSystemShardsEnabled has no non-test caller again; " +
			"the master switch is inert and `nerd features` is lying about it")
	}
}
