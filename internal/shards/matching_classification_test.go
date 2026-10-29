package shards

import (
	"reflect"
	"testing"
)

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
