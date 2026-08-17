package shards

import "testing"

func TestCanSpecialistExecute(t *testing.T) {
	tests := []struct {
		name       string
		nameInput  string
		expected   bool
	}{
		{"existing specialist lowercase", "goexpert", true},
		{"existing specialist mixed case", " GoExpert ", true},
		{"advisory specialist", "securityauditor", false},
		{"unknown specialist", "unknown", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanSpecialistExecute(tt.nameInput)
			if got != tt.expected {
				t.Errorf("CanSpecialistExecute(%q) = %v, want %v", tt.nameInput, got, tt.expected)
			}
		})
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

func TestGetSpecialistClassification(t *testing.T) {
	tests := []struct {
		name       string
		nameInput  string
		expected   SpecialistClassification
		expectedOk bool
	}{
		{"existing specialist lowercase", "goexpert", DefaultSpecialistClassifications["goexpert"], true},
		{"existing specialist mixed case", " GoExpert ", DefaultSpecialistClassifications["goexpert"], true},
		{"unknown specialist", "unknown", SpecialistClassification{}, false},
		{"empty string", "", SpecialistClassification{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetSpecialistClassification(tt.nameInput)
			if ok != tt.expectedOk {
				t.Errorf("GetSpecialistClassification(%q) ok = %v, want %v", tt.nameInput, ok, tt.expectedOk)
			}
			if ok && got.CanExecute != tt.expected.CanExecute {
				t.Errorf("GetSpecialistClassification(%q) got CanExecute = %v, want CanExecute %v", tt.nameInput, got.CanExecute, tt.expected.CanExecute)
			}
		})
	}
}
