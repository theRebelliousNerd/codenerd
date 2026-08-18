package shards

import (
	"reflect"
	"testing"
)

func TestIsExecutorSpecialist(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"executor specialist (lowercase)", "mangleexpert", true},
		{"executor specialist (mixed case)", " MangleExpert ", true},
		{"observer specialist", "northstar", false},
		{"advisor specialist", "securityauditor", false},
		{"unknown specialist", "unknown", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExecutorSpecialist(tt.input)
			if result != tt.expected {
				t.Errorf("IsExecutorSpecialist(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsStrategicAdvisor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"strategic advisor (lowercase)", "securityauditor", true},
		{"strategic advisor (mixed case)", " SecurityAuditor ", true},
		{"technical expert", "goexpert", false},
		{"unknown specialist", "unknown", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStrategicAdvisor(tt.input)
			if result != tt.expected {
				t.Errorf("IsStrategicAdvisor(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldSpecialistExecuteTask(t *testing.T) {
	tests := []struct {
		name       string
		specialist string
		confidence float64
		want       bool
	}{
		{
			name:       "Executor specialist with high confidence",
			specialist: "goexpert",
			confidence: 0.9,
			want:       true,
		},
		{
			name:       "Executor specialist with exact threshold confidence",
			specialist: "goexpert",
			confidence: 0.8,
			want:       false,
		},
		{
			name:       "Executor specialist with low confidence",
			specialist: "goexpert",
			confidence: 0.7,
			want:       false,
		},
		{
			name:       "Non-executor specialist with high confidence",
			specialist: "securityauditor",
			confidence: 0.9,
			want:       false,
		},
		{
			name:       "Unknown specialist",
			specialist: "unknown",
			confidence: 0.9,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSpecialistExecuteTask(tt.specialist, tt.confidence)
			if got != tt.want {
				t.Errorf("ShouldSpecialistExecuteTask(%q, %v) = %v, want %v", tt.specialist, tt.confidence, got, tt.want)
			}
		})
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
		{"existing specialist different", "cobraexpert", DefaultSpecialistClassifications["cobraexpert"], true},
		{"unknown specialist", "unknown", SpecialistClassification{}, false},
		{"empty string", "", SpecialistClassification{}, false},
		{"only spaces", "   ", SpecialistClassification{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetSpecialistClassification(tt.nameInput)
			if ok != tt.expectedOk {
				t.Errorf("GetSpecialistClassification(%q) ok = %v, want %v", tt.nameInput, ok, tt.expectedOk)
			}
			if ok && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetSpecialistClassification(%q) got = %+v, want %+v", tt.nameInput, got, tt.expected)
			}
		})
	}
}

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
