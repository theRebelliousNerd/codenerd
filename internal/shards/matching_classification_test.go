package shards

import "testing"

func TestCanSpecialistExecute(t *testing.T) {
	// goexpert is an executor and can execute; case/space-insensitive lookup.
	if !CanSpecialistExecute("goexpert") {
		t.Error("goexpert should be able to execute")
	}
	if !CanSpecialistExecute("  GoExpert  ") {
		t.Error("specialist lookup should normalize case and whitespace")
	}
	// securityauditor is advisory and cannot execute.
	if CanSpecialistExecute("securityauditor") {
		t.Error("securityauditor is advisory and should not execute")
	}
	// Unknown specialists default to advisory (no execution).
	if CanSpecialistExecute("totally-unknown") {
		t.Error("unknown specialists should default to non-executing")
	}
}

func TestIsExecutorSpecialist(t *testing.T) {
	if !IsExecutorSpecialist("mangleexpert") {
		t.Error("mangleexpert should be an executor specialist")
	}
	if IsExecutorSpecialist("northstar") {
		t.Error("northstar is an observer, not an executor")
	}
	if IsExecutorSpecialist("unknown") {
		t.Error("unknown specialist should not be an executor")
	}
}

func TestIsStrategicAdvisor(t *testing.T) {
	if !IsStrategicAdvisor("securityauditor") {
		t.Error("securityauditor is a strategic advisor")
	}
	if IsStrategicAdvisor("goexpert") {
		t.Error("goexpert is technical-tier, not strategic")
	}
	if IsStrategicAdvisor("unknown") {
		t.Error("unknown specialist should not be a strategic advisor")
	}
}

func TestGetAllPatterns(t *testing.T) {
	patterns := GetAllPatterns()
	if len(patterns) == 0 {
		t.Error("GetAllPatterns should return the core technology patterns")
	}
}

func TestShouldIncludeGenericShard_UnknownVerbDefaultsTrue(t *testing.T) {
	if !ShouldIncludeGenericShard("a-verb-with-no-config") {
		t.Error("an unconfigured verb should default to including the generic shard")
	}
}
