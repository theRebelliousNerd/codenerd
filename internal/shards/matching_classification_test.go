package shards
import "testing"



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
		agentName  string
		confidence float64
		expected   bool
	}{
		{"executor with high confidence", "goexpert", 0.9, true},
		{"executor with exact threshold confidence", "goexpert", 0.8, false},
		{"executor with low confidence", "goexpert", 0.7, false},
		{"advisor with high confidence", "securityauditor", 0.9, false},
		{"unknown agent", "unknown", 0.9, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSpecialistExecuteTask(tt.agentName, tt.confidence)
			if result != tt.expected {
				t.Errorf("ShouldSpecialistExecuteTask(%q, %v) = %v, want %v", tt.agentName, tt.confidence, result, tt.expected)
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


func TestCanSpecialistExecute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"executor_specialist", "goexpert", true},
		{"advisor_specialist", "securityauditor", false},
		{"unknown_specialist", "unknown_agent", false},
		{"case_insensitive", "GoExpert", true},
		{"whitespace_trimming", "  goexpert  ", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CanSpecialistExecute(tc.input)
			if result != tc.expected {
				t.Errorf("CanSpecialistExecute(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}
