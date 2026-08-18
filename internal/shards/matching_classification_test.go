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
		name      string
		nameInput string
		expected  bool
	}{
		{"existing specialist lowercase", "goexpert", true},
		{"existing specialist mixed case", " GoExpert ", true},
		{"advisory specialist", "securityauditor", false},
		{"unknown specialist", "totally-unknown", false},
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
