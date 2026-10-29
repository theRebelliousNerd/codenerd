package core

import "testing"

// core_limits.max_facts_in_kernel and core_limits.max_derived_facts_limit are
// process-wide: boot installs them once (ConfigureFactLimits) and every kernel
// the process builds afterwards -- and every clone of one -- runs under them.
// Until 2026-09-18 nothing installed them: no production code called
// SetMaxFacts or SetDerivedFactLimit, so the config said 100,000 derived facts
// while the kernel log said 500,000, and both ceilings were constants a real
// repository's world model had outgrown.
//
// Not parallel on purpose: the ceilings are process state.
func TestConfiguredFactLimitsReachEveryKernel(t *testing.T) {
	t.Cleanup(func() { ConfigureFactLimits(0, 0) })
	const wantFacts, wantDerived = 3_000_001, 7_000_001
	ConfigureFactLimits(wantFacts, wantDerived)

	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if got := k.GetMaxFacts(); got != wantFacts {
		t.Errorf("a kernel built after boot has EDB ceiling %d, want the configured %d", got, wantFacts)
	}
	if got := k.GetDerivedFactLimit(); got != wantDerived {
		t.Errorf("a kernel built after boot has derived-fact ceiling %d, want the configured %d", got, wantDerived)
	}

	clone := k.Clone()
	if got := clone.GetMaxFacts(); got != wantFacts {
		t.Errorf("a clone (the dreamer simulates on one) has EDB ceiling %d, want %d", got, wantFacts)
	}
	if got := clone.GetDerivedFactLimit(); got != wantDerived {
		t.Errorf("a clone (the dreamer simulates on one) has derived-fact ceiling %d, want %d", got, wantDerived)
	}

	// A kernel's own ceiling still wins: rule_court sizes its sandbox to the trial.
	k.SetMaxFacts(4321)
	k.SetDerivedFactLimit(8765)
	if got := k.GetMaxFacts(); got != 4321 {
		t.Errorf("the kernel's own EDB ceiling lost to the process-wide one: got %d", got)
	}
	if got := k.GetDerivedFactLimit(); got != 8765 {
		t.Errorf("the kernel's own derived-fact ceiling lost to the process-wide one: got %d", got)
	}
}

// With nothing configured the ceilings are backstops, not working budgets. The
// floors are the sizes this repository was measured at when the old constants
// failed it: 307,455 knowledge-graph rows against a 250,000 EDB ceiling, and a
// dreamer simulation of 500,016 facts against a 500,000 derived ceiling.
func TestUnconfiguredFactLimitsAreBackstops(t *testing.T) {
	ConfigureFactLimits(0, 0)
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if got := k.GetMaxFacts(); got < 4*307_455 {
		t.Errorf("default EDB ceiling %d leaves under 4x headroom over a measured 307,455-row load", got)
	}
	if got := k.GetDerivedFactLimit(); got < 4*500_016 {
		t.Errorf("default derived-fact ceiling %d leaves under 4x headroom over a measured 500,016-fact simulation", got)
	}
}
