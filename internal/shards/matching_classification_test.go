package shards

import (
	"reflect"
	"testing"
)

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
